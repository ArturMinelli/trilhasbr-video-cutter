package domain

import "time"

type Video struct {
	ID              int64      `json:"id"`
	WorkspaceID     int64      `json:"workspaceId"`
	Name            string     `json:"name"`
	Path            string     `json:"path"`
	StartedAt       *time.Time `json:"startedAt,omitempty"`
	FinishedAt      *time.Time `json:"finishedAt,omitempty"`
	DurationSeconds float64    `json:"durationSeconds"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type VideoInsert struct {
	Name            string
	Path            string
	StartedAt       *time.Time
	FinishedAt      *time.Time
	DurationSeconds float64
}
