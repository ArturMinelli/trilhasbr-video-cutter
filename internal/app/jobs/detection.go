package jobs

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"trilhasbr-video-cutter/internal/apperrors"
)

const StatusCancelled Status = "CANCELLED"

type DetectionSnapshot struct {
	JobID        string                 `json:"jobId"`
	WorkspaceID  int64                  `json:"workspaceId"`
	Status       Status                 `json:"status"`
	VideosTotal  int                    `json:"videosTotal"`
	VideosDone   int                    `json:"videosDone"`
	CurrentVideo string                 `json:"currentVideo"`
	TracksFound  int                    `json:"tracksFound"`
	Error        *apperrors.PublicError `json:"error,omitempty"`
	CreatedAt    time.Time              `json:"createdAt"`
	UpdatedAt    time.Time              `json:"updatedAt"`
	FinishedAt   *time.Time             `json:"finishedAt,omitempty"`
}

type detectionJob struct {
	DetectionSnapshot
	cancel context.CancelFunc
}

type DetectionManager struct {
	mu   sync.RWMutex
	jobs map[string]*detectionJob
}

func NewDetectionManager() *DetectionManager {
	return &DetectionManager{jobs: make(map[string]*detectionJob)}
}

// Create registers a pending job; cancel is called by Cancel (may be nil until SetCancel).
func (m *DetectionManager) Create(workspaceID int64, cancel context.CancelFunc) DetectionSnapshot {
	now := time.Now().UTC()
	job := &detectionJob{
		DetectionSnapshot: DetectionSnapshot{JobID: uuid.NewString(), WorkspaceID: workspaceID, Status: StatusPending, CreatedAt: now, UpdatedAt: now},
		cancel:            cancel,
	}
	m.mu.Lock()
	m.jobs[job.JobID] = job
	m.mu.Unlock()
	return job.DetectionSnapshot
}

// update runs fn on a live job under lock; terminal jobs are left untouched.
func (m *DetectionManager) update(jobID string, fn func(j *detectionJob)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j := m.jobs[jobID]
	if j == nil || j.FinishedAt != nil {
		return
	}
	fn(j)
	j.UpdatedAt = time.Now().UTC()
}

func (m *DetectionManager) finish(jobID string, status Status, err error) {
	m.update(jobID, func(j *detectionJob) {
		now := time.Now().UTC()
		j.Status = status
		j.FinishedAt = &now
		if err != nil {
			j.Error = apperrors.ToPublic(err)
		}
	})
}

func (m *DetectionManager) SetRunning(jobID string, videosTotal int) {
	m.update(jobID, func(j *detectionJob) { j.Status = StatusRunning; j.VideosTotal = videosTotal })
}

func (m *DetectionManager) SetProgress(jobID string, videosDone int, currentVideo string, tracksFound int) {
	m.update(jobID, func(j *detectionJob) {
		j.Status = StatusRunning
		j.VideosDone, j.CurrentVideo, j.TracksFound = videosDone, currentVideo, tracksFound
	})
}

func (m *DetectionManager) SetSucceeded(jobID string) { m.finish(jobID, StatusSucceeded, nil) }
func (m *DetectionManager) SetCancelled(jobID string) { m.finish(jobID, StatusCancelled, nil) }
func (m *DetectionManager) SetFailed(jobID string, err error) {
	m.finish(jobID, StatusFailed, err)
}

// Cancel invokes the job's cancel func; the runner must then call SetCancelled. Returns false for unknown/finished jobs.
func (m *DetectionManager) Cancel(jobID string) bool {
	m.mu.RLock()
	j := m.jobs[jobID]
	var cancel context.CancelFunc
	if j != nil && j.FinishedAt == nil {
		cancel = j.cancel
	}
	m.mu.RUnlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

func (m *DetectionManager) Get(jobID string) (DetectionSnapshot, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	j := m.jobs[jobID]
	if j == nil {
		return DetectionSnapshot{}, false
	}
	return j.DetectionSnapshot, true
}

// WorkspaceBusy reports whether a detection job for the workspace is pending or running.
func (m *DetectionManager) WorkspaceBusy(workspaceID int64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, j := range m.jobs {
		if j.WorkspaceID == workspaceID && j.FinishedAt == nil {
			return true
		}
	}
	return false
}
