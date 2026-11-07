package actions

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"trilhasbr-video-cutter/internal/domain"
)

type workspaceRepoStub struct {
	workspace domain.Workspace
}

func (s *workspaceRepoStub) UpsertByDirectory(context.Context, string, string) (domain.Workspace, error) {
	return s.workspace, nil
}

func (s *workspaceRepoStub) ListWithStats(context.Context) ([]domain.WorkspaceSummary, error) {
	return nil, nil
}

func (s *workspaceRepoStub) GetByID(context.Context, int64) (domain.Workspace, error) {
	return s.workspace, nil
}

type videoRepoStub struct {
	videos             []domain.Video
	cleanupCalled      bool
	cleanupWorkspaceID int64
	cleanupDirectory   string
}

func (s *videoRepoStub) ReplaceWorkspaceVideos(context.Context, int64, []domain.VideoInsert) error {
	return nil
}

func (s *videoRepoStub) ListByWorkspace(context.Context, int64, int, int) ([]domain.Video, int, error) {
	return nil, 0, nil
}

func (s *videoRepoStub) ListByWorkspaceAll(context.Context, int64) ([]domain.Video, error) {
	return s.videos, nil
}

func (s *videoRepoStub) GetByID(context.Context, int64) (domain.Video, error) {
	return domain.Video{}, nil
}

func (s *videoRepoStub) DeleteWorkspaceExportVideos(_ context.Context, workspaceID int64, workspaceDirectory string) (int, error) {
	s.cleanupCalled = true
	s.cleanupWorkspaceID = workspaceID
	s.cleanupDirectory = workspaceDirectory
	return 1, nil
}

func (s *videoRepoStub) FindOverlapping(context.Context, int64, time.Time, time.Time) ([]domain.Video, error) {
	return nil, nil
}

func (s *videoRepoStub) ShiftVideoTimes(context.Context, int64, []int64, time.Duration) (int, error) {
	return 0, nil
}

func TestWorkspaceExplorerAction_ComputesRelativePathsAndRunsCleanup(t *testing.T) {
	workspaceDir := filepath.Join(string(filepath.Separator), "tmp", "workspace")
	workspace := domain.Workspace{ID: 7, Directory: workspaceDir, Name: "workspace"}
	videos := []domain.Video{
		{
			ID:          1,
			WorkspaceID: workspace.ID,
			Name:        "root",
			Path:        filepath.Join(workspaceDir, "root.mp4"),
		},
		{
			ID:          2,
			WorkspaceID: workspace.ID,
			Name:        "nested",
			Path:        filepath.Join(workspaceDir, "camera", "sub", "nested.mp4"),
		},
	}

	videoRepo := &videoRepoStub{videos: videos}
	action := WorkspaceExplorerAction{
		WorkspaceRepo: &workspaceRepoStub{workspace: workspace},
		VideoRepo:     videoRepo,
	}

	result, err := action.Execute(context.Background(), workspace.ID)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}

	if !videoRepo.cleanupCalled {
		t.Fatal("expected cleanup to be called")
	}
	if videoRepo.cleanupWorkspaceID != workspace.ID {
		t.Fatalf("cleanup workspace id mismatch: got %d", videoRepo.cleanupWorkspaceID)
	}
	if videoRepo.cleanupDirectory != workspaceDir {
		t.Fatalf("cleanup directory mismatch: got %s", videoRepo.cleanupDirectory)
	}

	if len(result.Videos) != 2 {
		t.Fatalf("expected 2 explorer videos, got %d", len(result.Videos))
	}

	if result.Videos[0].RelativePath != "root.mp4" {
		t.Fatalf("unexpected root relative path: %s", result.Videos[0].RelativePath)
	}
	if result.Videos[0].RelativeDir != "" {
		t.Fatalf("unexpected root relative dir: %s", result.Videos[0].RelativeDir)
	}

	if result.Videos[1].RelativePath != "camera/sub/nested.mp4" {
		t.Fatalf("unexpected nested relative path: %s", result.Videos[1].RelativePath)
	}
	if result.Videos[1].RelativeDir != "camera/sub" {
		t.Fatalf("unexpected nested relative dir: %s", result.Videos[1].RelativeDir)
	}
}

func (s *workspaceRepoStub) Delete(context.Context, int64) error { return nil }
