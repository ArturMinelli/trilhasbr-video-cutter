package ffmpeg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"trilhasbr-video-cutter/internal/apperrors"
)

// Frame is one decoded RGB24 frame (post-rotation, post-scale), len(RGB) == Width*Height*3.
type Frame struct {
	Index   int
	TimeSec float64 // Index / fps, offset from video start
	Width   int
	Height  int
	RGB     []byte
}

// FrameStream pulls sampled frames from an ffmpeg subprocess. Not safe for concurrent Next calls.
type FrameStream struct {
	Width, Height int // frame size after rotation and optional downscale
	FPS           float64

	cmd    *exec.Cmd
	stdout io.ReadCloser
	stderr *tailBuffer
	ctx    context.Context
	path   string

	next      int
	waitOnce  sync.Once
	waitErr   error
	finishErr error // sticky terminal error (io.EOF on clean end)
}

// OpenFrames starts ffmpeg sampling the video at fps (1-15). maxWidth > 0 downscales
// (keeping aspect, even dimensions) when the video is wider; 0 keeps the source size.
func (c *Client) OpenFrames(ctx context.Context, path string, fps float64, maxWidth int) (*FrameStream, error) {
	if fps < 1 || fps > 15 {
		return nil, apperrors.New(apperrors.CodeValidation, "sample fps must be between 1 and 15", map[string]any{"fps": fps})
	}
	bins, err := c.getBinaries(ctx)
	if err != nil {
		return nil, err
	}
	w, h, err := probeSize(ctx, bins.FFprobePath, path)
	if err != nil {
		return nil, err
	}

	filter := "fps=" + strconv.FormatFloat(fps, 'f', -1, 64)
	if maxWidth > 0 && w > maxWidth {
		nh := int(math.Round(float64(h)*float64(maxWidth)/float64(w)/2)) * 2
		if nh < 2 {
			nh = 2
		}
		w, h = maxWidth&^1, nh
		filter += fmt.Sprintf(",scale=%d:%d", w, h)
	}

	// ffmpeg autorotates by default, so w/h from probeSize are already post-rotation.
	cmd := exec.CommandContext(ctx, bins.FFmpegPath,
		"-v", "error", "-nostdin", "-i", path,
		"-an", "-sn", "-vf", filter,
		"-f", "rawvideo", "-pix_fmt", "rgb24", "-")
	cmd.WaitDelay = time.Second // force-close pipes if the process lingers after kill
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeIO, "failed to open ffmpeg pipe", err, map[string]any{"path": path})
	}
	tail := &tailBuffer{max: 2048}
	cmd.Stderr = tail
	if err := cmd.Start(); err != nil {
		return nil, apperrors.Wrap(apperrors.CodeIO, "failed to start ffmpeg", err, map[string]any{"path": path})
	}
	return &FrameStream{Width: w, Height: h, FPS: fps, cmd: cmd, stdout: stdout, stderr: tail, ctx: ctx, path: path}, nil
}

// Next returns the next frame, io.EOF at a clean end, ctx.Err() if cancelled.
// The returned RGB slice is owned by the caller.
func (s *FrameStream) Next() (Frame, error) {
	if s.finishErr != nil {
		return Frame{}, s.finishErr
	}
	buf := make([]byte, s.Width*s.Height*3)
	_, err := io.ReadFull(s.stdout, buf)
	if err == nil {
		f := Frame{Index: s.next, TimeSec: float64(s.next) / s.FPS, Width: s.Width, Height: s.Height, RGB: buf}
		s.next++
		return f, nil
	}
	if ctxErr := s.ctx.Err(); ctxErr != nil {
		s.finishErr = ctxErr
	} else if werr := s.wait(); werr != nil || errors.Is(err, io.ErrUnexpectedEOF) {
		s.finishErr = apperrors.Wrap(apperrors.CodeIO, "ffmpeg frame decode failed", errors.Join(werr, err),
			map[string]any{"path": s.path, "stderr": s.stderr.String(), "frames": s.next})
	} else {
		s.finishErr = io.EOF
	}
	return Frame{}, s.finishErr
}

// Close kills ffmpeg (if still running) and reaps it. Safe to call multiple times.
func (s *FrameStream) Close() error {
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	_ = s.stdout.Close()
	s.wait()
	return nil
}

func (s *FrameStream) wait() error {
	s.waitOnce.Do(func() { s.waitErr = s.cmd.Wait() })
	return s.waitErr
}

// probeSize returns the displayed (post-rotation) video size.
func probeSize(ctx context.Context, ffprobe, path string) (int, int, error) {
	out, err := runCommand(ctx, ffprobe, "-v", "error", "-select_streams", "v:0",
		"-print_format", "json", "-show_entries", "stream=width,height:stream_tags=rotate:stream_side_data=rotation", path)
	if err != nil {
		return 0, 0, apperrors.Wrap(apperrors.CodeProbe, "ffprobe size failed", err, map[string]any{"path": path})
	}
	var p struct {
		Streams []struct {
			Width, Height int
			Tags          map[string]string `json:"tags"`
			SideData      []struct {
				Rotation float64 `json:"rotation"`
			} `json:"side_data_list"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &p); err != nil || len(p.Streams) == 0 || p.Streams[0].Width <= 0 || p.Streams[0].Height <= 0 {
		return 0, 0, apperrors.New(apperrors.CodeProbe, "no video stream found", map[string]any{"path": path})
	}
	st := p.Streams[0]
	rot := 0.0
	if r, perr := strconv.ParseFloat(st.Tags["rotate"], 64); perr == nil {
		rot = r
	}
	for _, sd := range st.SideData {
		if sd.Rotation != 0 {
			rot = sd.Rotation
		}
	}
	if q := int(math.Abs(math.Round(rot))) % 180; q == 90 {
		return st.Height, st.Width, nil
	}
	return st.Width, st.Height, nil
}

// tailBuffer keeps only the last max bytes written (bounded stderr capture).
type tailBuffer struct {
	mu  sync.Mutex
	max int
	b   []byte
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.b = append(t.b, p...)
	if len(t.b) > t.max {
		t.b = t.b[len(t.b)-t.max:]
	}
	return len(p), nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.TrimSpace(string(t.b))
}
