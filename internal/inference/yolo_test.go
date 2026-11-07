package inference

import (
	"math"
	"testing"
)

func TestDecodeYOLOSynthetic(t *testing.T) {
	out := make([]float32, yoloChannels*yoloAnchors)
	set := func(i int, cx, cy, w, h, conf float32) {
		out[i], out[yoloAnchors+i], out[2*yoloAnchors+i], out[3*yoloAnchors+i], out[4*yoloAnchors+i] = cx, cy, w, h, conf
	}
	// 1920x1080 frame -> scale 2/3, vertical pad (1280-720)/2 = 280.
	scale, padX, padY := float32(1280)/1920, float32(0), float32(280)
	set(0, 640, 640, 100, 200, 0.9) // kept
	set(1, 642, 642, 100, 200, 0.8) // duplicate, suppressed by NMS
	set(2, 200, 400, 60, 120, 0.2)  // below threshold
	set(3, 1000, 500, 80, 160, 0.5) // second person
	boxes := decodeYOLO(out, scale, padX, padY, 1920, 1080)
	if len(boxes) != 2 {
		t.Fatalf("want 2 boxes, got %d: %+v", len(boxes), boxes)
	}
	b := boxes[0]
	cx, cy := (b.X1+b.X2)/2, (b.Y1+b.Y2)/2
	if math.Abs(float64(cx-960)) > 1 || math.Abs(float64(cy-540)) > 1 || math.Abs(float64(b.X2-b.X1-150)) > 1 {
		t.Fatalf("unexpected unletterboxed box %+v", b)
	}
}

func TestLetterboxAndL2(t *testing.T) {
	f := Frame{W: 4, H: 2, RGB: make([]byte, 4*2*3)}
	for i := range f.RGB {
		f.RGB[i] = 255
	}
	dst := make([]float32, 3*yoloSize*yoloSize)
	scale, padX, padY := letterbox(f, dst)
	if scale != 320 || padX != 0 || padY != 320 {
		t.Fatalf("scale/pad = %v %v %v", scale, padX, padY)
	}
	if dst[0] != float32(114.0/255.0) || dst[yoloSize*(yoloSize/2)+10] != 1 {
		t.Fatal("padding / content values wrong")
	}
	v := []float32{3, 4}
	l2(v)
	if math.Abs(float64(v[0]-0.6)) > 1e-6 || math.Abs(float64(v[1]-0.8)) > 1e-6 {
		t.Fatalf("l2 = %v", v)
	}
}
