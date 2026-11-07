package actions

import (
	"context"
	"time"

	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/repository"
)

type VideoShiftAction struct {
	WorkspaceRepo repository.WorkspaceRepository
	VideoRepo     repository.VideoRepository
}

type ShiftVideosTimeInput struct {
	WorkspaceID       int64
	VideoIDs          []int64
	ShiftMilliseconds int64
}

type ShiftVideosTimeResult struct {
	Updated int
}

func (a *VideoShiftAction) Execute(ctx context.Context, input ShiftVideosTimeInput) (ShiftVideosTimeResult, error) {
	if input.WorkspaceID <= 0 {
		return ShiftVideosTimeResult{}, apperrors.New(apperrors.CodeValidation, "workspaceId must be greater than 0", nil)
	}
	if len(input.VideoIDs) == 0 {
		return ShiftVideosTimeResult{}, apperrors.New(apperrors.CodeValidation, "at least one video id must be provided", nil)
	}
	if input.ShiftMilliseconds == 0 {
		return ShiftVideosTimeResult{}, apperrors.New(apperrors.CodeValidation, "shiftMilliseconds must not be zero", nil)
	}
	if a.WorkspaceRepo == nil || a.VideoRepo == nil {
		return ShiftVideosTimeResult{}, apperrors.New(apperrors.CodeInternal, "video shift action is not fully configured", nil)
	}

	uniqueIDs := dedupeVideoIDs(input.VideoIDs)
	if len(uniqueIDs) == 0 {
		return ShiftVideosTimeResult{}, apperrors.New(apperrors.CodeValidation, "at least one valid video id must be provided", nil)
	}

	if _, err := a.WorkspaceRepo.GetByID(ctx, input.WorkspaceID); err != nil {
		return ShiftVideosTimeResult{}, err
	}

	updated, err := a.VideoRepo.ShiftVideoTimes(ctx, input.WorkspaceID, uniqueIDs, time.Duration(input.ShiftMilliseconds)*time.Millisecond)
	if err != nil {
		return ShiftVideosTimeResult{}, err
	}

	return ShiftVideosTimeResult{Updated: updated}, nil
}

func dedupeVideoIDs(raw []int64) []int64 {
	seen := make(map[int64]struct{}, len(raw))
	result := make([]int64, 0, len(raw))
	for _, id := range raw {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}
