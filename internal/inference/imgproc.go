package inference

// resizeRGB is a plain bilinear resize of packed RGB24.
func resizeRGB(src []byte, sw, sh, dw, dh int) []byte {
	dst := make([]byte, dw*dh*3)
	xs := float32(sw) / float32(dw)
	ys := float32(sh) / float32(dh)
	for y := 0; y < dh; y++ {
		fy := (float32(y)+0.5)*ys - 0.5
		y0 := int(fy)
		if fy < 0 {
			y0, fy = 0, 0
		}
		y1 := y0 + 1
		if y1 >= sh {
			y1 = sh - 1
		}
		wy := fy - float32(y0)
		for x := 0; x < dw; x++ {
			fx := (float32(x)+0.5)*xs - 0.5
			x0 := int(fx)
			if fx < 0 {
				x0, fx = 0, 0
			}
			x1 := x0 + 1
			if x1 >= sw {
				x1 = sw - 1
			}
			wx := fx - float32(x0)
			o := (y*dw + x) * 3
			for c := 0; c < 3; c++ {
				top := float32(src[(y0*sw+x0)*3+c])*(1-wx) + float32(src[(y0*sw+x1)*3+c])*wx
				bot := float32(src[(y1*sw+x0)*3+c])*(1-wx) + float32(src[(y1*sw+x1)*3+c])*wx
				dst[o+c] = byte(top*(1-wy) + bot*wy + 0.5)
			}
		}
	}
	return dst
}
