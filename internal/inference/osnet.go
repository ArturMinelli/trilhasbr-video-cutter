package inference

import "math"

const (
	osnetW, osnetH = 128, 256
	osnetMaxBatch  = 16
)

var (
	imgMean = [3]float32{0.485, 0.456, 0.406}
	imgStd  = [3]float32{0.229, 0.224, 0.225}
)

// osnetInput fills planar NCHW float32 (ImageNet-normalized) for the given crops.
func osnetInput(crops []Frame) []float32 {
	plane := osnetW * osnetH
	data := make([]float32, len(crops)*3*plane)
	for n, c := range crops {
		rs := resizeRGB(c.RGB, c.W, c.H, osnetW, osnetH)
		base := n * 3 * plane
		for i := 0; i < plane; i++ {
			for ch := 0; ch < 3; ch++ {
				data[base+ch*plane+i] = (float32(rs[i*3+ch])/255 - imgMean[ch]) / imgStd[ch]
			}
		}
	}
	return data
}

// l2 normalizes v in place.
func l2(v []float32) {
	var s float64
	for _, x := range v {
		s += float64(x) * float64(x)
	}
	n := float32(math.Sqrt(s))
	if n == 0 {
		return
	}
	for i := range v {
		v[i] /= n
	}
}
