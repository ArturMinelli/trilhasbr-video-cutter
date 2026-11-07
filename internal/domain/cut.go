package domain

import "time"

type CutWindow struct {
	AnchorLocal    time.Time `json:"anchorLocal"`
	AnchorUTC      time.Time `json:"anchorUtc"`
	WindowStartUTC time.Time `json:"windowStartUtc"`
	WindowEndUTC   time.Time `json:"windowEndUtc"`
	SecondsBefore  int       `json:"secondsBefore"`
	SecondsAfter   int       `json:"secondsAfter"`
}

type SegmentPlan struct {
	VideoID            int64     `json:"videoId"`
	VideoPath          string    `json:"videoPath"`
	VideoStartUTC      time.Time `json:"videoStartUtc"`
	VideoDurationSec   float64   `json:"videoDurationSec"`
	SegmentStartSec    float64   `json:"segmentStartSec"`
	SegmentDurationSec float64   `json:"segmentDurationSec"`
}

type OutputFile struct {
	Path            string  `json:"path"`
	DurationSeconds float64 `json:"durationSeconds"`
}

type CutResult struct {
	JobID            string       `json:"jobId"`
	OutputDirectory  string       `json:"outputDirectory"`
	PerSourceOutputs []OutputFile `json:"perSourceOutputs"`
	MergedOutput     *OutputFile  `json:"mergedOutput,omitempty"`
	Warnings         []string     `json:"warnings"`
}
