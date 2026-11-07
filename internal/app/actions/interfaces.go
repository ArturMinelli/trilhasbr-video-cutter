package actions

import (
	"context"

	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/domain"
)

type DirectoryPicker interface {
	PickDirectory(ctx context.Context) (string, error)
}

type CutProgressEvent struct {
	JobID             string `json:"jobId"`
	TotalSegments     int    `json:"totalSegments"`
	CompletedSegments int    `json:"completedSegments"`
	CurrentFile       string `json:"currentFile"`
}

type CutFinishedEvent struct {
	JobID  string           `json:"jobId"`
	Result domain.CutResult `json:"result"`
}

type CutFailedEvent struct {
	JobID string                 `json:"jobId"`
	Error *apperrors.PublicError `json:"error"`
}

type CutCSVProgressEvent struct {
	JobID         string `json:"jobId"`
	TotalCuts     int    `json:"totalCuts"`
	CompletedCuts int    `json:"completedCuts"`
	CurrentRow    int    `json:"currentRow"`
	CurrentName   string `json:"currentName"`
}

type CutCSVFinishedEvent struct {
	JobID           string `json:"jobId"`
	OutputDirectory string `json:"outputDirectory"`
	TotalCuts       int    `json:"totalCuts"`
	CompletedCuts   int    `json:"completedCuts"`
}

type CutCSVFailedEvent struct {
	JobID string                 `json:"jobId"`
	Row   int                    `json:"row"`
	Name  string                 `json:"name"`
	Error *apperrors.PublicError `json:"error"`
}

type CutEventPublisher interface {
	PublishProgress(ctx context.Context, event CutProgressEvent)
	PublishFinished(ctx context.Context, event CutFinishedEvent)
	PublishFailed(ctx context.Context, event CutFailedEvent)
}
