package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"trilhasbr-video-cutter/internal/apperrors"
)

func OpenFolder(path string) error {
	target := path
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		target = filepath.Dir(path)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("explorer", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}

	if err := cmd.Run(); err != nil {
		return apperrors.Wrap(apperrors.CodeIO, "failed to open folder in file manager", err, map[string]any{"path": target, "os": runtime.GOOS})
	}

	return nil
}
