package repository

import (
	"context"
	"time"

	"trilhasbr-video-cutter/internal/domain"
)

type WorkspaceRepository interface {
	UpsertByDirectory(ctx context.Context, directory string, name string) (domain.Workspace, error)
	ListWithStats(ctx context.Context) ([]domain.WorkspaceSummary, error)
	GetByID(ctx context.Context, workspaceID int64) (domain.Workspace, error)
	// Delete removes the workspace row; FK cascades remove its videos, detection runs, tracks and groups.
	Delete(ctx context.Context, workspaceID int64) error
}

type VideoRepository interface {
	ReplaceWorkspaceVideos(ctx context.Context, workspaceID int64, videos []domain.VideoInsert) error
	ListByWorkspace(ctx context.Context, workspaceID int64, page int, pageSize int) ([]domain.Video, int, error)
	ListByWorkspaceAll(ctx context.Context, workspaceID int64) ([]domain.Video, error)
	GetByID(ctx context.Context, videoID int64) (domain.Video, error)
	DeleteWorkspaceExportVideos(ctx context.Context, workspaceID int64, workspaceDirectory string) (int, error)
	FindOverlapping(ctx context.Context, workspaceID int64, windowStartUTC time.Time, windowEndUTC time.Time) ([]domain.Video, error)
	ShiftVideoTimes(ctx context.Context, workspaceID int64, videoIDs []int64, shiftBy time.Duration) (int, error)
}

type DetectionRepository interface {
	CreateRun(ctx context.Context, workspaceID int64, sampleFPS float64, modelVersion string, videosTotal int) (domain.DetectionRun, error)
	FinishRun(ctx context.Context, runID int64, status domain.DetectionStatus, videosDone int, tracksFound int) error
	// GetVideoAnalysis returns (nil, nil) when the video was never analyzed.
	GetVideoAnalysis(ctx context.Context, videoID int64) (*domain.VideoAnalysis, error)
	UpsertVideoAnalysis(ctx context.Context, analysis domain.VideoAnalysis) error
	// ReplaceTracksForVideo atomically deletes the video's tracks, inserts the new ones (each in its own group)
	// and removes empty groups. Returned tracks carry their new IDs.
	ReplaceTracksForVideo(ctx context.Context, videoID int64, tracks []domain.Track) ([]domain.Track, error)
	// ListVideoTrackBoxes returns a video's visible tracks with their per-frame boxes (normalized 0..1), by start time.
	ListVideoTrackBoxes(ctx context.Context, videoID int64) ([]domain.VideoTrackBoxes, error)
	ListGroups(ctx context.Context, filter domain.GroupFilter) ([]domain.PersonGroup, int, error)
	// MergeGroups moves all tracks into the lowest group ID and returns it (needs >= 2 existing groups of one workspace).
	MergeGroups(ctx context.Context, groupIDs []int64) (int64, error)
	// SplitTrack moves a track into a new group and returns the new group ID.
	SplitTrack(ctx context.Context, groupID int64, trackID int64) (int64, error)
	RemoveTrack(ctx context.Context, trackID int64) error
	// SetGroupLabel names a group ("" clears it); unknown group -> validation error.
	SetGroupLabel(ctx context.Context, groupID int64, label string) error
}
