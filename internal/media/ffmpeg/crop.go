package ffmpeg

import (
	"image"
	"image/jpeg"
	"os"
	"path/filepath"

	"trilhasbr-video-cutter/internal/apperrors"
)

// WriteJPEG saves the box (x1,y1)-(x2,y2) of f as a JPEG, clamped to the frame.
func WriteJPEG(path string, f Frame, x1, y1, x2, y2 int) error {
	x1, x2 = max(0, min(x1, f.Width)), max(0, min(x2, f.Width))
	y1, y2 = max(0, min(y1, f.Height)), max(0, min(y2, f.Height))
	if x2 <= x1 || y2 <= y1 || len(f.RGB) < f.Width*f.Height*3 {
		return apperrors.New(apperrors.CodeValidation, "empty crop box", map[string]any{"box": []int{x1, y1, x2, y2}})
	}
	img := image.NewRGBA(image.Rect(0, 0, x2-x1, y2-y1))
	for y := y1; y < y2; y++ {
		for x := x1; x < x2; x++ {
			s, d := (y*f.Width+x)*3, img.PixOffset(x-x1, y-y1)
			img.Pix[d], img.Pix[d+1], img.Pix[d+2], img.Pix[d+3] = f.RGB[s], f.RGB[s+1], f.RGB[s+2], 255
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return apperrors.Wrap(apperrors.CodeIO, "failed to create crop dir", err, map[string]any{"path": path})
	}
	out, err := os.Create(path)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeIO, "failed to create crop file", err, map[string]any{"path": path})
	}
	defer out.Close()
	if err := jpeg.Encode(out, img, &jpeg.Options{Quality: 85}); err != nil {
		return apperrors.Wrap(apperrors.CodeIO, "failed to encode crop", err, map[string]any{"path": path})
	}
	return nil
}
