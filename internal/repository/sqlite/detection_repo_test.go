package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"trilhasbr-video-cutter/internal/domain"
)

func setupDetection(t *testing.T) (context.Context, *sql.DB, *DetectionRepository, int64, []domain.Video) {
	t.Helper()
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = RunMigrations(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err = RunMigrations(ctx, db); err != nil { // idempotent
		t.Fatal(err)
	}
	dir := t.TempDir()
	ws, err := NewWorkspaceRepository(db).UpsertByDirectory(ctx, dir, "ws")
	if err != nil {
		t.Fatal(err)
	}
	vr := NewVideoRepository(db)
	if err = vr.ReplaceWorkspaceVideos(ctx, ws.ID, []domain.VideoInsert{
		{Name: "a", Path: filepath.Join(dir, "a.mp4"), DurationSeconds: 60},
		{Name: "b", Path: filepath.Join(dir, "b.mp4"), DurationSeconds: 60},
	}); err != nil {
		t.Fatal(err)
	}
	videos, err := vr.ListByWorkspaceAll(ctx, ws.ID)
	if err != nil || len(videos) != 2 {
		t.Fatalf("videos: %v %d", err, len(videos))
	}
	return ctx, db, NewDetectionRepository(db), ws.ID, videos
}

func count(t *testing.T, db *sql.DB, q string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(q).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestDetectionRepository_ReplaceAndCascade(t *testing.T) {
	ctx, db, repo, wsID, videos := setupDetection(t)
	emb := make([]float32, domain.EmbeddingDim)
	emb[3] = 0.5
	tracks := []domain.Track{
		{StartSec: 0, EndSec: 10, AvgConfidence: 0.9, DetectionCount: 10, Embedding: emb},
		{StartSec: 20, EndSec: 22, AvgConfidence: 0.4, DetectionCount: 5},
	}
	out, err := repo.ReplaceTracksForVideo(ctx, videos[0].ID, tracks)
	if err != nil || len(out) != 2 || out[0].ID == 0 {
		t.Fatalf("replace: %v %+v", err, out)
	}
	groups, total, err := repo.ListGroups(ctx, domain.GroupFilter{WorkspaceID: wsID})
	if err != nil || total != 2 || len(groups) != 2 {
		t.Fatalf("list: %v total=%d", err, total)
	}
	if groups[0].Tracks[0].VideoName != "a" {
		t.Fatalf("track roundtrip: %+v", groups[0].Tracks[0])
	}
	var storedEmbedding []byte
	if err := db.QueryRow(`SELECT embedding FROM tracks WHERE id = ?`, out[0].ID).Scan(&storedEmbedding); err != nil {
		t.Fatal(err)
	}
	decoded := decodeEmbedding(storedEmbedding)
	if len(decoded) != domain.EmbeddingDim || decoded[3] != 0.5 {
		t.Fatalf("embedding roundtrip: len=%d [3]=%v", len(decoded), decoded[3])
	}

	// Re-run on the same video replaces tracks and drops old groups.
	if _, err = repo.ReplaceTracksForVideo(ctx, videos[0].ID, tracks[:1]); err != nil {
		t.Fatal(err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM tracks`); n != 1 {
		t.Fatalf("tracks=%d", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM person_groups`); n != 1 {
		t.Fatalf("groups=%d (empty groups must be removed)", n)
	}

	// Bad embedding length is rejected and leaves data untouched.
	if _, err = repo.ReplaceTracksForVideo(ctx, videos[0].ID, []domain.Track{{Embedding: []float32{1}}}); err == nil {
		t.Fatal("expected embedding validation error")
	}
	if n := count(t, db, `SELECT COUNT(*) FROM tracks`); n != 1 {
		t.Fatalf("tracks changed after failed replace: %d", n)
	}

	// Deleting a video cascades to tracks, links and (via sweep) groups.
	if _, err = db.Exec(`DELETE FROM videos WHERE id = ?`, videos[0].ID); err != nil {
		t.Fatal(err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM tracks`) + count(t, db, `SELECT COUNT(*) FROM person_group_tracks`); n != 0 {
		t.Fatalf("cascade left %d rows", n)
	}
}

func TestDetectionRepository_MergeSplitRemoveFilters(t *testing.T) {
	ctx, db, repo, wsID, videos := setupDetection(t)
	if _, err := repo.ReplaceTracksForVideo(ctx, videos[0].ID, []domain.Track{
		{StartSec: 0, EndSec: 10, AvgConfidence: 0.9, DetectionCount: 30},
		{StartSec: 30, EndSec: 31, AvgConfidence: 0.3, DetectionCount: 10},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReplaceTracksForVideo(ctx, videos[1].ID, []domain.Track{
		{StartSec: 5, EndSec: 15, AvgConfidence: 0.7, DetectionCount: 10},
	}); err != nil {
		t.Fatal(err)
	}

	list := func(f domain.GroupFilter) []domain.PersonGroup {
		f.WorkspaceID = wsID
		g, _, err := repo.ListGroups(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		return g
	}
	if n := len(list(domain.GroupFilter{MinDurationSec: 5})); n != 2 {
		t.Fatalf("min duration filter: %d", n)
	}
	if n := len(list(domain.GroupFilter{MinConfidence: 0.5})); n != 2 {
		t.Fatalf("min confidence filter: %d", n)
	}
	all := list(domain.GroupFilter{})
	if len(all) != 3 {
		t.Fatalf("groups=%d", len(all))
	}

	// Merge the two 10 s tracks (different videos): duration sum, weighted confidence, 2 videos.
	target, err := repo.MergeGroups(ctx, []int64{all[2].ID, all[0].ID})
	if err != nil || target != all[0].ID {
		t.Fatalf("merge: %v target=%d", err, target)
	}
	merged := list(domain.GroupFilter{MinDurationSec: 20})
	if len(merged) != 1 || merged[0].TotalDurationSec != 20 || merged[0].VideoCount != 2 {
		t.Fatalf("merged: %+v", merged)
	}
	if want := (0.9*30 + 0.7*10) / 40; merged[0].AvgConfidence < want-1e-9 || merged[0].AvgConfidence > want+1e-9 {
		t.Fatalf("weighted confidence %v want %v", merged[0].AvgConfidence, want)
	}
	if _, err = repo.MergeGroups(ctx, []int64{target}); err == nil {
		t.Fatal("merge of one group must fail")
	}

	// Split one track back out.
	newID, err := repo.SplitTrack(ctx, target, merged[0].Tracks[1].ID)
	if err != nil || newID == target {
		t.Fatalf("split: %v", err)
	}
	if n := len(list(domain.GroupFilter{})); n != 3 {
		t.Fatalf("after split groups=%d", n)
	}
	if _, err = repo.SplitTrack(ctx, target, 99999); err == nil {
		t.Fatal("split of foreign track must fail")
	}

	// Remove hides the track and drops its now-empty group.
	if err = repo.RemoveTrack(ctx, merged[0].Tracks[1].ID); err != nil {
		t.Fatal(err)
	}
	if n := len(list(domain.GroupFilter{})); n != 2 {
		t.Fatalf("after remove groups=%d", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM tracks WHERE removed = 1`); n != 1 {
		t.Fatalf("removed=%d", n)
	}
	if err = repo.RemoveTrack(ctx, 99999); err == nil {
		t.Fatal("remove unknown track must fail")
	}

	// Pagination keeps total.
	g, total, err := repo.ListGroups(ctx, domain.GroupFilter{WorkspaceID: wsID, Page: 2, PageSize: 1})
	if err != nil || total != 2 || len(g) != 1 {
		t.Fatalf("paging: %v total=%d len=%d", err, total, len(g))
	}
}

func TestDetectionRepository_RunsAndAnalyses(t *testing.T) {
	ctx, _, repo, wsID, videos := setupDetection(t)
	if _, err := repo.CreateRun(ctx, wsID, 20, "v1", 2); err == nil {
		t.Fatal("sample_fps 20 must violate CHECK (1-15)")
	}
	run, err := repo.CreateRun(ctx, wsID, 5, "v1", 2)
	if err != nil || run.Status != domain.DetectionRunning {
		t.Fatalf("create run: %v", err)
	}
	if err = repo.FinishRun(ctx, run.ID, domain.DetectionCancelled, 1, 4); err != nil {
		t.Fatal(err)
	}
	if a, err := repo.GetVideoAnalysis(ctx, videos[0].ID); err != nil || a != nil {
		t.Fatalf("expected nil analysis, got %v %v", a, err)
	}
	a := domain.VideoAnalysis{VideoID: videos[0].ID, SampleFPS: 5, ModelVersion: "v1", FileSize: 10, FileMTime: 100, Status: domain.AnalysisDone}
	if err = repo.UpsertVideoAnalysis(ctx, a); err != nil {
		t.Fatal(err)
	}
	a.FileSize, a.Status, a.Error = 11, domain.AnalysisFailed, "boom"
	if err = repo.UpsertVideoAnalysis(ctx, a); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetVideoAnalysis(ctx, videos[0].ID)
	if err != nil || got == nil || *got != a {
		t.Fatalf("analysis roundtrip: %v %+v", err, got)
	}
	a.Status = "BOGUS"
	if err = repo.UpsertVideoAnalysis(ctx, a); err == nil {
		t.Fatal("status CHECK must reject BOGUS")
	}
}

func TestDetectionRepository_VideoTrackBoxes(t *testing.T) {
	ctx, _, repo, _, videos := setupDetection(t)
	pathA := []domain.TrackBox{{T: 0, X1: 0.1, Y1: 0.2, X2: 0.3, Y2: 0.9}, {T: 0.2, X1: 0.12, Y1: 0.2, X2: 0.32, Y2: 0.9}}
	pathB := []domain.TrackBox{{T: 5, X1: 0.5, Y1: 0.1, X2: 0.7, Y2: 0.8}}
	out, err := repo.ReplaceTracksForVideo(ctx, videos[0].ID, []domain.Track{
		{StartSec: 5, EndSec: 6, AvgConfidence: 0.8, DetectionCount: 1, Boxes: pathB},
		{StartSec: 0, EndSec: 1, AvgConfidence: 0.9, DetectionCount: 2, Boxes: pathA},
		{StartSec: 9, EndSec: 10, AvgConfidence: 0.7, DetectionCount: 1}, // no path: must be skipped
		{StartSec: 11, EndSec: 12, AvgConfidence: 0.7, DetectionCount: 1, Boxes: pathB},
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := repo.ListVideoTrackBoxes(ctx, videos[0].ID)
	if err != nil || len(got) != 3 {
		t.Fatalf("list: %v %+v", err, got)
	}
	if got[0].TrackID != out[1].ID || got[1].TrackID != out[0].ID { // sorted by start_sec
		t.Fatalf("order: %+v", got)
	}
	if len(got[0].Boxes) != 2 || got[0].Boxes[1] != pathA[1] {
		t.Fatalf("boxes round trip: %+v", got[0].Boxes)
	}
	if got[0].GroupID == 0 || got[0].GroupID == got[1].GroupID {
		t.Fatalf("each track starts in its own group: %+v", got)
	}

	// Merging changes the reported group id; removing hides the track.
	merged, err := repo.MergeGroups(ctx, []int64{got[0].GroupID, got[1].GroupID})
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.RemoveTrack(ctx, out[3].ID); err != nil {
		t.Fatal(err)
	}
	got, err = repo.ListVideoTrackBoxes(ctx, videos[0].ID)
	if err != nil || len(got) != 2 || got[0].GroupID != merged || got[1].GroupID != merged {
		t.Fatalf("after merge/remove: %v %+v (merged=%d)", err, got, merged)
	}

	// Other video: empty, never nil.
	none, err := repo.ListVideoTrackBoxes(ctx, videos[1].ID)
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("empty video: %v %#v", err, none)
	}
}

func TestDecodeBoxesRejectsPartialRows(t *testing.T) {
	if _, err := decodeBoxes(make([]byte, 4*boxFloats+4)); err == nil {
		t.Fatal("want error for length that is not a multiple of 5 floats")
	}
}

func TestDetectionRepository_GroupLabels(t *testing.T) {
	ctx, db, repo, wsID, videos := setupDetection(t)
	path := []domain.TrackBox{{T: 0, X1: 0.1, Y1: 0.2, X2: 0.3, Y2: 0.9}}
	out, err := repo.ReplaceTracksForVideo(ctx, videos[0].ID, []domain.Track{
		{StartSec: 0, EndSec: 5, AvgConfidence: 0.9, DetectionCount: 5, Boxes: path},
		{StartSec: 10, EndSec: 15, AvgConfidence: 0.9, DetectionCount: 5, Boxes: path},
		{StartSec: 20, EndSec: 25, AvgConfidence: 0.9, DetectionCount: 5, Boxes: path},
	})
	if err != nil {
		t.Fatal(err)
	}
	boxes, err := repo.ListVideoTrackBoxes(ctx, videos[0].ID)
	if err != nil || len(boxes) != 3 {
		t.Fatalf("boxes: %v %+v", err, boxes)
	}
	g1, g2, g3 := boxes[0].GroupID, boxes[1].GroupID, boxes[2].GroupID
	labelOf := func(groupID int64) string {
		t.Helper()
		groups, _, listErr := repo.ListGroups(ctx, domain.GroupFilter{WorkspaceID: wsID})
		if listErr != nil {
			t.Fatal(listErr)
		}
		for _, g := range groups {
			if g.ID == groupID {
				return g.Label
			}
		}
		t.Fatalf("group %d not listed", groupID)
		return ""
	}

	// set, list, and the overlay query both return the name; unnamed is "".
	if err = repo.SetGroupLabel(ctx, g2, "Ana"); err != nil {
		t.Fatal(err)
	}
	if labelOf(g1) != "" || labelOf(g2) != "Ana" {
		t.Fatalf("labels: g1=%q g2=%q", labelOf(g1), labelOf(g2))
	}
	boxes, _ = repo.ListVideoTrackBoxes(ctx, videos[0].ID)
	if boxes[0].Label != "" || boxes[1].Label != "Ana" {
		t.Fatalf("boxes labels: %+v", boxes)
	}

	// clear stores NULL; unknown group is a validation error.
	if err = repo.SetGroupLabel(ctx, g2, ""); err != nil {
		t.Fatal(err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM person_groups WHERE label IS NULL`); n != 3 {
		t.Fatalf("want all 3 labels NULL after clear, got %d", n)
	}
	if err = repo.SetGroupLabel(ctx, 99999, "x"); err == nil {
		t.Fatal("unknown group must fail")
	}

	// merge: target without a name adopts the first name (lowest id first).
	_ = repo.SetGroupLabel(ctx, g2, "Ana")
	_ = repo.SetGroupLabel(ctx, g3, "Bia")
	target, err := repo.MergeGroups(ctx, []int64{g3, g1, g2})
	if err != nil || target != g1 {
		t.Fatalf("merge: %v target=%d want %d", err, target, g1)
	}
	if got := labelOf(target); got != "Ana" {
		t.Fatalf("merged label = %q, want Ana", got)
	}

	// split: new group is unnamed, the original keeps its name.
	newID, err := repo.SplitTrack(ctx, target, out[2].ID)
	if err != nil {
		t.Fatal(err)
	}
	if labelOf(target) != "Ana" || labelOf(newID) != "" {
		t.Fatalf("split labels: original=%q new=%q", labelOf(target), labelOf(newID))
	}

	// merge: a named target keeps its own name over lower-priority names.
	_ = repo.SetGroupLabel(ctx, newID, "Zed")
	target, err = repo.MergeGroups(ctx, []int64{target, newID})
	if err != nil || labelOf(target) != "Ana" {
		t.Fatalf("named target must keep its name: %v %q", err, labelOf(target))
	}
}

func TestListGroupsFiltersSelectedIDs(t *testing.T) {
	ctx, _, repo, workspace, videos := setupDetection(t)
	_, err := repo.ReplaceTracksForVideo(ctx, videos[0].ID, []domain.Track{{StartSec: 1, EndSec: 2}, {StartSec: 3, EndSec: 4}})
	if err != nil {
		t.Fatal(err)
	}
	groups, _, err := repo.ListGroups(ctx, domain.GroupFilter{WorkspaceID: workspace})
	if err != nil || len(groups) != 2 {
		t.Fatalf("groups: %v %v", groups, err)
	}
	selected, total, err := repo.ListGroups(ctx, domain.GroupFilter{WorkspaceID: workspace, GroupIDs: []int64{groups[1].ID, groups[1].ID, -1}})
	if err != nil || total != 1 || len(selected) != 1 || selected[0].ID != groups[1].ID {
		t.Fatalf("selected: %v total %d error %v", selected, total, err)
	}
	other, total, err := repo.ListGroups(ctx, domain.GroupFilter{WorkspaceID: workspace + 999, GroupIDs: []int64{groups[1].ID}})
	if err != nil || total != 0 || len(other) != 0 {
		t.Fatalf("unexpected cross-workspace result: %v %d %v", other, total, err)
	}
}
