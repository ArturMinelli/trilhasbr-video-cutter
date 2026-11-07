package actions

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/domain"
	"trilhasbr-video-cutter/internal/media/ffmpeg"
	"trilhasbr-video-cutter/internal/media/timestamp"
	"trilhasbr-video-cutter/internal/platform"
	"trilhasbr-video-cutter/internal/repository"
)

type WorkspaceLoadAction struct {
	Picker        DirectoryPicker
	WorkspaceRepo repository.WorkspaceRepository
	VideoRepo     repository.VideoRepository
	MediaProbe    ffmpeg.MediaProbe
	Logger        *slog.Logger
	WorkerCount   int
	// Guard (optional) refuses a refresh while the workspace is busy (e.g. detection running).
	Guard func(workspaceID int64) error
}

type FileFailure struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type WorkspaceLoadResult struct {
	WorkspaceID        int64         `json:"workspaceId"`
	WorkspaceDirectory string        `json:"workspaceDirectory"`
	WorkspaceName      string        `json:"workspaceName"`
	WorkspaceCreatedAt time.Time     `json:"workspaceCreatedAt"`
	WorkspaceUpdatedAt time.Time     `json:"workspaceUpdatedAt"`
	TotalScanned       int           `json:"totalScanned"`
	Indexed            int           `json:"indexed"`
	NullStartedAtCount int           `json:"nullStartedAtCount"`
	Failures           []FileFailure `json:"failures"`
}

func (a *WorkspaceLoadAction) Execute(ctx context.Context) (WorkspaceLoadResult, error) {
	if a.Picker == nil {
		return WorkspaceLoadResult{}, apperrors.New(apperrors.CodeInternal, "directory picker is not configured", nil)
	}
	if a.MediaProbe == nil {
		return WorkspaceLoadResult{}, apperrors.New(apperrors.CodeInternal, "media probe is not configured", nil)
	}

	rawDirectory, err := a.Picker.PickDirectory(ctx)
	if err != nil {
		return WorkspaceLoadResult{}, apperrors.Wrap(apperrors.CodeIO, "failed to select workspace directory", err, nil)
	}
	if strings.TrimSpace(rawDirectory) == "" {
		return WorkspaceLoadResult{}, apperrors.New(apperrors.CodeValidation, "workspace selection was cancelled", nil)
	}

	normalizedDirectory, err := platform.NormalizeDirectory(rawDirectory)
	if err != nil {
		return WorkspaceLoadResult{}, err
	}

	workspaceName := platform.WorkspaceNameFromDirectory(normalizedDirectory)
	workspace, err := a.WorkspaceRepo.UpsertByDirectory(ctx, normalizedDirectory, workspaceName)
	if err != nil {
		return WorkspaceLoadResult{}, err
	}

	return a.index(ctx, workspace)
}

// Refresh re-scans the stored directory of an existing workspace (no picker).
// Videos still on disk keep their IDs (and detection results); videos that vanished are removed.
// If the directory is gone, nothing is touched and a typed IO error is returned.
func (a *WorkspaceLoadAction) Refresh(ctx context.Context, workspaceID int64) (WorkspaceLoadResult, error) {
	if a.MediaProbe == nil {
		return WorkspaceLoadResult{}, apperrors.New(apperrors.CodeInternal, "media probe is not configured", nil)
	}
	workspace, err := a.WorkspaceRepo.GetByID(ctx, workspaceID)
	if err != nil {
		return WorkspaceLoadResult{}, err
	}
	if a.Guard != nil {
		if err = a.Guard(workspaceID); err != nil {
			return WorkspaceLoadResult{}, err
		}
	}
	if info, statErr := os.Stat(workspace.Directory); statErr != nil || !info.IsDir() {
		return WorkspaceLoadResult{}, apperrors.New(apperrors.CodeIO, "workspace directory is not available; existing data was kept", map[string]any{"directory": workspace.Directory})
	}
	// Bump updated_at; the name stays as the user last saw it.
	if workspace, err = a.WorkspaceRepo.UpsertByDirectory(ctx, workspace.Directory, workspace.Name); err != nil {
		return WorkspaceLoadResult{}, err
	}
	return a.index(ctx, workspace)
}

// index scans the workspace directory and replaces its stored videos. Shared by load and refresh.
func (a *WorkspaceLoadAction) index(ctx context.Context, workspace domain.Workspace) (WorkspaceLoadResult, error) {
	videoPaths, err := discoverMP4Files(workspace.Directory)
	if err != nil {
		return WorkspaceLoadResult{}, err
	}

	indexedVideos, failures := a.probeVideos(ctx, videoPaths)
	// A cancelled scan is partial: replacing now would delete the unprobed videos and their detection data.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return WorkspaceLoadResult{}, apperrors.Wrap(apperrors.CodeIO, "workspace scan was cancelled", ctxErr, nil)
	}

	// Every probe failing because ffmpeg is missing is a setup problem, not an empty folder.
	if len(indexedVideos) == 0 && len(failures) > 0 && allFFmpegMissing(failures) {
		return WorkspaceLoadResult{}, apperrors.New(apperrors.CodeFFmpegNotFound, "ffmpeg/ffprobe binaries were not found; no videos could be indexed", map[string]any{"scanned": len(videoPaths)})
	}

	toStore, err := a.keepFailedExisting(ctx, workspace.ID, indexedVideos, failures)
	if err != nil {
		return WorkspaceLoadResult{}, err
	}
	if replaceErr := a.VideoRepo.ReplaceWorkspaceVideos(ctx, workspace.ID, toStore); replaceErr != nil {
		return WorkspaceLoadResult{}, replaceErr
	}

	nullStartedAtCount := 0
	for _, video := range toStore {
		if video.StartedAt == nil {
			nullStartedAtCount++
		}
	}

	return WorkspaceLoadResult{
		WorkspaceID:        workspace.ID,
		WorkspaceDirectory: workspace.Directory,
		WorkspaceName:      workspace.Name,
		WorkspaceCreatedAt: workspace.CreatedAt,
		WorkspaceUpdatedAt: workspace.UpdatedAt,
		TotalScanned:       len(videoPaths),
		Indexed:            len(indexedVideos),
		NullStartedAtCount: nullStartedAtCount,
		Failures:           failures,
	}, nil
}

func allFFmpegMissing(failures []FileFailure) bool {
	for _, failure := range failures {
		if !strings.Contains(failure.Reason, string(apperrors.CodeFFmpegNotFound)) {
			return false
		}
	}
	return true
}

// keepFailedExisting re-adds already-stored videos whose probe failed this time (still on disk),
// so a transient probe error does not delete them and cascade away their detection results.
func (a *WorkspaceLoadAction) keepFailedExisting(ctx context.Context, workspaceID int64, indexed []domain.VideoInsert, failures []FileFailure) ([]domain.VideoInsert, error) {
	if len(failures) == 0 {
		return indexed, nil
	}
	failed := make(map[string]bool, len(failures))
	for _, failure := range failures {
		failed[failure.Path] = true
	}
	existing, err := a.VideoRepo.ListByWorkspaceAll(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	toStore := append([]domain.VideoInsert(nil), indexed...)
	for _, video := range existing {
		if failed[video.Path] {
			toStore = append(toStore, domain.VideoInsert{
				Name: video.Name, Path: video.Path, StartedAt: video.StartedAt,
				FinishedAt: video.FinishedAt, DurationSeconds: video.DurationSeconds,
			})
		}
	}
	return toStore, nil
}

func (a *WorkspaceLoadAction) probeVideos(ctx context.Context, videoPaths []string) ([]domain.VideoInsert, []FileFailure) {
	if len(videoPaths) == 0 {
		return []domain.VideoInsert{}, []FileFailure{}
	}

	workerCount := a.WorkerCount
	if workerCount <= 0 {
		workerCount = runtime.NumCPU()
	}
	if workerCount < 1 {
		workerCount = 1
	}
	if workerCount > 8 {
		workerCount = 8
	}

	type probeResult struct {
		insert  *domain.VideoInsert
		failure *FileFailure
	}

	jobs := make(chan string)
	results := make(chan probeResult)
	var wg sync.WaitGroup

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				info, err := a.MediaProbe.Probe(ctx, path)
				if err != nil {
					if a.Logger != nil {
						a.Logger.Warn("failed to probe video", slog.String("path", path), slog.String("error", err.Error()))
					}
					results <- probeResult{failure: &FileFailure{Path: path, Reason: err.Error()}}
					continue
				}

				startedAt := info.StartedAt
				if startedAt == nil {
					if fromName, ok := timestamp.ParseFilenameTime(path, time.Local); ok {
						startedAt = fromName
					}
				}

				var finishedAt *time.Time
				if startedAt != nil {
					computed := startedAt.Add(time.Duration(info.DurationSec * float64(time.Second))).UTC()
					finishedAt = &computed
				}

				videoName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
				if videoName == "" {
					videoName = filepath.Base(path)
				}

				results <- probeResult{insert: &domain.VideoInsert{
					Name:            videoName,
					Path:            path,
					StartedAt:       startedAt,
					FinishedAt:      finishedAt,
					DurationSeconds: info.DurationSec,
				}}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, path := range videoPaths {
			select {
			case <-ctx.Done():
				return
			case jobs <- path:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	indexed := make([]domain.VideoInsert, 0, len(videoPaths))
	failures := make([]FileFailure, 0)

	for result := range results {
		if result.failure != nil {
			failures = append(failures, *result.failure)
			continue
		}
		if result.insert != nil {
			indexed = append(indexed, *result.insert)
		}
	}

	return indexed, failures
}

func discoverMP4Files(root string) ([]string, error) {
	paths := make([]string, 0, 128)
	cleanRoot := filepath.Clean(root)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if strings.EqualFold(d.Name(), "exports") && filepath.Clean(path) != cleanRoot {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(d.Name()), ".mp4") {
			absolute, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			paths = append(paths, absolute)
		}
		return nil
	})
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeIO, "failed to scan workspace directory", err, map[string]any{"directory": root})
	}
	return paths, nil
}
