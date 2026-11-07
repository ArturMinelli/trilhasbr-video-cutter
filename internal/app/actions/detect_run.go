package actions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/domain"
	"trilhasbr-video-cutter/internal/inference"
	"trilhasbr-video-cutter/internal/media/ffmpeg"
	"trilhasbr-video-cutter/internal/repository"
	"trilhasbr-video-cutter/internal/tracking"
)

const (
	DefaultSampleFPS = 5.0
	minSampleFPS     = 1.0
	maxSampleFPS     = 15.0

	embedMinScore  = 0.5 // detections below this skip re-ID (tracker still uses them in stage 2)
	embedMinHeight = 64  // px; tiny crops give junk embeddings
	maxDecodeWidth = 1920
)

type StartDetectionInput struct {
	WorkspaceID int64   `json:"workspaceId"`
	SampleFPS   float64 `json:"sampleFps"` // 0 = default (5)
}

type DetectQueuedVideo struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type DetectProgressEvent struct {
	Queue        []DetectQueuedVideo `json:"queue"`
	JobID        string              `json:"jobId"`
	VideosTotal  int                 `json:"videosTotal"`
	VideosDone   int                 `json:"videosDone"`
	CurrentVideo string              `json:"currentVideo"`
	TracksFound  int                 `json:"tracksFound"`
	VideoID      int64               `json:"videoId"`
	VideoName    string              `json:"videoName"`
	VideoStatus  string              `json:"videoStatus"`
	Percent      *float64            `json:"percent,omitempty"`
}

type DetectFinishedEvent struct {
	JobID       string `json:"jobId"`
	TracksFound int    `json:"tracksFound"`
	ElapsedMs   int64  `json:"elapsedMs"`
}

type DetectFailedEvent struct {
	JobID string                 `json:"jobId"`
	Error *apperrors.PublicError `json:"error"`
}

type DetectVideoFailure struct {
	VideoID int64                  `json:"videoId"`
	Name    string                 `json:"name"`
	Error   *apperrors.PublicError `json:"error"`
}

// DetectResult summarizes a run. Status is SUCCEEDED, FAILED (every analyzed video failed) or CANCELLED.
// TracksFound counts tracks created by this run; PeopleTotal is the workspace gallery size afterwards.
type DetectResult struct {
	RunID         int64                  `json:"runId"`
	Status        domain.DetectionStatus `json:"status"`
	VideosTotal   int                    `json:"videosTotal"`
	VideosDone    int                    `json:"videosDone"` // analyzed + skipped + failed
	VideosSkipped int                    `json:"videosSkipped"`
	VideosFailed  int                    `json:"videosFailed"`
	TracksFound   int                    `json:"tracksFound"`
	PeopleTotal   int                    `json:"peopleTotal"`
	ElapsedMs     int64                  `json:"elapsedMs"`
	Failures      []DetectVideoFailure   `json:"failures,omitempty"`
}

// FrameOpener is satisfied by *ffmpeg.Client.
type FrameOpener interface {
	OpenFrames(ctx context.Context, path string, fps float64, maxWidth int) (*ffmpeg.FrameStream, error)
}

type DetectRunAction struct {
	WorkspaceRepo repository.WorkspaceRepository
	VideoRepo     repository.VideoRepository
	DetectionRepo repository.DetectionRepository
	Media         FrameOpener
	Engine        inference.Engine
	ModelVersion  string
	ThumbDir      string // thumbnails go to <ThumbDir>/<workspaceID>/<videoID>-<trackID>.jpg
	Logger        *slog.Logger
	Workers       int // 0 = max(2, NumCPU/3)
}

// Validate checks the sampling rate (0 = default, otherwise 1-15 fps).
func (in StartDetectionInput) Validate() error {
	if in.SampleFPS != 0 && (in.SampleFPS < minSampleFPS || in.SampleFPS > maxSampleFPS) {
		return apperrors.New(apperrors.CodeValidation, "sample fps must be between 1 and 15", map[string]any{"sampleFps": in.SampleFPS})
	}
	return nil
}

func (a *DetectRunAction) Execute(ctx context.Context, jobID string, in StartDetectionInput, onProgress func(DetectProgressEvent)) (DetectResult, error) {
	if err := in.Validate(); err != nil {
		return DetectResult{}, err
	}
	fps := in.SampleFPS
	if fps == 0 {
		fps = DefaultSampleFPS
	}
	if _, err := a.WorkspaceRepo.GetByID(ctx, in.WorkspaceID); err != nil {
		return DetectResult{}, err
	}
	videos, err := a.VideoRepo.ListByWorkspaceAll(ctx, in.WorkspaceID)
	if err != nil {
		return DetectResult{}, err
	}
	logger := a.Logger
	if logger == nil {
		logger = slog.Default()
	}

	run, err := a.DetectionRepo.CreateRun(ctx, in.WorkspaceID, fps, a.ModelVersion, len(videos))
	if err != nil {
		return DetectResult{}, err
	}

	start := time.Now()
	res := DetectResult{RunID: run.ID, VideosTotal: len(videos)}
	var mu sync.Mutex
	emit := func(v domain.Video, status string, percent *float64) {
		mu.Lock()
		defer mu.Unlock()
		if onProgress != nil {
			onProgress(DetectProgressEvent{JobID: jobID, VideosTotal: res.VideosTotal, VideosDone: res.VideosDone, TracksFound: res.TracksFound, VideoID: v.ID, VideoName: v.Name, VideoStatus: status, Percent: percent})
		}
	}
	if onProgress != nil {
		queue := make([]DetectQueuedVideo, 0, len(videos))
		for _, v := range videos {
			queue = append(queue, DetectQueuedVideo{ID: v.ID, Name: v.Name})
		}
		onProgress(DetectProgressEvent{JobID: jobID, VideosTotal: len(videos), Queue: queue})
	}
	finishVideo := func(v domain.Video, tracks int, skipped bool, verr error) {
		mu.Lock()
		defer mu.Unlock()
		res.VideosDone++
		res.TracksFound += tracks
		switch {
		case skipped:
			res.VideosSkipped++
		case verr != nil:
			res.VideosFailed++
			res.Failures = append(res.Failures, DetectVideoFailure{VideoID: v.ID, Name: v.Name, Error: apperrors.ToPublic(verr)})
		}
		status := "completed"
		if skipped {
			status = "skipped"
		} else if verr != nil {
			status = "failed"
		}
		if onProgress != nil {
			onProgress(DetectProgressEvent{JobID: jobID, VideosTotal: res.VideosTotal, VideosDone: res.VideosDone, CurrentVideo: v.Name, TracksFound: res.TracksFound, VideoID: v.ID, VideoName: v.Name, VideoStatus: status})
		}
	}

	workers := a.Workers
	if workers <= 0 {
		workers = max(2, runtime.NumCPU()/3)
	}
	workers = min(workers, max(len(videos), 1))

	queue := make(chan domain.Video)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for v := range queue {
				if ctx.Err() != nil {
					return
				}
				emit(v, "processing", nil)
				tracks, skipped, verr := a.processVideo(ctx, v, fps, logger, func(status string, percent *float64) { emit(v, status, percent) })
				if ctx.Err() != nil {
					return // in-flight video is not persisted and not counted
				}
				if verr != nil {
					logger.Error("detection failed for video", "videoId", v.ID, "path", v.Path, "error", verr)
					a.recordFailure(ctx, v, fps, verr)
				}
				finishVideo(v, tracks, skipped, verr)
			}
		}()
	}
feed:
	for _, v := range videos {
		select {
		case queue <- v:
		case <-ctx.Done():
			break feed
		}
	}
	close(queue)
	wg.Wait()

	switch {
	case ctx.Err() != nil:
		res.Status = domain.DetectionCancelled
	case res.VideosFailed > 0 && res.VideosFailed == res.VideosDone-res.VideosSkipped:
		res.Status = domain.DetectionFailed
	default:
		res.Status = domain.DetectionSucceeded
	}
	res.ElapsedMs = time.Since(start).Milliseconds()

	// The run record must be closed even when ctx was cancelled.
	bg := context.WithoutCancel(ctx)
	if _, total, lerr := a.DetectionRepo.ListGroups(bg, domain.GroupFilter{WorkspaceID: in.WorkspaceID, Page: 1, PageSize: 1}); lerr == nil {
		res.PeopleTotal = total
	}
	if ferr := a.DetectionRepo.FinishRun(bg, run.ID, res.Status, res.VideosDone, res.TracksFound); ferr != nil {
		return res, ferr
	}
	logger.Info("detection finished", "runId", run.ID, "status", res.Status, "videos", res.VideosDone, "skipped", res.VideosSkipped, "failed", res.VideosFailed, "tracks", res.TracksFound, "elapsedMs", res.ElapsedMs)
	return res, nil
}

func (a *DetectRunAction) recordFailure(ctx context.Context, v domain.Video, fps float64, verr error) {
	size, mtime := fileKey(v.Path) // zero on stat failure; never matches a real file
	_ = a.DetectionRepo.UpsertVideoAnalysis(context.WithoutCancel(ctx), domain.VideoAnalysis{
		VideoID: v.ID, SampleFPS: fps, ModelVersion: a.ModelVersion, FileSize: size, FileMTime: mtime,
		Status: domain.AnalysisFailed, Error: verr.Error(),
	})
}

func fileKey(path string) (size, mtime int64) {
	st, err := os.Stat(path)
	if err != nil {
		return 0, 0
	}
	return st.Size(), st.ModTime().Unix()
}

// processVideo analyzes one video. Returns the number of tracks stored, or skipped=true when the stored analysis matches.
func (a *DetectRunAction) processVideo(ctx context.Context, v domain.Video, fps float64, logger *slog.Logger, progress func(string, *float64)) (tracks int, skipped bool, err error) {
	st, err := os.Stat(v.Path)
	if err != nil {
		return 0, false, apperrors.Wrap(apperrors.CodeIO, "cannot read video file", err, map[string]any{"path": v.Path})
	}
	size, mtime := st.Size(), st.ModTime().Unix()

	prev, err := a.DetectionRepo.GetVideoAnalysis(ctx, v.ID)
	if err != nil {
		return 0, false, err
	}
	if prev != nil && prev.Status == domain.AnalysisDone && prev.SampleFPS == fps &&
		prev.ModelVersion == a.ModelVersion && prev.FileSize == size && prev.FileMTime == mtime {
		return 0, true, nil
	}

	started := time.Now()
	found, frames, frameSz, err := a.analyze(ctx, v, fps, progress)
	if err != nil {
		return 0, false, err
	}
	progress("finalizing", nil)
	if err = a.writeThumbnails(ctx, v, fps, found); err != nil {
		// A missing thumbnail must not discard the tracks; the gallery falls back to no image.
		logger.Warn("thumbnail generation failed", "videoId", v.ID, "error", err)
	}

	stored, err := a.DetectionRepo.ReplaceTracksForVideo(ctx, v.ID, toDomainTracks(v, a.ThumbDir, found, frameSz.w, frameSz.h))
	if err != nil {
		return 0, false, err
	}
	if err = a.DetectionRepo.UpsertVideoAnalysis(ctx, domain.VideoAnalysis{
		VideoID: v.ID, SampleFPS: fps, ModelVersion: a.ModelVersion, FileSize: size, FileMTime: mtime, Status: domain.AnalysisDone,
	}); err != nil {
		return 0, false, err
	}
	logger.Info("video analyzed", "videoId", v.ID, "frames", frames, "tracks", len(stored), "elapsedMs", time.Since(started).Milliseconds())
	return len(stored), false, nil
}

// analyze decodes sampled frames, detects people, embeds confident crops and runs the tracker.
// frameSize is the decoded frame size (after rotation and downscale) that tracker boxes are expressed in.
type frameSize struct{ w, h int }

func (a *DetectRunAction) analyze(ctx context.Context, v domain.Video, fps float64, progress func(string, *float64)) ([]tracking.Track, int, frameSize, error) {
	stream, err := a.Media.OpenFrames(ctx, v.Path, fps, maxDecodeWidth)
	if err != nil {
		return nil, 0, frameSize{}, err
	}
	defer stream.Close()
	size := frameSize{stream.Width, stream.Height}

	tr := tracking.New(tracking.Config{FPS: fps})
	frames := 0
	last := time.Time{}
	for {
		f, err := stream.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, frames, size, err
		}
		frames++
		frame := inference.Frame{W: f.Width, H: f.Height, RGB: f.RGB}
		boxes, err := a.Engine.Detect(frame)
		if err != nil {
			return nil, frames, size, apperrors.Wrap(inference.CodeInference, "person detection failed", err, map[string]any{"videoId": v.ID, "frame": f.Index})
		}
		dets := make([]tracking.Detection, len(boxes))
		var crops []inference.Frame
		var cropOf []int // crops[i] belongs to dets[cropOf[i]]
		for i, b := range boxes {
			dets[i] = tracking.Detection{Box: tracking.Box{float64(b.X1), float64(b.Y1), float64(b.X2), float64(b.Y2)}, Score: float64(b.Conf), FrameIndex: f.Index}
			if b.Conf < embedMinScore || b.Y2-b.Y1 < embedMinHeight {
				continue
			}
			if c, ok := inference.Crop(frame, b); ok {
				crops = append(crops, c)
				cropOf = append(cropOf, i)
			}
		}
		if len(crops) > 0 {
			embs, err := a.Engine.Embed(crops)
			if err != nil {
				return nil, frames, size, apperrors.Wrap(inference.CodeInference, "re-id embedding failed", err, map[string]any{"videoId": v.ID, "frame": f.Index})
			}
			for i, e := range embs {
				dets[cropOf[i]].Embedding = e
			}
		}
		tr.Update(f.TimeSec, dets) // every frame, including empty ones
		if time.Since(last) >= 500*time.Millisecond {
			var percent *float64
			if v.DurationSeconds > 0 {
				value := math.Min(99, math.Max(0, f.TimeSec/v.DurationSeconds*100))
				percent = &value
			}
			progress("processing", percent)
			last = time.Now()
		}
	}
	return tr.Finish(), frames, size, nil
}

// writeThumbnails fills thumbnail files for each track's best detection.
// ponytail: second decode pass (decode is cheap next to inference) keeps memory bounded instead of
// buffering crops for every live track; revisit if decode time shows up in profiles.
func (a *DetectRunAction) writeThumbnails(ctx context.Context, v domain.Video, fps float64, found []tracking.Track) error {
	if len(found) == 0 || a.ThumbDir == "" {
		return nil
	}
	want := map[int][]int{} // frame index -> indexes into found
	last := 0
	for i, t := range found {
		want[t.Best.FrameIndex] = append(want[t.Best.FrameIndex], i)
		last = max(last, t.Best.FrameIndex)
	}
	stream, err := a.Media.OpenFrames(ctx, v.Path, fps, maxDecodeWidth)
	if err != nil {
		return err
	}
	defer stream.Close()
	var firstErr error
	for {
		f, err := stream.Next()
		if err != nil {
			if !errors.Is(err, io.EOF) && firstErr == nil {
				firstErr = err
			}
			return firstErr
		}
		for _, i := range want[f.Index] {
			b := found[i].Best.Box
			path := thumbPath(a.ThumbDir, v, found[i].ID)
			if werr := ffmpeg.WriteJPEG(path, f, int(b[0]), int(b[1]), int(b[2]+0.5), int(b[3]+0.5)); werr != nil && firstErr == nil {
				firstErr = werr
			}
		}
		if f.Index >= last {
			return firstErr
		}
	}
}

func thumbPath(dir string, v domain.Video, trackID int) string {
	return filepath.Join(dir, fmt.Sprint(v.WorkspaceID), fmt.Sprintf("%d-%d.jpg", v.ID, trackID))
}

func toDomainTracks(v domain.Video, thumbDir string, found []tracking.Track, w, h int) []domain.Track {
	out := make([]domain.Track, 0, len(found))
	for _, t := range found {
		bbox, _ := json.Marshal([]float64{t.Best.Box[0], t.Best.Box[1], t.Best.Box[2], t.Best.Box[3]})
		dt := domain.Track{
			VideoID: v.ID, StartSec: t.StartSec, EndSec: t.EndSec, AvgConfidence: t.MeanScore,
			DetectionCount: t.Detections, BestBBox: string(bbox), Boxes: normalizePath(t.Path, w, h),
		}
		if thumbDir != "" {
			if p := thumbPath(thumbDir, v, t.ID); fileExists(p) {
				dt.ThumbnailPath = p
			}
		}
		if len(t.Embedding) == domain.EmbeddingDim {
			dt.Embedding = t.Embedding
		}
		out = append(out, dt)
	}
	return out
}

// normalizePath converts pixel boxes to 0..1 of the decoded frame (clamped); degenerate boxes are dropped.
func normalizePath(path []tracking.PathPoint, w, h int) []domain.TrackBox {
	if w <= 0 || h <= 0 {
		return nil
	}
	clamp := func(v float64) float32 { return float32(math.Min(1, math.Max(0, v))) }
	out := make([]domain.TrackBox, 0, len(path))
	for _, p := range path {
		b := domain.TrackBox{T: float32(p.TimeSec), X1: clamp(p.Box[0] / float64(w)), Y1: clamp(p.Box[1] / float64(h)), X2: clamp(p.Box[2] / float64(w)), Y2: clamp(p.Box[3] / float64(h))}
		if b.X2 <= b.X1 || b.Y2 <= b.Y1 {
			continue
		}
		out = append(out, b)
	}
	return out
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
