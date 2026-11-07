package platform

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"trilhasbr-video-cutter/internal/apperrors"
)

type BinaryPair struct {
	FFmpegPath  string
	FFprobePath string
}

type FFmpegResolver struct {
	BundledRoot string
}

func NewFFmpegResolver(bundledRoot string) *FFmpegResolver {
	return &FFmpegResolver{BundledRoot: bundledRoot}
}

func (r *FFmpegResolver) Resolve(_ context.Context) (BinaryPair, error) {
	target := runtime.GOOS + "-" + runtime.GOARCH
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}

	candidateRoots := make([]string, 0, 3)
	if r.BundledRoot != "" {
		candidateRoots = append(candidateRoots, r.BundledRoot)
	}
	if executablePath, err := os.Executable(); err == nil {
		executableDir := filepath.Dir(executablePath)
		candidateRoots = append(candidateRoots,
			filepath.Join(executableDir, "resources", "ffmpeg"),
			filepath.Join(executableDir, "ffmpeg"),
		)
	}

	for _, root := range candidateRoots {
		ffmpegPath := filepath.Join(root, target, "ffmpeg"+ext)
		ffprobePath := filepath.Join(root, target, "ffprobe"+ext)
		if isRunnableBinary(ffmpegPath) && isRunnableBinary(ffprobePath) {
			return BinaryPair{FFmpegPath: ffmpegPath, FFprobePath: ffprobePath}, nil
		}
	}

	ffmpegPath, ffmpegOK := lookPathWithFallback("ffmpeg")
	ffprobePath, ffprobeOK := lookPathWithFallback("ffprobe")
	if ffmpegOK && ffprobeOK {
		return BinaryPair{FFmpegPath: ffmpegPath, FFprobePath: ffprobePath}, nil
	}

	return BinaryPair{}, apperrors.New(apperrors.CodeFFmpegNotFound, "ffmpeg/ffprobe binaries were not found", map[string]any{
		"bundledRoot": r.BundledRoot,
		"target":      target,
	})
}

// commonBinaryDirs are checked after PATH: apps launched from Finder get a minimal PATH
// that omits Homebrew, where ffmpeg usually lives on macOS.
var commonBinaryDirs = []string{"/opt/homebrew/bin", "/usr/local/bin"}

func lookPathWithFallback(name string) (string, bool) {
	if found, err := exec.LookPath(name); err == nil {
		return found, true
	}
	for _, dir := range commonBinaryDirs {
		if candidate := filepath.Join(dir, name); isRunnableBinary(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func isRunnableBinary(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}
