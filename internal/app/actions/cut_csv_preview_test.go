package actions

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"trilhasbr-video-cutter/internal/domain"
)

type csvPickerStub struct {
	path string
	err  error
}

func (s *csvPickerStub) PickCSVFile(context.Context) (string, error) {
	return s.path, s.err
}

type previewWorkspaceRepoStub struct {
	workspace domain.Workspace
}

func (s *previewWorkspaceRepoStub) UpsertByDirectory(context.Context, string, string) (domain.Workspace, error) {
	return domain.Workspace{}, nil
}

func (s *previewWorkspaceRepoStub) ListWithStats(context.Context) ([]domain.WorkspaceSummary, error) {
	return nil, nil
}

func (s *previewWorkspaceRepoStub) GetByID(context.Context, int64) (domain.Workspace, error) {
	return s.workspace, nil
}

type previewPlannerStub struct {
	plan CutRunPlan
	err  error
}

func (s *previewPlannerStub) BuildPlanWithWorkspace(context.Context, domain.Workspace, StartCutInput) (CutRunPlan, error) {
	return s.plan, s.err
}

type csvPreviewVideoRepoStub struct {
	videos []domain.Video
}

func (s *csvPreviewVideoRepoStub) ReplaceWorkspaceVideos(context.Context, int64, []domain.VideoInsert) error {
	return nil
}

func (s *csvPreviewVideoRepoStub) ListByWorkspace(context.Context, int64, int, int) ([]domain.Video, int, error) {
	return nil, 0, nil
}

func (s *csvPreviewVideoRepoStub) ListByWorkspaceAll(context.Context, int64) ([]domain.Video, error) {
	return nil, nil
}

func (s *csvPreviewVideoRepoStub) GetByID(context.Context, int64) (domain.Video, error) {
	return domain.Video{}, nil
}

func (s *csvPreviewVideoRepoStub) DeleteWorkspaceExportVideos(context.Context, int64, string) (int, error) {
	return 0, nil
}

func (s *csvPreviewVideoRepoStub) FindOverlapping(context.Context, int64, time.Time, time.Time) ([]domain.Video, error) {
	return s.videos, nil
}

func (s *csvPreviewVideoRepoStub) ShiftVideoTimes(context.Context, int64, []int64, time.Duration) (int, error) {
	return 0, nil
}

func TestCutCSVPreviewAction_DetectsDuplicateSanitizedNames(t *testing.T) {
	tempDir := t.TempDir()
	csvPath := filepath.Join(tempDir, "cuts.csv")
	content := "name,datetime,seconds_before,seconds_after\ncut:one,2026-03-04T14:00:00,10,10\ncut/one,2026-03-04T14:01:00,10,10\n"
	if err := os.WriteFile(csvPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write csv: %v", err)
	}

	action := &CutCSVPreviewAction{
		Picker:    &csvPickerStub{path: csvPath},
		Workspace: &previewWorkspaceRepoStub{workspace: domain.Workspace{ID: 1, Directory: tempDir}},
		Planner:   &previewPlannerStub{plan: CutRunPlan{OutputDir: filepath.Join(tempDir, "exports", "ok"), Plans: []domain.SegmentPlan{{VideoID: 1}}}},
	}

	result, err := action.Execute(context.Background(), PreviewCutsFromCSVInput{WorkspaceID: 1})
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if len(result.Issues) == 0 {
		t.Fatalf("expected duplicate-name issue")
	}

	found := false
	for _, issue := range result.Issues {
		if strings.Contains(issue.Message, "duplicate output folder name") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected duplicate output folder issue, got %+v", result.Issues)
	}
}

func TestCutCSVPreviewAction_RejectsExistingNonEmptyOutputFolder(t *testing.T) {
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "exports", "my-cut")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "existing.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("failed to write existing output file: %v", err)
	}

	csvPath := filepath.Join(tempDir, "cuts.csv")
	content := "name,datetime,seconds_before,seconds_after\nmy-cut,2026-03-04T14:00:00,10,10\n"
	if err := os.WriteFile(csvPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write csv: %v", err)
	}

	videoStart := time.Date(2026, 3, 4, 13, 59, 0, 0, time.Local)
	action := &CutCSVPreviewAction{
		Picker:    &csvPickerStub{path: csvPath},
		Workspace: &previewWorkspaceRepoStub{workspace: domain.Workspace{ID: 1, Directory: tempDir}},
		Planner: &CutExecuteAction{
			VideoRepo: &csvPreviewVideoRepoStub{videos: []domain.Video{{ID: 1, Path: filepath.Join(tempDir, "video.mp4"), StartedAt: &videoStart, DurationSeconds: 180}}},
		},
	}

	result, err := action.Execute(context.Background(), PreviewCutsFromCSVInput{WorkspaceID: 1})
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if len(result.Issues) == 0 {
		t.Fatalf("expected non-empty folder issue")
	}
	if !strings.Contains(result.Issues[0].Message, "output folder already exists and is not empty") {
		t.Fatalf("unexpected issue: %+v", result.Issues[0])
	}
}

func TestCutCSVPreviewAction_RejectsRowsWithNoOverlap(t *testing.T) {
	tempDir := t.TempDir()
	csvPath := filepath.Join(tempDir, "cuts.csv")
	content := "name,datetime,seconds_before,seconds_after\nmy-cut,2026-03-04T14:00:00,10,10\n"
	if err := os.WriteFile(csvPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write csv: %v", err)
	}

	action := &CutCSVPreviewAction{
		Picker:    &csvPickerStub{path: csvPath},
		Workspace: &previewWorkspaceRepoStub{workspace: domain.Workspace{ID: 1, Directory: tempDir}},
		Planner: &CutExecuteAction{
			VideoRepo: &csvPreviewVideoRepoStub{videos: nil},
		},
	}

	result, err := action.Execute(context.Background(), PreviewCutsFromCSVInput{WorkspaceID: 1})
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if len(result.Issues) == 0 {
		t.Fatalf("expected no-overlap issue")
	}
	if !strings.Contains(result.Issues[0].Message, "no videos overlap the requested time window") {
		t.Fatalf("unexpected issue: %+v", result.Issues[0])
	}
}

func (s *previewWorkspaceRepoStub) Delete(context.Context, int64) error { return nil }
