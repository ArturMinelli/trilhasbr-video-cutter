package ffmpeg

import (
	"context"
	"time"
)

type ProbeInfo struct {
	DurationSec float64           `json:"durationSec"`
	StartedAt   *time.Time        `json:"startedAt,omitempty"`
	CodecInfo   map[string]string `json:"codecInfo,omitempty"`
}

type CutMode string

const (
	CutModeFastCopyThenPrecise CutMode = "fast_copy_then_precise"
)

type CutSegmentInput struct {
	InputPath   string  `json:"inputPath"`
	OutputPath  string  `json:"outputPath"`
	StartSec    float64 `json:"startSec"`
	DurationSec float64 `json:"durationSec"`
	Mode        CutMode `json:"mode"`
}

type CutSegmentOutput struct {
	OutputPath        string  `json:"outputPath"`
	ActualDurationSec float64 `json:"actualDurationSec"`
	UsedFallback      bool    `json:"usedFallback"`
	LogsPath          string  `json:"logsPath,omitempty"`
}

type MergeInput struct {
	SegmentPaths []string `json:"segmentPaths"`
	OutputPath   string   `json:"outputPath"`
}

type MergeOutput struct {
	OutputPath  string   `json:"outputPath"`
	DurationSec float64  `json:"durationSec"`
	Warnings    []string `json:"warnings"`
}

type ThumbnailInput struct {
	InputPath  string  `json:"inputPath"`
	OutputPath string  `json:"outputPath"`
	AtSecond   float64 `json:"atSecond"`
	Width      int     `json:"width"`
}

type ThumbnailOutput struct {
	OutputPath string `json:"outputPath"`
}

type MediaProbe interface {
	Probe(ctx context.Context, inputPath string) (ProbeInfo, error)
}

type MediaCutter interface {
	CutSegment(ctx context.Context, input CutSegmentInput) (CutSegmentOutput, error)
}

type MediaMerger interface {
	MergeConcat(ctx context.Context, input MergeInput) (MergeOutput, error)
}

type MediaThumbnailer interface {
	GenerateThumbnail(ctx context.Context, input ThumbnailInput) (ThumbnailOutput, error)
}

type Service interface {
	MediaProbe
	MediaCutter
	MediaMerger
}
