// Package inference wraps ONNX Runtime (cgo) for person detection and re-ID embeddings.
package inference

import "trilhasbr-video-cutter/internal/apperrors"

// Frame is a packed RGB24 image (len(RGB) == W*H*3).
type Frame struct {
	W, H int
	RGB  []byte
}

// Box is a person detection in source-frame pixel coordinates.
type Box struct {
	X1, Y1, X2, Y2, Conf float32
}

const EmbeddingDim = 512

// Engine runs the detector and the re-ID model. Implementations are safe for concurrent use.
type Engine interface {
	// Detect returns person boxes (conf >= 0.35, NMS IoU 0.5) in frame pixel coordinates.
	Detect(f Frame) ([]Box, error)
	// Embed returns one L2-normalized 512-d embedding per crop.
	Embed(crops []Frame) ([][]float32, error)
	Close() error
}

// Crop copies the box region (clamped to the frame) into a new Frame. ok is false if the region is empty.
func Crop(f Frame, b Box) (Frame, bool) {
	x1, y1 := clampInt(int(b.X1), 0, f.W), clampInt(int(b.Y1), 0, f.H)
	x2, y2 := clampInt(int(b.X2+0.5), 0, f.W), clampInt(int(b.Y2+0.5), 0, f.H)
	w, h := x2-x1, y2-y1
	if w < 2 || h < 2 {
		return Frame{}, false
	}
	out := Frame{W: w, H: h, RGB: make([]byte, w*h*3)}
	for y := 0; y < h; y++ {
		copy(out.RGB[y*w*3:(y+1)*w*3], f.RGB[((y1+y)*f.W+x1)*3:((y1+y)*f.W+x2)*3])
	}
	return out, true
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// CodeInference is local so this package does not depend on apperrors gaining the constant.
const CodeInference apperrors.Code = "INFERENCE_ERROR"
