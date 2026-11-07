package actions

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/media/ffmpeg"
	"trilhasbr-video-cutter/internal/platform"
	"trilhasbr-video-cutter/internal/repository"
)

type VideoPreviewAction struct {
	VideoRepo      repository.VideoRepository
	MediaProbe     ffmpeg.MediaProbe
	MediaThumbnail ffmpeg.MediaThumbnailer
	ThumbnailRoot  string
	PreviewWidth   int

	previewOnce  sync.Once
	previewSlots chan struct{}
}

type VideoPreviewResult struct {
	VideoID  int64
	DataURL  string
	Cached   bool
	FilePath string
}

func (a *VideoPreviewAction) Execute(ctx context.Context, videoID int64) (VideoPreviewResult, error) {
	if videoID <= 0 {
		return VideoPreviewResult{}, apperrors.New(apperrors.CodeValidation, "videoId must be greater than 0", nil)
	}
	if a.VideoRepo == nil || a.MediaProbe == nil || a.MediaThumbnail == nil {
		return VideoPreviewResult{}, apperrors.New(apperrors.CodeInternal, "video preview action is not fully configured", nil)
	}
	if strings.TrimSpace(a.ThumbnailRoot) == "" {
		return VideoPreviewResult{}, apperrors.New(apperrors.CodeInternal, "thumbnail root directory is not configured", nil)
	}

	video, err := a.VideoRepo.GetByID(ctx, videoID)
	if err != nil {
		return VideoPreviewResult{}, err
	}

	workspaceThumbDir := filepath.Join(a.ThumbnailRoot, strconv.FormatInt(video.WorkspaceID, 10))
	if err = platform.EnsureDir(workspaceThumbDir); err != nil {
		return VideoPreviewResult{}, err
	}

	sourceFingerprint, err := videoSourceFingerprint(video.Path)
	if err != nil {
		return VideoPreviewResult{}, err
	}

	thumbnailPath := filepath.Join(workspaceThumbDir, fmt.Sprintf("%d-%s.jpg", video.ID, sourceFingerprint))
	if dataURL, existsErr := readImageDataURLIfExists(thumbnailPath); existsErr != nil {
		return VideoPreviewResult{}, existsErr
	} else if dataURL != "" {
		return VideoPreviewResult{
			VideoID:  video.ID,
			DataURL:  dataURL,
			Cached:   true,
			FilePath: thumbnailPath,
		}, nil
	}

	// Limit concurrent ffmpeg processes when many cards become visible together.
	a.previewOnce.Do(func() { a.previewSlots = make(chan struct{}, 2) })
	select {
	case a.previewSlots <- struct{}{}:
		defer func() { <-a.previewSlots }()
	case <-ctx.Done():
		return VideoPreviewResult{}, ctx.Err()
	}
	// Another request may have generated the same preview while this one waited.
	if dataURL, readErr := readImageDataURLIfExists(thumbnailPath); readErr != nil {
		return VideoPreviewResult{}, readErr
	} else if dataURL != "" {
		return VideoPreviewResult{VideoID: video.ID, DataURL: dataURL, Cached: true, FilePath: thumbnailPath}, nil
	}

	probe, err := a.MediaProbe.Probe(ctx, video.Path)
	if err != nil {
		return VideoPreviewResult{}, err
	}

	previewAtSec := computePreviewTimestamp(probe.DurationSec, video.DurationSeconds)
	width := a.PreviewWidth
	if width <= 0 {
		width = 320
	}

	if _, err = a.MediaThumbnail.GenerateThumbnail(ctx, ffmpeg.ThumbnailInput{
		InputPath:  video.Path,
		OutputPath: thumbnailPath,
		AtSecond:   previewAtSec,
		Width:      width,
	}); err != nil {
		return VideoPreviewResult{}, apperrors.Wrap(apperrors.CodeProbe, "failed to generate preview thumbnail", err, map[string]any{
			"videoId": video.ID,
			"path":    video.Path,
		})
	}

	dataURL, err := readImageDataURLIfExists(thumbnailPath)
	if err != nil {
		return VideoPreviewResult{}, err
	}
	if dataURL == "" {
		return VideoPreviewResult{}, apperrors.New(apperrors.CodeProbe, "thumbnail generation finished but output image is missing", map[string]any{
			"videoId":         video.ID,
			"thumbnailPath":   thumbnailPath,
			"sourceVideoPath": video.Path,
		})
	}

	return VideoPreviewResult{
		VideoID:  video.ID,
		DataURL:  dataURL,
		Cached:   false,
		FilePath: thumbnailPath,
	}, nil
}

func computePreviewTimestamp(probedDuration float64, fallbackDuration float64) float64 {
	duration := probedDuration
	if duration <= 0 {
		duration = fallbackDuration
	}
	if duration <= 0 {
		return 0
	}

	previewAt := duration * 0.1
	if previewAt < 1 {
		previewAt = 1
	}

	half := duration * 0.5
	if half > 0 && previewAt > half {
		previewAt = half
	}

	return math.Max(0, previewAt)
}

func videoSourceFingerprint(sourcePath string) (string, error) {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return "", apperrors.Wrap(apperrors.CodeIO, "failed to read source video metadata for preview cache", err, map[string]any{"path": sourcePath})
	}

	raw := fmt.Sprintf("%s|%d|%d", filepath.Clean(sourcePath), info.Size(), info.ModTime().UTC().UnixNano())
	hash := sha1.Sum([]byte(raw))
	return hex.EncodeToString(hash[:8]), nil
}

func readImageDataURLIfExists(imagePath string) (string, error) {
	stat, err := os.Stat(imagePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", apperrors.Wrap(apperrors.CodeIO, "failed to read thumbnail file metadata", err, map[string]any{"path": imagePath})
	}
	if stat.Size() <= 0 {
		return "", nil
	}

	content, err := os.ReadFile(imagePath)
	if err != nil {
		return "", apperrors.Wrap(apperrors.CodeIO, "failed to read thumbnail file", err, map[string]any{"path": imagePath})
	}
	if len(content) == 0 {
		return "", nil
	}

	encoded := base64.StdEncoding.EncodeToString(content)
	return "data:image/jpeg;base64," + encoded, nil
}
