package csvimport

import (
	"strings"
	"testing"
)

func TestParseCuts_ValidCSV(t *testing.T) {
	data := "name,datetime,seconds_before,seconds_after\ncut-1,2026-03-04T14:00:00,10,15\n"
	result := ParseCuts(strings.NewReader(data))

	if len(result.Issues) > 0 {
		t.Fatalf("expected no issues, got %+v", result.Issues)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	if result.Rows[0].Name != "cut-1" {
		t.Fatalf("unexpected row name: %s", result.Rows[0].Name)
	}
}

func TestParseCuts_MissingHeader(t *testing.T) {
	data := "name,seconds_before,seconds_after\ncut-1,10,15\n"
	result := ParseCuts(strings.NewReader(data))

	if len(result.Issues) == 0 {
		t.Fatalf("expected issues for missing header")
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Column == "datetime" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected missing datetime issue, got %+v", result.Issues)
	}
}

func TestParseCuts_WrongColumnCount(t *testing.T) {
	data := "name,datetime,seconds_before,seconds_after\ncut-1,2026-03-04T14:00:00,10\n"
	result := ParseCuts(strings.NewReader(data))

	if len(result.Issues) == 0 {
		t.Fatalf("expected issue for wrong column count")
	}
	if result.Issues[0].Column != "row" {
		t.Fatalf("expected row issue, got %+v", result.Issues[0])
	}
}

func TestParseCuts_InvalidInteger(t *testing.T) {
	data := "name,datetime,seconds_before,seconds_after\ncut-1,2026-03-04T14:00:00,abc,15\n"
	result := ParseCuts(strings.NewReader(data))

	if len(result.Issues) == 0 {
		t.Fatalf("expected issue for invalid integer")
	}
	if result.Issues[0].Column != "seconds_before" {
		t.Fatalf("expected seconds_before issue, got %+v", result.Issues[0])
	}
}

func TestParseCuts_InvalidDatetime(t *testing.T) {
	data := "name,datetime,seconds_before,seconds_after\ncut-1,not-a-date,10,15\n"
	result := ParseCuts(strings.NewReader(data))

	if len(result.Issues) == 0 {
		t.Fatalf("expected issue for invalid datetime")
	}
	if result.Issues[0].Column != "datetime" {
		t.Fatalf("expected datetime issue, got %+v", result.Issues[0])
	}
}
