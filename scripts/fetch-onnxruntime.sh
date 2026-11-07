#!/usr/bin/env bash
# Download the official ONNX Runtime (macOS arm64) into resources/onnxruntime/darwin-arm64/.
# The detection models are committed in resources/models/, so only the native library is fetched.
# Safe to re-run: skips when the library is already present and matches the pinned hash.
set -euo pipefail

ORT_VERSION=1.31.0
TGZ_SHA256=1031b36dd3bdaa8d976a6130d9cf2a05e9be320b93ae804f8427b91dc3432afc  # onnxruntime-osx-arm64-1.31.0.tgz
DYLIB_SHA256=2a50a63e7f41f8d4d551d3ee47248df3864f6540109fce8ad520ca3b51af4b70 # lib/libonnxruntime.1.31.0.dylib

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEST_DIR="$ROOT/resources/onnxruntime/darwin-arm64"
DEST="$DEST_DIR/libonnxruntime.dylib"
URL="https://github.com/microsoft/onnxruntime/releases/download/v$ORT_VERSION/onnxruntime-osx-arm64-$ORT_VERSION.tgz"

sha() { shasum -a 256 "$1" | awk '{print $1}'; }

if [ -f "$DEST" ] && [ "$(sha "$DEST")" = "$DYLIB_SHA256" ]; then
  echo "onnxruntime $ORT_VERSION already present: $DEST"
  exit 0
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$DEST_DIR"

echo "downloading onnxruntime $ORT_VERSION ..."
ok=0
for attempt in 1 2 3; do
  if curl -fL --connect-timeout 20 --max-time 600 -sS -o "$TMP/ort.tgz" "$URL" && [ "$(sha "$TMP/ort.tgz")" = "$TGZ_SHA256" ]; then
    ok=1
    break
  fi
  echo "download attempt $attempt failed (network error or checksum mismatch)" >&2
  sleep $((attempt * 3))
done
[ "$ok" = 1 ] || { echo "ERROR: could not download a verified onnxruntime from $URL" >&2; exit 1; }

tar xzf "$TMP/ort.tgz" -C "$TMP"
SRC="$TMP/onnxruntime-osx-arm64-$ORT_VERSION/lib/libonnxruntime.$ORT_VERSION.dylib"
[ "$(sha "$SRC")" = "$DYLIB_SHA256" ] || { echo "ERROR: extracted library hash mismatch" >&2; exit 1; }

cp "$SRC" "$DEST.tmp" && mv -f "$DEST.tmp" "$DEST" # atomic replace
echo "installed $DEST"
