#!/usr/bin/env bash
# Download the detection models into resources/models/ from the GitHub release (scripts/release.env).
# Every file is verified against the PINNED sha256 in resources/models/manifest.json: a download is never trusted.
# Safe to re-run and works offline when the files are already present and valid.
# MODELS_BASE_URL overrides the release URL (a mirror, or file:///some/dir for tests).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEST_DIR="$ROOT/resources/models"
MANIFEST="$DEST_DIR/manifest.json"
MODELS="person-yolo11n.onnx osnet_x1_0_msmt17.onnx"

# shellcheck source=scripts/release.env
. "$ROOT/scripts/release.env"
BASE_URL="${MODELS_BASE_URL:-https://github.com/$GITHUB_REPO/releases/download/$MODELS_TAG}"

sha() { shasum -a 256 "$1" | awk '{print $1}'; }
manifest_sha() { sed -n "/\"$1\"/,/sha256/s/.*\"sha256\": *\"\([0-9a-f]*\)\".*/\1/p" "$MANIFEST"; }
die() { echo "ERROR: $*" >&2; exit 1; }

[ -f "$MANIFEST" ] || die "$MANIFEST is missing (it is committed in the repository; check your checkout)."

TMP=""
trap '[ -z "$TMP" ] || rm -rf "$TMP"' EXIT

for model in $MODELS; do
  want="$(manifest_sha "$model")"
  [ -n "$want" ] || die "no sha256 for $model in resources/models/manifest.json."
  dest="$DEST_DIR/$model"

  if [ -f "$dest" ] && [ "$(sha "$dest")" = "$want" ]; then
    echo "model already present: $model"
    continue
  fi

  case "$BASE_URL" in
    *CHANGE-ME*) die "scripts/release.env still has the placeholder GITHUB_REPO; set it to <owner>/<repo>, or set MODELS_BASE_URL." ;;
  esac
  [ -n "$TMP" ] || TMP="$(mktemp -d)"
  echo "downloading model $model ..."
  ok=0
  for attempt in 1 2 3; do
    if curl -fL --connect-timeout 20 --max-time 600 -sS -o "$TMP/$model" "$BASE_URL/$model" && [ "$(sha "$TMP/$model")" = "$want" ]; then
      ok=1
      break
    fi
    echo "download attempt $attempt failed for $model (network error or checksum mismatch)" >&2
    sleep $((attempt * 3))
  done
  [ "$ok" = 1 ] || die "could not download a verified $model from $BASE_URL/$model (expected sha256 $want). Check your network, or set MODELS_BASE_URL to a mirror."

  mkdir -p "$DEST_DIR"
  cp "$TMP/$model" "$dest.tmp" && mv -f "$dest.tmp" "$dest" # atomic replace
  echo "installed $dest"
done
