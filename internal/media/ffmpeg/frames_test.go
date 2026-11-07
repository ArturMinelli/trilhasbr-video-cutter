package ffmpeg

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"trilhasbr-video-cutter/internal/platform"
)

// testVideo is a short real video (set FRAMES_TEST_VIDEO); tests skip without it.
var testVideo = os.Getenv("FRAMES_TEST_VIDEO")

func testClient(t *testing.T) *Client {
	t.Helper()
	if _, err := os.Stat(testVideo); testVideo == "" || err != nil {
		t.Skip("set FRAMES_TEST_VIDEO to a short video to run this test")
	}
	if _, err := platform.NewFFmpegResolver("").Resolve(context.Background()); err != nil {
		t.Skip("ffmpeg not available")
	}
	return NewClient(platform.NewFFmpegResolver(""), nil)
}

func TestFrameStreamFullRead(t *testing.T) {
	c := testClient(t)
	s, err := c.OpenFrames(context.Background(), testVideo, 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.Width != 480 || s.Height != 854 {
		t.Fatalf("size = %dx%d, want 480x854", s.Width, s.Height)
	}
	n := 0
	var last Frame
	for {
		f, err := s.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		last, n = f, n+1
	}
	if want := 6.49 * 5; float64(n) < want-2 || float64(n) > want+2 { // ~32 frames
		t.Fatalf("frames = %d, want ≈ %.0f", n, want)
	}
	if last.TimeSec != float64(n-1)/5 {
		t.Fatalf("last TimeSec = %v", last.TimeSec)
	}
	if err := WriteJPEG(filepath.Join(t.TempDir(), "c.jpg"), last, -10, 100, 200, 5000); err != nil {
		t.Fatal(err)
	}
}

func TestFrameStreamScaleAndCancel(t *testing.T) {
	c := testClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	s, err := c.OpenFrames(ctx, testVideo, 15, 240)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.Width != 240 || s.Height != 428 {
		t.Fatalf("scaled size = %dx%d, want 240x428", s.Width, s.Height)
	}
	f, err := s.Next()
	if err != nil || len(f.RGB) != 240*428*3 {
		t.Fatalf("first frame: err=%v len=%d", err, len(f.RGB))
	}
	cancel()
	start := time.Now()
	for {
		if _, err = s.Next(); err != nil {
			break
		}
	}
	if !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) { // may have drained before cancel landed
		t.Fatalf("err = %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatalf("cancel took %v", time.Since(start))
	}
}

func TestFrameStreamBadFPS(t *testing.T) {
	if _, err := NewClient(platform.NewFFmpegResolver(""), nil).OpenFrames(context.Background(), "x", 30, 0); err == nil {
		t.Fatal("want validation error")
	}
}
