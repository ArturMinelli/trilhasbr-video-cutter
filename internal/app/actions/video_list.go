package actions

import (
	"context"

	"trilhasbr-video-cutter/internal/domain"
	"trilhasbr-video-cutter/internal/repository"
)

type VideoListAction struct {
	VideoRepo repository.VideoRepository
}

type VideoListResult struct {
	Videos   []domain.Video
	Page     int
	PageSize int
	Total    int
}

func (a *VideoListAction) Execute(ctx context.Context, workspaceID int64, page int, pageSize int) (VideoListResult, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 25
	}

	videos, total, err := a.VideoRepo.ListByWorkspace(ctx, workspaceID, page, pageSize)
	if err != nil {
		return VideoListResult{}, err
	}

	return VideoListResult{
		Videos:   videos,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}
