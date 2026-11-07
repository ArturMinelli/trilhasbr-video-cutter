package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"trilhasbr-video-cutter/internal/apperrors"
)

// CodeModelNotFound is local so this package does not depend on apperrors gaining the constant.
const CodeModelNotFound apperrors.Code = "MODEL_NOT_FOUND"

const (
	YOLOModelFile  = "person-yolo11n.onnx"
	OSNetModelFile = "osnet_x1_0_msmt17.onnx"
)

type ModelPaths struct {
	YOLO    string
	OSNet   string
	ORTLib  string
	Version string // short hash of the model sha256s; used as detection model_version
}

type ModelResolver struct {
	// BundledRoot is the `resources` directory (contains models/ and onnxruntime/<goos>-<goarch>/).
	BundledRoot string
}

func NewModelResolver(bundledRoot string) *ModelResolver {
	return &ModelResolver{BundledRoot: bundledRoot}
}

func (r *ModelResolver) Resolve() (ModelPaths, error) {
	target := runtime.GOOS + "-" + runtime.GOARCH
	lib := map[string]string{"windows": "onnxruntime.dll", "linux": "libonnxruntime.so"}[runtime.GOOS]
	if lib == "" {
		lib = "libonnxruntime.dylib"
	}

	roots := []string{}
	if r.BundledRoot != "" {
		roots = append(roots, r.BundledRoot)
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		roots = append(roots, filepath.Join(dir, "resources"), filepath.Join(dir, "..", "Resources", "resources"), dir)
	}

	for _, root := range roots {
		p := ModelPaths{
			YOLO:   filepath.Join(root, "models", YOLOModelFile),
			OSNet:  filepath.Join(root, "models", OSNetModelFile),
			ORTLib: filepath.Join(root, "onnxruntime", target, lib),
		}
		if !fileExists(p.YOLO) || !fileExists(p.OSNet) || !fileExists(p.ORTLib) {
			continue
		}
		version, err := verifyModels(filepath.Join(root, "models"))
		if err != nil {
			return ModelPaths{}, err
		}
		p.Version = version
		return p, nil
	}

	return ModelPaths{}, apperrors.New(CodeModelNotFound, "detection models or onnxruntime library were not found", map[string]any{
		"bundledRoot": r.BundledRoot,
		"target":      target,
		"hint":        "run scripts/fetch-onnxruntime.sh (models are committed under resources/models)",
	})
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// verifyModels checks each model against manifest.json and returns a short combined version.
func verifyModels(dir string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return "", apperrors.Wrap(CodeModelNotFound, "models manifest.json is missing", err, map[string]any{"dir": dir})
	}
	var manifest map[string]struct {
		SHA256 string `json:"sha256"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return "", apperrors.Wrap(CodeModelNotFound, "models manifest.json is invalid", err, map[string]any{"dir": dir})
	}

	combined := sha256.New()
	for _, name := range []string{YOLOModelFile, OSNetModelFile} {
		want := manifest[name].SHA256
		got, err := fileSHA256(filepath.Join(dir, name))
		if err != nil {
			return "", apperrors.Wrap(CodeModelNotFound, "failed to read model", err, map[string]any{"model": name})
		}
		if want == "" || got != want {
			return "", apperrors.New(CodeModelNotFound, "model checksum mismatch", map[string]any{"model": name, "want": want, "got": got})
		}
		combined.Write([]byte(got))
	}
	return hex.EncodeToString(combined.Sum(nil))[:12], nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
