package ffmpeg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/media/timestamp"
	"trilhasbr-video-cutter/internal/platform"
)

type binaryResolver interface {
	Resolve(ctx context.Context) (platform.BinaryPair, error)
}

type Client struct {
	resolver binaryResolver
	logger   *slog.Logger

	mu     sync.RWMutex
	cached *platform.BinaryPair
}

func NewClient(resolver binaryResolver, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{resolver: resolver, logger: logger}
}

func (c *Client) Probe(ctx context.Context, inputPath string) (ProbeInfo, error) {
	binaries, err := c.getBinaries(ctx)
	if err != nil {
		return ProbeInfo{}, err
	}

	args := []string{
		"-v", "error",
		"-print_format", "json",
		"-show_entries", "format=duration:format_tags=creation_time:stream=index,codec_type,codec_name:stream_tags=creation_time",
		inputPath,
	}
	output, runErr := runCommand(ctx, binaries.FFprobePath, args...)
	if runErr != nil {
		return ProbeInfo{}, apperrors.Wrap(apperrors.CodeProbe, "ffprobe command failed", runErr, map[string]any{"inputPath": inputPath})
	}

	var parsed struct {
		Format struct {
			Duration string            `json:"duration"`
			Tags     map[string]string `json:"tags"`
		} `json:"format"`
		Streams []struct {
			CodecType string            `json:"codec_type"`
			CodecName string            `json:"codec_name"`
			Tags      map[string]string `json:"tags"`
		} `json:"streams"`
	}
	if unmarshalErr := json.Unmarshal(output, &parsed); unmarshalErr != nil {
		return ProbeInfo{}, apperrors.Wrap(apperrors.CodeProbe, "failed to parse ffprobe output", unmarshalErr, map[string]any{"inputPath": inputPath})
	}

	duration, parseErr := strconv.ParseFloat(parsed.Format.Duration, 64)
	if parseErr != nil || duration <= 0 {
		return ProbeInfo{}, apperrors.New(apperrors.CodeProbe, "ffprobe did not return a valid video duration", map[string]any{"inputPath": inputPath})
	}

	codecInfo := map[string]string{}
	for _, stream := range parsed.Streams {
		if stream.CodecType != "" {
			codecInfo[stream.CodecType] = stream.CodecName
		}
	}

	var startedAt *time.Time
	if rawCreation := firstNonEmpty(
		parsed.Format.Tags["creation_time"],
		parsed.Format.Tags["com.apple.quicktime.creationdate"],
		parsed.Format.Tags["date"],
	); rawCreation != "" {
		parsedTime, timeErr := timestamp.ParseMetadataTime(rawCreation)
		if timeErr == nil {
			startedAt = parsedTime
		}
	}
	if startedAt == nil {
		for _, stream := range parsed.Streams {
			rawCreation := firstNonEmpty(stream.Tags["creation_time"], stream.Tags["date"])
			if rawCreation == "" {
				continue
			}
			if parsedTime, timeErr := timestamp.ParseMetadataTime(rawCreation); timeErr == nil {
				startedAt = parsedTime
				break
			}
		}
	}

	return ProbeInfo{
		DurationSec: duration,
		StartedAt:   startedAt,
		CodecInfo:   codecInfo,
	}, nil
}

func (c *Client) CutSegment(ctx context.Context, input CutSegmentInput) (CutSegmentOutput, error) {
	binaries, err := c.getBinaries(ctx)
	if err != nil {
		return CutSegmentOutput{}, err
	}

	if input.DurationSec <= 0 {
		return CutSegmentOutput{}, apperrors.New(apperrors.CodeValidation, "cut duration must be greater than 0", map[string]any{"durationSec": input.DurationSec})
	}
	if input.StartSec < 0 {
		return CutSegmentOutput{}, apperrors.New(apperrors.CodeValidation, "cut start must be >= 0", map[string]any{"startSec": input.StartSec})
	}

	if err = os.MkdirAll(filepath.Dir(input.OutputPath), 0o755); err != nil {
		return CutSegmentOutput{}, apperrors.Wrap(apperrors.CodeIO, "failed to prepare output directory for cut", err, map[string]any{"outputPath": input.OutputPath})
	}

	copyArgs := []string{
		"-y",
		"-v", "error",
		"-ss", formatSeconds(input.StartSec),
		"-i", input.InputPath,
		"-t", formatSeconds(input.DurationSec),
		"-c", "copy",
		"-movflags", "+faststart",
		input.OutputPath,
	}

	copyErr := runBareCommand(ctx, binaries.FFmpegPath, copyArgs...)
	if copyErr == nil {
		if output, validateErr := c.validateOutput(ctx, input.OutputPath, input.DurationSec); validateErr == nil {
			return CutSegmentOutput{
				OutputPath:        input.OutputPath,
				ActualDurationSec: output.DurationSec,
				UsedFallback:      false,
			}, nil
		}
	}

	reencodeArgs := []string{
		"-y",
		"-v", "error",
		"-ss", formatSeconds(input.StartSec),
		"-i", input.InputPath,
		"-t", formatSeconds(input.DurationSec),
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "23",
		"-c:a", "aac",
		"-movflags", "+faststart",
		"-avoid_negative_ts", "make_zero",
		input.OutputPath,
	}
	if reencodeErr := runBareCommand(ctx, binaries.FFmpegPath, reencodeArgs...); reencodeErr != nil {
		return CutSegmentOutput{}, apperrors.Wrap(apperrors.CodeCut, "ffmpeg cut failed in both copy and fallback modes", reencodeErr, map[string]any{"inputPath": input.InputPath, "outputPath": input.OutputPath})
	}

	output, validateErr := c.validateOutput(ctx, input.OutputPath, input.DurationSec)
	if validateErr != nil {
		return CutSegmentOutput{}, apperrors.Wrap(apperrors.CodeCut, "generated cut output is invalid", validateErr, map[string]any{"outputPath": input.OutputPath})
	}

	return CutSegmentOutput{
		OutputPath:        input.OutputPath,
		ActualDurationSec: output.DurationSec,
		UsedFallback:      true,
	}, nil
}

func (c *Client) MergeConcat(ctx context.Context, input MergeInput) (MergeOutput, error) {
	binaries, err := c.getBinaries(ctx)
	if err != nil {
		return MergeOutput{}, err
	}
	if len(input.SegmentPaths) == 0 {
		return MergeOutput{}, apperrors.New(apperrors.CodeValidation, "cannot merge without segment files", nil)
	}
	if err = os.MkdirAll(filepath.Dir(input.OutputPath), 0o755); err != nil {
		return MergeOutput{}, apperrors.Wrap(apperrors.CodeIO, "failed to prepare merge output directory", err, map[string]any{"outputPath": input.OutputPath})
	}

	listFile, err := os.CreateTemp("", "trilhasbr-concat-*.txt")
	if err != nil {
		return MergeOutput{}, apperrors.Wrap(apperrors.CodeIO, "failed to create concat list file", err, nil)
	}
	listPath := listFile.Name()
	defer func() {
		_ = listFile.Close()
		_ = os.Remove(listPath)
	}()

	for _, segment := range input.SegmentPaths {
		escaped := strings.ReplaceAll(segment, "'", "'\\''")
		if _, writeErr := listFile.WriteString("file '" + escaped + "'\n"); writeErr != nil {
			return MergeOutput{}, apperrors.Wrap(apperrors.CodeIO, "failed to write concat list", writeErr, nil)
		}
	}
	if err = listFile.Close(); err != nil {
		return MergeOutput{}, apperrors.Wrap(apperrors.CodeIO, "failed to close concat list file", err, nil)
	}

	warnings := make([]string, 0, 1)
	copyArgs := []string{
		"-y",
		"-v", "error",
		"-f", "concat",
		"-safe", "0",
		"-i", listPath,
		"-c", "copy",
		input.OutputPath,
	}
	copyErr := runBareCommand(ctx, binaries.FFmpegPath, copyArgs...)
	if copyErr != nil {
		warnings = append(warnings, "merge copy mode failed, using re-encode fallback")
		reencodeArgs := []string{
			"-y",
			"-v", "error",
			"-f", "concat",
			"-safe", "0",
			"-i", listPath,
			"-c:v", "libx264",
			"-preset", "fast",
			"-crf", "23",
			"-c:a", "aac",
			"-movflags", "+faststart",
			input.OutputPath,
		}
		if reencodeErr := runBareCommand(ctx, binaries.FFmpegPath, reencodeArgs...); reencodeErr != nil {
			return MergeOutput{}, apperrors.Wrap(apperrors.CodeMerge, "ffmpeg merge failed in both copy and fallback modes", reencodeErr, map[string]any{"outputPath": input.OutputPath})
		}
	}

	validated, validateErr := c.validateOutput(ctx, input.OutputPath, 0)
	if validateErr != nil {
		return MergeOutput{}, apperrors.Wrap(apperrors.CodeMerge, "merged output failed validation", validateErr, map[string]any{"outputPath": input.OutputPath})
	}

	return MergeOutput{
		OutputPath:  input.OutputPath,
		DurationSec: validated.DurationSec,
		Warnings:    warnings,
	}, nil
}

func (c *Client) GenerateThumbnail(ctx context.Context, input ThumbnailInput) (ThumbnailOutput, error) {
	binaries, err := c.getBinaries(ctx)
	if err != nil {
		return ThumbnailOutput{}, err
	}

	if strings.TrimSpace(input.InputPath) == "" {
		return ThumbnailOutput{}, apperrors.New(apperrors.CodeValidation, "thumbnail input path is required", nil)
	}
	if strings.TrimSpace(input.OutputPath) == "" {
		return ThumbnailOutput{}, apperrors.New(apperrors.CodeValidation, "thumbnail output path is required", nil)
	}
	if input.AtSecond < 0 {
		return ThumbnailOutput{}, apperrors.New(apperrors.CodeValidation, "thumbnail timestamp must be >= 0", map[string]any{"atSecond": input.AtSecond})
	}

	width := input.Width
	if width <= 0 {
		width = 320
	}

	if err = os.MkdirAll(filepath.Dir(input.OutputPath), 0o755); err != nil {
		return ThumbnailOutput{}, apperrors.Wrap(apperrors.CodeIO, "failed to prepare thumbnail output directory", err, map[string]any{"outputPath": input.OutputPath})
	}

	args := []string{
		"-y",
		"-v", "error",
		"-ss", formatSeconds(input.AtSecond),
		"-i", input.InputPath,
		"-frames:v", "1",
		"-vf", fmt.Sprintf("scale=%d:-2", width),
		"-q:v", "3",
		input.OutputPath,
	}
	if runErr := runBareCommand(ctx, binaries.FFmpegPath, args...); runErr != nil {
		return ThumbnailOutput{}, apperrors.Wrap(apperrors.CodeProbe, "failed to generate thumbnail with ffmpeg", runErr, map[string]any{
			"inputPath":  input.InputPath,
			"outputPath": input.OutputPath,
			"atSecond":   input.AtSecond,
		})
	}

	info, statErr := os.Stat(input.OutputPath)
	if statErr != nil {
		return ThumbnailOutput{}, apperrors.Wrap(apperrors.CodeIO, "thumbnail file was not created", statErr, map[string]any{"outputPath": input.OutputPath})
	}
	if info.Size() <= 0 {
		return ThumbnailOutput{}, apperrors.New(apperrors.CodeProbe, "generated thumbnail file is empty", map[string]any{"outputPath": input.OutputPath})
	}

	return ThumbnailOutput{OutputPath: input.OutputPath}, nil
}

func (c *Client) validateOutput(ctx context.Context, outputPath string, expectedDuration float64) (ProbeInfo, error) {
	probeInfo, err := c.Probe(ctx, outputPath)
	if err != nil {
		return ProbeInfo{}, err
	}
	if probeInfo.DurationSec <= 0 {
		return ProbeInfo{}, apperrors.New(apperrors.CodeProbe, "output file has zero duration", map[string]any{"outputPath": outputPath})
	}
	if expectedDuration > 0 {
		minAccepted := math.Min(0.4, expectedDuration*0.2)
		if probeInfo.DurationSec < expectedDuration-minAccepted {
			return ProbeInfo{}, apperrors.New(apperrors.CodeProbe, "output duration is shorter than expected", map[string]any{
				"outputPath":       outputPath,
				"expectedDuration": expectedDuration,
				"actualDuration":   probeInfo.DurationSec,
			})
		}
	}
	return probeInfo, nil
}

func (c *Client) getBinaries(ctx context.Context) (platform.BinaryPair, error) {
	c.mu.RLock()
	if c.cached != nil {
		pair := *c.cached
		c.mu.RUnlock()
		return pair, nil
	}
	c.mu.RUnlock()

	pair, err := c.resolver.Resolve(ctx)
	if err != nil {
		return platform.BinaryPair{}, err
	}

	c.mu.Lock()
	c.cached = &pair
	c.mu.Unlock()

	return pair, nil
}

func runCommand(ctx context.Context, cmdPath string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, cmdPath, args...)
	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func runBareCommand(ctx context.Context, cmdPath string, args ...string) error {
	cmd := exec.CommandContext(ctx, cmdPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func formatSeconds(v float64) string {
	return strconv.FormatFloat(v, 'f', 3, 64)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
