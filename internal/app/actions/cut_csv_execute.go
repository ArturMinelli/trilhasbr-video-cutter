package actions

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/domain"
	"trilhasbr-video-cutter/internal/repository"
)

type StartCutsFromCSVItem struct {
	Row           int    `json:"row"`
	Name          string `json:"name"`
	DateTimeLocal string `json:"datetimeLocal"`
	SecondsBefore int    `json:"secondsBefore"`
	SecondsAfter  int    `json:"secondsAfter"`
}

type StartCutsFromCSVInput struct {
	WorkspaceID int64                  `json:"workspaceId"`
	Cuts        []StartCutsFromCSVItem `json:"cuts"`
}

type StartCutsFromCSVResult struct {
	JobID           string `json:"jobId"`
	OutputDirectory string `json:"outputDirectory"`
	TotalCuts       int    `json:"totalCuts"`
	CompletedCuts   int    `json:"completedCuts"`
}

type CSVBatchRunner interface {
	BuildPlanWithWorkspace(ctx context.Context, workspace domain.Workspace, input StartCutInput) (CutRunPlan, error)
	ExecutePlanned(ctx context.Context, jobID string, plan CutRunPlan, onProgress func(CutProgressEvent)) (domain.CutResult, error)
}

type CutCSVExecuteAction struct {
	Workspace repository.WorkspaceRepository
	Runner    CSVBatchRunner
}

func (a *CutCSVExecuteAction) Execute(ctx context.Context, jobID string, input StartCutsFromCSVInput, onProgress func(CutCSVProgressEvent)) (StartCutsFromCSVResult, error) {
	if input.WorkspaceID <= 0 {
		return StartCutsFromCSVResult{}, apperrors.New(apperrors.CodeValidation, "workspaceId must be greater than 0", nil)
	}
	if len(input.Cuts) == 0 {
		return StartCutsFromCSVResult{}, apperrors.New(apperrors.CodeValidation, "cuts list is required", nil)
	}
	if a.Workspace == nil {
		return StartCutsFromCSVResult{}, apperrors.New(apperrors.CodeInternal, "workspace repository is not configured", nil)
	}
	if a.Runner == nil {
		return StartCutsFromCSVResult{}, apperrors.New(apperrors.CodeInternal, "csv cut runner is not configured", nil)
	}

	workspace, err := a.Workspace.GetByID(ctx, input.WorkspaceID)
	if err != nil {
		return StartCutsFromCSVResult{}, err
	}

	completed := 0
	for index, cut := range input.Cuts {
		startInput := StartCutInput{
			WorkspaceID:   workspace.ID,
			Name:          strings.TrimSpace(cut.Name),
			DateTimeLocal: strings.TrimSpace(cut.DateTimeLocal),
			SecondsBefore: cut.SecondsBefore,
			SecondsAfter:  cut.SecondsAfter,
		}

		plan, planErr := a.Runner.BuildPlanWithWorkspace(ctx, workspace, startInput)
		if planErr != nil {
			return StartCutsFromCSVResult{}, withCSVRowContext(planErr, cut.Row, cut.Name)
		}

		_, executeErr := a.Runner.ExecutePlanned(ctx, fmt.Sprintf("%s-%d", jobID, index+1), plan, nil)
		if executeErr != nil {
			return StartCutsFromCSVResult{}, withCSVRowContext(executeErr, cut.Row, cut.Name)
		}

		completed++
		if onProgress != nil {
			onProgress(CutCSVProgressEvent{
				JobID:         jobID,
				TotalCuts:     len(input.Cuts),
				CompletedCuts: completed,
				CurrentRow:    cut.Row,
				CurrentName:   cut.Name,
			})
		}
	}

	return StartCutsFromCSVResult{
		JobID:           jobID,
		OutputDirectory: filepath.Join(workspace.Directory, "exports"),
		TotalCuts:       len(input.Cuts),
		CompletedCuts:   completed,
	}, nil
}

func withCSVRowContext(err error, row int, name string) error {
	base := apperrors.As(err)
	details := map[string]any{
		"row":  row,
		"name": name,
	}
	for key, value := range base.Details {
		details[key] = value
	}

	message := fmt.Sprintf("row %d (%s): %s", row, strings.TrimSpace(name), base.Message)
	if strings.TrimSpace(name) == "" {
		message = fmt.Sprintf("row %d: %s", row, base.Message)
	}
	return apperrors.Wrap(base.Code, message, err, details)
}
