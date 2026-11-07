package actions

import (
	"context"
	"time"

	"trilhasbr-video-cutter/internal/repository"
)

type WorkspaceListAction struct {
	WorkspaceRepo repository.WorkspaceRepository
}

type WorkspaceSummaryResult struct {
	ID                 int64
	Directory          string
	Name               string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	VideoCount         int
	NullStartedAtCount int
}

func (a *WorkspaceListAction) Execute(ctx context.Context) ([]WorkspaceSummaryResult, error) {
	workspaces, err := a.WorkspaceRepo.ListWithStats(ctx)
	if err != nil {
		return nil, err
	}

	results := make([]WorkspaceSummaryResult, 0, len(workspaces))
	for _, workspace := range workspaces {
		results = append(results, WorkspaceSummaryResult{
			ID:                 workspace.ID,
			Directory:          workspace.Directory,
			Name:               workspace.Name,
			CreatedAt:          workspace.CreatedAt,
			UpdatedAt:          workspace.UpdatedAt,
			VideoCount:         workspace.VideoCount,
			NullStartedAtCount: workspace.NullStartedAtCount,
		})
	}
	return results, nil
}
