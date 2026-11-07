package actions

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"trilhasbr-video-cutter/internal/domain"
)

func testGroup(id int64, tracks ...domain.Track) domain.PersonGroup {
	return domain.PersonGroup{ID: id, Tracks: tracks}
}

func TestBuildPersonCutPlan_ClampAndPadding(t *testing.T) {
	videos := map[int64]domain.Video{1: {ID: 1, Path: "/a.mp4", DurationSeconds: 30}}
	groups := []domain.PersonGroup{
		testGroup(1, domain.Track{VideoID: 1, StartSec: 1, EndSec: 28}),
		testGroup(2, domain.Track{VideoID: 1, StartSec: 5, EndSec: 10}),
	}

	plans := BuildPersonCutPlan(groups, videos, 3, 3)
	if len(plans) != 2 {
		t.Fatalf("expected 2 plans, got %d", len(plans))
	}
	first := plans[0].Segments[0]
	if first.SegmentStartSec != 0 || first.SegmentDurationSec != 30 {
		t.Fatalf("expected clamp to [0,30], got start=%.1f dur=%.1f", first.SegmentStartSec, first.SegmentDurationSec)
	}
	second := plans[1].Segments[0]
	if second.SegmentStartSec != 2 || second.SegmentDurationSec != 11 {
		t.Fatalf("expected [2,13], got start=%.1f dur=%.1f", second.SegmentStartSec, second.SegmentDurationSec)
	}

	zero := BuildPersonCutPlan(groups[1:], videos, 0, 0)
	if s := zero[0].Segments[0]; s.SegmentStartSec != 5 || s.SegmentDurationSec != 5 {
		t.Fatalf("padding 0 should keep track range, got start=%.1f dur=%.1f", s.SegmentStartSec, s.SegmentDurationSec)
	}
}

func TestBuildPersonCutPlan_MergesOverlappingWindows(t *testing.T) {
	videos := map[int64]domain.Video{1: {ID: 1, Path: "/a.mp4", DurationSeconds: 100}}
	group := testGroup(1,
		domain.Track{VideoID: 1, StartSec: 30, EndSec: 40},
		domain.Track{VideoID: 1, StartSec: 10, EndSec: 20},
		domain.Track{VideoID: 1, StartSec: 22, EndSec: 25}, // 2s pad bridges 20..22
		domain.Track{VideoID: 1, StartSec: 80, EndSec: 85},
	)

	segments := BuildPersonCutPlan([]domain.PersonGroup{group}, videos, 2, 2)[0].Segments
	if len(segments) != 3 {
		t.Fatalf("expected 3 segments, got %d: %+v", len(segments), segments)
	}
	if segments[0].SegmentStartSec != 8 || segments[0].SegmentDurationSec != 19 {
		t.Fatalf("expected first merged [8,27], got start=%.1f dur=%.1f", segments[0].SegmentStartSec, segments[0].SegmentDurationSec)
	}
	if segments[1].SegmentStartSec != 28 {
		t.Fatalf("expected second segment at 28, got %.1f", segments[1].SegmentStartSec)
	}
}

func TestBuildPersonCutPlan_OrdersVideosByStartedAtNilLast(t *testing.T) {
	early := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	late := early.Add(time.Hour)
	videos := map[int64]domain.Video{
		1: {ID: 1, Path: "/late.mp4", StartedAt: &late, DurationSeconds: 20},
		2: {ID: 2, Path: "/zz-nil.mp4", DurationSeconds: 20},
		3: {ID: 3, Path: "/early.mp4", StartedAt: &early, DurationSeconds: 20},
		4: {ID: 4, Path: "/aa-nil.mp4", DurationSeconds: 20},
	}
	group := testGroup(1,
		domain.Track{VideoID: 1, StartSec: 1, EndSec: 2},
		domain.Track{VideoID: 2, StartSec: 1, EndSec: 2},
		domain.Track{VideoID: 3, StartSec: 1, EndSec: 2},
		domain.Track{VideoID: 4, StartSec: 1, EndSec: 2},
	)

	var got []string
	for _, s := range BuildPersonCutPlan([]domain.PersonGroup{group}, videos, 0, 0)[0].Segments {
		got = append(got, filepath.Base(s.VideoPath))
	}
	want := []string{"early.mp4", "late.mp4", "aa-nil.mp4", "zz-nil.mp4"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order mismatch: got %v want %v", got, want)
		}
	}
}

func TestBuildPersonCutPlan_SkipsUnknownVideoAndEmptyGroups(t *testing.T) {
	videos := map[int64]domain.Video{1: {ID: 1, Path: "/a.mp4", DurationSeconds: 10}}
	plans := BuildPersonCutPlan([]domain.PersonGroup{
		testGroup(1, domain.Track{VideoID: 99, StartSec: 1, EndSec: 2}),
		testGroup(2, domain.Track{VideoID: 1, StartSec: 20, EndSec: 30}), // beyond duration
	}, videos, 0, 0)
	if len(plans) != 0 {
		t.Fatalf("expected no plans, got %+v", plans)
	}
}

type personRunnerStub struct{ plans []CutRunPlan }

func (s *personRunnerStub) ExecutePlanned(_ context.Context, _ string, plan CutRunPlan, onProgress func(CutProgressEvent)) (domain.CutResult, error) {
	s.plans = append(s.plans, plan)
	_ = os.MkdirAll(plan.MergedDir, 0o755)
	merged := filepath.Join(plan.MergedDir, "final.mp4")
	_ = os.WriteFile(merged, []byte("x"), 0o644)
	if onProgress != nil {
		onProgress(CutProgressEvent{CompletedSegments: len(plan.Plans)})
	}
	return domain.CutResult{MergedOutput: &domain.OutputFile{Path: merged, DurationSeconds: 1}}, nil
}

func TestPersonCutExecute_NamesClipsAndCleansScratch(t *testing.T) {
	dir := t.TempDir()
	runner := &personRunnerStub{}
	action := &PersonCutAction{
		WorkspaceRepo: personWorkspaceStub{dir: dir},
		VideoRepo:     personVideoStub{},
		Detection: personDetectionStub{groups: []domain.PersonGroup{
			testGroup(7, domain.Track{VideoID: 1, StartSec: 1, EndSec: 3}),
			testGroup(8, domain.Track{VideoID: 1, StartSec: 5, EndSec: 6}),
		}},
		Runner: runner,
	}

	var last CutProgressEvent
	result, err := action.Execute(context.Background(), "job", StartPeopleCutInput{
		WorkspaceID: 1, GroupIDs: []int64{7, 8}, Name: "crowd", SecondsBefore: 1, SecondsAfter: 1,
	}, func(e CutProgressEvent) { last = e })
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"person-7.mp4", "person-8.mp4"} {
		if _, statErr := os.Stat(filepath.Join(dir, "exports", "crowd", id)); statErr != nil {
			t.Fatalf("missing %s: %v", id, statErr)
		}
	}
	if entries, _ := os.ReadDir(filepath.Join(dir, "exports", "crowd")); len(entries) != 2 {
		t.Fatalf("scratch dirs were not cleaned: %d entries", len(entries))
	}
	if len(result.PerSourceOutputs) != 2 || last.TotalSegments != 2 || last.CompletedSegments != 2 {
		t.Fatalf("unexpected result/progress: %+v %+v", result, last)
	}

	if _, err = action.Execute(context.Background(), "job", StartPeopleCutInput{WorkspaceID: 1, GroupIDs: []int64{7, 99}, Name: "other", SecondsBefore: 1}, nil); err == nil {
		t.Fatal("expected error for a missing group")
	}
}
