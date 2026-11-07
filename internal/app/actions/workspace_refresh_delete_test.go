package actions

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/domain"
	"trilhasbr-video-cutter/internal/media/ffmpeg"
	"trilhasbr-video-cutter/internal/repository/sqlite"
)

// refreshProbe succeeds with a fixed duration unless the path is in fail.
type refreshProbe struct{ fail map[string]bool }

func (p *refreshProbe) Probe(_ context.Context, path string) (ffmpeg.ProbeInfo, error) {
	if p.fail[path] {
		return ffmpeg.ProbeInfo{}, errors.New("probe failed")
	}
	return ffmpeg.ProbeInfo{DurationSec: 10}, nil
}

type refreshEnv struct {
	ctx    context.Context
	db     *sql.DB
	dir    string
	ws     domain.Workspace
	probe  *refreshProbe
	load   *WorkspaceLoadAction
	videos *sqlite.VideoRepository
	detect *sqlite.DetectionRepository
	wsRepo *sqlite.WorkspaceRepository
}

func setupRefresh(t *testing.T, files ...string) *refreshEnv {
	t.Helper()
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
	for _, f := range files {
		mustWriteFile(t, filepath.Join(dir, f))
	}
	e := &refreshEnv{ctx: ctx, db: db, dir: dir, probe: &refreshProbe{fail: map[string]bool{}},
		videos: sqlite.NewVideoRepository(db), detect: sqlite.NewDetectionRepository(db), wsRepo: sqlite.NewWorkspaceRepository(db)}
	e.ws, err = e.wsRepo.UpsertByDirectory(ctx, dir, "ws")
	if err != nil {
		t.Fatal(err)
	}
	e.load = &WorkspaceLoadAction{WorkspaceRepo: e.wsRepo, VideoRepo: e.videos, MediaProbe: e.probe}
	if _, err = e.load.Refresh(ctx, e.ws.ID); err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *refreshEnv) videoIDs(t *testing.T) map[string]int64 {
	t.Helper()
	all, err := e.videos.ListByWorkspaceAll(e.ctx, e.ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]int64{}
	for _, v := range all {
		ids[filepath.Base(v.Path)] = v.ID
	}
	return ids
}

func (e *refreshEnv) addTrack(t *testing.T, videoID int64) {
	t.Helper()
	if _, err := e.detect.ReplaceTracksForVideo(e.ctx, videoID, []domain.Track{{VideoID: videoID, StartSec: 1, EndSec: 5, AvgConfidence: 0.9, DetectionCount: 4}}); err != nil {
		t.Fatal(err)
	}
}

func (e *refreshEnv) groups(t *testing.T) int {
	t.Helper()
	_, total, err := e.detect.ListGroups(e.ctx, domain.GroupFilter{WorkspaceID: e.ws.ID, Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	return total
}

func TestWorkspaceRefresh_KeepsIDsAndDetectionsRemovesVanished(t *testing.T) {
	e := setupRefresh(t, "a.mp4", "b.mp4")
	before := e.videoIDs(t)
	e.addTrack(t, before["a.mp4"])
	e.addTrack(t, before["b.mp4"])

	// b vanishes from disk, c appears, exports must stay ignored.
	if err := os.Remove(filepath.Join(e.dir, "b.mp4")); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(e.dir, "c.mp4"))
	mustWriteFile(t, filepath.Join(e.dir, "exports", "out.mp4"))

	res, err := e.load.Refresh(e.ctx, e.ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalScanned != 2 || res.Indexed != 2 {
		t.Fatalf("want 2 scanned/indexed (a, c), got %+v", res)
	}
	after := e.videoIDs(t)
	if len(after) != 2 || after["a.mp4"] != before["a.mp4"] || after["c.mp4"] == 0 {
		t.Fatalf("a keeps id, b gone, c new: before=%v after=%v", before, after)
	}
	if got := e.groups(t); got != 1 {
		t.Fatalf("a's detection must survive, b's must cascade away; groups=%d", got)
	}
}

func TestWorkspaceRefresh_MissingDirectoryKeepsData(t *testing.T) {
	e := setupRefresh(t, "a.mp4")
	e.addTrack(t, e.videoIDs(t)["a.mp4"])
	if err := os.RemoveAll(e.dir); err != nil {
		t.Fatal(err)
	}

	_, err := e.load.Refresh(e.ctx, e.ws.ID)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeIO {
		t.Fatalf("want IO_ERROR, got %v", err)
	}
	if len(e.videoIDs(t)) != 1 || e.groups(t) != 1 {
		t.Fatal("videos and detections must be kept when the directory is unavailable")
	}
}

func TestWorkspaceRefresh_ProbeFailureKeepsExistingVideo(t *testing.T) {
	e := setupRefresh(t, "a.mp4", "b.mp4")
	before := e.videoIDs(t)
	e.addTrack(t, before["a.mp4"])
	e.probe.fail[filepath.Join(e.dir, "a.mp4")] = true

	res, err := e.load.Refresh(e.ctx, e.ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 1 || res.Indexed != 1 {
		t.Fatalf("want 1 failure, 1 indexed: %+v", res)
	}
	after := e.videoIDs(t)
	if after["a.mp4"] != before["a.mp4"] || e.groups(t) != 1 {
		t.Fatalf("transient probe failure must not drop a.mp4 or its detections: %v", after)
	}
}

func TestWorkspaceRefresh_GuardRefuses(t *testing.T) {
	e := setupRefresh(t, "a.mp4")
	e.load.Guard = WorkspaceBusyGuard(func(int64) bool { return true })
	if _, err := e.load.Refresh(e.ctx, e.ws.ID); err == nil {
		t.Fatal("refresh must be refused while the workspace is busy")
	}
}

func TestWorkspaceDelete_CascadesAndOnlyRemovesAppArtifacts(t *testing.T) {
	e := setupRefresh(t, "a.mp4")
	e.addTrack(t, e.videoIDs(t)["a.mp4"])
	if _, err := e.detect.CreateRun(e.ctx, e.ws.ID, 5, "v", 1); err != nil {
		t.Fatal(err)
	}

	appData := t.TempDir()
	people := filepath.Join(appData, "people")
	thumbs := filepath.Join(appData, "thumbnails")
	id := strconv.FormatInt(e.ws.ID, 10)
	mine := filepath.Join(people, id)
	others := filepath.Join(people, "999")
	for _, p := range []string{filepath.Join(mine, "x.jpg"), filepath.Join(others, "y.jpg"), filepath.Join(thumbs, id, "t.jpg")} {
		mustWriteFile(t, p)
	}
	del := &WorkspaceDeleteAction{WorkspaceRepo: e.wsRepo, ArtifactRoots: []string{thumbs, people}}

	if err := del.Execute(e.ctx, e.ws.ID); err != nil {
		t.Fatal(err)
	}

	for _, table := range []string{"workspaces", "videos", "detection_runs", "video_analyses", "tracks", "person_groups", "person_group_tracks"} {
		var n int
		if err := e.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil || n != 0 {
			t.Fatalf("table %s must be empty after cascade (n=%d err=%v)", table, n, err)
		}
	}
	for _, gone := range []string{mine, filepath.Join(thumbs, id)} {
		if _, err := os.Stat(gone); !os.IsNotExist(err) {
			t.Fatalf("%s should have been removed", gone)
		}
	}
	for _, kept := range []string{others, filepath.Join(e.dir, "a.mp4"), e.dir} {
		if _, err := os.Stat(kept); err != nil {
			t.Fatalf("%s must be kept: %v", kept, err)
		}
	}
}

func TestWorkspaceDelete_GuardAndUnknownWorkspace(t *testing.T) {
	e := setupRefresh(t, "a.mp4")
	del := &WorkspaceDeleteAction{WorkspaceRepo: e.wsRepo, Guard: WorkspaceBusyGuard(func(int64) bool { return true })}
	if err := del.Execute(e.ctx, e.ws.ID); err == nil {
		t.Fatal("delete must be refused while detection is running")
	}
	if _, err := e.wsRepo.GetByID(e.ctx, e.ws.ID); err != nil {
		t.Fatal("workspace must still exist after a refused delete")
	}
	del.Guard = nil
	if err := del.Execute(e.ctx, 424242); err == nil {
		t.Fatal("deleting an unknown workspace must error")
	}
}

type missingFFmpegProbe struct{}

func (missingFFmpegProbe) Probe(context.Context, string) (ffmpeg.ProbeInfo, error) {
	return ffmpeg.ProbeInfo{}, apperrors.New(apperrors.CodeFFmpegNotFound, "ffmpeg/ffprobe binaries were not found", nil)
}

func TestRefreshReportsMissingFFmpegAndKeepsVideos(t *testing.T) {
	e := setupRefresh(t, "a.mp4", "b.mp4")
	before := e.videoIDs(t)

	broken := &WorkspaceLoadAction{WorkspaceRepo: e.wsRepo, VideoRepo: e.videos, MediaProbe: missingFFmpegProbe{}}
	_, err := broken.Refresh(e.ctx, e.ws.ID)

	if appErr := apperrors.As(err); appErr == nil || appErr.Code != apperrors.CodeFFmpegNotFound {
		t.Fatalf("want FFMPEG_NOT_FOUND, got %v", err)
	}
	after := e.videoIDs(t)
	if len(after) != len(before) {
		t.Fatalf("videos changed: before=%v after=%v", before, after)
	}
}
