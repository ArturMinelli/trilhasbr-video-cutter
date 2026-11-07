package inference

import "sort"

const (
	yoloSize     = 1280
	yoloAnchors  = 33600 // 160²+80²+40² (strides 8/16/32 at 1280)
	yoloChannels = 84    // 4 box + 80 classes; class 0 = person
	confThresh   = 0.35
	nmsIoU       = 0.5
	padValue     = 114.0 / 255.0
)

// letterbox resizes f to fit yoloSize×yoloSize (padding 114) and writes planar CHW float32 into dst.
// Returns the scale and padding needed to map boxes back.
func letterbox(f Frame, dst []float32) (scale, padX, padY float32) {
	scale = float32(yoloSize) / float32(max(f.W, f.H))
	nw, nh := max(1, int(float32(f.W)*scale+0.5)), max(1, int(float32(f.H)*scale+0.5))
	nw, nh = min(nw, yoloSize), min(nh, yoloSize)
	px, py := (yoloSize-nw)/2, (yoloSize-nh)/2
	padX, padY = float32(px), float32(py)

	plane := yoloSize * yoloSize
	for i := range dst[:3*plane] {
		dst[i] = padValue
	}
	rs := resizeRGB(f.RGB, f.W, f.H, nw, nh)
	for y := 0; y < nh; y++ {
		row := (py+y)*yoloSize + px
		for x := 0; x < nw; x++ {
			o := (y*nw + x) * 3
			dst[row+x] = float32(rs[o]) / 255
			dst[plane+row+x] = float32(rs[o+1]) / 255
			dst[2*plane+row+x] = float32(rs[o+2]) / 255
		}
	}
	return scale, padX, padY
}

// decodeYOLO turns the 1×84×33600 channel-major output into person boxes in source-frame pixels.
func decodeYOLO(out []float32, scale, padX, padY float32, srcW, srcH int) []Box {
	var boxes []Box
	for i := 0; i < yoloAnchors; i++ {
		conf := out[4*yoloAnchors+i]
		if conf < confThresh {
			continue
		}
		cx, cy, w, h := out[i], out[yoloAnchors+i], out[2*yoloAnchors+i], out[3*yoloAnchors+i]
		b := Box{
			X1:   clampF((cx-w/2-padX)/scale, 0, float32(srcW)),
			Y1:   clampF((cy-h/2-padY)/scale, 0, float32(srcH)),
			X2:   clampF((cx+w/2-padX)/scale, 0, float32(srcW)),
			Y2:   clampF((cy+h/2-padY)/scale, 0, float32(srcH)),
			Conf: conf,
		}
		if b.X2-b.X1 >= 2 && b.Y2-b.Y1 >= 2 {
			boxes = append(boxes, b)
		}
	}
	return nms(boxes, nmsIoU)
}

func nms(boxes []Box, thresh float32) []Box {
	sort.Slice(boxes, func(i, j int) bool { return boxes[i].Conf > boxes[j].Conf })
	var keep []Box
	for _, b := range boxes {
		ok := true
		for _, k := range keep {
			if iou(b, k) > thresh {
				ok = false
				break
			}
		}
		if ok {
			keep = append(keep, b)
		}
	}
	return keep
}

func iou(a, b Box) float32 {
	ix := max(0, min(a.X2, b.X2)-max(a.X1, b.X1))
	iy := max(0, min(a.Y2, b.Y2)-max(a.Y1, b.Y1))
	inter := ix * iy
	union := (a.X2-a.X1)*(a.Y2-a.Y1) + (b.X2-b.X1)*(b.Y2-b.Y1) - inter
	if union <= 0 {
		return 0
	}
	return inter / union
}

func clampF(v, lo, hi float32) float32 { return max(lo, min(hi, v)) }
