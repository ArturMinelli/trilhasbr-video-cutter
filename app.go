package main

import (
	"context"
	"net/http"

	"trilhasbr-video-cutter/internal/api/wails"
	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/bootstrap"
)

type App struct {
	ctx     context.Context
	service *wailsapi.Service
	closeFn func()
	media   http.Handler
	initErr error
}

func NewApp() *App {
	deps, err := bootstrap.Build(context.Background())
	if err != nil {
		return &App{initErr: err}
	}
	return &App{
		service: deps.Service,
		closeFn: deps.Close,
		media:   deps.MediaHandler,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.service != nil {
		a.service.Startup(ctx)
	}
}

func (a *App) shutdown(ctx context.Context) {
	_ = ctx
	if a.closeFn != nil {
		a.closeFn()
	}
}

func (a *App) ListWorkspaces() wailsapi.ListWorkspacesResponse {
	if a.initErr != nil {
		return wailsapi.ListWorkspacesResponse{Error: appError(a.initErr)}
	}
	return a.service.ListWorkspaces()
}

func (a *App) LoadWorkspaceByDialog() wailsapi.LoadWorkspaceResponse {
	if a.initErr != nil {
		return wailsapi.LoadWorkspaceResponse{Error: appError(a.initErr)}
	}
	return a.service.LoadWorkspaceByDialog()
}

func (a *App) RefreshWorkspace(workspaceID int64) wailsapi.LoadWorkspaceResponse {
	if a.initErr != nil {
		return wailsapi.LoadWorkspaceResponse{Error: appError(a.initErr)}
	}
	return a.service.RefreshWorkspace(workspaceID)
}

func (a *App) DeleteWorkspace(workspaceID int64) wailsapi.ActionResponse {
	if a.initErr != nil {
		return wailsapi.ActionResponse{Error: appError(a.initErr)}
	}
	return a.service.DeleteWorkspace(workspaceID)
}

func (a *App) GetWorkspaceVideos(workspaceID int64, page int, pageSize int) wailsapi.PagedVideosResponse {
	if a.initErr != nil {
		return wailsapi.PagedVideosResponse{Error: appError(a.initErr)}
	}
	return a.service.GetWorkspaceVideos(workspaceID, page, pageSize)
}

func (a *App) GetWorkspaceExplorer(workspaceID int64) wailsapi.WorkspaceExplorerResponse {
	if a.initErr != nil {
		return wailsapi.WorkspaceExplorerResponse{Error: appError(a.initErr)}
	}
	return a.service.GetWorkspaceExplorer(workspaceID)
}

func (a *App) GetVideoPreview(videoID int64) wailsapi.VideoPreviewResponse {
	if a.initErr != nil {
		return wailsapi.VideoPreviewResponse{VideoID: videoID, Error: appError(a.initErr)}
	}
	return a.service.GetVideoPreview(videoID)
}

func (a *App) StartCut(request wailsapi.StartCutRequest) wailsapi.StartCutResponse {
	if a.initErr != nil {
		return wailsapi.StartCutResponse{Error: appError(a.initErr)}
	}
	return a.service.StartCut(request)
}

func (a *App) PreviewCutsFromCSV(request wailsapi.PreviewCutsFromCSVRequest) wailsapi.PreviewCutsFromCSVResponse {
	if a.initErr != nil {
		return wailsapi.PreviewCutsFromCSVResponse{Error: appError(a.initErr)}
	}
	return a.service.PreviewCutsFromCSV(request)
}

func (a *App) StartCutsFromCSV(request wailsapi.StartCutsFromCSVRequest) wailsapi.StartCutsFromCSVResponse {
	if a.initErr != nil {
		return wailsapi.StartCutsFromCSVResponse{Error: appError(a.initErr)}
	}
	return a.service.StartCutsFromCSV(request)
}

func (a *App) ShiftVideosTime(request wailsapi.ShiftVideosTimeRequest) wailsapi.ShiftVideosTimeResponse {
	if a.initErr != nil {
		return wailsapi.ShiftVideosTimeResponse{Error: appError(a.initErr)}
	}
	return a.service.ShiftVideosTime(request)
}

func (a *App) GetCutJob(jobID string) wailsapi.CutJobStatusResponse {
	if a.initErr != nil {
		return wailsapi.CutJobStatusResponse{Error: appError(a.initErr)}
	}
	return a.service.GetCutJob(jobID)
}

func (a *App) OpenFolder(path string) wailsapi.ActionResponse {
	if a.initErr != nil {
		return wailsapi.ActionResponse{Success: false, Error: appError(a.initErr)}
	}
	return a.service.OpenFolder(path)
}

func (a *App) StartDetection(request wailsapi.StartDetectionRequest) wailsapi.StartDetectionResponse {
	if a.initErr != nil {
		return wailsapi.StartDetectionResponse{Error: appError(a.initErr)}
	}
	return a.service.StartDetection(request)
}

func (a *App) CancelDetection(jobID string) wailsapi.ActionResponse {
	if a.initErr != nil {
		return wailsapi.ActionResponse{Error: appError(a.initErr)}
	}
	return a.service.CancelDetection(jobID)
}

func (a *App) GetDetectionJob(jobID string) wailsapi.DetectionJobResponse {
	if a.initErr != nil {
		return wailsapi.DetectionJobResponse{Error: appError(a.initErr)}
	}
	return a.service.GetDetectionJob(jobID)
}

func (a *App) ListPeople(request wailsapi.ListPeopleRequest) wailsapi.ListPeopleResponse {
	if a.initErr != nil {
		return wailsapi.ListPeopleResponse{Error: appError(a.initErr)}
	}
	return a.service.ListPeople(request)
}

func (a *App) ListPeopleIDs(request wailsapi.ListPeopleIDsRequest) wailsapi.ListPeopleIDsResponse {
	if a.initErr != nil {
		return wailsapi.ListPeopleIDsResponse{IDs: []int64{}, Error: appError(a.initErr)}
	}
	return a.service.ListPeopleIDs(request)
}

func (a *App) MergePeople(groupIDs []int64) wailsapi.PersonGroupResponse {
	if a.initErr != nil {
		return wailsapi.PersonGroupResponse{Error: appError(a.initErr)}
	}
	return a.service.MergePeople(groupIDs)
}

func (a *App) SplitPerson(groupID int64, trackID int64) wailsapi.PersonGroupResponse {
	if a.initErr != nil {
		return wailsapi.PersonGroupResponse{Error: appError(a.initErr)}
	}
	return a.service.SplitPerson(groupID, trackID)
}

func (a *App) RemoveTrack(trackID int64) wailsapi.ActionResponse {
	if a.initErr != nil {
		return wailsapi.ActionResponse{Error: appError(a.initErr)}
	}
	return a.service.RemoveTrack(trackID)
}

func (a *App) RenamePerson(groupID int64, label string) wailsapi.ActionResponse {
	if a.initErr != nil {
		return wailsapi.ActionResponse{Error: appError(a.initErr)}
	}
	return a.service.RenamePerson(groupID, label)
}

func (a *App) StartPeopleCut(request wailsapi.StartPeopleCutRequest) wailsapi.StartCutResponse {
	if a.initErr != nil {
		return wailsapi.StartCutResponse{Error: appError(a.initErr)}
	}
	return a.service.StartPeopleCut(request)
}

func (a *App) GetVideoTrackBoxes(videoID int64) wailsapi.VideoTrackBoxesResponse {
	if a.initErr != nil {
		return wailsapi.VideoTrackBoxesResponse{VideoID: videoID, Tracks: []wailsapi.VideoTrackBoxesDTO{}, Error: appError(a.initErr)}
	}
	return a.service.GetVideoTrackBoxes(videoID)
}

func appError(err error) *wailsapi.AppError {
	if err == nil {
		return nil
	}
	return apperrors.ToPublic(err)
}
