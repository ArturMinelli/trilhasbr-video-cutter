package domain

import "time"

type DetectionStatus string

const (
	DetectionRunning   DetectionStatus = "RUNNING"
	DetectionSucceeded DetectionStatus = "SUCCEEDED"
	DetectionFailed    DetectionStatus = "FAILED"
	DetectionCancelled DetectionStatus = "CANCELLED"
)

const (
	AnalysisDone   = "DONE"
	AnalysisFailed = "FAILED"
)

// EmbeddingDim is the OSNet embedding size stored per track (512 float32).
const EmbeddingDim = 512

type DetectionRun struct {
	ID          int64           `json:"id"`
	WorkspaceID int64           `json:"workspaceId"`
	SampleFPS   float64         `json:"sampleFps"`
	ModelVer    string          `json:"modelVersion"`
	Status      DetectionStatus `json:"status"`
	StartedAt   time.Time       `json:"startedAt"`
	FinishedAt  *time.Time      `json:"finishedAt,omitempty"`
	VideosTotal int             `json:"videosTotal"`
	VideosDone  int             `json:"videosDone"`
	TracksFound int             `json:"tracksFound"`
}

// VideoAnalysis is the skip/resume key: a video is skipped when all four key fields match and Status is DONE.
type VideoAnalysis struct {
	VideoID      int64   `json:"videoId"`
	SampleFPS    float64 `json:"sampleFps"`
	ModelVersion string  `json:"modelVersion"`
	FileSize     int64   `json:"fileSize"`
	FileMTime    int64   `json:"fileMtime"` // unix seconds
	Status       string  `json:"status"`    // AnalysisDone | AnalysisFailed
	Error        string  `json:"error,omitempty"`
}

// TrackBox is one sampled-frame box of a track. T is seconds from the start of the video;
// coordinates are normalized 0..1 by the decoded frame size, so they do not depend on resolution.
type TrackBox struct {
	T, X1, Y1, X2, Y2 float32
}

type Track struct {
	ID             int64      `json:"id"`
	VideoID        int64      `json:"videoId"`
	VideoName      string     `json:"videoName,omitempty"` // filled by ListGroups
	StartSec       float64    `json:"startSec"`
	EndSec         float64    `json:"endSec"`
	AvgConfidence  float64    `json:"avgConfidence"`
	DetectionCount int        `json:"detectionCount"`
	BestBBox       string     `json:"bestBbox,omitempty"` // JSON
	ThumbnailPath  string     `json:"thumbnailPath,omitempty"`
	Embedding      []float32  `json:"-"` // len 0 or EmbeddingDim
	Boxes          []TrackBox `json:"-"` // per-frame path, time-ordered; empty for videos analyzed before paths were stored
}

// VideoTrackBoxes is a track's path for the player overlay, with the group it belongs to.
type VideoTrackBoxes struct {
	TrackID  int64
	GroupID  int64
	Label    string // the group's name, "" when unnamed
	StartSec float64
	EndSec   float64
	Boxes    []TrackBox
}

type PersonGroup struct {
	ID               int64   `json:"id"`
	WorkspaceID      int64   `json:"workspaceId"`
	Label            string  `json:"label,omitempty"`
	Tracks           []Track `json:"tracks"`
	TotalDurationSec float64 `json:"totalDurationSec"` // sum of track durations
	AvgConfidence    float64 `json:"avgConfidence"`    // detection-count weighted
	VideoCount       int     `json:"videoCount"`
}

type GroupFilter struct {
	GroupIDs       []int64
	WorkspaceID    int64
	MinDurationSec float64
	MinConfidence  float64
	Page           int // 1-based
	PageSize       int
	Sort           string
}
