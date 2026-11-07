package jobs

import (
	"context"
	"errors"
	"testing"
)

func TestDetectionManager_Lifecycle(t *testing.T) {
	m := NewDetectionManager()
	ctx, cancel := context.WithCancel(context.Background())
	s := m.Create(7, cancel)
	if s.Status != StatusPending {
		t.Fatal(s.Status)
	}
	m.SetRunning(s.JobID, 3)
	m.SetProgress(s.JobID, 1, "a.mp4", 7)
	got, _ := m.Get(s.JobID)
	if got.VideosTotal != 3 || got.VideosDone != 1 || got.CurrentVideo != "a.mp4" || got.TracksFound != 7 {
		t.Fatalf("%+v", got)
	}
	if !m.Cancel(s.JobID) || ctx.Err() == nil {
		t.Fatal("cancel must trigger context")
	}
	m.SetCancelled(s.JobID)
	m.SetFailed(s.JobID, errors.New("late")) // terminal state is sticky
	got, _ = m.Get(s.JobID)
	if got.Status != StatusCancelled || got.FinishedAt == nil || got.Error != nil {
		t.Fatalf("%+v", got)
	}
	if m.Cancel(s.JobID) || m.Cancel("nope") {
		t.Fatal("cancel of finished/unknown job must be false")
	}
}

func TestDetectionManager_WorkspaceBusy(t *testing.T) {
	m := NewDetectionManager()
	job := m.Create(7, func() {})
	if !m.WorkspaceBusy(7) || m.WorkspaceBusy(8) {
		t.Fatal("pending job must make only its own workspace busy")
	}
	m.SetSucceeded(job.JobID)
	if m.WorkspaceBusy(7) {
		t.Fatal("finished job must not keep the workspace busy")
	}
}
