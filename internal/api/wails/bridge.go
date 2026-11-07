package wailsapi

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"trilhasbr-video-cutter/internal/app/actions"
)

type RuntimeBridge struct{}

func NewRuntimeBridge() *RuntimeBridge {
	return &RuntimeBridge{}
}

func (b *RuntimeBridge) PickDirectory(ctx context.Context) (string, error) {
	return runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{
		Title: "Select workspace directory",
	})
}

func (b *RuntimeBridge) PickCSVFile(ctx context.Context) (string, error) {
	return runtime.OpenFileDialog(ctx, runtime.OpenDialogOptions{
		Title: "Select cuts CSV file",
		Filters: []runtime.FileFilter{
			{DisplayName: "CSV files (*.csv)", Pattern: "*.csv"},
			{DisplayName: "All files", Pattern: "*"},
		},
	})
}

func (b *RuntimeBridge) PublishProgress(ctx context.Context, event actions.CutProgressEvent) {
	runtime.EventsEmit(ctx, "cut:progress", event)
}

func (b *RuntimeBridge) PublishFinished(ctx context.Context, event actions.CutFinishedEvent) {
	runtime.EventsEmit(ctx, "cut:finished", event)
}

func (b *RuntimeBridge) PublishFailed(ctx context.Context, event actions.CutFailedEvent) {
	runtime.EventsEmit(ctx, "cut:failed", event)
}

func (b *RuntimeBridge) PublishCSVProgress(ctx context.Context, event actions.CutCSVProgressEvent) {
	runtime.EventsEmit(ctx, "cut:csv:progress", event)
}

func (b *RuntimeBridge) PublishCSVFinished(ctx context.Context, event actions.CutCSVFinishedEvent) {
	runtime.EventsEmit(ctx, "cut:csv:finished", event)
}

func (b *RuntimeBridge) PublishCSVFailed(ctx context.Context, event actions.CutCSVFailedEvent) {
	runtime.EventsEmit(ctx, "cut:csv:failed", event)
}

func (b *RuntimeBridge) PublishDetectProgress(ctx context.Context, event actions.DetectProgressEvent) {
	runtime.EventsEmit(ctx, "detect:progress", event)
}

func (b *RuntimeBridge) PublishDetectFinished(ctx context.Context, event actions.DetectFinishedEvent) {
	runtime.EventsEmit(ctx, "detect:finished", event)
}

func (b *RuntimeBridge) PublishDetectFailed(ctx context.Context, event actions.DetectFailedEvent) {
	runtime.EventsEmit(ctx, "detect:failed", event)
}
