package actions

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/domain"
	"trilhasbr-video-cutter/internal/platform"
	"trilhasbr-video-cutter/internal/repository"
)

type StartPeopleCutInput struct {
	WorkspaceID   int64   `json:"workspaceId"`
	GroupIDs      []int64 `json:"groupIds"`
	Name          string  `json:"name"`
	SecondsBefore int     `json:"secondsBefore"`
	SecondsAfter  int     `json:"secondsAfter"`
}

// PersonCutRunner is satisfied by *CutExecuteAction.
type PersonCutRunner interface {
	ExecutePlanned(ctx context.Context, jobID string, plan CutRunPlan, onProgress func(CutProgressEvent)) (domain.CutResult, error)
}

// PersonCutPlan is the segment plan for one person group.
type PersonCutPlan struct {
	GroupID  int64
	Segments []domain.SegmentPlan
}

type PersonCutAction struct {
	WorkspaceRepo repository.WorkspaceRepository
	VideoRepo     repository.VideoRepository
	Detection     repository.DetectionRepository
	Runner        PersonCutRunner
}

// BuildPersonCutPlan maps each group's tracks to padded windows clamped to [0, duration], merges overlapping
// windows within a video, and orders segments by video started_at (nil last, then name/path).
// Groups whose tracks point at unknown videos are skipped; groups without any segment are omitted.
func BuildPersonCutPlan(groups []domain.PersonGroup, videos map[int64]domain.Video, before, after float64) []PersonCutPlan {
	plans := make([]PersonCutPlan, 0, len(groups))
	for _, group := range groups {
		windows := map[int64][][2]float64{}
		for _, track := range group.Tracks {
			video, ok := videos[track.VideoID]
			if !ok || video.DurationSeconds <= 0 {
				continue
			}
			start := math.Max(0, track.StartSec-before)
			end := math.Min(video.DurationSeconds, track.EndSec+after)
			if end > start {
				windows[track.VideoID] = append(windows[track.VideoID], [2]float64{start, end})
			}
		}

		var segments []domain.SegmentPlan
		for videoID, list := range windows {
			video := videos[videoID]
			sort.Slice(list, func(i, j int) bool { return list[i][0] < list[j][0] })
			merged := [][2]float64{list[0]}
			for _, w := range list[1:] {
				last := &merged[len(merged)-1]
				if w[0] <= last[1] {
					last[1] = math.Max(last[1], w[1])
				} else {
					merged = append(merged, w)
				}
			}
			for _, w := range merged {
				segment := domain.SegmentPlan{
					VideoID:            video.ID,
					VideoPath:          video.Path,
					VideoDurationSec:   video.DurationSeconds,
					SegmentStartSec:    w[0],
					SegmentDurationSec: w[1] - w[0],
				}
				if video.StartedAt != nil {
					segment.VideoStartUTC = video.StartedAt.UTC()
				}
				segments = append(segments, segment)
			}
		}
		if len(segments) == 0 {
			continue
		}

		sort.Slice(segments, func(i, j int) bool {
			a, b := segments[i], segments[j]
			aNil, bNil := a.VideoStartUTC.IsZero(), b.VideoStartUTC.IsZero()
			switch {
			case aNil != bNil:
				return bNil // videos with a start time first
			case !aNil && !a.VideoStartUTC.Equal(b.VideoStartUTC):
				return a.VideoStartUTC.Before(b.VideoStartUTC)
			case a.VideoPath != b.VideoPath:
				return a.VideoPath < b.VideoPath
			}
			return a.SegmentStartSec < b.SegmentStartSec
		})
		plans = append(plans, PersonCutPlan{GroupID: group.ID, Segments: segments})
	}
	return plans
}

// Execute writes one clip per selected group to <workspace>/exports/<name>/person-<groupID>.mp4.
// Each group reuses CutExecuteAction.ExecutePlanned (bounded parallel cuts + concat) in a scratch dir.
func (a *PersonCutAction) Execute(ctx context.Context, jobID string, input StartPeopleCutInput, onProgress func(CutProgressEvent)) (domain.CutResult, error) {
	if input.WorkspaceID <= 0 {
		return domain.CutResult{}, apperrors.New(apperrors.CodeValidation, "workspaceId must be greater than 0", nil)
	}
	if len(input.GroupIDs) == 0 {
		return domain.CutResult{}, apperrors.New(apperrors.CodeValidation, "groupIds is required", nil)
	}
	if strings.TrimSpace(input.Name) == "" {
		return domain.CutResult{}, apperrors.New(apperrors.CodeValidation, "name is required", nil)
	}
	if input.SecondsBefore < 0 || input.SecondsAfter < 0 {
		return domain.CutResult{}, apperrors.New(apperrors.CodeValidation, "secondsBefore and secondsAfter must be >= 0", nil)
	}

	workspace, err := a.WorkspaceRepo.GetByID(ctx, input.WorkspaceID)
	if err != nil {
		return domain.CutResult{}, err
	}
	outputDir, err := resolveRunOutputDirectory(workspace.Directory, input.Name)
	if err != nil {
		return domain.CutResult{}, err
	}

	wanted := map[int64]bool{}
	for _, id := range input.GroupIDs {
		wanted[id] = true
	}
	groups, _, err := a.Detection.ListGroups(ctx, domain.GroupFilter{WorkspaceID: input.WorkspaceID, GroupIDs: input.GroupIDs, PageSize: len(wanted)})
	if err != nil {
		return domain.CutResult{}, err
	}
	if len(groups) != len(wanted) {
		return domain.CutResult{}, apperrors.New(apperrors.CodeValidation, "one or more selected people no longer exist", map[string]any{"groupIds": input.GroupIDs})
	}

	videos := map[int64]domain.Video{}
	for _, group := range groups {
		for _, track := range group.Tracks {
			if _, ok := videos[track.VideoID]; ok {
				continue
			}
			video, getErr := a.VideoRepo.GetByID(ctx, track.VideoID)
			if getErr != nil {
				return domain.CutResult{}, getErr
			}
			videos[track.VideoID] = video
		}
	}

	plans := BuildPersonCutPlan(groups, videos, float64(input.SecondsBefore), float64(input.SecondsAfter))
	if len(plans) == 0 {
		return domain.CutResult{}, apperrors.New(apperrors.CodeValidation, "selected people have no cuttable segments", nil)
	}
	if err := platform.EnsureDir(outputDir); err != nil {
		return domain.CutResult{}, err
	}

	total, done := 0, 0
	for _, plan := range plans {
		total += len(plan.Segments)
	}

	result := domain.CutResult{JobID: jobID, OutputDirectory: outputDir, Warnings: []string{}}
	for _, plan := range plans {
		scratch := filepath.Join(outputDir, fmt.Sprintf(".work-person-%d", plan.GroupID))
		offset := done
		run, runErr := a.Runner.ExecutePlanned(ctx, jobID, CutRunPlan{
			Input:       StartCutInput{WorkspaceID: input.WorkspaceID, Name: input.Name},
			Plans:       plan.Segments,
			OutputDir:   scratch,
			SegmentsDir: filepath.Join(scratch, "segments"),
			MergedDir:   filepath.Join(scratch, "merged"),
		}, func(e CutProgressEvent) {
			if onProgress != nil {
				e.TotalSegments, e.CompletedSegments = total, offset+e.CompletedSegments
				onProgress(e)
			}
		})
		if runErr != nil {
			_ = os.RemoveAll(scratch)
			return domain.CutResult{}, runErr
		}
		done += len(plan.Segments)

		if run.MergedOutput == nil {
			_ = os.RemoveAll(scratch)
			return domain.CutResult{}, apperrors.New(apperrors.CodeCut, "no clip was generated for person", map[string]any{"groupId": plan.GroupID})
		}
		final := filepath.Join(outputDir, fmt.Sprintf("person-%d.mp4", plan.GroupID))
		if renameErr := os.Rename(run.MergedOutput.Path, final); renameErr != nil {
			_ = os.RemoveAll(scratch)
			return domain.CutResult{}, apperrors.Wrap(apperrors.CodeIO, "failed to move person clip", renameErr, map[string]any{"from": run.MergedOutput.Path, "to": final})
		}
		_ = os.RemoveAll(scratch)
		result.PerSourceOutputs = append(result.PerSourceOutputs, domain.OutputFile{Path: final, DurationSeconds: run.MergedOutput.DurationSeconds})
		result.Warnings = append(result.Warnings, run.Warnings...)
	}
	return result, nil
}

var _ PersonCutRunner = (*CutExecuteAction)(nil)
