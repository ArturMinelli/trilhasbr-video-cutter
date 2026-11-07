package actions

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/repository"
)

// WorkspaceDeleteAction removes a workspace from the app: its database rows (videos, detection runs,
// tracks and groups cascade) and the app-generated artifacts for it.
//
// It NEVER touches the user's video files, the workspace folder, or exported cuts.
type WorkspaceDeleteAction struct {
	WorkspaceRepo repository.WorkspaceRepository
	// ArtifactRoots are app-data directories holding one subdirectory per workspace, named by its numeric ID
	// (people crops, preview thumbnails). Only <root>/<id> is ever removed.
	ArtifactRoots []string
	// Guard (optional) refuses the delete while the workspace is busy (e.g. detection running).
	Guard  func(workspaceID int64) error
	Logger *slog.Logger
}

func (a *WorkspaceDeleteAction) Execute(ctx context.Context, workspaceID int64) error {
	if workspaceID <= 0 {
		return apperrors.New(apperrors.CodeValidation, "invalid workspace id", map[string]any{"workspaceId": workspaceID})
	}
	if _, err := a.WorkspaceRepo.GetByID(ctx, workspaceID); err != nil {
		return err
	}
	if a.Guard != nil {
		if err := a.Guard(workspaceID); err != nil {
			return err
		}
	}
	// Database first: if file cleanup fails we leave orphan files, never a workspace with missing artifacts.
	if err := a.WorkspaceRepo.Delete(ctx, workspaceID); err != nil {
		return err
	}
	for _, root := range a.ArtifactRoots {
		if root == "" {
			continue
		}
		dir := filepath.Join(root, strconv.FormatInt(workspaceID, 10))
		if err := os.RemoveAll(dir); err != nil && a.Logger != nil {
			a.Logger.Warn("failed to remove workspace artifacts", slog.String("dir", dir), slog.String("error", err.Error()))
		}
	}
	return nil
}

// WorkspaceBusyGuard builds a Guard that refuses while detection is active for the workspace.
func WorkspaceBusyGuard(busy func(workspaceID int64) bool) func(int64) error {
	return func(workspaceID int64) error {
		if busy != nil && busy(workspaceID) {
			return apperrors.New(apperrors.CodeValidation, "detection is running for this workspace; cancel it first", map[string]any{"workspaceId": workspaceID})
		}
		return nil
	}
}
