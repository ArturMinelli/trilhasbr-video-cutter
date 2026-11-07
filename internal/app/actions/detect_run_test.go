package actions

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"

	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/domain"
	"trilhasbr-video-cutter/internal/inference"
	"trilhasbr-video-cutter/internal/media/ffmpeg"
	"trilhasbr-video-cutter/internal/platform"
	"trilhasbr-video-cutter/internal/repository/sqlite"
)

var workersForTest = 1

// fakeEngine reports one steady person on every frame.
type fakeEngine struct{ detects atomic.Int64 }

func (e *fakeEngine) Detect(f inference.Frame) ([]inference.Box, error) {
	e.detects.Add(1)
	return []inference.Box{{X1: 40, Y1: 60, X2: 160, Y2: 360, Conf: 0.9}}, nil
}

func (e *fakeEngine) Embed(crops []inference.Frame) ([][]float32, error) {
	out := make([][]float32, len(crops))
	for i := range out {
		out[i] = make([]float32, inference.EmbeddingDim)
		out[i][0] = 1
	}
	return out, nil
}

func (e *fakeEngine) Close() error { return nil }

type detectEnv struct {
	ctx    context.Context
	db     *sql.DB
	action *DetectRunAction
	engine *fakeEngine
	wsID   int64
	videos []domain.Video
}

func setupDetect(t *testing.T, names ...string) *detectEnv {
	t.Helper()
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = sqlite.RunMigrations(ctx, db); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	ws, err := sqlite.NewWorkspaceRepository(db).UpsertByDirectory(ctx, dir, "ws")
	if err != nil {
		t.Fatal(err)
	}
	inserts := make([]domain.VideoInsert, len(names))
	for i, n := range names {
		p := filepath.Join(dir, n+".mp4")
		if n != "missing" {
			out, cerr := exec.Command(ffmpegPath, "-v", "error", "-f", "lavfi", "-i", "testsrc=size=320x480:rate=10:duration=3",
				"-pix_fmt", "yuv420p", "-y", p).CombinedOutput()
			if cerr != nil {
				t.Fatalf("generate video: %v %s", cerr, out)
			}
		}
		inserts[i] = domain.VideoInsert{Name: n, Path: p, DurationSeconds: 3}
	}
	vr := sqlite.NewVideoRepository(db)
	if err = vr.ReplaceWorkspaceVideos(ctx, ws.ID, inserts); err != nil {
		t.Fatal(err)
	}
	videos, err := vr.ListByWorkspaceAll(ctx, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	eng := &fakeEngine{}
	return &detectEnv{ctx: ctx, db: db, engine: eng, wsID: ws.ID, videos: videos, action: &DetectRunAction{
		WorkspaceRepo: sqlite.NewWorkspaceRepository(db),
		VideoRepo:     vr,
		DetectionRepo: sqlite.NewDetectionRepository(db),
		Media:         ffmpeg.NewClient(platform.NewFFmpegResolver(""), nil),
		Engine:        eng,
		ModelVersion:  "test-v1",
		ThumbDir:      filepath.Join(t.TempDir(), "people"),
		Workers:       workersForTest,
	}}
}

func (e *detectEnv) count(t *testing.T, q string) int {
	t.Helper()
	var n int
	if err := e.db.QueryRow(q).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestDetectRun_PersistsTracksAndSkipsOnRerun(t *testing.T) {
	env := setupDetect(t, "a", "b")
	res, err := env.action.Execute(env.ctx, "j1", StartDetectionInput{WorkspaceID: env.wsID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != domain.DetectionSucceeded || res.VideosDone != 2 || res.TracksFound != 2 || res.PeopleTotal != 2 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if n := env.count(t, `SELECT COUNT(*) FROM tracks`); n != 2 {
		t.Fatalf("tracks stored: %d", n)
	}
	var thumb string
	if err = env.db.QueryRow(`SELECT thumbnail_path FROM tracks LIMIT 1`).Scan(&thumb); err != nil || thumb == "" {
		t.Fatalf("thumbnail path missing: %q %v", thumb, err)
	}
	if _, err = os.Stat(thumb); err != nil {
		t.Fatalf("thumbnail file missing: %v", err)
	}

	before := env.engine.detects.Load()
	res, err = env.action.Execute(env.ctx, "j2", StartDetectionInput{WorkspaceID: env.wsID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.VideosSkipped != 2 || env.engine.detects.Load() != before {
		t.Fatalf("expected both videos skipped without inference: %+v", res)
	}
	if res.PeopleTotal != 2 {
		t.Fatalf("people total after skip: %d", res.PeopleTotal)
	}

	// Changing fps changes the skip key: videos are re-analyzed and old tracks replaced, not duplicated.
	res, err = env.action.Execute(env.ctx, "j3", StartDetectionInput{WorkspaceID: env.wsID, SampleFPS: 2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.VideosSkipped != 0 || env.engine.detects.Load() == before {
		t.Fatalf("expected re-analysis at new fps: %+v", res)
	}
	if n := env.count(t, `SELECT COUNT(*) FROM tracks`); n != 2 {
		t.Fatalf("tracks after re-analysis: %d", n)
	}
}

func TestDetectRun_InvalidFPS(t *testing.T) {
	env := setupDetect(t, "a")
	for _, fps := range []float64{0.5, 16} {
		_, err := env.action.Execute(env.ctx, "j", StartDetectionInput{WorkspaceID: env.wsID, SampleFPS: fps}, nil)
		if apperrors.As(err).Code != apperrors.CodeValidation {
			t.Fatalf("fps %v: expected validation error, got %v", fps, err)
		}
	}
}

func TestDetectRun_FailingVideoDoesNotAbortOthers(t *testing.T) {
	env := setupDetect(t, "a", "missing", "b")
	res, err := env.action.Execute(env.ctx, "j", StartDetectionInput{WorkspaceID: env.wsID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != domain.DetectionSucceeded || res.VideosFailed != 1 || res.VideosDone != 3 || res.TracksFound != 2 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if len(res.Failures) != 1 || res.Failures[0].Name != "missing" {
		t.Fatalf("failures: %+v", res.Failures)
	}
	if n := env.count(t, `SELECT COUNT(*) FROM video_analyses WHERE status = 'FAILED'`); n != 1 {
		t.Fatalf("failed analyses recorded: %d", n)
	}
}

func TestDetectRun_CancelKeepsFinishedVideos(t *testing.T) {
	env := setupDetect(t, "a", "b", "c")
	ctx, cancel := context.WithCancel(env.ctx)
	defer cancel()
	res, err := env.action.Execute(ctx, "j", StartDetectionInput{WorkspaceID: env.wsID}, func(e DetectProgressEvent) {
		if e.VideosDone == 1 {
			cancel() // Workers=1: video 1 is finished, 2 and 3 have not started
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != domain.DetectionCancelled || res.VideosDone != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if n := env.count(t, `SELECT COUNT(*) FROM video_analyses WHERE status = 'DONE'`); n != 1 {
		t.Fatalf("finished analyses kept: %d", n)
	}
	if n := env.count(t, `SELECT COUNT(*) FROM tracks`); n != 1 {
		t.Fatalf("tracks kept: %d", n)
	}
	if n := env.count(t, `SELECT COUNT(*) FROM detection_runs WHERE status = 'CANCELLED'`); n != 1 {
		t.Fatalf("run not closed as CANCELLED")
	}
}

func TestDetectRun_ParallelWorkers(t *testing.T) {
	workersForTest = 3
	defer func() { workersForTest = 1 }()
	env := setupDetect(t, "a", "b", "c", "d", "e")
	res, err := env.action.Execute(env.ctx, "j", StartDetectionInput{WorkspaceID: env.wsID}, func(DetectProgressEvent) {})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != domain.DetectionSucceeded || res.VideosDone != 5 || res.TracksFound != 5 {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestDetectRun_PersistsNormalizedBoxPath(t *testing.T) {
	env := setupDetect(t, "a")
	if _, err := env.action.Execute(env.ctx, "j", StartDetectionInput{WorkspaceID: env.wsID}, nil); err != nil {
		t.Fatal(err)
	}
	got, err := env.action.DetectionRepo.ListVideoTrackBoxes(env.ctx, env.videos[0].ID)
	if err != nil || len(got) != 1 {
		t.Fatalf("list: %v %+v", err, got)
	}
	boxes := got[0].Boxes
	if len(boxes) < 10 { // 3 s at the default 5 fps, one steady person
		t.Fatalf("want a point per sampled frame, got %d", len(boxes))
	}
	for i, b := range boxes {
		for _, v := range []float32{b.X1, b.Y1, b.X2, b.Y2} {
			if v < 0 || v > 1 {
				t.Fatalf("box %d out of 0..1: %+v", i, b)
			}
		}
		if b.X2 <= b.X1 || b.Y2 <= b.Y1 {
			t.Fatalf("box %d degenerate: %+v", i, b)
		}
		if i > 0 && b.T <= boxes[i-1].T {
			t.Fatalf("time not increasing at %d: %v then %v", i, boxes[i-1].T, b.T)
		}
	}
	// Fake engine box (40,60)-(160,360) in the 320x480 frame.
	want := [4]float32{0.125, 0.125, 0.5, 0.75}
	first := boxes[0]
	for i, v := range []float32{first.X1, first.Y1, first.X2, first.Y2} {
		if d := v - want[i]; d > 1e-4 || d < -1e-4 {
			t.Fatalf("first box not normalized by frame size: %+v, want %v", first, want)
		}
	}
}
