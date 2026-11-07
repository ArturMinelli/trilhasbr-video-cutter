package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/domain"
)

func TestVideoRepository_DeleteWorkspaceExportVideos(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.sqlite")

	db, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	defer db.Close()

	if err = RunMigrations(ctx, db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	workspaceRepo := NewWorkspaceRepository(db)
	videoRepo := NewVideoRepository(db)

	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	workspace, err := workspaceRepo.UpsertByDirectory(ctx, workspaceDir, "workspace")
	if err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	videos := []domain.VideoInsert{
		{Name: "root", Path: filepath.Join(workspaceDir, "root.mp4"), DurationSeconds: 10},
		{Name: "sub", Path: filepath.Join(workspaceDir, "camera", "sub.mp4"), DurationSeconds: 12},
		{Name: "export1", Path: filepath.Join(workspaceDir, "exports", "old.mp4"), DurationSeconds: 8},
		{Name: "export2", Path: filepath.Join(workspaceDir, "nested", "Exports", "old2.mp4"), DurationSeconds: 9},
	}
	if err = videoRepo.ReplaceWorkspaceVideos(ctx, workspace.ID, videos); err != nil {
		t.Fatalf("failed to seed videos: %v", err)
	}

	deleted, err := videoRepo.DeleteWorkspaceExportVideos(ctx, workspace.ID, workspaceDir)
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("expected 2 deleted rows, got %d", deleted)
	}

	remaining, err := videoRepo.ListByWorkspaceAll(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("failed listing remaining videos: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("expected 2 remaining videos, got %d", len(remaining))
	}
	for _, video := range remaining {
		if pathContainsExportsSegment(video.Path, workspaceDir) {
			t.Fatalf("found export video after cleanup: %s", video.Path)
		}
	}
}

func TestPathContainsExportsSegment_IgnoresParentExportsInWorkspacePath(t *testing.T) {
	workspaceDir := filepath.Join(string(filepath.Separator), "tmp", "exports", "workspace")
	videoPath := filepath.Join(workspaceDir, "camera", "clip.mp4")
	if pathContainsExportsSegment(videoPath, workspaceDir) {
		t.Fatalf("expected false for path %q in workspace %q", videoPath, workspaceDir)
	}
}

func TestHasExportsSegment_AcceptsMixedPathSeparators(t *testing.T) {
	if !hasExportsSegment(`camera\Exports\clip.mp4`) {
		t.Fatal("expected exports segment detection for windows-style path")
	}
}

func TestVideoRepository_ShiftVideoTimes(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "shift.sqlite")

	db, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	defer db.Close()

	if err = RunMigrations(ctx, db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	workspaceRepo := NewWorkspaceRepository(db)
	videoRepo := NewVideoRepository(db)

	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	workspace, err := workspaceRepo.UpsertByDirectory(ctx, workspaceDir, "workspace")
	if err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	startA := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	finishA := startA.Add(45 * time.Second)
	startB := startA.Add(2 * time.Minute)
	finishB := startB.Add(30 * time.Second)

	seed := []domain.VideoInsert{
		{Name: "a", Path: filepath.Join(workspaceDir, "a.mp4"), StartedAt: &startA, FinishedAt: &finishA, DurationSeconds: 45},
		{Name: "b", Path: filepath.Join(workspaceDir, "b.mp4"), StartedAt: &startB, FinishedAt: &finishB, DurationSeconds: 30},
	}
	if err = videoRepo.ReplaceWorkspaceVideos(ctx, workspace.ID, seed); err != nil {
		t.Fatalf("failed to seed videos: %v", err)
	}

	current, err := videoRepo.ListByWorkspaceAll(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("failed to list videos: %v", err)
	}
	if len(current) != 2 {
		t.Fatalf("expected 2 videos, got %d", len(current))
	}

	byID := make(map[int64]domain.Video, len(current))
	for _, video := range current {
		byID[video.ID] = video
	}

	shiftBy := 95 * time.Second
	videoIDs := []int64{current[0].ID, current[1].ID}
	updated, err := videoRepo.ShiftVideoTimes(ctx, workspace.ID, videoIDs, shiftBy)
	if err != nil {
		t.Fatalf("shift failed: %v", err)
	}
	if updated != 2 {
		t.Fatalf("expected 2 updated rows, got %d", updated)
	}

	shifted, err := videoRepo.ListByWorkspaceAll(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("failed to list shifted videos: %v", err)
	}
	if len(shifted) != 2 {
		t.Fatalf("expected 2 shifted videos, got %d", len(shifted))
	}

	for _, video := range shifted {
		if video.StartedAt == nil || video.FinishedAt == nil {
			t.Fatalf("video %d missing shifted timestamps", video.ID)
		}
		original := byID[video.ID]
		if original.StartedAt == nil || original.FinishedAt == nil {
			t.Fatalf("video %d missing original timestamps", video.ID)
		}
		if !video.StartedAt.Equal(original.StartedAt.Add(shiftBy)) {
			t.Fatalf("unexpected shifted start for %d: got %s", video.ID, video.StartedAt.UTC().Format(time.RFC3339Nano))
		}
		if !video.FinishedAt.Equal(original.FinishedAt.Add(shiftBy)) {
			t.Fatalf("unexpected shifted finish for %d: got %s", video.ID, video.FinishedAt.UTC().Format(time.RFC3339Nano))
		}
	}
}

func TestVideoRepository_ShiftVideoTimes_RollbackOnMissingTimestamp(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "shift-rollback.sqlite")

	db, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	defer db.Close()

	if err = RunMigrations(ctx, db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	workspaceRepo := NewWorkspaceRepository(db)
	videoRepo := NewVideoRepository(db)

	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	workspace, err := workspaceRepo.UpsertByDirectory(ctx, workspaceDir, "workspace")
	if err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	start := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	finish := start.Add(20 * time.Second)
	seed := []domain.VideoInsert{
		{Name: "valid", Path: filepath.Join(workspaceDir, "valid.mp4"), StartedAt: &start, FinishedAt: &finish, DurationSeconds: 20},
		{Name: "invalid", Path: filepath.Join(workspaceDir, "invalid.mp4"), DurationSeconds: 10},
	}
	if err = videoRepo.ReplaceWorkspaceVideos(ctx, workspace.ID, seed); err != nil {
		t.Fatalf("failed to seed videos: %v", err)
	}

	current, err := videoRepo.ListByWorkspaceAll(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("failed to list videos: %v", err)
	}
	if len(current) != 2 {
		t.Fatalf("expected 2 videos, got %d", len(current))
	}

	_, err = videoRepo.ShiftVideoTimes(ctx, workspace.ID, []int64{current[0].ID, current[1].ID}, 30*time.Second)
	if err == nil {
		t.Fatal("expected error when shifting video with missing timestamps")
	}
	if appErr := apperrors.As(err); appErr.Code != apperrors.CodeValidation {
		t.Fatalf("expected validation error, got %s", appErr.Code)
	}

	afterFailure, err := videoRepo.ListByWorkspaceAll(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("failed to list videos after failed shift: %v", err)
	}
	if len(afterFailure) != 2 {
		t.Fatalf("expected 2 videos after failure, got %d", len(afterFailure))
	}

	validBeforeByID := make(map[int64]domain.Video, len(current))
	for _, video := range current {
		validBeforeByID[video.ID] = video
	}

	for _, video := range afterFailure {
		original := validBeforeByID[video.ID]
		if original.StartedAt == nil || original.FinishedAt == nil {
			if video.StartedAt != nil || video.FinishedAt != nil {
				t.Fatal("expected untouched nil timestamps after rollback")
			}
			continue
		}

		if video.StartedAt == nil || video.FinishedAt == nil {
			t.Fatal("expected original timestamps to remain on valid row")
		}
		if !video.StartedAt.Equal(*original.StartedAt) || !video.FinishedAt.Equal(*original.FinishedAt) {
			t.Fatal("expected shift transaction rollback to preserve original timestamps")
		}
	}
}

func TestReplaceWorkspaceVideosKeepsIDsAndDropsStale(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "keep.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = RunMigrations(ctx, db); err != nil {
		t.Fatal(err)
	}
	videoRepo := NewVideoRepository(db)
	ws, err := NewWorkspaceRepository(db).UpsertByDirectory(ctx, "/tmp/ws-keep", "ws-keep")
	if err != nil {
		t.Fatal(err)
	}
	seed := []domain.VideoInsert{
		{Name: "a.mp4", Path: "/tmp/ws-keep/a.mp4", DurationSeconds: 10},
		{Name: "b.mp4", Path: "/tmp/ws-keep/b.mp4", DurationSeconds: 20},
	}
	if err = videoRepo.ReplaceWorkspaceVideos(ctx, ws.ID, seed); err != nil {
		t.Fatal(err)
	}
	before, _, _ := videoRepo.ListByWorkspace(ctx, ws.ID, 1, 50)
	ids := map[string]int64{}
	for _, v := range before {
		ids[v.Path] = v.ID
	}
	// a stays (new duration), b disappears, c is new.
	next := []domain.VideoInsert{
		{Name: "a.mp4", Path: "/tmp/ws-keep/a.mp4", DurationSeconds: 11},
		{Name: "c.mp4", Path: "/tmp/ws-keep/c.mp4", DurationSeconds: 5},
	}
	if err = videoRepo.ReplaceWorkspaceVideos(ctx, ws.ID, next); err != nil {
		t.Fatal(err)
	}
	after, total, _ := videoRepo.ListByWorkspace(ctx, ws.ID, 1, 50)
	if total != 2 {
		t.Fatalf("want 2 videos, got %d", total)
	}
	for _, v := range after {
		if v.Path == "/tmp/ws-keep/a.mp4" && (v.ID != ids[v.Path] || v.DurationSeconds != 11) {
			t.Fatalf("a.mp4 id/duration not preserved/updated: %+v", v)
		}
		if v.Path == "/tmp/ws-keep/b.mp4" {
			t.Fatal("stale b.mp4 should be deleted")
		}
	}
}
