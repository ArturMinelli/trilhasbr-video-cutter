package actions

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"trilhasbr-video-cutter/internal/app/csvimport"
	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/domain"
	"trilhasbr-video-cutter/internal/repository"
)

type CSVFilePicker interface {
	PickCSVFile(ctx context.Context) (string, error)
}

type CSVPreflightPlanner interface {
	BuildPlanWithWorkspace(ctx context.Context, workspace domain.Workspace, input StartCutInput) (CutRunPlan, error)
}

type PreviewCutsFromCSVInput struct {
	WorkspaceID int64 `json:"workspaceId"`
}

type CSVImportIssue struct {
	Row     int    `json:"row"`
	Column  string `json:"column"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

type CSVImportCut struct {
	Row             int    `json:"row"`
	Name            string `json:"name"`
	DateTimeLocal   string `json:"datetimeLocal"`
	SecondsBefore   int    `json:"secondsBefore"`
	SecondsAfter    int    `json:"secondsAfter"`
	OutputDirectory string `json:"outputDirectory"`
	TotalSegments   int    `json:"totalSegments"`
}

type PreviewCutsFromCSVResult struct {
	FileName  string           `json:"fileName,omitempty"`
	Cuts      []CSVImportCut   `json:"cuts,omitempty"`
	Issues    []CSVImportIssue `json:"issues,omitempty"`
	Cancelled bool             `json:"cancelled"`
}

type CutCSVPreviewAction struct {
	Picker    CSVFilePicker
	Workspace repository.WorkspaceRepository
	Planner   CSVPreflightPlanner
}

func (a *CutCSVPreviewAction) Execute(ctx context.Context, input PreviewCutsFromCSVInput) (PreviewCutsFromCSVResult, error) {
	if input.WorkspaceID <= 0 {
		return PreviewCutsFromCSVResult{}, apperrors.New(apperrors.CodeValidation, "workspaceId must be greater than 0", nil)
	}
	if a.Picker == nil {
		return PreviewCutsFromCSVResult{}, apperrors.New(apperrors.CodeInternal, "csv picker is not configured", nil)
	}
	if a.Workspace == nil {
		return PreviewCutsFromCSVResult{}, apperrors.New(apperrors.CodeInternal, "workspace repository is not configured", nil)
	}
	if a.Planner == nil {
		return PreviewCutsFromCSVResult{}, apperrors.New(apperrors.CodeInternal, "cut planner is not configured", nil)
	}

	selectedPath, err := a.Picker.PickCSVFile(ctx)
	if err != nil {
		return PreviewCutsFromCSVResult{}, err
	}
	if strings.TrimSpace(selectedPath) == "" {
		return PreviewCutsFromCSVResult{Cancelled: true}, nil
	}

	file, err := os.Open(selectedPath)
	if err != nil {
		return PreviewCutsFromCSVResult{}, apperrors.Wrap(apperrors.CodeIO, "failed to open selected CSV file", err, map[string]any{"path": selectedPath})
	}
	defer file.Close()

	parseResult := csvimport.ParseCuts(file)
	result := PreviewCutsFromCSVResult{
		FileName: filepath.Base(selectedPath),
		Cuts:     make([]CSVImportCut, 0),
		Issues:   make([]CSVImportIssue, 0),
	}
	for _, issue := range parseResult.Issues {
		result.Issues = append(result.Issues, CSVImportIssue(issue))
	}
	if len(result.Issues) > 0 {
		sortIssues(result.Issues)
		return result, nil
	}

	workspace, err := a.Workspace.GetByID(ctx, input.WorkspaceID)
	if err != nil {
		return PreviewCutsFromCSVResult{}, err
	}

	firstFolderRow := map[string]int{}
	duplicateRows := map[int]bool{}
	for _, row := range parseResult.Rows {
		sanitized := sanitizeFolderName(row.Name)
		if sanitized == "" {
			result.Issues = append(result.Issues, CSVImportIssue{Row: row.Row, Column: "name", Message: "name resulted in an invalid folder name"})
			continue
		}

		key := strings.ToLower(sanitized)
		if firstRow, ok := firstFolderRow[key]; ok {
			duplicateRows[row.Row] = true
			result.Issues = append(result.Issues, CSVImportIssue{
				Row:     row.Row,
				Column:  "name",
				Message: "duplicate output folder name in CSV",
				Value:   "conflicts with row " + strconv.Itoa(firstRow),
			})
			continue
		}
		firstFolderRow[key] = row.Row
	}

	for _, row := range parseResult.Rows {
		if duplicateRows[row.Row] {
			continue
		}

		plan, planErr := a.Planner.BuildPlanWithWorkspace(ctx, workspace, StartCutInput{
			WorkspaceID:   workspace.ID,
			Name:          row.Name,
			DateTimeLocal: row.DateTime,
			SecondsBefore: row.SecondsBefore,
			SecondsAfter:  row.SecondsAfter,
		})
		if planErr != nil {
			appErr := apperrors.As(planErr)
			if appErr.Code == apperrors.CodeValidation {
				result.Issues = append(result.Issues, CSVImportIssue{Row: row.Row, Column: "row", Message: appErr.Message})
				continue
			}
			return PreviewCutsFromCSVResult{}, planErr
		}

		result.Cuts = append(result.Cuts, CSVImportCut{
			Row:             row.Row,
			Name:            row.Name,
			DateTimeLocal:   row.DateTime,
			SecondsBefore:   row.SecondsBefore,
			SecondsAfter:    row.SecondsAfter,
			OutputDirectory: plan.OutputDir,
			TotalSegments:   len(plan.Plans),
		})
	}

	sortIssues(result.Issues)
	return result, nil
}

func sortIssues(issues []CSVImportIssue) {
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Row == issues[j].Row {
			return issues[i].Column < issues[j].Column
		}
		return issues[i].Row < issues[j].Row
	})
}
