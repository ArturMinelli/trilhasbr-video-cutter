package domain

import "time"

type Workspace struct {
	ID        int64     `json:"id"`
	Directory string    `json:"directory"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type WorkspaceSummary struct {
	Workspace
	VideoCount         int `json:"videoCount"`
	NullStartedAtCount int `json:"nullStartedAtCount"`
}
