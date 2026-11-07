package actions

import (
	"context"
	"time"

	"trilhasbr-video-cutter/internal/domain"
)

type personWorkspaceStub struct{ dir string }

func (s personWorkspaceStub) UpsertByDirectory(context.Context, string, string) (domain.Workspace, error) {
	return domain.Workspace{}, nil
}
func (s personWorkspaceStub) ListWithStats(context.Context) ([]domain.WorkspaceSummary, error) {
	return nil, nil
}
func (s personWorkspaceStub) GetByID(context.Context, int64) (domain.Workspace, error) {
	return domain.Workspace{ID: 1, Directory: s.dir}, nil
}

type personVideoStub struct{}

func (personVideoStub) ReplaceWorkspaceVideos(context.Context, int64, []domain.VideoInsert) error {
	return nil
}
func (personVideoStub) ListByWorkspace(context.Context, int64, int, int) ([]domain.Video, int, error) {
	return nil, 0, nil
}
func (personVideoStub) ListByWorkspaceAll(context.Context, int64) ([]domain.Video, error) {
	return nil, nil
}
func (personVideoStub) GetByID(_ context.Context, id int64) (domain.Video, error) {
	return domain.Video{ID: id, Path: "/v.mp4", DurationSeconds: 60}, nil
}
func (personVideoStub) DeleteWorkspaceExportVideos(context.Context, int64, string) (int, error) {
	return 0, nil
}
func (personVideoStub) FindOverlapping(context.Context, int64, time.Time, time.Time) ([]domain.Video, error) {
	return nil, nil
}
func (personVideoStub) ShiftVideoTimes(context.Context, int64, []int64, time.Duration) (int, error) {
	return 0, nil
}

type personDetectionStub struct{ groups []domain.PersonGroup }

func (s personDetectionStub) CreateRun(context.Context, int64, float64, string, int) (domain.DetectionRun, error) {
	return domain.DetectionRun{}, nil
}
func (s personDetectionStub) FinishRun(context.Context, int64, domain.DetectionStatus, int, int) error {
	return nil
}
func (s personDetectionStub) GetVideoAnalysis(context.Context, int64) (*domain.VideoAnalysis, error) {
	return nil, nil
}
func (s personDetectionStub) UpsertVideoAnalysis(context.Context, domain.VideoAnalysis) error {
	return nil
}
func (s personDetectionStub) ReplaceTracksForVideo(context.Context, int64, []domain.Track) ([]domain.Track, error) {
	return nil, nil
}
func (s personDetectionStub) ListVideoTrackBoxes(context.Context, int64) ([]domain.VideoTrackBoxes, error) {
	return nil, nil
}
func (s personDetectionStub) ListGroups(_ context.Context, filter domain.GroupFilter) ([]domain.PersonGroup, int, error) {
	if len(filter.GroupIDs) == 0 {
		return s.groups, len(s.groups), nil
	}
	var groups []domain.PersonGroup
	for _, group := range s.groups {
		for _, id := range filter.GroupIDs {
			if group.ID == id {
				groups = append(groups, group)
				break
			}
		}
	}
	return groups, len(groups), nil
}
func (s personDetectionStub) MergeGroups(context.Context, []int64) (int64, error) { return 0, nil }
func (s personDetectionStub) SplitTrack(context.Context, int64, int64) (int64, error) {
	return 0, nil
}
func (s personDetectionStub) RemoveTrack(context.Context, int64) error           { return nil }
func (s personDetectionStub) SetGroupLabel(context.Context, int64, string) error { return nil }

func (s personWorkspaceStub) Delete(context.Context, int64) error { return nil }
