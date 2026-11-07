package wailsapi

import (
	"trilhasbr-video-cutter/internal/app/jobs"
	"trilhasbr-video-cutter/internal/apperrors"
)

type AppError = apperrors.PublicError

type WorkspaceDTO struct {
	ID                 int64  `json:"id"`
	Directory          string `json:"directory"`
	Name               string `json:"name"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
	VideoCount         int    `json:"videoCount"`
	NullStartedAtCount int    `json:"nullStartedAtCount"`
}

type VideoDTO struct {
	ID              int64   `json:"id"`
	WorkspaceID     int64   `json:"workspaceId"`
	Name            string  `json:"name"`
	Path            string  `json:"path"`
	StartedAt       *string `json:"startedAt,omitempty"`
	FinishedAt      *string `json:"finishedAt,omitempty"`
	DurationSeconds float64 `json:"durationSeconds"`
}

type VideoExplorerDTO struct {
	ID              int64   `json:"id"`
	WorkspaceID     int64   `json:"workspaceId"`
	Name            string  `json:"name"`
	Path            string  `json:"path"`
	RelativePath    string  `json:"relativePath"`
	RelativeDir     string  `json:"relativeDir"`
	StartedAt       *string `json:"startedAt,omitempty"`
	FinishedAt      *string `json:"finishedAt,omitempty"`
	DurationSeconds float64 `json:"durationSeconds"`
}

type FileFailureDTO struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type WorkspaceLoadSummaryDTO struct {
	TotalScanned       int              `json:"totalScanned"`
	Indexed            int              `json:"indexed"`
	NullStartedAtCount int              `json:"nullStartedAtCount"`
	Failures           []FileFailureDTO `json:"failures"`
}

type ListWorkspacesResponse struct {
	Workspaces []WorkspaceDTO `json:"workspaces,omitempty"`
	Error      *AppError      `json:"error,omitempty"`
}

type LoadWorkspaceResponse struct {
	Workspace WorkspaceDTO            `json:"workspace,omitempty"`
	Summary   WorkspaceLoadSummaryDTO `json:"summary,omitempty"`
	Error     *AppError               `json:"error,omitempty"`
}

type PagedVideosResponse struct {
	Videos   []VideoDTO `json:"videos,omitempty"`
	Page     int        `json:"page"`
	PageSize int        `json:"pageSize"`
	Total    int        `json:"total"`
	Error    *AppError  `json:"error,omitempty"`
}

type WorkspaceExplorerResponse struct {
	WorkspaceID        int64              `json:"workspaceId"`
	WorkspaceDirectory string             `json:"workspaceDirectory"`
	Videos             []VideoExplorerDTO `json:"videos,omitempty"`
	Error              *AppError          `json:"error,omitempty"`
}

type VideoPreviewResponse struct {
	VideoID int64     `json:"videoId"`
	DataURL string    `json:"dataUrl,omitempty"`
	Cached  bool      `json:"cached"`
	Error   *AppError `json:"error,omitempty"`
}

type StartCutRequest struct {
	WorkspaceID   int64  `json:"workspaceId"`
	Name          string `json:"name"`
	DateTimeLocal string `json:"datetimeLocal"`
	SecondsBefore int    `json:"secondsBefore"`
	SecondsAfter  int    `json:"secondsAfter"`
}

type StartCutResponse struct {
	JobID string    `json:"jobId,omitempty"`
	Error *AppError `json:"error,omitempty"`
}

type ShiftVideosTimeRequest struct {
	WorkspaceID       int64   `json:"workspaceId"`
	VideoIDs          []int64 `json:"videoIds"`
	ShiftMilliseconds int64   `json:"shiftMilliseconds"`
}

type ShiftVideosTimeResponse struct {
	Updated int       `json:"updated"`
	Error   *AppError `json:"error,omitempty"`
}

type CutJobStatusResponse struct {
	Job   *jobs.Snapshot `json:"job,omitempty"`
	Error *AppError      `json:"error,omitempty"`
}

type ActionResponse struct {
	Success bool      `json:"success"`
	Error   *AppError `json:"error,omitempty"`
}

type CSVImportIssueDTO struct {
	Row     int    `json:"row"`
	Column  string `json:"column"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

type CSVImportCutDTO struct {
	Row             int    `json:"row"`
	Name            string `json:"name"`
	DateTimeLocal   string `json:"datetimeLocal"`
	SecondsBefore   int    `json:"secondsBefore"`
	SecondsAfter    int    `json:"secondsAfter"`
	OutputDirectory string `json:"outputDirectory,omitempty"`
	TotalSegments   int    `json:"totalSegments,omitempty"`
}

type PreviewCutsFromCSVRequest struct {
	WorkspaceID int64 `json:"workspaceId"`
}

type PreviewCutsFromCSVResponse struct {
	FileName  string              `json:"fileName,omitempty"`
	Cuts      []CSVImportCutDTO   `json:"cuts,omitempty"`
	Issues    []CSVImportIssueDTO `json:"issues,omitempty"`
	Cancelled bool                `json:"cancelled"`
	Error     *AppError           `json:"error,omitempty"`
}

type StartCutsFromCSVRequest struct {
	WorkspaceID int64             `json:"workspaceId"`
	Cuts        []CSVImportCutDTO `json:"cuts"`
}

type StartCutsFromCSVResponse struct {
	JobID string    `json:"jobId,omitempty"`
	Error *AppError `json:"error,omitempty"`
}
