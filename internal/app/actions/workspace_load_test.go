package actions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverMP4Files_SkipsExportsDirectoriesRecursively(t *testing.T) {
	root := t.TempDir()

	mustWriteFile(t, filepath.Join(root, "root.mp4"))
	mustWriteFile(t, filepath.Join(root, "nested", "camera.mp4"))
	mustWriteFile(t, filepath.Join(root, "exports", "skip-this.mp4"))
	mustWriteFile(t, filepath.Join(root, "nested", "Exports", "skip-too.mp4"))

	paths, err := discoverMP4Files(root)
	if err != nil {
		t.Fatalf("discoverMP4Files returned error: %v", err)
	}

	if len(paths) != 2 {
		t.Fatalf("expected 2 mp4 files after skipping exports, got %d (%v)", len(paths), paths)
	}

	for _, item := range paths {
		if filepath.Base(item) == "skip-this.mp4" || filepath.Base(item) == "skip-too.mp4" {
			t.Fatalf("found mp4 from exports directory in results: %s", item)
		}
	}
}

func mustWriteFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create parent directories: %v", err)
	}
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
}
