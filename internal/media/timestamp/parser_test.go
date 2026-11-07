package timestamp

import (
	"testing"
	"time"
)

func TestParseMetadataTime_RFC3339(t *testing.T) {
	parsed, err := ParseMetadataTime("2026-03-04T13:15:21Z")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if parsed == nil {
		t.Fatal("expected parsed value")
	}
	if parsed.UTC().Format(time.RFC3339) != "2026-03-04T13:15:21Z" {
		t.Fatalf("unexpected parse result: %s", parsed.UTC().Format(time.RFC3339))
	}
}

func TestParseFilenameTime_CommonPatterns(t *testing.T) {
	loc := time.FixedZone("-0300", -3*60*60)

	value, ok := ParseFilenameTime("CAM_20260304_142530.mp4", loc)
	if !ok || value == nil {
		t.Fatal("expected to parse CAM_20260304_142530.mp4")
	}
	if value.UTC().Format(time.RFC3339) != "2026-03-04T17:25:30Z" {
		t.Fatalf("unexpected timestamp %s", value.UTC().Format(time.RFC3339))
	}

	value2, ok2 := ParseFilenameTime("clip-2026-03-04T09-10-11.mp4", loc)
	if !ok2 || value2 == nil {
		t.Fatal("expected to parse clip-2026-03-04T09-10-11.mp4")
	}
	if value2.UTC().Format(time.RFC3339) != "2026-03-04T12:10:11Z" {
		t.Fatalf("unexpected timestamp %s", value2.UTC().Format(time.RFC3339))
	}
}
