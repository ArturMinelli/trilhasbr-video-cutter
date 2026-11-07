package actions

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/domain"
	"trilhasbr-video-cutter/internal/media/ffmpeg"
	"trilhasbr-video-cutter/internal/platform"
	"trilhasbr-video-cutter/internal/repository"
)

type StartCutInput struct {
	WorkspaceID   int64  `json:"workspaceId"`
	Name          string `json:"name"`
	DateTimeLocal string `json:"datetimeLocal"`
	SecondsBefore int    `json:"secondsBefore"`
	SecondsAfter  int    `json:"secondsAfter"`
}

type CutRunPlan struct {
	Workspace   domain.Workspace     `json:"workspace"`
	Input       StartCutInput        `json:"input"`
	Window      domain.CutWindow     `json:"window"`
	Plans       []domain.SegmentPlan `json:"plans"`
	OutputDir   string               `json:"outputDir"`
	SegmentsDir string               `json:"segmentsDir"`
	MergedDir   string               `json:"mergedDir"`
}

type CutExecuteAction struct {
	WorkspaceRepo repository.WorkspaceRepository
	VideoRepo     repository.VideoRepository
	Media         ffmpeg.Service
	Logger        *slog.Logger
	Parallelism   int
}

func (a *CutExecuteAction) Execute(ctx context.Context, jobID string, input StartCutInput, onProgress func(CutProgressEvent)) (domain.CutResult, error) {
	plan, err := a.BuildPlan(ctx, input)
	if err != nil {
		return domain.CutResult{}, err
	}
	return a.ExecutePlanned(ctx, jobID, plan, onProgress)
}

func (a *CutExecuteAction) BuildPlan(ctx context.Context, input StartCutInput) (CutRunPlan, error) {
	if err := validateStartCutInput(input, true); err != nil {
		return CutRunPlan{}, err
	}

	workspace, err := a.WorkspaceRepo.GetByID(ctx, input.WorkspaceID)
	if err != nil {
		return CutRunPlan{}, err
	}

	return a.BuildPlanWithWorkspace(ctx, workspace, input)
}

func (a *CutExecuteAction) BuildPlanWithWorkspace(ctx context.Context, workspace domain.Workspace, input StartCutInput) (CutRunPlan, error) {
	if err := validateStartCutInput(input, false); err != nil {
		return CutRunPlan{}, err
	}
	if workspace.ID <= 0 {
		return CutRunPlan{}, apperrors.New(apperrors.CodeValidation, "workspaceId must be greater than 0", nil)
	}
	if strings.TrimSpace(workspace.Directory) == "" {
		return CutRunPlan{}, apperrors.New(apperrors.CodeValidation, "workspace directory is required", nil)
	}

	window, err := buildCutWindow(input.DateTimeLocal, input.SecondsBefore, input.SecondsAfter)
	if err != nil {
		return CutRunPlan{}, err
	}

	videos, err := a.VideoRepo.FindOverlapping(ctx, workspace.ID, window.WindowStartUTC, window.WindowEndUTC)
	if err != nil {
		return CutRunPlan{}, err
	}
	if len(videos) == 0 {
		return CutRunPlan{}, apperrors.New(apperrors.CodeValidation, "no videos overlap the requested time window", map[string]any{
			"workspaceId": workspace.ID,
			"windowStart": window.WindowStartUTC.Format(time.RFC3339),
			"windowEnd":   window.WindowEndUTC.Format(time.RFC3339),
		})
	}

	plans := BuildSegmentPlans(videos, window.WindowStartUTC, window.WindowEndUTC)
	if len(plans) == 0 {
		return CutRunPlan{}, apperrors.New(apperrors.CodeValidation, "no valid segment plans found for selected time window", nil)
	}

	outputDir, err := resolveRunOutputDirectory(workspace.Directory, input.Name)
	if err != nil {
		return CutRunPlan{}, err
	}

	return CutRunPlan{
		Workspace:   workspace,
		Input:       input,
		Window:      window,
		Plans:       plans,
		OutputDir:   outputDir,
		SegmentsDir: filepath.Join(outputDir, "segments"),
		MergedDir:   filepath.Join(outputDir, "merged"),
	}, nil
}

func (a *CutExecuteAction) ExecutePlanned(ctx context.Context, jobID string, plan CutRunPlan, onProgress func(CutProgressEvent)) (domain.CutResult, error) {
	if strings.TrimSpace(plan.OutputDir) == "" {
		return domain.CutResult{}, apperrors.New(apperrors.CodeValidation, "output directory is required", nil)
	}
	if len(plan.Plans) == 0 {
		return domain.CutResult{}, apperrors.New(apperrors.CodeValidation, "no valid segment plans found for selected time window", nil)
	}

	if err := platform.EnsureDir(plan.SegmentsDir); err != nil {
		return domain.CutResult{}, err
	}
	if err := platform.EnsureDir(plan.MergedDir); err != nil {
		return domain.CutResult{}, err
	}

	parallelism := a.Parallelism
	if parallelism <= 0 {
		parallelism = max(1, min(4, runtime.NumCPU()-1))
	}

	type segmentOutcome struct {
		index  int
		output domain.OutputFile
		err    error
	}

	semaphore := make(chan struct{}, parallelism)
	results := make(chan segmentOutcome, len(plan.Plans))

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	var completed int64
	for idx, segmentPlan := range plan.Plans {
		idx := idx
		segmentPlan := segmentPlan
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-ctx.Done():
				results <- segmentOutcome{index: idx, err: ctx.Err()}
				return
			case semaphore <- struct{}{}:
			}
			defer func() { <-semaphore }()

			segmentOutput := filepath.Join(
				plan.SegmentsDir,
				fmt.Sprintf("%04d_%s.mp4", idx+1, sanitizeFileName(filepath.Base(segmentPlan.VideoPath))),
			)

			cutOutput, cutErr := a.Media.CutSegment(ctx, ffmpeg.CutSegmentInput{
				InputPath:   segmentPlan.VideoPath,
				OutputPath:  segmentOutput,
				StartSec:    segmentPlan.SegmentStartSec,
				DurationSec: segmentPlan.SegmentDurationSec,
				Mode:        ffmpeg.CutModeFastCopyThenPrecise,
			})
			if cutErr != nil {
				results <- segmentOutcome{index: idx, err: cutErr}
				cancel()
				return
			}

			progress := int(atomic.AddInt64(&completed, 1))
			if onProgress != nil {
				onProgress(CutProgressEvent{
					JobID:             jobID,
					TotalSegments:     len(plan.Plans),
					CompletedSegments: progress,
					CurrentFile:       segmentPlan.VideoPath,
				})
			}

			results <- segmentOutcome{
				index: idx,
				output: domain.OutputFile{
					Path:            cutOutput.OutputPath,
					DurationSeconds: cutOutput.ActualDurationSec,
				},
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	perSourceOutputs := make([]domain.OutputFile, len(plan.Plans))
	for outcome := range results {
		if outcome.err != nil {
			if ctx.Err() != nil && outcome.err == context.Canceled {
				continue
			}
			return domain.CutResult{}, apperrors.Wrap(apperrors.CodeCut, "failed to cut one or more video segments", outcome.err, map[string]any{"jobId": jobID})
		}
		perSourceOutputs[outcome.index] = outcome.output
	}

	segmentPaths := make([]string, 0, len(perSourceOutputs))
	for _, output := range perSourceOutputs {
		if output.Path != "" {
			segmentPaths = append(segmentPaths, output.Path)
		}
	}
	if len(segmentPaths) == 0 {
		return domain.CutResult{}, apperrors.New(apperrors.CodeCut, "no segments were generated", map[string]any{"jobId": jobID})
	}

	mergedPath := filepath.Join(plan.MergedDir, "final.mp4")
	merged, mergeErr := a.Media.MergeConcat(ctx, ffmpeg.MergeInput{
		SegmentPaths: segmentPaths,
		OutputPath:   mergedPath,
	})
	if mergeErr != nil {
		return domain.CutResult{}, apperrors.Wrap(apperrors.CodeMerge, "failed to merge segment files", mergeErr, map[string]any{"jobId": jobID, "outputDir": plan.OutputDir})
	}

	warnings := append([]string{}, merged.Warnings...)
	return domain.CutResult{
		JobID:            jobID,
		OutputDirectory:  plan.OutputDir,
		PerSourceOutputs: perSourceOutputs,
		MergedOutput: &domain.OutputFile{
			Path:            merged.OutputPath,
			DurationSeconds: merged.DurationSec,
		},
		Warnings: warnings,
	}, nil
}

func BuildSegmentPlans(videos []domain.Video, windowStartUTC time.Time, windowEndUTC time.Time) []domain.SegmentPlan {
	if len(videos) == 0 || !windowEndUTC.After(windowStartUTC) {
		return nil
	}

	type sortableVideo struct {
		video domain.Video
		start time.Time
	}
	sorted := make([]sortableVideo, 0, len(videos))
	for _, video := range videos {
		if video.StartedAt == nil || video.DurationSeconds <= 0 {
			continue
		}
		sorted = append(sorted, sortableVideo{video: video, start: video.StartedAt.UTC()})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].start.Equal(sorted[j].start) {
			return sorted[i].video.Path < sorted[j].video.Path
		}
		return sorted[i].start.Before(sorted[j].start)
	})

	plans := make([]domain.SegmentPlan, 0, len(sorted))
	for _, candidate := range sorted {
		videoStart := candidate.start
		videoEnd := videoStart.Add(time.Duration(candidate.video.DurationSeconds * float64(time.Second)))
		if candidate.video.FinishedAt != nil && candidate.video.FinishedAt.After(videoStart) {
			videoEnd = candidate.video.FinishedAt.UTC()
		}
		if !videoEnd.After(windowStartUTC) || !windowEndUTC.After(videoStart) {
			continue
		}

		segmentStartSec := math.Max(0, windowStartUTC.Sub(videoStart).Seconds())
		segmentEndSec := math.Min(candidate.video.DurationSeconds, windowEndUTC.Sub(videoStart).Seconds())
		segmentDuration := segmentEndSec - segmentStartSec
		if segmentDuration <= 0 {
			continue
		}

		plans = append(plans, domain.SegmentPlan{
			VideoID:            candidate.video.ID,
			VideoPath:          candidate.video.Path,
			VideoStartUTC:      videoStart,
			VideoDurationSec:   candidate.video.DurationSeconds,
			SegmentStartSec:    segmentStartSec,
			SegmentDurationSec: segmentDuration,
		})
	}

	return plans
}

func validateStartCutInput(input StartCutInput, requireWorkspace bool) error {
	if requireWorkspace && input.WorkspaceID <= 0 {
		return apperrors.New(apperrors.CodeValidation, "workspaceId must be greater than 0", nil)
	}
	if strings.TrimSpace(input.Name) == "" {
		return apperrors.New(apperrors.CodeValidation, "name is required", nil)
	}
	if input.SecondsBefore < 0 || input.SecondsAfter < 0 {
		return apperrors.New(apperrors.CodeValidation, "secondsBefore and secondsAfter must be >= 0", map[string]any{
			"secondsBefore": input.SecondsBefore,
			"secondsAfter":  input.SecondsAfter,
		})
	}
	if input.SecondsBefore == 0 && input.SecondsAfter == 0 {
		return apperrors.New(apperrors.CodeValidation, "cut window cannot have zero total duration", nil)
	}
	return nil
}

func buildCutWindow(rawDateTime string, secondsBefore int, secondsAfter int) (domain.CutWindow, error) {
	anchorLocal, err := parseDateTimeLocal(rawDateTime)
	if err != nil {
		return domain.CutWindow{}, err
	}

	anchorUTC := anchorLocal.UTC()
	windowStart := anchorUTC.Add(-time.Duration(secondsBefore) * time.Second)
	windowEnd := anchorUTC.Add(time.Duration(secondsAfter) * time.Second)

	if !windowEnd.After(windowStart) {
		return domain.CutWindow{}, apperrors.New(apperrors.CodeValidation, "invalid time window", map[string]any{"secondsBefore": secondsBefore, "secondsAfter": secondsAfter})
	}

	return domain.CutWindow{
		AnchorLocal:    anchorLocal,
		AnchorUTC:      anchorUTC,
		WindowStartUTC: windowStart,
		WindowEndUTC:   windowEnd,
		SecondsBefore:  secondsBefore,
		SecondsAfter:   secondsAfter,
	}, nil
}

func parseDateTimeLocal(rawDateTime string) (time.Time, error) {
	rawDateTime = strings.TrimSpace(rawDateTime)
	if rawDateTime == "" {
		return time.Time{}, apperrors.New(apperrors.CodeValidation, "datetimeLocal is required", nil)
	}

	layoutsWithLocation := []string{
		"2006-01-02T15:04",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04",
		"2006-01-02 15:04:05",
	}
	layoutsWithZone := []string{time.RFC3339Nano, time.RFC3339}

	for _, layout := range layoutsWithZone {
		if value, err := time.Parse(layout, rawDateTime); err == nil {
			return value.In(time.Local), nil
		}
	}
	for _, layout := range layoutsWithLocation {
		if value, err := time.ParseInLocation(layout, rawDateTime, time.Local); err == nil {
			return value, nil
		}
	}

	return time.Time{}, apperrors.New(apperrors.CodeValidation, "datetimeLocal has an invalid format", map[string]any{"datetimeLocal": rawDateTime})
}

func resolveRunOutputDirectory(workspaceDirectory string, requestedName string) (string, error) {
	if strings.TrimSpace(workspaceDirectory) == "" {
		return "", apperrors.New(apperrors.CodeValidation, "workspace directory is required", nil)
	}

	exportsRoot := filepath.Join(workspaceDirectory, "exports")
	folderName := sanitizeFolderName(requestedName)
	if folderName == "" {
		return "", apperrors.New(apperrors.CodeValidation, "name resulted in an invalid folder name", nil)
	}

	outputDir := filepath.Join(exportsRoot, folderName)
	if stat, err := os.Stat(outputDir); err == nil && stat.IsDir() {
		entries, readErr := os.ReadDir(outputDir)
		if readErr == nil && len(entries) > 0 {
			return "", apperrors.New(apperrors.CodeValidation, "output folder already exists and is not empty", map[string]any{"outputDir": outputDir})
		}
	}

	return outputDir, nil
}

func sanitizeFolderName(name string) string {
	clean := strings.TrimSpace(name)
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	clean = replacer.Replace(clean)
	clean = strings.Trim(clean, ".")
	return clean
}

func sanitizeFileName(name string) string {
	clean := strings.TrimSpace(name)
	clean = strings.TrimSuffix(clean, filepath.Ext(clean))
	if clean == "" {
		return "segment"
	}
	replacer := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(clean)
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
