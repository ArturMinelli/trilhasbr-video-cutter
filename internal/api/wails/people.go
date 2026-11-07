package wailsapi

import (
	"context"

	"trilhasbr-video-cutter/internal/app/actions"
	"trilhasbr-video-cutter/internal/app/jobs"
	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/domain"
)

// DetectEventBridge is optional: the runtime bridge implements it, test stubs need not.
type DetectEventBridge interface {
	PublishDetectProgress(ctx context.Context, event actions.DetectProgressEvent)
	PublishDetectFinished(ctx context.Context, event actions.DetectFinishedEvent)
	PublishDetectFailed(ctx context.Context, event actions.DetectFailedEvent)
}

type PersonTrackDTO struct {
	ID        int64   `json:"id"`
	VideoID   int64   `json:"videoId"`
	VideoName string  `json:"videoName"`
	StartSec  float64 `json:"startSec"`
	EndSec    float64 `json:"endSec"`
}

type PersonGroupDTO struct {
	ID               int64            `json:"id"`
	Label            string           `json:"label,omitempty"` // user-given name; empty when unnamed
	ThumbnailDataURL string           `json:"thumbnailDataUrl"`
	StartSec         *float64         `json:"startSec,omitempty"`
	TrackCount       int              `json:"trackCount"`
	VideoCount       int              `json:"videoCount"`
	TotalDurationSec float64          `json:"totalDurationSec"`
	AvgConfidence    float64          `json:"avgConfidence"`
	Tracks           []PersonTrackDTO `json:"tracks"`
}

type StartDetectionRequest struct {
	WorkspaceID int64   `json:"workspaceId"`
	SampleFPS   float64 `json:"sampleFps"`
}

type StartDetectionResponse struct {
	JobID string    `json:"jobId,omitempty"`
	Error *AppError `json:"error,omitempty"`
}

type DetectionJobResponse struct {
	Job   *jobs.DetectionSnapshot `json:"job,omitempty"`
	Error *AppError               `json:"error,omitempty"`
}

type ListPeopleRequest struct {
	WorkspaceID    int64   `json:"workspaceId"`
	MinDurationSec float64 `json:"minDurationSec"`
	MinConfidence  float64 `json:"minConfidence"`
	Page           int     `json:"page"`
	PageSize       int     `json:"pageSize"`
}

type ListPeopleResponse struct {
	Items []PersonGroupDTO `json:"items,omitempty"`
	Total int              `json:"total"`
	Error *AppError        `json:"error,omitempty"`
}

type PersonGroupResponse struct {
	Group *PersonGroupDTO `json:"group,omitempty"`
	Error *AppError       `json:"error,omitempty"`
}

type StartPeopleCutRequest struct {
	WorkspaceID   int64   `json:"workspaceId"`
	GroupIDs      []int64 `json:"groupIds"`
	Name          string  `json:"name"`
	SecondsBefore int     `json:"secondsBefore"`
	SecondsAfter  int     `json:"secondsAfter"`
}

type VideoTrackBoxesDTO struct {
	TrackID  int64        `json:"trackId"`
	GroupID  int64        `json:"groupId"`
	Label    string       `json:"label,omitempty"` // the group's name; empty when unnamed
	StartSec float64      `json:"startSec"`
	EndSec   float64      `json:"endSec"`
	Boxes    [][5]float32 `json:"boxes"` // rows of [t, x1, y1, x2, y2]; t in seconds, coordinates normalized 0..1
}

type VideoTrackBoxesResponse struct {
	VideoID   int64                `json:"videoId"`
	SampleFPS float64              `json:"sampleFps"`
	Tracks    []VideoTrackBoxesDTO `json:"tracks"`
	Error     *AppError            `json:"error,omitempty"`
}

func toPersonGroupDTO(group domain.PersonGroup) PersonGroupDTO {
	dto := PersonGroupDTO{
		ID:               group.ID,
		Label:            group.Label,
		TrackCount:       len(group.Tracks),
		VideoCount:       group.VideoCount,
		TotalDurationSec: group.TotalDurationSec,
		AvgConfidence:    group.AvgConfidence,
		Tracks:           make([]PersonTrackDTO, 0, len(group.Tracks)),
	}
	for i, track := range group.Tracks {
		if i == 0 {
			start := track.StartSec
			dto.StartSec = &start
			dto.ThumbnailDataURL = actions.ThumbnailDataURL(track.ThumbnailPath)
		}
		dto.Tracks = append(dto.Tracks, PersonTrackDTO{
			ID: track.ID, VideoID: track.VideoID, VideoName: track.VideoName, StartSec: track.StartSec, EndSec: track.EndSec,
		})
	}
	return dto
}

func (s *Service) StartDetection(request StartDetectionRequest) StartDetectionResponse {
	if s.detectRun == nil || s.detectJobs == nil {
		return StartDetectionResponse{Error: apperrors.ToPublic(apperrors.New(apperrors.CodeInternal, "detection is not configured", nil))}
	}
	// Validate and open the engine up front so setup errors (bad fps, missing models) are returned directly.
	action, err := s.detectRun()
	if err != nil {
		return StartDetectionResponse{Error: apperrors.ToPublic(err)}
	}
	input := actions.StartDetectionInput{WorkspaceID: request.WorkspaceID, SampleFPS: request.SampleFPS}
	if err = input.Validate(); err != nil {
		return StartDetectionResponse{Error: apperrors.ToPublic(err)}
	}

	ctx, cancel := context.WithCancel(s.getContext())
	job := s.detectJobs.Create(input.WorkspaceID, cancel)
	s.detectJobs.SetRunning(job.JobID, 0)
	events, _ := s.bridge.(DetectEventBridge)

	go func() {
		defer cancel()
		result, runErr := action.Execute(ctx, job.JobID, input, func(progress actions.DetectProgressEvent) {
			s.detectJobs.SetRunning(job.JobID, progress.VideosTotal)
			s.detectJobs.SetProgress(job.JobID, progress.VideosDone, progress.CurrentVideo, progress.TracksFound)
			if events != nil {
				events.PublishDetectProgress(s.getContext(), progress)
			}
		})
		switch {
		case runErr != nil:
			s.detectJobs.SetFailed(job.JobID, runErr)
			if events != nil {
				events.PublishDetectFailed(s.getContext(), actions.DetectFailedEvent{JobID: job.JobID, Error: apperrors.ToPublic(runErr)})
			}
		case result.Status == domain.DetectionCancelled:
			s.detectJobs.SetCancelled(job.JobID)
		case result.Status == domain.DetectionFailed:
			failure := apperrors.New(apperrors.CodeDetection, "every video failed to analyze", map[string]any{"failures": result.Failures})
			s.detectJobs.SetFailed(job.JobID, failure)
			if events != nil {
				events.PublishDetectFailed(s.getContext(), actions.DetectFailedEvent{JobID: job.JobID, Error: apperrors.ToPublic(failure)})
			}
		default:
			s.detectJobs.SetSucceeded(job.JobID)
			if events != nil {
				events.PublishDetectFinished(s.getContext(), actions.DetectFinishedEvent{JobID: job.JobID, TracksFound: result.TracksFound, ElapsedMs: result.ElapsedMs})
			}
		}
	}()

	return StartDetectionResponse{JobID: job.JobID}
}

func (s *Service) CancelDetection(jobID string) ActionResponse {
	if s.detectJobs == nil || !s.detectJobs.Cancel(jobID) {
		return ActionResponse{Error: apperrors.ToPublic(apperrors.New(apperrors.CodeValidation, "no running detection job", map[string]any{"jobId": jobID}))}
	}
	return ActionResponse{Success: true}
}

func (s *Service) GetDetectionJob(jobID string) DetectionJobResponse {
	if s.detectJobs != nil {
		if job, found := s.detectJobs.Get(jobID); found {
			return DetectionJobResponse{Job: &job}
		}
	}
	return DetectionJobResponse{Error: apperrors.ToPublic(apperrors.New(apperrors.CodeValidation, "job not found", map[string]any{"jobId": jobID}))}
}

func (s *Service) ListPeople(request ListPeopleRequest) ListPeopleResponse {
	groups, total, err := s.personList.List(s.getContext(), actions.ListPeopleInput{
		WorkspaceID: request.WorkspaceID, MinDurationSec: request.MinDurationSec, MinConfidence: request.MinConfidence,
		Page: request.Page, PageSize: request.PageSize,
	})
	if err != nil {
		return ListPeopleResponse{Error: apperrors.ToPublic(err)}
	}
	items := make([]PersonGroupDTO, 0, len(groups))
	for _, group := range groups {
		items = append(items, toPersonGroupDTO(group))
	}
	return ListPeopleResponse{Items: items, Total: total}
}

// MergePeople returns an id-only group: callers just need the merged group's id to select it.
func (s *Service) MergePeople(groupIDs []int64) PersonGroupResponse {
	id, err := s.personList.Merge(s.getContext(), groupIDs)
	if err != nil {
		return PersonGroupResponse{Error: apperrors.ToPublic(err)}
	}
	return PersonGroupResponse{Group: &PersonGroupDTO{ID: id, Tracks: []PersonTrackDTO{}}}
}

func (s *Service) SplitPerson(groupID int64, trackID int64) PersonGroupResponse {
	id, err := s.personList.Split(s.getContext(), groupID, trackID)
	if err != nil {
		return PersonGroupResponse{Error: apperrors.ToPublic(err)}
	}
	return PersonGroupResponse{Group: &PersonGroupDTO{ID: id, Tracks: []PersonTrackDTO{}}}
}

func (s *Service) RemoveTrack(trackID int64) ActionResponse {
	if err := s.personList.Remove(s.getContext(), trackID); err != nil {
		return ActionResponse{Error: apperrors.ToPublic(err)}
	}
	return ActionResponse{Success: true}
}

// RenamePerson names a person; an empty label clears the name.
func (s *Service) RenamePerson(groupID int64, label string) ActionResponse {
	if err := s.personList.Rename(s.getContext(), groupID, label); err != nil {
		return ActionResponse{Error: apperrors.ToPublic(err)}
	}
	return ActionResponse{Success: true}
}

func (s *Service) StartPeopleCut(request StartPeopleCutRequest) StartCutResponse {
	job := s.jobManager.Create()
	s.jobManager.SetRunning(job.JobID)

	ctx := s.getContext()
	go func() {
		result, err := s.personCut.Execute(ctx, job.JobID, actions.StartPeopleCutInput{
			WorkspaceID: request.WorkspaceID, GroupIDs: request.GroupIDs, Name: request.Name,
			SecondsBefore: request.SecondsBefore, SecondsAfter: request.SecondsAfter,
		}, func(progress actions.CutProgressEvent) {
			s.jobManager.SetProgress(progress.JobID, progress.CompletedSegments, progress.TotalSegments, progress.CurrentFile)
			if s.bridge != nil {
				s.bridge.PublishProgress(ctx, progress)
			}
		})
		if err != nil {
			s.jobManager.SetFailed(job.JobID, err)
			if s.bridge != nil {
				s.bridge.PublishFailed(ctx, actions.CutFailedEvent{JobID: job.JobID, Error: apperrors.ToPublic(err)})
			}
			return
		}
		s.jobManager.SetSucceeded(job.JobID, result)
		if s.bridge != nil {
			s.bridge.PublishFinished(ctx, actions.CutFinishedEvent{JobID: job.JobID, Result: result})
		}
	}()

	return StartCutResponse{JobID: job.JobID}
}

// GetVideoTrackBoxes returns the per-frame boxes of a video's people for the player overlay.
func (s *Service) GetVideoTrackBoxes(videoID int64) VideoTrackBoxesResponse {
	response := VideoTrackBoxesResponse{VideoID: videoID, Tracks: []VideoTrackBoxesDTO{}}
	result, err := s.personList.VideoBoxes(s.getContext(), videoID)
	if err != nil {
		response.Error = apperrors.ToPublic(err)
		return response
	}
	response.SampleFPS = result.SampleFPS
	for _, track := range result.Tracks {
		dto := VideoTrackBoxesDTO{TrackID: track.TrackID, GroupID: track.GroupID, Label: track.Label, StartSec: track.StartSec, EndSec: track.EndSec,
			Boxes: make([][5]float32, len(track.Boxes))}
		for i, b := range track.Boxes {
			dto.Boxes[i] = [5]float32{b.T, b.X1, b.Y1, b.X2, b.Y2}
		}
		response.Tracks = append(response.Tracks, dto)
	}
	return response
}
