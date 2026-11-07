package inference

import (
	"math"
	"path/filepath"
	"testing"

	"trilhasbr-video-cutter/internal/platform"
)

// TestRuntimeLoadsAndRuns proves the native ONNX Runtime and both models load and execute on this machine
// (what a fresh install must guarantee), using synthetic pixels, so no photo of a person is needed.
// It skips when the models or libonnxruntime are not present (fetch with scripts/fetch-onnxruntime.sh).
func TestRuntimeLoadsAndRuns(t *testing.T) {
	paths, err := platform.NewModelResolver(filepath.Join("..", "..", "resources")).Resolve()
	if err != nil {
		t.Skipf("models/onnxruntime missing: %v", err)
	}
	e, err := Open(Config{LibPath: paths.ORTLib, YOLOPath: paths.YOLO, OSNetPath: paths.OSNet, Sessions: 1})
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}
	defer e.Close()
	t.Logf("providers: yolo=%s osnet=%s", e.YOLOProvider, e.OSNetProvider)

	blank := Frame{W: 640, H: 480, RGB: make([]byte, 640*480*3)}
	if _, err = e.Detect(blank); err != nil { // zero boxes on a blank frame is fine; it must not error
		t.Fatalf("detect on a blank frame: %v", err)
	}

	crop := Frame{W: 64, H: 128, RGB: make([]byte, 64*128*3)}
	for i := range crop.RGB {
		crop.RGB[i] = byte(i * 7) // deterministic non-constant pixels
	}
	embs, err := e.Embed([]Frame{crop})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if len(embs) != 1 || len(embs[0]) != EmbeddingDim {
		t.Fatalf("want one %d-d embedding, got %d vectors (first len %d)", EmbeddingDim, len(embs), func() int {
			if len(embs) > 0 {
				return len(embs[0])
			}
			return 0
		}())
	}
	var sum float64
	for _, v := range embs[0] {
		sum += float64(v) * float64(v)
	}
	if norm := math.Sqrt(sum); math.Abs(norm-1) > 1e-3 {
		t.Fatalf("embedding is not L2-normalized: norm=%f", norm)
	}
}
