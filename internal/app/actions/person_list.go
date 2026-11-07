package actions

import (
	"context"
	"encoding/base64"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/domain"
	"trilhasbr-video-cutter/internal/repository"
)

type ListPeopleInput struct {
	WorkspaceID    int64   `json:"workspaceId"`
	MinDurationSec float64 `json:"minDurationSec"`
	MinConfidence  float64 `json:"minConfidence"`
	Page           int     `json:"page"`
	PageSize       int     `json:"pageSize"`
}

// PersonListAction lists and curates person groups (merge / split / remove).
type PersonListAction struct {
	Repo repository.DetectionRepository
}

// List returns groups (each track carries VideoName and ThumbnailPath) and the filtered total.
func (a *PersonListAction) List(ctx context.Context, in ListPeopleInput) ([]domain.PersonGroup, int, error) {
	if in.WorkspaceID <= 0 {
		return nil, 0, apperrors.New(apperrors.CodeValidation, "workspaceId must be greater than 0", nil)
	}
	if in.MinDurationSec < 0 || in.MinConfidence < 0 || in.MinConfidence > 1 {
		return nil, 0, apperrors.New(apperrors.CodeValidation, "minDurationSec must be >= 0 and minConfidence within 0..1", nil)
	}
	return a.Repo.ListGroups(ctx, domain.GroupFilter{
		WorkspaceID:    in.WorkspaceID,
		MinDurationSec: in.MinDurationSec,
		MinConfidence:  in.MinConfidence,
		Page:           in.Page,
		PageSize:       in.PageSize,
	})
}

// Merge moves all tracks into one group and returns its ID.
func (a *PersonListAction) Merge(ctx context.Context, groupIDs []int64) (int64, error) {
	seen := map[int64]bool{}
	for _, id := range groupIDs {
		if id <= 0 {
			return 0, apperrors.New(apperrors.CodeValidation, "group ids must be greater than 0", nil)
		}
		seen[id] = true
	}
	if len(seen) < 2 {
		return 0, apperrors.New(apperrors.CodeValidation, "at least two distinct groups are required to merge", map[string]any{"groupIds": groupIDs})
	}
	return a.Repo.MergeGroups(ctx, groupIDs)
}

// Split moves a track out of its group into a new group and returns the new group ID.
func (a *PersonListAction) Split(ctx context.Context, groupID int64, trackID int64) (int64, error) {
	if groupID <= 0 || trackID <= 0 {
		return 0, apperrors.New(apperrors.CodeValidation, "groupId and trackId must be greater than 0", nil)
	}
	return a.Repo.SplitTrack(ctx, groupID, trackID)
}

// MaxPersonLabelRunes is the longest person name accepted.
const MaxPersonLabelRunes = 60

// Rename names a person group. Surrounding whitespace is trimmed; an empty label clears the name.
func (a *PersonListAction) Rename(ctx context.Context, groupID int64, label string) error {
	if groupID <= 0 {
		return apperrors.New(apperrors.CodeValidation, "groupId must be greater than 0", nil)
	}
	label = strings.TrimSpace(label)
	if utf8.RuneCountInString(label) > MaxPersonLabelRunes {
		return apperrors.New(apperrors.CodeValidation, "name is too long (max 60 characters)", map[string]any{"max": MaxPersonLabelRunes})
	}
	for _, r := range label {
		if unicode.IsControl(r) {
			return apperrors.New(apperrors.CodeValidation, "name must not contain control characters", nil)
		}
	}
	return a.Repo.SetGroupLabel(ctx, groupID, label)
}

// Remove hides a track (false detection).
func (a *PersonListAction) Remove(ctx context.Context, trackID int64) error {
	if trackID <= 0 {
		return apperrors.New(apperrors.CodeValidation, "trackId must be greater than 0", nil)
	}
	return a.Repo.RemoveTrack(ctx, trackID)
}

// ThumbnailDataURL inlines a JPEG as a data URL; a missing or unreadable file yields "" (the UI shows a placeholder).
func ThumbnailDataURL(path string) string {
	if path == "" {
		return ""
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(raw)
}

// VideoBoxesResult is what the player overlay needs for one video.
type VideoBoxesResult struct {
	SampleFPS float64 // 0 when the video was never analyzed
	Tracks    []domain.VideoTrackBoxes
}

// VideoBoxes returns a video's visible tracks with their per-frame boxes (normalized 0..1).
func (a *PersonListAction) VideoBoxes(ctx context.Context, videoID int64) (VideoBoxesResult, error) {
	if videoID <= 0 {
		return VideoBoxesResult{}, apperrors.New(apperrors.CodeValidation, "videoId must be greater than 0", nil)
	}
	tracks, err := a.Repo.ListVideoTrackBoxes(ctx, videoID)
	if err != nil {
		return VideoBoxesResult{}, err
	}
	analysis, err := a.Repo.GetVideoAnalysis(ctx, videoID)
	if err != nil {
		return VideoBoxesResult{}, err
	}
	result := VideoBoxesResult{Tracks: tracks}
	if analysis != nil {
		result.SampleFPS = analysis.SampleFPS
	}
	return result, nil
}
