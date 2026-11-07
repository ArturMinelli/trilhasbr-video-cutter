package actions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/domain"
	"trilhasbr-video-cutter/internal/media/ffmpeg"
)

type previewVideoRepoStub struct {
	video domain.Video
}

func (s *previewVideoRepoStub) ReplaceWorkspaceVideos(context.Context, int64, []domain.VideoInsert) error {
	return nil
}

func (s *previewVideoRepoStub) ListByWorkspace(context.Context, int64, int, int) ([]domain.Video, int, error) {
	return nil, 0, nil
}

func (s *previewVideoRepoStub) ListByWorkspaceAll(context.Context, int64) ([]domain.Video, error) {
	return nil, nil
}

func (s *previewVideoRepoStub) GetByID(context.Context, int64) (domain.Video, error) {
	return s.video, nil
}

func (s *previewVideoRepoStub) DeleteWorkspaceExportVideos(context.Context, int64, string) (int, error) {
	return 0, nil
}

func (s *previewVideoRepoStub) FindOverlapping(context.Context, int64, time.Time, time.Time) ([]domain.Video, error) {
	return nil, nil
}

func (s *previewVideoRepoStub) ShiftVideoTimes(context.Context, int64, []int64, time.Duration) (int, error) {
	return 0, nil
}

type probeStub struct {
	info      ffmpeg.ProbeInfo
	err       error
	callCount int
}

func (s *probeStub) Probe(context.Context, string) (ffmpeg.ProbeInfo, error) {
	s.callCount++
	if s.err != nil {
		return ffmpeg.ProbeInfo{}, s.err
	}
	return s.info, nil
}

type thumbnailStub struct {
	err       error
	callCount int
}

func (s *thumbnailStub) GenerateThumbnail(_ context.Context, input ffmpeg.ThumbnailInput) (ffmpeg.ThumbnailOutput, error) {
	s.callCount++
	if s.err != nil {
		return ffmpeg.ThumbnailOutput{}, s.err
	}
	if err := os.MkdirAll(filepath.Dir(input.OutputPath), 0o755); err != nil {
		return ffmpeg.ThumbnailOutput{}, err
	}
	if err := os.WriteFile(input.OutputPath, []byte("jpeg"), 0o644); err != nil {
		return ffmpeg.ThumbnailOutput{}, err
	}
	return ffmpeg.ThumbnailOutput{OutputPath: input.OutputPath}, nil
}

func TestVideoPreviewAction_CacheHit(t *testing.T) {
	ctx := context.Background()
	sourcePath := filepath.Join(t.TempDir(), "source.mp4")
	if err := os.WriteFile(sourcePath, []byte("video"), 0o644); err != nil {
		t.Fatalf("failed to create source video: %v", err)
	}

	video := domain.Video{ID: 5, WorkspaceID: 99, Path: sourcePath, DurationSeconds: 20}
	repo := &previewVideoRepoStub{video: video}
	probe := &probeStub{info: ffmpeg.ProbeInfo{DurationSec: 20}}
	thumb := &thumbnailStub{}

	fingerprint, err := videoSourceFingerprint(sourcePath)
	if err != nil {
		t.Fatalf("failed to compute fingerprint: %v", err)
	}

	thumbnailRoot := t.TempDir()
	workspaceDir := filepath.Join(thumbnailRoot, "99")
	if err = os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("failed to create workspace thumbnail directory: %v", err)
	}
	thumbnailPath := filepath.Join(workspaceDir, "5-"+fingerprint+".jpg")
	if err = os.WriteFile(thumbnailPath, []byte("jpeg"), 0o644); err != nil {
		t.Fatalf("failed to create cached thumbnail file: %v", err)
	}

	action := VideoPreviewAction{
		VideoRepo:      repo,
		MediaProbe:     probe,
		MediaThumbnail: thumb,
		ThumbnailRoot:  thumbnailRoot,
		PreviewWidth:   320,
	}

	result, err := action.Execute(ctx, video.ID)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if !result.Cached {
		t.Fatal("expected preview result to come from cache")
	}
	if !strings.HasPrefix(result.DataURL, "data:image/jpeg;base64,") {
		t.Fatalf("unexpected data url format: %s", result.DataURL)
	}
	if probe.callCount != 0 {
		t.Fatalf("expected probe to not run on cache hit, got %d", probe.callCount)
	}
	if thumb.callCount != 0 {
		t.Fatalf("expected thumbnail generation to not run on cache hit, got %d", thumb.callCount)
	}
}

func TestVideoPreviewAction_CacheMissGeneratesThumbnail(t *testing.T) {
	ctx := context.Background()
	sourcePath := filepath.Join(t.TempDir(), "source.mp4")
	if err := os.WriteFile(sourcePath, []byte("video"), 0o644); err != nil {
		t.Fatalf("failed to create source video: %v", err)
	}

	video := domain.Video{ID: 8, WorkspaceID: 77, Path: sourcePath, DurationSeconds: 20}
	repo := &previewVideoRepoStub{video: video}
	probe := &probeStub{info: ffmpeg.ProbeInfo{DurationSec: 20}}
	thumb := &thumbnailStub{}

	action := VideoPreviewAction{
		VideoRepo:      repo,
		MediaProbe:     probe,
		MediaThumbnail: thumb,
		ThumbnailRoot:  t.TempDir(),
		PreviewWidth:   320,
	}

	result, err := action.Execute(ctx, video.ID)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if result.Cached {
		t.Fatal("expected cache miss result")
	}
	if probe.callCount != 1 {
		t.Fatalf("expected probe call count to be 1, got %d", probe.callCount)
	}
	if thumb.callCount != 1 {
		t.Fatalf("expected thumbnail call count to be 1, got %d", thumb.callCount)
	}
	if !strings.HasPrefix(result.DataURL, "data:image/jpeg;base64,") {
		t.Fatalf("unexpected data url format: %s", result.DataURL)
	}
}

func TestVideoPreviewAction_ThumbnailGenerationFailure(t *testing.T) {
	ctx := context.Background()
	sourcePath := filepath.Join(t.TempDir(), "source.mp4")
	if err := os.WriteFile(sourcePath, []byte("video"), 0o644); err != nil {
		t.Fatalf("failed to create source video: %v", err)
	}

	video := domain.Video{ID: 9, WorkspaceID: 55, Path: sourcePath, DurationSeconds: 20}
	repo := &previewVideoRepoStub{video: video}
	probe := &probeStub{info: ffmpeg.ProbeInfo{DurationSec: 20}}
	thumb := &thumbnailStub{err: errors.New("ffmpeg failed")}

	action := VideoPreviewAction{
		VideoRepo:      repo,
		MediaProbe:     probe,
		MediaThumbnail: thumb,
		ThumbnailRoot:  t.TempDir(),
		PreviewWidth:   320,
	}

	_, err := action.Execute(ctx, video.ID)
	if err == nil {
		t.Fatal("expected an error from thumbnail generation failure")
	}
	appErr := apperrors.As(err)
	if appErr.Code != apperrors.CodeProbe {
		t.Fatalf("expected code %s, got %s", apperrors.CodeProbe, appErr.Code)
	}
}
