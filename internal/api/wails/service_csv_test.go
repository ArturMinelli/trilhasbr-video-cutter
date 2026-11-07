package wailsapi

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"trilhasbr-video-cutter/internal/app/actions"
	"trilhasbr-video-cutter/internal/domain"
)

type serviceCSVPickerStub struct{}

func (s *serviceCSVPickerStub) PickCSVFile(context.Context) (string, error) {
	return "", nil
}

type serviceCSVWorkspaceRepoStub struct {
	workspace domain.Workspace
}

func (s *serviceCSVWorkspaceRepoStub) UpsertByDirectory(context.Context, string, string) (domain.Workspace, error) {
	return domain.Workspace{}, nil
}

func (s *serviceCSVWorkspaceRepoStub) ListWithStats(context.Context) ([]domain.WorkspaceSummary, error) {
	return nil, nil
}

func (s *serviceCSVWorkspaceRepoStub) GetByID(context.Context, int64) (domain.Workspace, error) {
	return s.workspace, nil
}

type serviceCSVPlannerStub struct{}

func (s *serviceCSVPlannerStub) BuildPlanWithWorkspace(context.Context, domain.Workspace, actions.StartCutInput) (actions.CutRunPlan, error) {
	return actions.CutRunPlan{}, nil
}

type serviceCSVRunnerStub struct{}

func (s *serviceCSVRunnerStub) BuildPlanWithWorkspace(_ context.Context, workspace domain.Workspace, input actions.StartCutInput) (actions.CutRunPlan, error) {
	return actions.CutRunPlan{
		Workspace: workspace,
		Input:     input,
		Plans:     []domain.SegmentPlan{{VideoID: 1, VideoPath: "video.mp4", SegmentDurationSec: 1}},
		OutputDir: filepath.Join(workspace.Directory, "exports", "row"),
	}, nil
}

func (s *serviceCSVRunnerStub) ExecutePlanned(_ context.Context, _ string, plan actions.CutRunPlan, _ func(actions.CutProgressEvent)) (domain.CutResult, error) {
	return domain.CutResult{OutputDirectory: plan.OutputDir}, nil
}

type serviceCSVBridgeStub struct {
	csvProgress chan actions.CutCSVProgressEvent
	csvFinished chan actions.CutCSVFinishedEvent
	csvFailed   chan actions.CutCSVFailedEvent
}

func (s *serviceCSVBridgeStub) PublishProgress(context.Context, actions.CutProgressEvent) {}
func (s *serviceCSVBridgeStub) PublishFinished(context.Context, actions.CutFinishedEvent) {}
func (s *serviceCSVBridgeStub) PublishFailed(context.Context, actions.CutFailedEvent)     {}

func (s *serviceCSVBridgeStub) PublishCSVProgress(_ context.Context, event actions.CutCSVProgressEvent) {
	s.csvProgress <- event
}

func (s *serviceCSVBridgeStub) PublishCSVFinished(_ context.Context, event actions.CutCSVFinishedEvent) {
	s.csvFinished <- event
}

func (s *serviceCSVBridgeStub) PublishCSVFailed(_ context.Context, event actions.CutCSVFailedEvent) {
	s.csvFailed <- event
}

func TestService_PreviewCutsFromCSV_Cancelled(t *testing.T) {
	service := NewService(ServiceParams{
		CutCSVPreview: &actions.CutCSVPreviewAction{
			Picker:    &serviceCSVPickerStub{},
			Workspace: &serviceCSVWorkspaceRepoStub{workspace: domain.Workspace{ID: 1, Directory: t.TempDir()}},
			Planner:   &serviceCSVPlannerStub{},
		},
	})

	response := service.PreviewCutsFromCSV(PreviewCutsFromCSVRequest{WorkspaceID: 1})
	if response.Error != nil {
		t.Fatalf("expected nil error, got %+v", response.Error)
	}
	if !response.Cancelled {
		t.Fatalf("expected cancelled=true")
	}
}

func TestService_StartCutsFromCSV_EmitsEvents(t *testing.T) {
	bridge := &serviceCSVBridgeStub{
		csvProgress: make(chan actions.CutCSVProgressEvent, 4),
		csvFinished: make(chan actions.CutCSVFinishedEvent, 1),
		csvFailed:   make(chan actions.CutCSVFailedEvent, 1),
	}

	workspaceDir := t.TempDir()
	service := NewService(ServiceParams{
		Bridge: bridge,
		CutCSVExecute: &actions.CutCSVExecuteAction{
			Workspace: &serviceCSVWorkspaceRepoStub{workspace: domain.Workspace{ID: 1, Directory: workspaceDir}},
			Runner:    &serviceCSVRunnerStub{},
		},
	})

	response := service.StartCutsFromCSV(StartCutsFromCSVRequest{
		WorkspaceID: 1,
		Cuts: []CSVImportCutDTO{{
			Row:           2,
			Name:          "first",
			DateTimeLocal: "2026-03-04T14:00:00",
			SecondsBefore: 10,
			SecondsAfter:  10,
		}},
	})
	if response.Error != nil {
		t.Fatalf("unexpected start error: %+v", response.Error)
	}
	if response.JobID == "" {
		t.Fatalf("expected non-empty job id")
	}

	select {
	case finished := <-bridge.csvFinished:
		if finished.JobID != response.JobID {
			t.Fatalf("finished event job mismatch. expected=%s got=%s", response.JobID, finished.JobID)
		}
		if finished.CompletedCuts != 1 {
			t.Fatalf("expected completedCuts=1, got %+v", finished)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for csv finished event")
	}

	select {
	case failed := <-bridge.csvFailed:
		t.Fatalf("did not expect failed event: %+v", failed)
	default:
	}
}

func (s *serviceCSVWorkspaceRepoStub) Delete(context.Context, int64) error { return nil }
