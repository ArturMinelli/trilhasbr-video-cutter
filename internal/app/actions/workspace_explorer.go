package actions

import (
	"context"
	"path"
	"path/filepath"
	"strings"

	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/domain"
	"trilhasbr-video-cutter/internal/repository"
)

type WorkspaceExplorerAction struct {
	WorkspaceRepo repository.WorkspaceRepository
	VideoRepo     repository.VideoRepository
}

type ExplorerVideo struct {
	Video        domain.Video
	RelativePath string
	RelativeDir  string
}

type WorkspaceExplorerResult struct {
	WorkspaceID        int64
	WorkspaceDirectory string
	Videos             []ExplorerVideo
}

func (a *WorkspaceExplorerAction) Execute(ctx context.Context, workspaceID int64) (WorkspaceExplorerResult, error) {
	if workspaceID <= 0 {
		return WorkspaceExplorerResult{}, apperrors.New(apperrors.CodeValidation, "workspaceId must be greater than 0", nil)
	}
	if a.WorkspaceRepo == nil || a.VideoRepo == nil {
		return WorkspaceExplorerResult{}, apperrors.New(apperrors.CodeInternal, "workspace explorer action is not fully configured", nil)
	}

	workspace, err := a.WorkspaceRepo.GetByID(ctx, workspaceID)
	if err != nil {
		return WorkspaceExplorerResult{}, err
	}

	if _, err = a.VideoRepo.DeleteWorkspaceExportVideos(ctx, workspace.ID, workspace.Directory); err != nil {
		return WorkspaceExplorerResult{}, err
	}

	videos, err := a.VideoRepo.ListByWorkspaceAll(ctx, workspace.ID)
	if err != nil {
		return WorkspaceExplorerResult{}, err
	}

	items := make([]ExplorerVideo, 0, len(videos))
	for _, video := range videos {
		relativePath, relativeDir := buildWorkspaceRelativePaths(workspace.Directory, video.Path)
		items = append(items, ExplorerVideo{
			Video:        video,
			RelativePath: relativePath,
			RelativeDir:  relativeDir,
		})
	}

	return WorkspaceExplorerResult{
		WorkspaceID:        workspace.ID,
		WorkspaceDirectory: workspace.Directory,
		Videos:             items,
	}, nil
}

func buildWorkspaceRelativePaths(workspaceDirectory string, absoluteVideoPath string) (string, string) {
	relativePath := filepath.Base(absoluteVideoPath)

	if rel, err := filepath.Rel(workspaceDirectory, absoluteVideoPath); err == nil {
		cleanRel := filepath.Clean(rel)
		if cleanRel != "." && !strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) && cleanRel != ".." {
			relativePath = cleanRel
		}
	}

	relativePath = strings.TrimPrefix(filepath.ToSlash(relativePath), "./")
	relativeDir := path.Dir(relativePath)
	if relativeDir == "." {
		relativeDir = ""
	}
	return relativePath, relativeDir
}
