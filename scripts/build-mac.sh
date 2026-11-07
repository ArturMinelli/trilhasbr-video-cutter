#!/usr/bin/env bash
# Build the macOS app and bundle the detection models + ONNX Runtime into it.
# Fetches the ONNX Runtime library and the detection models first if missing (scripts/fetch-onnxruntime.sh, scripts/fetch-models.sh).
# Does not rely on the caller's shell state: it puts Homebrew and GOPATH/bin on PATH itself.
set -euo pipefail
cd "$(dirname "$0")/.."

BREW="${INSTALL_BREW:-/opt/homebrew/bin/brew}" # install.sh exports INSTALL_BREW
[ -x "$BREW" ] && eval "$("$BREW" shellenv)"
command -v go >/dev/null 2>&1 || { echo "ERROR: go not found; run ./install.sh" >&2; exit 1; }
GOBIN_DIR="$(GOTOOLCHAIN=local go env GOPATH)/bin"
export PATH="$GOBIN_DIR:$PATH"
# shellcheck source=scripts/toolchain.env
. scripts/toolchain.env # (cwd is the repo root here)
# Pinned Go toolchain: Homebrew's newest Go breaks `wails build` (see scripts/toolchain.env); Go downloads it on first use.
export GOTOOLCHAIN="$GO_TOOLCHAIN"
command -v wails >/dev/null 2>&1 || { echo "ERROR: wails not found; run ./install.sh" >&2; exit 1; }

scripts/fetch-onnxruntime.sh
scripts/fetch-models.sh
for f in resources/models/person-yolo11n.onnx resources/models/osnet_x1_0_msmt17.onnx resources/models/manifest.json; do
  [ -f "$f" ] || { echo "ERROR: missing $f (run scripts/fetch-models.sh)" >&2; exit 1; }
done

wails build -platform darwin/arm64

APP=build/bin/trilhasbr-video-cutter.app
[ -d "$APP" ] || { echo "ERROR: wails did not produce $APP" >&2; exit 1; }
DEST="$APP/Contents/Resources/resources" # one of the places internal/platform/models.go looks
rm -rf "$DEST"
mkdir -p "$DEST/models" "$DEST/onnxruntime/darwin-arm64"
cp resources/models/*.onnx resources/models/manifest.json "$DEST/models/"
cp resources/onnxruntime/darwin-arm64/libonnxruntime.dylib "$DEST/onnxruntime/darwin-arm64/"

# Local ad-hoc signature (arm64 requires signed code); not notarized, so not for distribution.
codesign --force --deep -s - "$APP"
codesign --verify --deep --strict "$APP"
echo "built $APP"
