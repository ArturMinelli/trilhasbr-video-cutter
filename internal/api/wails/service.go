package wailsapi

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"trilhasbr-video-cutter/internal/app/actions"
	"trilhasbr-video-cutter/internal/app/jobs"
	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/platform"
)

type RuntimeEventBridge interface {
	PublishProgress(ctx context.Context, event actions.CutProgressEvent)
	PublishFinished(ctx context.Context, event actions.CutFinishedEvent)
	PublishFailed(ctx context.Context, event actions.CutFailedEvent)
	PublishCSVProgress(ctx context.Context, event actions.CutCSVProgressEvent)
	PublishCSVFinished(ctx context.Context, event actions.CutCSVFinishedEvent)
	PublishCSVFailed(ctx context.Context, event actions.CutCSVFailedEvent)
}

type Service struct {
	mu sync.RWMutex

	ctx context.Context

	logger *slog.Logger

	bridge          RuntimeEventBridge
	workspaceList   *actions.WorkspaceListAction
	workspaceLoad   *actions.WorkspaceLoadAction
	workspaceDelete *actions.WorkspaceDeleteAction
	workspaceExpo   *actions.WorkspaceExplorerAction
	videoList       *actions.VideoListAction
	videoPreview    *actions.VideoPreviewAction
	videoShift      *actions.VideoShiftAction
	cutExecute      *actions.CutExecuteAction
	cutCSVPreview   *actions.CutCSVPreviewAction
	cutCSVExecute   *actions.CutCSVExecuteAction
	jobManager      *jobs.Manager

	detectRun  func() (*actions.DetectRunAction, error)
	detectJobs *jobs.DetectionManager
	personList *actions.PersonListAction
	personCut  *actions.PersonCutAction
}

type ServiceParams struct {
	Logger          *slog.Logger
	Bridge          RuntimeEventBridge
	WorkspaceList   *actions.WorkspaceListAction
	WorkspaceLoad   *actions.WorkspaceLoadAction
	WorkspaceDelete *actions.WorkspaceDeleteAction
	WorkspaceExpo   *actions.WorkspaceExplorerAction
	VideoList       *actions.VideoListAction
	VideoPreview    *actions.VideoPreviewAction
	VideoShift      *actions.VideoShiftAction
	CutExecute      *actions.CutExecuteAction
	CutCSVPreview   *actions.CutCSVPreviewAction
	CutCSVExecute   *actions.CutCSVExecuteAction
	JobManager      *jobs.Manager

	// DetectRun builds the detection action on demand (opens the ONNX engine lazily).
	DetectRun  func() (*actions.DetectRunAction, error)
	DetectJobs *jobs.DetectionManager
	PersonList *actions.PersonListAction
	PersonCut  *actions.PersonCutAction
}

func NewService(params ServiceParams) *Service {
	logger := params.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		logger:          logger,
		bridge:          params.Bridge,
		workspaceList:   params.WorkspaceList,
		workspaceLoad:   params.WorkspaceLoad,
		workspaceDelete: params.WorkspaceDelete,
		workspaceExpo:   params.WorkspaceExpo,
		videoList:       params.VideoList,
		videoPreview:    params.VideoPreview,
		videoShift:      params.VideoShift,
		cutExecute:      params.CutExecute,
		cutCSVPreview:   params.CutCSVPreview,
		cutCSVExecute:   params.CutCSVExecute,
		jobManager:      params.JobManager,
		detectRun:       params.DetectRun,
		detectJobs:      params.DetectJobs,
		personList:      params.PersonList,
		personCut:       params.PersonCut,
	}
}

func (s *Service) Startup(ctx context.Context) {
	s.mu.Lock()
	s.ctx = ctx
	s.mu.Unlock()
}

func (s *Service) getContext() context.Context {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}

func (s *Service) ListWorkspaces() ListWorkspacesResponse {
	workspaces, err := s.workspaceList.Execute(s.getContext())
	if err != nil {
		return ListWorkspacesResponse{Error: apperrors.ToPublic(err)}
	}

	items := make([]WorkspaceDTO, 0, len(workspaces))
	for _, workspace := range workspaces {
		items = append(items, WorkspaceDTO{
			ID:                 workspace.ID,
			Directory:          workspace.Directory,
			Name:               workspace.Name,
			CreatedAt:          workspace.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:          workspace.UpdatedAt.UTC().Format(time.RFC3339),
			VideoCount:         workspace.VideoCount,
			NullStartedAtCount: workspace.NullStartedAtCount,
		})
	}

	return ListWorkspacesResponse{Workspaces: items}
}

func (s *Service) LoadWorkspaceByDialog() LoadWorkspaceResponse {
	result, err := s.workspaceLoad.Execute(s.getContext())
	if err != nil {
		return LoadWorkspaceResponse{Error: apperrors.ToPublic(err)}
	}
	return toLoadWorkspaceResponse(result)
}

// RefreshWorkspace re-scans the workspace's stored directory and reloads its videos.
func (s *Service) RefreshWorkspace(workspaceID int64) LoadWorkspaceResponse {
	result, err := s.workspaceLoad.Refresh(s.getContext(), workspaceID)
	if err != nil {
		return LoadWorkspaceResponse{Error: apperrors.ToPublic(err)}
	}
	return toLoadWorkspaceResponse(result)
}

// DeleteWorkspace removes the workspace and its app data; never the user's videos or exports.
func (s *Service) DeleteWorkspace(workspaceID int64) ActionResponse {
	if err := s.workspaceDelete.Execute(s.getContext(), workspaceID); err != nil {
		return ActionResponse{Error: apperrors.ToPublic(err)}
	}
	return ActionResponse{Success: true}
}

func toLoadWorkspaceResponse(result actions.WorkspaceLoadResult) LoadWorkspaceResponse {
	failures := make([]FileFailureDTO, 0, len(result.Failures))
	for _, failure := range result.Failures {
		failures = append(failures, FileFailureDTO{Path: failure.Path, Reason: failure.Reason})
	}

	return LoadWorkspaceResponse{
		Workspace: WorkspaceDTO{
			ID:        result.WorkspaceID,
			Directory: result.WorkspaceDirectory,
			Name:      result.WorkspaceName,
			CreatedAt: result.WorkspaceCreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt: result.WorkspaceUpdatedAt.UTC().Format(time.RFC3339),
		},
		Summary: WorkspaceLoadSummaryDTO{
			TotalScanned:       result.TotalScanned,
			Indexed:            result.Indexed,
			NullStartedAtCount: result.NullStartedAtCount,
			Failures:           failures,
		},
	}
}

func (s *Service) GetWorkspaceVideos(workspaceID int64, page int, pageSize int) PagedVideosResponse {
	result, err := s.videoList.Execute(s.getContext(), workspaceID, page, pageSize)
	if err != nil {
		return PagedVideosResponse{Error: apperrors.ToPublic(err)}
	}

	videos := make([]VideoDTO, 0, len(result.Videos))
	for _, video := range result.Videos {
		var startedAt *string
		if video.StartedAt != nil {
			timestamp := video.StartedAt.UTC().Format(time.RFC3339)
			startedAt = &timestamp
		}
		var finishedAt *string
		if video.FinishedAt != nil {
			timestamp := video.FinishedAt.UTC().Format(time.RFC3339)
			finishedAt = &timestamp
		}

		videos = append(videos, VideoDTO{
			ID:              video.ID,
			WorkspaceID:     video.WorkspaceID,
			Name:            video.Name,
			Path:            video.Path,
			StartedAt:       startedAt,
			FinishedAt:      finishedAt,
			DurationSeconds: video.DurationSeconds,
		})
	}

	return PagedVideosResponse{
		Videos:   videos,
		Page:     result.Page,
		PageSize: result.PageSize,
		Total:    result.Total,
	}
}

func (s *Service) GetWorkspaceExplorer(workspaceID int64) WorkspaceExplorerResponse {
	if s.workspaceExpo == nil {
		return WorkspaceExplorerResponse{Error: apperrors.ToPublic(apperrors.New(apperrors.CodeInternal, "workspace explorer action is not configured", nil))}
	}

	result, err := s.workspaceExpo.Execute(s.getContext(), workspaceID)
	if err != nil {
		return WorkspaceExplorerResponse{Error: apperrors.ToPublic(err)}
	}

	videos := make([]VideoExplorerDTO, 0, len(result.Videos))
	for _, item := range result.Videos {
		var startedAt *string
		if item.Video.StartedAt != nil {
			timestamp := item.Video.StartedAt.UTC().Format(time.RFC3339)
			startedAt = &timestamp
		}
		var finishedAt *string
		if item.Video.FinishedAt != nil {
			timestamp := item.Video.FinishedAt.UTC().Format(time.RFC3339)
			finishedAt = &timestamp
		}

		videos = append(videos, VideoExplorerDTO{
			ID:              item.Video.ID,
			WorkspaceID:     item.Video.WorkspaceID,
			Name:            item.Video.Name,
			Path:            item.Video.Path,
			RelativePath:    item.RelativePath,
			RelativeDir:     item.RelativeDir,
			StartedAt:       startedAt,
			FinishedAt:      finishedAt,
			DurationSeconds: item.Video.DurationSeconds,
		})
	}

	return WorkspaceExplorerResponse{
		WorkspaceID:        result.WorkspaceID,
		WorkspaceDirectory: result.WorkspaceDirectory,
		Videos:             videos,
	}
}

func (s *Service) GetVideoPreview(videoID int64) VideoPreviewResponse {
	if s.videoPreview == nil {
		return VideoPreviewResponse{
			VideoID: videoID,
			Error:   apperrors.ToPublic(apperrors.New(apperrors.CodeInternal, "video preview action is not configured", nil)),
		}
	}

	result, err := s.videoPreview.Execute(s.getContext(), videoID)
	if err != nil {
		return VideoPreviewResponse{
			VideoID: videoID,
			Error:   apperrors.ToPublic(err),
		}
	}

	return VideoPreviewResponse{
		VideoID: result.VideoID,
		DataURL: result.DataURL,
		Cached:  result.Cached,
	}
}

func (s *Service) ShiftVideosTime(request ShiftVideosTimeRequest) ShiftVideosTimeResponse {
	if s.videoShift == nil {
		return ShiftVideosTimeResponse{
			Error: apperrors.ToPublic(apperrors.New(apperrors.CodeInternal, "video shift action is not configured", nil)),
		}
	}

	result, err := s.videoShift.Execute(s.getContext(), actions.ShiftVideosTimeInput{
		WorkspaceID:       request.WorkspaceID,
		VideoIDs:          request.VideoIDs,
		ShiftMilliseconds: request.ShiftMilliseconds,
	})
	if err != nil {
		return ShiftVideosTimeResponse{Error: apperrors.ToPublic(err)}
	}

	return ShiftVideosTimeResponse{Updated: result.Updated}
}

func (s *Service) StartCut(request StartCutRequest) StartCutResponse {
	job := s.jobManager.Create()
	s.jobManager.SetRunning(job.JobID)

	ctx := s.getContext()
	go func() {
		result, err := s.cutExecute.Execute(ctx, job.JobID, actions.StartCutInput{
			WorkspaceID:   request.WorkspaceID,
			Name:          request.Name,
			DateTimeLocal: request.DateTimeLocal,
			SecondsBefore: request.SecondsBefore,
			SecondsAfter:  request.SecondsAfter,
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

func (s *Service) PreviewCutsFromCSV(request PreviewCutsFromCSVRequest) PreviewCutsFromCSVResponse {
	if s.cutCSVPreview == nil {
		return PreviewCutsFromCSVResponse{Error: apperrors.ToPublic(apperrors.New(apperrors.CodeInternal, "csv preview action is not configured", nil))}
	}

	result, err := s.cutCSVPreview.Execute(s.getContext(), actions.PreviewCutsFromCSVInput{WorkspaceID: request.WorkspaceID})
	if err != nil {
		return PreviewCutsFromCSVResponse{Error: apperrors.ToPublic(err)}
	}

	issues := make([]CSVImportIssueDTO, 0, len(result.Issues))
	for _, issue := range result.Issues {
		issues = append(issues, CSVImportIssueDTO{
			Row:     issue.Row,
			Column:  issue.Column,
			Message: issue.Message,
			Value:   issue.Value,
		})
	}

	cuts := make([]CSVImportCutDTO, 0, len(result.Cuts))
	for _, cut := range result.Cuts {
		cuts = append(cuts, CSVImportCutDTO{
			Row:             cut.Row,
			Name:            cut.Name,
			DateTimeLocal:   cut.DateTimeLocal,
			SecondsBefore:   cut.SecondsBefore,
			SecondsAfter:    cut.SecondsAfter,
			OutputDirectory: cut.OutputDirectory,
			TotalSegments:   cut.TotalSegments,
		})
	}

	response := PreviewCutsFromCSVResponse{
		FileName:  result.FileName,
		Cuts:      cuts,
		Issues:    issues,
		Cancelled: result.Cancelled,
	}
	if len(issues) > 0 {
		response.Error = apperrors.ToPublic(apperrors.New(apperrors.CodeValidation, "csv validation failed", map[string]any{"issueCount": len(issues)}))
	}

	return response
}

func (s *Service) StartCutsFromCSV(request StartCutsFromCSVRequest) StartCutsFromCSVResponse {
	if s.cutCSVExecute == nil {
		return StartCutsFromCSVResponse{Error: apperrors.ToPublic(apperrors.New(apperrors.CodeInternal, "csv execute action is not configured", nil))}
	}

	jobID := uuid.NewString()
	cuts := make([]actions.StartCutsFromCSVItem, 0, len(request.Cuts))
	for _, cut := range request.Cuts {
		cuts = append(cuts, actions.StartCutsFromCSVItem{
			Row:           cut.Row,
			Name:          cut.Name,
			DateTimeLocal: cut.DateTimeLocal,
			SecondsBefore: cut.SecondsBefore,
			SecondsAfter:  cut.SecondsAfter,
		})
	}

	ctx := s.getContext()
	go func() {
		result, err := s.cutCSVExecute.Execute(ctx, jobID, actions.StartCutsFromCSVInput{
			WorkspaceID: request.WorkspaceID,
			Cuts:        cuts,
		}, func(progress actions.CutCSVProgressEvent) {
			if s.bridge != nil {
				s.bridge.PublishCSVProgress(ctx, progress)
			}
		})
		if err != nil {
			if s.bridge != nil {
				publicErr := apperrors.ToPublic(err)
				s.bridge.PublishCSVFailed(ctx, actions.CutCSVFailedEvent{
					JobID: jobID,
					Row:   intDetail(publicErr, "row"),
					Name:  stringDetail(publicErr, "name"),
					Error: publicErr,
				})
			}
			return
		}

		if s.bridge != nil {
			s.bridge.PublishCSVFinished(ctx, actions.CutCSVFinishedEvent{
				JobID:           result.JobID,
				OutputDirectory: result.OutputDirectory,
				TotalCuts:       result.TotalCuts,
				CompletedCuts:   result.CompletedCuts,
			})
		}
	}()

	return StartCutsFromCSVResponse{JobID: jobID}
}

func intDetail(err *apperrors.PublicError, key string) int {
	if err == nil || err.Details == nil {
		return 0
	}
	value, ok := err.Details[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func stringDetail(err *apperrors.PublicError, key string) string {
	if err == nil || err.Details == nil {
		return ""
	}
	value, ok := err.Details[key]
	if !ok {
		return ""
	}
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}

func (s *Service) GetCutJob(jobID string) CutJobStatusResponse {
	job, found := s.jobManager.Get(jobID)
	if !found {
		return CutJobStatusResponse{
			Error: apperrors.ToPublic(apperrors.New(apperrors.CodeValidation, "job not found", map[string]any{"jobId": jobID})),
		}
	}
	return CutJobStatusResponse{Job: &job}
}

func (s *Service) OpenFolder(path string) ActionResponse {
	if err := platform.OpenFolder(path); err != nil {
		return ActionResponse{Success: false, Error: apperrors.ToPublic(err)}
	}
	return ActionResponse{Success: true}
}
