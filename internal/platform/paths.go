package platform

import (
	"os"
	"path/filepath"
	"strings"

	"trilhasbr-video-cutter/internal/apperrors"
)

func NormalizeDirectory(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", apperrors.New(apperrors.CodeValidation, "directory path is empty", nil)
	}

	absolute, err := filepath.Abs(trimmed)
	if err != nil {
		return "", apperrors.Wrap(apperrors.CodeIO, "failed to get absolute path", err, map[string]any{"path": trimmed})
	}

	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		absolute = resolved
	}

	stat, err := os.Stat(absolute)
	if err != nil {
		return "", apperrors.Wrap(apperrors.CodeIO, "directory not accessible", err, map[string]any{"path": absolute})
	}
	if !stat.IsDir() {
		return "", apperrors.New(apperrors.CodeValidation, "path is not a directory", map[string]any{"path": absolute})
	}

	return filepath.Clean(absolute), nil
}

func WorkspaceNameFromDirectory(directory string) string {
	name := filepath.Base(directory)
	if name == "." || name == string(filepath.Separator) || name == "" {
		return "Workspace"
	}
	return name
}

func EnsureDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return apperrors.Wrap(apperrors.CodeIO, "failed to create directory", err, map[string]any{"path": path})
	}
	return nil
}

func AppDataDir(appName string) (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", apperrors.Wrap(apperrors.CodeIO, "failed to locate user config directory", err, nil)
	}
	path := filepath.Join(configDir, appName)
	if err = EnsureDir(path); err != nil {
		return "", err
	}
	return path, nil
}
