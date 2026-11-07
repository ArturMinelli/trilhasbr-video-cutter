package actions

import (
	"testing"
	"time"

	"trilhasbr-video-cutter/internal/domain"
)

func TestBuildSegmentPlans_MultiVideoWindow(t *testing.T) {
	base := time.Date(2026, 3, 4, 14, 0, 0, 0, time.UTC)
	v1Start := base
	v2Start := base.Add(50 * time.Second)

	videos := []domain.Video{
		{ID: 1, Path: "/tmp/a.mp4", StartedAt: &v1Start, DurationSeconds: 60},
		{ID: 2, Path: "/tmp/b.mp4", StartedAt: &v2Start, DurationSeconds: 40},
	}

	windowStart := base.Add(40 * time.Second)
	windowEnd := base.Add(70 * time.Second)

	plans := BuildSegmentPlans(videos, windowStart, windowEnd)
	if len(plans) != 2 {
		t.Fatalf("expected 2 segment plans, got %d", len(plans))
	}

	if plans[0].VideoID != 1 {
		t.Fatalf("expected first plan to use first video, got video %d", plans[0].VideoID)
	}
	if plans[0].SegmentStartSec != 40 {
		t.Fatalf("expected first segment start=40, got %.3f", plans[0].SegmentStartSec)
	}
	if plans[0].SegmentDurationSec != 20 {
		t.Fatalf("expected first segment duration=20, got %.3f", plans[0].SegmentDurationSec)
	}

	if plans[1].VideoID != 2 {
		t.Fatalf("expected second plan to use second video, got video %d", plans[1].VideoID)
	}
	if plans[1].SegmentStartSec != 0 {
		t.Fatalf("expected second segment start=0, got %.3f", plans[1].SegmentStartSec)
	}
	if plans[1].SegmentDurationSec != 20 {
		t.Fatalf("expected second segment duration=20, got %.3f", plans[1].SegmentDurationSec)
	}
}

func TestBuildSegmentPlans_EmptyForNoOverlap(t *testing.T) {
	base := time.Date(2026, 3, 4, 14, 0, 0, 0, time.UTC)
	start := base
	videos := []domain.Video{
		{ID: 1, Path: "/tmp/a.mp4", StartedAt: &start, DurationSeconds: 10},
	}

	windowStart := base.Add(20 * time.Second)
	windowEnd := base.Add(25 * time.Second)

	plans := BuildSegmentPlans(videos, windowStart, windowEnd)
	if len(plans) != 0 {
		t.Fatalf("expected no plans, got %d", len(plans))
	}
}
