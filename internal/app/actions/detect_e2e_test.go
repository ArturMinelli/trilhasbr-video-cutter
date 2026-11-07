package actions

import (
	"context"
	"encoding/json"
	"image/jpeg"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"trilhasbr-video-cutter/internal/inference"
	"trilhasbr-video-cutter/internal/media/ffmpeg"
	"trilhasbr-video-cutter/internal/platform"
	"trilhasbr-video-cutter/internal/repository/sqlite"
)

type e2ePicker string

func (p e2ePicker) PickDirectory(context.Context) (string, error) { return string(p), nil }

func probeDuration(t *testing.T, path string) float64 {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", path).Output()
	if err != nil {
		t.Fatalf("ffprobe %s: %v", path, err)
	}
	d, _ := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	return d
}

// TestE2EDetectRealVideos runs detect -> list -> cut on a real folder with the real models.
// Usage: DETECT_E2E_DIR=<folder with .mp4> [DETECT_E2E_FPS=5] [DETECT_E2E_THUMBS=dir] [DETECT_E2E_WORKERS=n] go test ./internal/app/actions -run E2E -v -timeout 30m
func TestE2EDetectRealVideos(t *testing.T) {
	dir := os.Getenv("DETECT_E2E_DIR")
	if dir == "" {
		t.Skip("DETECT_E2E_DIR not set")
	}
	fps := 5.0
	if v, err := strconv.ParseFloat(os.Getenv("DETECT_E2E_FPS"), 64); err == nil {
		fps = v
	}
	ctx := context.Background()
	logger := slog.Default()

	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "e2e.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = sqlite.RunMigrations(ctx, db); err != nil {
		t.Fatal(err)
	}
	wsRepo, vidRepo, detRepo := sqlite.NewWorkspaceRepository(db), sqlite.NewVideoRepository(db), sqlite.NewDetectionRepository(db)
	media := ffmpeg.NewClient(platform.NewFFmpegResolver(filepath.Join("..", "..", "..", "resources", "ffmpeg")), logger)

	load, err := (&WorkspaceLoadAction{Picker: e2ePicker(dir), WorkspaceRepo: wsRepo, VideoRepo: vidRepo, MediaProbe: media, Logger: logger}).Execute(ctx)
	if err != nil || load.Indexed == 0 {
		t.Fatalf("load workspace: indexed=%d err=%v", load.Indexed, err)
	}
	videos, _ := vidRepo.ListByWorkspaceAll(ctx, load.WorkspaceID)
	var totalSec float64
	for _, v := range videos {
		totalSec += v.DurationSeconds
	}
	t.Logf("workspace: %d videos, %.0fs of footage", len(videos), totalSec)

	paths, err := platform.NewModelResolver(filepath.Join("..", "..", "..", "resources")).Resolve()
	if err != nil {
		t.Fatal(err)
	}
	engine, err := inference.Open(inference.Config{LibPath: paths.ORTLib, YOLOPath: paths.YOLO, OSNetPath: paths.OSNet, Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	t.Logf("providers: yolo=%s osnet=%s", engine.YOLOProvider, engine.OSNetProvider)

	workers := 0 // default
	if v, err := strconv.Atoi(os.Getenv("DETECT_E2E_WORKERS")); err == nil {
		workers = v
	}
	thumbs := os.Getenv("DETECT_E2E_THUMBS") // optional: keep thumbnails for eyeballing
	if thumbs == "" {
		thumbs = filepath.Join(t.TempDir(), "people")
	}
	action := &DetectRunAction{
		WorkspaceRepo: wsRepo, VideoRepo: vidRepo, DetectionRepo: detRepo, Media: media, Engine: engine,
		ModelVersion: paths.Version, ThumbDir: thumbs, Logger: logger, Workers: workers,
	}
	in := StartDetectionInput{WorkspaceID: load.WorkspaceID, SampleFPS: fps}

	start := time.Now()
	res, err := action.Execute(ctx, "e2e", in, func(p DetectProgressEvent) {
		t.Logf("progress %d/%d %s tracks=%d", p.VideosDone, p.VideosTotal, p.CurrentVideo, p.TracksFound)
	})
	if err != nil {
		t.Fatal(err)
	}
	wall := time.Since(start)
	frames := totalSec * fps
	t.Logf("DETECT fps=%.0f wall=%s tracks=%d people=%d failed=%d | %.1fx realtime | %.0f ms/frame (wall/frames) | 1h extrapolation: %.1f min",
		fps, wall.Round(time.Millisecond), res.TracksFound, res.PeopleTotal, res.VideosFailed,
		totalSec/wall.Seconds(), wall.Seconds()*1000/frames, wall.Seconds()/totalSec*3600/60)
	if res.VideosFailed != 0 {
		t.Fatalf("failures: %+v", res.Failures)
	}

	// Re-run must skip everything quickly (SC-005).
	start = time.Now()
	again, err := action.Execute(ctx, "e2e-2", in, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("RERUN wall=%s skipped=%d/%d", time.Since(start).Round(time.Millisecond), again.VideosSkipped, again.VideosTotal)
	if again.VideosSkipped != again.VideosTotal || time.Since(start) > 10*time.Second {
		t.Fatalf("rerun not skipped/fast: %+v", again)
	}

	// Gallery.
	list := &PersonListAction{Repo: detRepo}
	groups, total, err := list.List(ctx, ListPeopleInput{WorkspaceID: load.WorkspaceID, Page: 1, PageSize: 500})
	if err != nil {
		t.Fatal(err)
	}
	perVideo := map[string]int{}
	for _, g := range groups {
		for _, tr := range g.Tracks {
			perVideo[tr.VideoName]++
			t.Logf("track id=%d %-40s %6.1f-%6.1fs conf=%.2f thumb=%s", tr.ID, tr.VideoName, tr.StartSec, tr.EndSec, g.AvgConfidence, tr.ThumbnailPath)
		}
		if len(g.Tracks) > 0 && g.Tracks[0].ThumbnailPath != "" {
			f, oerr := os.Open(g.Tracks[0].ThumbnailPath)
			if oerr != nil {
				t.Errorf("thumbnail missing: %v", oerr)
				continue
			}
			if _, derr := jpeg.Decode(f); derr != nil {
				t.Errorf("thumbnail not a valid jpeg: %v", derr)
			}
			f.Close()
		}
	}
	b, _ := json.Marshal(perVideo)
	t.Logf("PEOPLE groups=%d (total %d) tracks per video: %s", len(groups), total, b)
	if len(groups) == 0 {
		t.Skip("no people detected; nothing to cut")
	}

	// Cut the 2 longest groups with 3s padding.
	sort.Slice(groups, func(i, j int) bool { return groups[i].TotalDurationSec > groups[j].TotalDurationSec })
	pick := groups
	if len(pick) > 2 {
		pick = pick[:2]
	}
	ids := make([]int64, len(pick))
	for i, g := range pick {
		ids[i] = g.ID
	}
	cutter := &PersonCutAction{WorkspaceRepo: wsRepo, VideoRepo: vidRepo, Detection: detRepo,
		Runner: &CutExecuteAction{WorkspaceRepo: wsRepo, VideoRepo: vidRepo, Media: media, Logger: logger}}
	cut, err := cutter.Execute(ctx, "e2e-cut", StartPeopleCutInput{WorkspaceID: load.WorkspaceID, GroupIDs: ids,
		Name: "e2e-" + time.Now().Format("150405"), SecondsBefore: 3, SecondsAfter: 3}, nil)
	if err != nil {
		t.Fatal(err)
	}
	vidByID := map[int64]float64{}
	for _, v := range videos {
		vidByID[v.ID] = v.DurationSeconds
	}
	for i, out := range cut.PerSourceOutputs {
		got := probeDuration(t, out.Path)
		var want float64
		g := pick[i]
		for _, tr := range g.Tracks {
			lo, hi := math.Max(0, tr.StartSec-3), math.Min(vidByID[tr.VideoID], tr.EndSec+3)
			want += hi - lo // upper bound: overlapping windows in a video merge, so got <= want
		}
		t.Logf("CUT %s duration=%.1fs expected<=%.1fs (tracks=%d, own duration %.1fs)", out.Path, got, want, len(g.Tracks), g.TotalDurationSec)
		if got <= 0 {
			t.Errorf("clip %s has no duration", out.Path)
		}
	}
}
