package actions

import (
	"context"
	"path/filepath"
	"testing"

	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/domain"
)

type csvExecuteWorkspaceRepoStub struct {
	workspace domain.Workspace
}

func (s *csvExecuteWorkspaceRepoStub) UpsertByDirectory(context.Context, string, string) (domain.Workspace, error) {
	return domain.Workspace{}, nil
}

func (s *csvExecuteWorkspaceRepoStub) ListWithStats(context.Context) ([]domain.WorkspaceSummary, error) {
	return nil, nil
}

func (s *csvExecuteWorkspaceRepoStub) GetByID(context.Context, int64) (domain.Workspace, error) {
	return s.workspace, nil
}

type csvBatchRunnerStub struct {
	buildCalls   []StartCutInput
	executeCalls []CutRunPlan
	failOnName   string
}

func (s *csvBatchRunnerStub) BuildPlanWithWorkspace(_ context.Context, workspace domain.Workspace, input StartCutInput) (CutRunPlan, error) {
	s.buildCalls = append(s.buildCalls, input)
	return CutRunPlan{
		Workspace: workspace,
		Input:     input,
		Plans:     []domain.SegmentPlan{{VideoID: 1, VideoPath: "video.mp4", SegmentDurationSec: 1}},
		OutputDir: filepath.Join(workspace.Directory, "exports", sanitizeFolderName(input.Name)),
	}, nil
}

func (s *csvBatchRunnerStub) ExecutePlanned(_ context.Context, _ string, plan CutRunPlan, _ func(CutProgressEvent)) (domain.CutResult, error) {
	s.executeCalls = append(s.executeCalls, plan)
	if plan.Input.Name == s.failOnName {
		return domain.CutResult{}, apperrors.New(apperrors.CodeCut, "simulated cut failure", nil)
	}
	return domain.CutResult{OutputDirectory: plan.OutputDir}, nil
}

func TestCutCSVExecuteAction_Success(t *testing.T) {
	workspaceDir := t.TempDir()
	runner := &csvBatchRunnerStub{}
	action := &CutCSVExecuteAction{
		Workspace: &csvExecuteWorkspaceRepoStub{workspace: domain.Workspace{ID: 1, Directory: workspaceDir}},
		Runner:    runner,
	}

	progressEvents := make([]CutCSVProgressEvent, 0)
	result, err := action.Execute(context.Background(), "job-1", StartCutsFromCSVInput{
		WorkspaceID: 1,
		Cuts: []StartCutsFromCSVItem{
			{Row: 2, Name: "first", DateTimeLocal: "2026-03-04T14:00:00", SecondsBefore: 10, SecondsAfter: 10},
			{Row: 3, Name: "second", DateTimeLocal: "2026-03-04T14:01:00", SecondsBefore: 10, SecondsAfter: 10},
		},
	}, func(event CutCSVProgressEvent) {
		progressEvents = append(progressEvents, event)
	})
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}

	if result.CompletedCuts != 2 || result.TotalCuts != 2 {
		t.Fatalf("unexpected result counters: %+v", result)
	}
	expectedOutputDir := filepath.Join(workspaceDir, "exports")
	if result.OutputDirectory != expectedOutputDir {
		t.Fatalf("unexpected output directory. expected=%s got=%s", expectedOutputDir, result.OutputDirectory)
	}
	if len(runner.executeCalls) != 2 {
		t.Fatalf("expected 2 execute calls, got %d", len(runner.executeCalls))
	}
	if len(progressEvents) != 2 {
		t.Fatalf("expected 2 progress events, got %d", len(progressEvents))
	}
	if progressEvents[1].CompletedCuts != 2 {
		t.Fatalf("expected second progress completed=2, got %+v", progressEvents[1])
	}
}

func TestCutCSVExecuteAction_FailFastWithRowContext(t *testing.T) {
	workspaceDir := t.TempDir()
	runner := &csvBatchRunnerStub{failOnName: "second"}
	action := &CutCSVExecuteAction{
		Workspace: &csvExecuteWorkspaceRepoStub{workspace: domain.Workspace{ID: 1, Directory: workspaceDir}},
		Runner:    runner,
	}

	_, err := action.Execute(context.Background(), "job-2", StartCutsFromCSVInput{
		WorkspaceID: 1,
		Cuts: []StartCutsFromCSVItem{
			{Row: 2, Name: "first", DateTimeLocal: "2026-03-04T14:00:00", SecondsBefore: 10, SecondsAfter: 10},
			{Row: 3, Name: "second", DateTimeLocal: "2026-03-04T14:01:00", SecondsBefore: 10, SecondsAfter: 10},
			{Row: 4, Name: "third", DateTimeLocal: "2026-03-04T14:02:00", SecondsBefore: 10, SecondsAfter: 10},
		},
	}, nil)
	if err == nil {
		t.Fatalf("expected execution error")
	}

	appErr := apperrors.As(err)
	if appErr.Details["row"] != 3 {
		t.Fatalf("expected row detail = 3, got %+v", appErr.Details)
	}
	if appErr.Details["name"] != "second" {
		t.Fatalf("expected name detail = second, got %+v", appErr.Details)
	}
	if len(runner.executeCalls) != 2 {
		t.Fatalf("expected fail-fast after second row, got %d execute calls", len(runner.executeCalls))
	}
}

func (s *csvExecuteWorkspaceRepoStub) Delete(context.Context, int64) error { return nil }
