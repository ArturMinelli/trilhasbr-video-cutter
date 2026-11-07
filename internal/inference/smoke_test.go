package inference

import (
	"image"
	"image/draw"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"trilhasbr-video-cutter/internal/platform"
)

// Smoke test: needs models + libonnxruntime (scripts/fetch-onnxruntime.sh) and INFERENCE_SMOKE_FRAME=<png>.
// INFERENCE_SMOKE_PROVIDER=cpu|coreml|auto selects the provider (default auto).
func TestSmoke(t *testing.T) {
	framePath := os.Getenv("INFERENCE_SMOKE_FRAME")
	if framePath == "" {
		t.Skip("INFERENCE_SMOKE_FRAME not set")
	}
	paths, err := platform.NewModelResolver(filepath.Join("..", "..", "resources")).Resolve()
	if err != nil {
		t.Skipf("models/onnxruntime missing: %v", err)
	}
	fh, err := os.Open(framePath)
	if err != nil {
		t.Fatal(err)
	}
	img, _, err := image.Decode(fh)
	fh.Close()
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	frame := Frame{W: b.Dx(), H: b.Dy(), RGB: make([]byte, b.Dx()*b.Dy()*3)}
	for i := 0; i < b.Dx()*b.Dy(); i++ {
		copy(frame.RGB[i*3:i*3+3], rgba.Pix[i*4:i*4+3])
	}

	prov := Provider(os.Getenv("INFERENCE_SMOKE_PROVIDER"))
	e, err := Open(Config{LibPath: paths.ORTLib, YOLOPath: paths.YOLO, OSNetPath: paths.OSNet, Sessions: 1, Provider: prov})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	t.Logf("providers: yolo=%s osnet=%s", e.YOLOProvider, e.OSNetProvider)

	if _, err := e.Detect(frame); err != nil { // warmup
		t.Fatal(err)
	}
	start := time.Now()
	const n = 5
	var boxes []Box
	for i := 0; i < n; i++ {
		if boxes, err = e.Detect(frame); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("detect: %.1f ms/frame, %d boxes", float64(time.Since(start).Milliseconds())/n, len(boxes))
	if len(boxes) == 0 {
		t.Fatal("expected at least one person in the fixture frame")
	}

	var crops []Frame
	for _, bx := range boxes {
		if c, ok := Crop(frame, bx); ok {
			crops = append(crops, c)
		}
	}
	// Batch sizes vary in real use; time several (first call per size may compile on CoreML).
	var embs [][]float32
	for _, bs := range []int{1, 1, 3, 3, 5, 5} {
		batch := make([]Frame, bs)
		for i := range batch {
			batch[i] = crops[i%len(crops)]
		}
		start = time.Now()
		if embs, err = e.Embed(batch); err != nil {
			t.Fatal(err)
		}
		t.Logf("embed: batch %d in %d ms", bs, time.Since(start).Milliseconds())
	}
	for _, v := range embs {
		var s float64
		for _, x := range v {
			s += float64(x) * float64(x)
		}
		if len(v) != EmbeddingDim || math.Abs(s-1) > 1e-3 {
			t.Fatalf("bad embedding dim=%d norm2=%v", len(v), s)
		}
	}
}
