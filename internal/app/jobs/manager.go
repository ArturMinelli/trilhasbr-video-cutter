package jobs

import (
	"sync"
	"time"

	"github.com/google/uuid"

	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/domain"
)

type Status string

const (
	StatusPending   Status = "PENDING"
	StatusRunning   Status = "RUNNING"
	StatusSucceeded Status = "SUCCEEDED"
	StatusFailed    Status = "FAILED"
)

type Snapshot struct {
	JobID             string                 `json:"jobId"`
	Status            Status                 `json:"status"`
	TotalSegments     int                    `json:"totalSegments"`
	CompletedSegments int                    `json:"completedSegments"`
	CurrentFile       string                 `json:"currentFile"`
	Result            *domain.CutResult      `json:"result,omitempty"`
	Error             *apperrors.PublicError `json:"error,omitempty"`
	CreatedAt         time.Time              `json:"createdAt"`
	UpdatedAt         time.Time              `json:"updatedAt"`
	FinishedAt        *time.Time             `json:"finishedAt,omitempty"`
}

type cutJob struct {
	Snapshot
}

type Manager struct {
	mu   sync.RWMutex
	jobs map[string]*cutJob
}

func NewManager() *Manager {
	return &Manager{
		jobs: make(map[string]*cutJob),
	}
}

func (m *Manager) Create() Snapshot {
	jobID := uuid.NewString()
	now := time.Now().UTC()
	job := &cutJob{Snapshot: Snapshot{
		JobID:     jobID,
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}}
	m.mu.Lock()
	m.jobs[jobID] = job
	m.mu.Unlock()
	return job.Snapshot
}

func (m *Manager) SetRunning(jobID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.jobs[jobID]
	if job == nil {
		return
	}
	job.Status = StatusRunning
	job.UpdatedAt = time.Now().UTC()
}

func (m *Manager) SetProgress(jobID string, completed int, total int, currentFile string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.jobs[jobID]
	if job == nil {
		return
	}
	job.Status = StatusRunning
	job.CompletedSegments = completed
	job.TotalSegments = total
	job.CurrentFile = currentFile
	job.UpdatedAt = time.Now().UTC()
}

func (m *Manager) SetSucceeded(jobID string, result domain.CutResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.jobs[jobID]
	if job == nil {
		return
	}
	finishedAt := time.Now().UTC()
	job.Status = StatusSucceeded
	resultCopy := result
	job.Result = &resultCopy
	job.UpdatedAt = finishedAt
	job.FinishedAt = &finishedAt
}

func (m *Manager) SetFailed(jobID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.jobs[jobID]
	if job == nil {
		return
	}
	finishedAt := time.Now().UTC()
	job.Status = StatusFailed
	job.Error = apperrors.ToPublic(err)
	job.UpdatedAt = finishedAt
	job.FinishedAt = &finishedAt
}

func (m *Manager) Get(jobID string) (Snapshot, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	job := m.jobs[jobID]
	if job == nil {
		return Snapshot{}, false
	}
	copy := job.Snapshot
	return copy, true
}
