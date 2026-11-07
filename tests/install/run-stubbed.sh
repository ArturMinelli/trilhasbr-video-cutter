#!/usr/bin/env bash
# Runs the REAL install.sh against fake tools (tests/install/stubs/stub) in throwaway sandboxes, to exercise the
# paths a Mac that already has Homebrew/Go/Node/ffmpeg can never reach: first-time Command Line Tools install,
# Homebrew install, brew installs, go/node too old, no sudo, bad checksums, profile writing, Rosetta, odd invocation.
# Each scenario gets its own fake HOME and a PATH holding only the fakes plus symlinks to basic system tools.
# Nothing outside the sandbox is touched (the only shared file is the /tmp Command Line Tools placeholder, which
# install.sh itself creates and removes). Not covered: --install-app (it writes /Applications), real network installs.
#
#   tests/install/run-stubbed.sh            run every scenario
#   tests/install/run-stubbed.sh S03 S08    run scenarios whose id starts with one of the arguments
#   tests/install/run-stubbed.sh --list     list scenarios
#   INSTALL_SH_UNDER_TEST=/path/install.sh tests/install/run-stubbed.sh   test another copy of install.sh
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
STUBS="$ROOT/tests/install/stubs"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# Basic tools install.sh needs for real (everything else on PATH is a fake).
REAL_TOOLS="awk sed grep sort head tail tee df cat date mktemp rm mkdir cp mv touch tar shasum basename dirname ps ln chmod wc tr cut readlink env bash sh ls stat find uniq mkfifo expr cmp"

# A version of install.sh without the INSTALL_BREW hook would run the machine's real Homebrew: refuse.
if [ -n "${INSTALL_SH_UNDER_TEST:-}" ] && ! grep -q INSTALL_BREW "$INSTALL_SH_UNDER_TEST"; then
  echo "INSTALL_SH_UNDER_TEST must support INSTALL_BREW (otherwise the real Homebrew would be used)" >&2
  exit 1
fi

# ---------------------------------------------------------------- ONNX fixtures
mkdir -p "$WORK/ort/good/onnxruntime-osx-arm64-1.31.0/lib" "$WORK/ort/bad/onnxruntime-osx-arm64-1.31.0/lib"
printf 'fake-onnxruntime-dylib\n' > "$WORK/ort/good/onnxruntime-osx-arm64-1.31.0/lib/libonnxruntime.1.31.0.dylib"
printf 'tampered-dylib\n' > "$WORK/ort/bad/onnxruntime-osx-arm64-1.31.0/lib/libonnxruntime.1.31.0.dylib"
tar czf "$WORK/ort/good.tgz" -C "$WORK/ort/good" onnxruntime-osx-arm64-1.31.0
tar czf "$WORK/ort/bad.tgz" -C "$WORK/ort/bad" onnxruntime-osx-arm64-1.31.0
sha() { shasum -a 256 "$1" | awk '{print $1}'; }
GOOD_TGZ_SHA="$(sha "$WORK/ort/good.tgz")"
GOOD_DYLIB_SHA="$(sha "$WORK/ort/good/onnxruntime-osx-arm64-1.31.0/lib/libonnxruntime.1.31.0.dylib")"

# ---------------------------------------------------------------- model fixtures
# Fake model files; each sandbox gets a manifest.json whose pins are patched to match them (the real pins cannot).
mkdir -p "$WORK/models/good" "$WORK/models/bad"
printf 'fake-yolo-model\n' > "$WORK/models/good/person-yolo11n.onnx"
printf 'fake-osnet-model\n' > "$WORK/models/good/osnet_x1_0_msmt17.onnx"
printf 'tampered-yolo-model\n' > "$WORK/models/bad/person-yolo11n.onnx" # served by the fake release in the tampering scenario
printf 'fake-osnet-model\n' > "$WORK/models/bad/osnet_x1_0_msmt17.onnx"
GOOD_YOLO_SHA="$(sha "$WORK/models/good/person-yolo11n.onnx")"
GOOD_OSNET_SHA="$(sha "$WORK/models/good/osnet_x1_0_msmt17.onnx")"

# ------------------------------------------------------------------ test kit
PASS=0
FAIL=0
RESULTS=""
SCEN_ID=""
SCEN_TITLE=""
SCEN_FAILS=0
SCEN_DETAIL=""
SB=""
REPO=""
OUT=""
RC=0
EXTRA_ENV=()
SKIP_PROFILE=1
RUN_MODE=abs

begin() {
  SCEN_ID="$1"
  SCEN_TITLE="$2"
  SCEN_FAILS=0
  SCEN_DETAIL=""
  EXTRA_ENV=()
  SKIP_PROFILE=1
  RUN_MODE=abs
}

fail() {
  SCEN_FAILS=$((SCEN_FAILS + 1))
  SCEN_DETAIL="$SCEN_DETAIL
      - $1"
}

finish() {
  if [ "$SCEN_FAILS" = 0 ]; then
    PASS=$((PASS + 1))
    RESULTS="$RESULTS
PASS  $SCEN_ID  $SCEN_TITLE"
  else
    FAIL=$((FAIL + 1))
    RESULTS="$RESULTS
FAIL  $SCEN_ID  $SCEN_TITLE$SCEN_DETAIL
      --- last output lines:
$(tail -n 12 "$OUT" 2>/dev/null | sed 's/^/      | /')"
  fi
}

expect_rc() { [ "$RC" = "$1" ] || fail "exit code $RC, expected $1"; }
expect_rc_nonzero() { [ "$RC" != 0 ] || fail "exit code 0, expected a failure"; }
expect_out() { grep -qF -- "$1" "$OUT" || fail "output lacks: $1"; }
expect_no_out() { ! grep -qF -- "$1" "$OUT" || fail "output unexpectedly has: $1"; }
expect_log() { grep -qE -- "$1" "$SB/stub.log" 2>/dev/null || fail "stub log lacks /$1/"; }
expect_no_log() { ! grep -qE -- "$1" "$SB/stub.log" 2>/dev/null || fail "stub log unexpectedly has /$1/"; }
expect_log_count() { # expect_log_count REGEX N
  local n
  n=$(grep -cE -- "$1" "$SB/stub.log" 2>/dev/null || true)
  [ "$n" = "$2" ] || fail "stub log has $n x /$1/, expected $2"
}
expect_log_before() { # expect_log_before FIRST SECOND
  local a b
  a=$(grep -nE -- "$1" "$SB/stub.log" | head -n1 | cut -d: -f1)
  b=$(grep -nE -- "$2" "$SB/stub.log" | head -n1 | cut -d: -f1)
  { [ -n "$a" ] && [ -n "$b" ] && [ "$a" -lt "$b" ]; } || fail "expected /$1/ before /$2/ in the stub log (lines: ${a:-none}, ${b:-none})"
}
expect_env() { grep -qE -- "$1" "$SB/stub.log.env" 2>/dev/null || fail "env log lacks /$1/"; }
expect_no_env() { ! grep -qE -- "$1" "$SB/stub.log.env" 2>/dev/null || fail "env log unexpectedly has /$1/"; }
expect_file() { [ -e "$1" ] || fail "missing file: $1"; }
expect_no_file() { [ ! -e "$1" ] || fail "unexpected file: $1"; }

fake() { # fake NAME...: put the stub in the sandbox bin under each name
  local n
  for n in "$@"; do ln -sf "$STUBS/stub" "$SB/bin/$n"; done
}

# new_sandbox ID [REPO_SUBPATH]: fresh HOME, bin dir and a copy of the files under test.
new_sandbox() {
  local t
  SB="$WORK/$1"
  REPO="$SB/${2:-repo}"
  OUT="$SB/out.txt"
  mkdir -p "$SB/bin" "$SB/home" "$SB/state" "$SB/gopath/bin" "$SB/prefix/bin" \
    "$REPO/frontend" "$REPO/resources/onnxruntime/darwin-arm64"
  : > "$SB/stub.log"
  : > "$SB/stub.log.env"
  for t in $REAL_TOOLS; do
    ln -sf "$(command -v "$t")" "$SB/bin/$t"
  done
  fake uname sysctl sw_vers id sleep arch sudo xcode-select softwareupdate clang curl codesign
  # the files under test: real install.sh and scripts/, real go.mod / package files / models
  cp "${INSTALL_SH_UNDER_TEST:-$ROOT/install.sh}" "$REPO/install.sh" # INSTALL_SH_UNDER_TEST: run against another version (mutation check)
  cp -R "$ROOT/scripts" "$REPO/scripts"
  cp "$ROOT/go.mod" "$REPO/go.mod"
  cp "$ROOT/frontend/package.json" "$ROOT/frontend/package-lock.json" "$REPO/frontend/"
  mkdir -p "$REPO/resources/models"
  patch_model_pins
  place_models
  patch_release_env
  # fetch script: pin the fixture hashes (the real pins cannot match a fake tarball); default = lib already present
  patch_ort_pins
  place_ort_dylib
}

patch_model_pins() {
  sed -E "/person-yolo11n.onnx/,/sha256/ s/\"sha256\": *\"[0-9a-f]+\"/\"sha256\": \"$GOOD_YOLO_SHA\"/; /osnet_x1_0_msmt17.onnx/,/sha256/ s/\"sha256\": *\"[0-9a-f]+\"/\"sha256\": \"$GOOD_OSNET_SHA\"/" \
    "$ROOT/resources/models/manifest.json" > "$REPO/resources/models/manifest.json"
}
place_models() { cp "$WORK/models/good/"*.onnx "$REPO/resources/models/"; }
remove_models() { rm -f "$REPO/resources/models/"*.onnx; }
patch_release_env() { # a resolvable repo name so the fake release URL is well formed
  sed 's|^GITHUB_REPO=.*|GITHUB_REPO="test/repo"|' "$REPO/scripts/release.env" > "$REPO/scripts/release.env.new" && mv "$REPO/scripts/release.env.new" "$REPO/scripts/release.env"
}

patch_ort_pins() {
  local f="$REPO/scripts/fetch-onnxruntime.sh"
  sed -E "s/^TGZ_SHA256=[0-9a-f]+/TGZ_SHA256=$GOOD_TGZ_SHA/; s/^DYLIB_SHA256=[0-9a-f]+/DYLIB_SHA256=$GOOD_DYLIB_SHA/" "$f" > "$f.new" && mv "$f.new" "$f" && chmod +x "$f"
}
place_ort_dylib() { cp "$WORK/ort/good/onnxruntime-osx-arm64-1.31.0/lib/libonnxruntime.1.31.0.dylib" "$REPO/resources/onnxruntime/darwin-arm64/libonnxruntime.dylib"; }
remove_ort_dylib() { rm -f "$REPO/resources/onnxruntime/darwin-arm64/libonnxruntime.dylib"; }

# Tools a Mac with everything installed would have.
present_tools() {
  fake go node npm ffmpeg ffprobe
  ln -sf "$STUBS/stub" "$SB/gopath/bin/wails"
  ln -sf "$STUBS/stub" "$SB/prefix/bin/brew"
}

# run_install ARGS...: run the real install.sh in the sandbox; sets RC, writes $OUT, appends to the stub log.
run_install() {
  local cwd="$SB" env_prefix
  case "$RUN_MODE" in
    rel | sh | zsh | pipe) cwd="$REPO" ;;
    relother) cwd="$(dirname "$REPO")" ;;
    build) cwd="$REPO/scripts" ;; # build-mac.sh must also work when started from inside scripts/
  esac
  env_prefix=(HOME="$SB/home" PATH="$SB/bin" TERM=dumb STUB_HOME="$STUBS" STUB_LOG="$SB/stub.log"
    STUB_STATE="$SB/state" STUB_GOPATH="$SB/gopath" FAKE_PREFIX="$SB/prefix" FAKE_ORT_TGZ="$WORK/ort/good.tgz" FAKE_MODEL_DIR="$WORK/models/good"
    INSTALL_BREW="$SB/prefix/bin/brew")
  [ "$SKIP_PROFILE" = 1 ] && env_prefix+=(INSTALL_SKIP_PROFILE=1)
  (
    cd "$cwd" || exit 99
    case "$RUN_MODE" in
      abs) env -i "${env_prefix[@]}" ${EXTRA_ENV[@]+"${EXTRA_ENV[@]}"} /bin/bash "$REPO/install.sh" "$@" ;;
      rel) env -i "${env_prefix[@]}" ${EXTRA_ENV[@]+"${EXTRA_ENV[@]}"} ./install.sh "$@" ;;
      sh) env -i "${env_prefix[@]}" ${EXTRA_ENV[@]+"${EXTRA_ENV[@]}"} sh install.sh "$@" ;;
      relother) env -i "${env_prefix[@]}" ${EXTRA_ENV[@]+"${EXTRA_ENV[@]}"} /bin/bash "$(basename "$REPO")/install.sh" "$@" ;;
      zsh) env -i "${env_prefix[@]}" ${EXTRA_ENV[@]+"${EXTRA_ENV[@]}"} zsh install.sh "$@" ;;
      build) env -i "${env_prefix[@]}" ${EXTRA_ENV[@]+"${EXTRA_ENV[@]}"} /bin/bash ./build-mac.sh "$@" ;;
      pipe) env -i "${env_prefix[@]}" ${EXTRA_ENV[@]+"${EXTRA_ENV[@]}"} /bin/bash -c 'cat install.sh | /bin/bash -s -- "$@"' _ "$@" ;;
    esac
  ) < /dev/null > "$OUT" 2>&1 # straight to a file: output must be complete the moment install.sh exits
  RC=$?
}

# ---------------------------------------------------------------- scenarios
CLT_PLACEHOLDER=/tmp/.com.apple.dt.CommandLineTools.installondemand.in-progress-file

sc_s01a() {
  begin S01a "Rosetta (x86_64) shell on Apple Silicon re-runs natively"
  new_sandbox s01a; present_tools
  EXTRA_ENV=(FAKE_UNAME_M=x86_64 FAKE_ARM64=1)
  run_install --skip-build --yes
  expect_rc 0
  expect_out "Rosetta"
  expect_log "^arch -arm64 /bin/bash $REPO/install.sh --skip-build --yes$"
  finish
}
sc_s01b() {
  begin S01b "Intel Mac (no arm64) is refused with a clear message"
  new_sandbox s01b; present_tools
  EXTRA_ENV=(FAKE_UNAME_M=x86_64 FAKE_ARM64=0)
  run_install --skip-build --yes
  expect_rc 1
  expect_out "Apple Silicon (arm64) Mac is required; this machine reports x86_64"
  finish
}
sc_s01c() {
  begin S01c "non-macOS is refused"
  new_sandbox s01c; present_tools
  EXTRA_ENV=(FAKE_UNAME_S=Linux)
  run_install --skip-build --yes
  expect_rc 1
  expect_out "macOS is required (found Linux)"
  finish
}

sc_s02a() {
  begin S02a "CLT missing: picks the newest label, installs via softwareupdate, removes the placeholder"
  new_sandbox s02a; present_tools
  rm -f "$CLT_PLACEHOLDER"
  EXTRA_ENV=(FAKE_CLT=0 FAKE_SU_FIXTURE=softwareupdate-clt-3-labels.txt)
  run_install --skip-build --yes
  expect_rc 0
  expect_log "^softwareupdate: placeholder=present$"
  expect_log "^sudo softwareupdate -i Command Line Tools for Xcode-16.2 --verbose$"
  expect_no_log "softwareupdate -i Command Line Tools for Xcode-(16.0|15.4)"
  expect_no_log "softwareupdate -i macOS"
  expect_log_before "^softwareupdate -l" "^softwareupdate -i"
  expect_log "^sudo xcode-select --switch /Library/Developer/CommandLineTools$"
  expect_out "installing: Command Line Tools for Xcode-16.2"
  expect_out "Apple clang version"
  expect_no_file "$CLT_PLACEHOLDER"
  finish
}
sc_s02b() {
  begin S02b "CLT missing and not offered by Software Update: falls back to xcode-select --install and fails clearly"
  new_sandbox s02b; present_tools
  rm -f "$CLT_PLACEHOLDER"
  EXTRA_ENV=(FAKE_CLT=0 FAKE_SU_FIXTURE=softwareupdate-none.txt)
  run_install --skip-build --yes
  expect_rc 1
  expect_log_count "^softwareupdate -l" 3
  expect_log "^xcode-select --install$"
  expect_no_log "softwareupdate -i"
  expect_out "could not find the Command Line Tools in Software Update"
  expect_no_file "$CLT_PLACEHOLDER"
  finish
}

sc_s03() {
  begin S03 "nothing installed: Homebrew, go/node/ffmpeg, wails installed in order; profile written"
  new_sandbox s03
  fake sleep # (already) keep the retry back-offs instant
  SKIP_PROFILE=0
  run_install --skip-build --yes
  expect_rc 0
  expect_log "^curl .*https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh$"
  expect_log "^homebrew-installer NONINTERACTIVE=1 "
  expect_log "^brew install go node ffmpeg$"
  expect_log "^go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0$"
  expect_log_before "homebrew-installer" "^brew install"
  expect_log_before "^brew install" "^go install"
  expect_log "^sudo -n true$"
  expect_out "installing with Homebrew: go node ffmpeg"
  expect_out "wails doctor: ready"
  grep -qF '>>> trilhasbr-video-cutter install >>>' "$SB/home/.zprofile" 2>/dev/null || fail "profile block not written"
  finish
}
sc_s04a() {
  begin S04a "everything already installed: no brew install, no downloads, no wails install"
  new_sandbox s04a; present_tools
  run_install --skip-build --yes
  expect_rc 0
  expect_no_log "^brew install"
  expect_no_log "Homebrew/install"
  expect_no_log "^go install"
  expect_no_log "^sudo softwareupdate"
  expect_out "go, node and ffmpeg are already installed"
  expect_out "wails v2.11.0 already installed"
  finish
}
sc_s04b() {
  begin S04b "only ffmpeg missing: brew installs exactly ffmpeg"
  new_sandbox s04b; present_tools
  rm -f "$SB/bin/ffmpeg" "$SB/bin/ffprobe"
  run_install --skip-build --yes
  expect_rc 0
  expect_log "^brew install ffmpeg$"
  expect_no_log "^brew install .*(go|node)"
  finish
}
sc_s05a() {
  begin S05a "go older than go.mod: clear failure naming both versions"
  new_sandbox s05a; present_tools
  EXTRA_ENV=(FAKE_GO_VERSION=1.22.0)
  run_install --skip-build --yes
  expect_rc 1
  expect_log "^brew install go$"
  expect_out "Go >= 1.24.0 is required"
  expect_out "go1.22.0"
  finish
}
sc_s05b() {
  begin S05b "node older than the minimum: clear failure"
  new_sandbox s05b; present_tools
  EXTRA_ENV=(FAKE_NODE_MAJOR=16)
  run_install --skip-build --yes
  expect_rc 1
  expect_log "^brew install node$"
  expect_out "Node.js >= 18 with npm is required"
  finish
}
sc_s06a() {
  begin S06a "CLT missing, sudo needs a password, --yes, no TTY: fails early, before any long step"
  new_sandbox s06a; present_tools
  EXTRA_ENV=(FAKE_CLT=0 FAKE_SUDO_N=1 FAKE_SU_FIXTURE=softwareupdate-clt-3-labels.txt)
  run_install --skip-build --yes
  expect_rc 1
  expect_out "administrator rights are needed"
  expect_out "Run ./install.sh in a Terminal, or configure passwordless sudo"
  expect_no_log "^softwareupdate"
  expect_no_log "^sudo softwareupdate"
  expect_no_log "^brew"
  finish
}
sc_s06b() {
  begin S06b "Homebrew missing, sudo needs a password, no TTY: fails early, nothing downloaded"
  new_sandbox s06b
  EXTRA_ENV=(FAKE_SUDO_N=1)
  run_install --yes
  expect_rc 1
  expect_out "administrator rights are needed"
  expect_no_log "Homebrew/install"
  expect_no_log "^brew"
  finish
}
sc_s07() {
  begin S07 "running as root is refused"
  new_sandbox s07; present_tools
  EXTRA_ENV=(FAKE_UID=0)
  run_install --skip-build --yes
  expect_rc 1
  expect_out "do not run install.sh as root"
  expect_no_log "^brew"
  finish
}

sc_s08a() {
  begin S08a "ONNX Runtime download with a wrong checksum: retried 3x, fails, nothing installed"
  new_sandbox s08a; present_tools; remove_ort_dylib
  EXTRA_ENV=(FAKE_ORT_TGZ="$WORK/ort/bad.tgz")
  run_install --skip-build --yes
  expect_rc 1
  expect_log_count "^curl .*onnxruntime-osx-arm64-1.31.0.tgz$" 3
  expect_out "could not download a verified onnxruntime"
  expect_no_file "$REPO/resources/onnxruntime/darwin-arm64/libonnxruntime.dylib"
  finish
}
sc_s08b() {
  begin S08b "ONNX Runtime download with the right checksum: installed and verified"
  new_sandbox s08b; present_tools; remove_ort_dylib
  run_install --skip-build --yes
  expect_rc 0
  expect_log_count "^curl .*onnxruntime-osx-arm64-1.31.0.tgz$" 1
  expect_file "$REPO/resources/onnxruntime/darwin-arm64/libonnxruntime.dylib"
  [ "$(sha "$REPO/resources/onnxruntime/darwin-arm64/libonnxruntime.dylib")" = "$GOOD_DYLIB_SHA" ] || fail "installed dylib hash differs"
  finish
}
sc_s08c() {
  begin S08c "ONNX Runtime already present and valid: skipped without any download"
  new_sandbox s08c; present_tools
  run_install --skip-build --yes
  expect_rc 0
  expect_no_log "onnxruntime-osx-arm64"
  expect_out "already present"
  finish
}
sc_s08d() {
  begin S08d "ONNX Runtime present but corrupt: re-downloaded"
  new_sandbox s08d; present_tools
  printf 'corrupted\n' > "$REPO/resources/onnxruntime/darwin-arm64/libonnxruntime.dylib"
  run_install --skip-build --yes
  expect_rc 0
  expect_log_count "^curl .*onnxruntime-osx-arm64-1.31.0.tgz$" 1
  [ "$(sha "$REPO/resources/onnxruntime/darwin-arm64/libonnxruntime.dylib")" = "$GOOD_DYLIB_SHA" ] || fail "dylib was not repaired"
  finish
}
sc_s08e() {
  begin S08e "the repository's real ONNX pins are 64-hex sha256 values (the fixtures only patch a sandbox copy)"
  new_sandbox s08e
  grep -qE '^TGZ_SHA256=[0-9a-f]{64} ' "$ROOT/scripts/fetch-onnxruntime.sh" || fail "TGZ_SHA256 pin missing in the real script"
  grep -qE '^DYLIB_SHA256=[0-9a-f]{64} ' "$ROOT/scripts/fetch-onnxruntime.sh" || fail "DYLIB_SHA256 pin missing in the real script"
  # shellcheck disable=SC2016 # the script text contains a literal $ORT_VERSION
  grep -qF 'releases/download/v$ORT_VERSION/' "$ROOT/scripts/fetch-onnxruntime.sh" || fail "download URL not from the official release"
  OUT=/dev/null
  finish
}

sc_s09a() {
  begin S09a "a corrupt local model is replaced by a verified download (the other model is left alone)"
  new_sandbox s09a; present_tools
  printf 'garbage\n' > "$REPO/resources/models/person-yolo11n.onnx"
  run_install --skip-build --yes
  expect_rc 0
  expect_log_count "^curl .*person-yolo11n.onnx$" 1
  expect_no_log "^curl .*osnet_x1_0_msmt17.onnx"
  [ "$(sha "$REPO/resources/models/person-yolo11n.onnx")" = "$GOOD_YOLO_SHA" ] || fail "model was not repaired"
  finish
}
sc_s09b() {
  begin S09b "missing models are downloaded and verified"
  new_sandbox s09b; present_tools; remove_models
  run_install --skip-build --yes
  expect_rc 0
  expect_log_count "^curl .*person-yolo11n.onnx$" 1
  expect_log_count "^curl .*osnet_x1_0_msmt17.onnx$" 1
  expect_log "^curl .*https://github.com/test/repo/releases/download/models-v1/person-yolo11n.onnx$"
  [ "$(sha "$REPO/resources/models/osnet_x1_0_msmt17.onnx")" = "$GOOD_OSNET_SHA" ] || fail "osnet hash differs"
  expect_no_file "$REPO/resources/models/person-yolo11n.onnx.tmp"
  finish
}
sc_s09c() {
  begin S09c "a tampered download (wrong sha256) is retried 3x, then fails and installs nothing"
  new_sandbox s09c; present_tools; remove_models
  EXTRA_ENV=(FAKE_MODEL_DIR="$WORK/models/bad")
  run_install --skip-build --yes
  expect_rc 1
  expect_log_count "^curl .*person-yolo11n.onnx$" 3
  expect_out "could not download a verified person-yolo11n.onnx"
  expect_no_file "$REPO/resources/models/person-yolo11n.onnx"
  expect_no_file "$REPO/resources/models/person-yolo11n.onnx.tmp"
  finish
}
sc_s09d() {
  begin S09d "scripts/fetch-models.sh with valid models: no download at all (works offline)"
  new_sandbox s09d; present_tools
  # the script alone (install.sh itself has a network preflight even when everything is installed)
  (cd "$REPO" && env -i HOME="$SB/home" PATH="$SB/bin" STUB_HOME="$STUBS" STUB_LOG="$SB/stub.log" STUB_STATE="$SB/state" \
    FAKE_NET=0 FAKE_MODEL_DIR="$WORK/models/good" ./scripts/fetch-models.sh) < /dev/null > "$OUT" 2>&1
  RC=$?
  expect_rc 0
  expect_no_log "\\.onnx"
  expect_out "model already present: person-yolo11n.onnx"
  expect_out "model already present: osnet_x1_0_msmt17.onnx"
  finish
}
sc_s09e() {
  begin S09e "MODELS_BASE_URL overrides the release URL (mirror / file://)"
  new_sandbox s09e; present_tools; remove_models
  EXTRA_ENV=(MODELS_BASE_URL="file:///mirror/models")
  run_install --skip-build --yes
  expect_rc 0
  expect_log "^curl .*file:///mirror/models/person-yolo11n.onnx$"
  expect_no_log "github.com/test/repo"
  finish
}
sc_s09f() {
  begin S09f "unresolved GITHUB_REPO placeholder fails clearly before any download"
  new_sandbox s09f; present_tools; remove_models
  cp "$ROOT/scripts/release.env" "$REPO/scripts/release.env" # the real file, placeholder included
  run_install --skip-build --yes
  if grep -q '^GITHUB_REPO="CHANGE-ME' "$ROOT/scripts/release.env"; then
    expect_rc 1
    expect_out "still has the placeholder GITHUB_REPO"
    expect_no_log "\\.onnx"
  else
    expect_rc 0 # the placeholder has been replaced with a real repo name
  fi
  finish
}
sc_s09g() {
  begin S09g "the real release.env and fetch-models.sh: URL shape and 64-hex pins in manifest.json"
  new_sandbox s09g
  grep -qE '^GITHUB_REPO="[^"]+"' "$ROOT/scripts/release.env" || fail "GITHUB_REPO missing in release.env"
  grep -qE '^MODELS_TAG="[^"]+"' "$ROOT/scripts/release.env" || fail "MODELS_TAG missing in release.env"
  # shellcheck disable=SC2016 # the script text contains literal $GITHUB_REPO/$MODELS_TAG
  grep -qF 'https://github.com/$GITHUB_REPO/releases/download/$MODELS_TAG' "$ROOT/scripts/fetch-models.sh" || fail "release URL shape changed"
  [ "$(grep -cE '"sha256": *"[0-9a-f]{64}"' "$ROOT/resources/models/manifest.json")" = 2 ] || fail "manifest.json must pin two 64-hex sha256 values"
  OUT=/dev/null
  finish
}

sc_s10a() {
  begin S10a "zprofile block: written once, content correct, existing content kept, idempotent"
  new_sandbox s10a; present_tools
  SKIP_PROFILE=0
  printf 'export FOO=1\n' > "$SB/home/.zprofile"
  run_install --skip-build --yes
  expect_rc 0
  cp "$SB/home/.zprofile" "$SB/profile.after1"
  run_install --skip-build --yes
  expect_rc 0
  [ "$(grep -cF '>>> trilhasbr-video-cutter install >>>' "$SB/home/.zprofile")" = 1 ] || fail "marker block is not exactly once"
  [ "$(grep -cF '<<< trilhasbr-video-cutter install <<<' "$SB/home/.zprofile")" = 1 ] || fail "end marker is not exactly once"
  grep -qxF 'export FOO=1' "$SB/home/.zprofile" || fail "existing profile content was lost"
  # shellcheck disable=SC2016 # literal profile text
  grep -qF 'eval "$(/opt/homebrew/bin/brew shellenv)"' "$SB/home/.zprofile" || fail "brew shellenv line missing"
  grep -qF "export PATH=\"$SB/gopath/bin:\$PATH\"" "$SB/home/.zprofile" || fail "go bin PATH line missing/wrong"
  cmp -s "$SB/profile.after1" "$SB/home/.zprofile" || fail "second run changed the profile"
  expect_out "already configured"
  if command -v zsh > /dev/null 2>&1; then zsh -n "$SB/home/.zprofile" || fail "profile block is not valid zsh"; fi
  finish
}
sc_s10b() {
  begin S10b "INSTALL_SKIP_PROFILE=1 leaves the profile alone"
  new_sandbox s10b; present_tools
  run_install --skip-build --yes
  expect_rc 0
  expect_no_file "$SB/home/.zprofile"
  expect_out "leaving"
  finish
}

sc_s11() {
  begin S11 "flags: unknown -> 2; --install-app with --skip-build -> 2; --help -> 0"
  new_sandbox s11; present_tools
  run_install --bogus
  expect_rc 2
  expect_out "unknown option: --bogus"
  expect_out "Usage: ./install.sh"
  run_install --install-app --skip-build
  expect_rc 2
  expect_out "--install-app needs a build"
  run_install --help
  expect_rc 0
  expect_out "Usage: ./install.sh"
  expect_no_log "^brew"
  finish
}
sc_s12a() {
  begin S12a "curl | bash style (script on stdin) is detected and explained"
  new_sandbox s12a; present_tools
  RUN_MODE=pipe
  run_install --skip-build --yes
  expect_rc 2
  expect_out "cannot be piped from curl"
  expect_out "git clone"
  expect_no_log "^brew"
  finish
}
sc_s12b() {
  begin S12b "sh install.sh (bash in POSIX mode) re-runs with bash and works"
  new_sandbox s12b; present_tools
  RUN_MODE="sh"
  run_install --skip-build --yes
  expect_rc 0
  expect_out "Done."
  RUN_MODE="sh"
  run_install --help
  expect_rc 0
  expect_out "Usage: ./install.sh"
  finish
}
sc_s13a() {
  begin S13a "project path containing spaces, invoked as ./install.sh"
  new_sandbox s13a "my project dir/trilhasbr-video-cutter"; present_tools
  RUN_MODE=rel
  run_install --skip-build --yes
  expect_rc 0
  expect_out "Done."
  expect_file "$REPO/install.log"
  finish
}
sc_s13b() {
  begin S13b "relative path from another directory, with spaces, through the Rosetta re-exec"
  new_sandbox s13b "my project dir/trilhasbr-video-cutter"; present_tools
  RUN_MODE=relother
  EXTRA_ENV=(FAKE_UNAME_M=x86_64 FAKE_ARM64=1)
  run_install --skip-build --yes
  expect_rc 0
  expect_log "^arch -arm64 /bin/bash /.*/my project dir/trilhasbr-video-cutter/install.sh --skip-build --yes$" # absolute, not ../-relative
  finish
}
sc_s14() {
  begin S14 "bash 3.2 compatibility (macOS /bin/bash): syntax of all scripts, --help, full run"
  new_sandbox s14; present_tools
  local f v
  v=$(/bin/bash --version | head -n1)
  case "$v" in *"version 3."*) ;; *) fail "/bin/bash is not 3.x here ($v); the bash-3.2 check is not meaningful" ;; esac
  for f in install.sh scripts/build-mac.sh scripts/fetch-onnxruntime.sh; do
    /bin/bash -n "$REPO/$f" 2> "$SB/syntax.err" || fail "bash -n failed for $f: $(cat "$SB/syntax.err")"
  done
  run_install --skip-build --yes
  expect_rc 0
  expect_out "Done."
  finish
}
sc_s15() {
  begin S15 "no network: clear message before any install step"
  new_sandbox s15; present_tools
  EXTRA_ENV=(FAKE_NET=0)
  run_install --skip-build --yes
  expect_rc 1
  expect_out "cannot reach https://github.com"
  expect_no_log "^brew"
  finish
}
sc_s16a() {
  begin S16a "full default flow (with build): bundle verified, order of steps"
  new_sandbox s16a; present_tools
  run_install --yes
  expect_rc 0
  expect_log "^wails build -platform darwin/arm64$"
  expect_log "^codesign --force --deep -s - build/bin/trilhasbr-video-cutter.app$"
  expect_out "built build/bin/trilhasbr-video-cutter.app (signature and bundled models verified)"
  expect_file "$REPO/build/bin/trilhasbr-video-cutter.app/Contents/Resources/resources/models/person-yolo11n.onnx"
  expect_file "$REPO/build/bin/trilhasbr-video-cutter.app/Contents/Resources/resources/onnxruntime/darwin-arm64/libonnxruntime.dylib"
  expect_log_before "^npm ci" "^wails build"
  finish
}
sc_s16b() {
  begin S16b "--run-tests builds the frontend before go vet / go test"
  new_sandbox s16b; present_tools
  run_install --skip-build --run-tests --yes
  expect_rc 0
  expect_log "^npm run build$"
  expect_log "^go vet ./...$"
  expect_log "^go test ./...$"
  expect_log_before "^npm run build$" "^go vet"
  expect_log_before "^go vet" "^go test"
  finish
}
sc_s17() {
  begin S17 "dry run on a bare machine: prints the plan, changes nothing"
  new_sandbox s17
  rm -f "$CLT_PLACEHOLDER"
  SKIP_PROFILE=0
  EXTRA_ENV=(FAKE_CLT=0 INSTALL_DRY_RUN=1)
  run_install --install-app --yes
  expect_rc 0
  expect_out "DRY RUN"
  expect_out "[dry-run]"
  expect_no_log "^sudo softwareupdate"
  expect_no_log "^softwareupdate -i"
  expect_no_log "Homebrew/install"
  expect_no_log "^brew install"
  expect_no_log "^go install"
  expect_no_log "onnxruntime-osx-arm64"
  expect_no_log "^wails build"
  expect_no_file "$SB/home/.zprofile"
  expect_no_file "$SB/prefix/bin/brew"
  expect_no_file "$CLT_PLACEHOLDER"
  finish
}
sc_s18a() {
  begin S18a "wails doctor reporting missing required dependencies fails the install"
  new_sandbox s18a; present_tools
  EXTRA_ENV=(FAKE_DOCTOR_FIXTURE=wails-doctor-missing.txt)
  run_install --skip-build --yes
  expect_rc 1
  expect_out "wails doctor reports missing required dependencies"
  finish
}
sc_s18b() {
  begin S18b "wails of the wrong version is reinstalled at the version pinned in go.mod"
  new_sandbox s18b; present_tools
  EXTRA_ENV=(FAKE_WAILS_VERSION=v2.9.0)
  run_install --skip-build --yes
  expect_rc 0
  expect_log "^go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0$"
  finish
}
sc_s19() {
  begin S19 "output is complete when install.sh exits even if tee is slow (cleanup waits for tee)"
  new_sandbox s19; present_tools
  # a tee that only starts writing after 1s: without the wait in cleanup the last lines would be missing
  rm -f "$SB/bin/tee"
  printf '#!/bin/bash\n/bin/sleep 1\nexec /usr/bin/tee "$@"\n' > "$SB/bin/tee"
  chmod +x "$SB/bin/tee"
  run_install --skip-build --yes
  expect_rc 0
  [ "$(tail -n1 "$OUT")" = "  - Log of this run:  $REPO/install.log" ] || fail "final line missing right after exit: $(tail -n1 "$OUT")"
  [ "$(tail -n1 "$REPO/install.log")" = "  - Log of this run:  $REPO/install.log" ] || fail "install.log is missing the final line"
  finish
}
sc_s12c() {
  begin S12c "zsh install.sh (not bash at all) re-runs with bash and works"
  new_sandbox s12c; present_tools
  ln -sf "$(command -v zsh)" "$SB/bin/zsh"
  RUN_MODE=zsh
  run_install --skip-build --yes
  expect_rc 0
  expect_out "Done."
  finish
}

sc_s20a() {
  begin S20a "formulae installed by Homebrew but not on PATH (unlinked): used as is, zero brew install"
  new_sandbox s20a
  ln -sf "$STUBS/stub" "$SB/prefix/bin/brew"
  ln -sf "$STUBS/stub" "$SB/gopath/bin/wails"
  local f t
  for f in go node ffmpeg; do mkdir -p "$SB/prefix/opt/$f/bin"; done
  ln -sf "$STUBS/stub" "$SB/prefix/opt/go/bin/go"
  for t in node npm; do ln -sf "$STUBS/stub" "$SB/prefix/opt/node/bin/$t"; done
  for t in ffmpeg ffprobe; do ln -sf "$STUBS/stub" "$SB/prefix/opt/ffmpeg/bin/$t"; done
  EXTRA_ENV=(FAKE_KEGS="go node ffmpeg")
  run_install --skip-build --yes
  expect_rc 0
  expect_no_log "^brew install"
  expect_no_log "^brew upgrade"
  expect_out "go is installed by Homebrew but was not found on PATH"
  expect_out "ffmpeg is installed by Homebrew but was not found on PATH"
  finish
}
sc_s20b() {
  begin S20b "formulae installed and outdated: still zero brew install / upgrade"
  new_sandbox s20b; present_tools
  EXTRA_ENV=(FAKE_KEGS="go node ffmpeg" FAKE_OUTDATED="go node ffmpeg")
  run_install --skip-build --yes
  expect_rc 0
  expect_no_log "^brew (install|upgrade|reinstall)"
  finish
}
sc_s20c() {
  begin S20c "Homebrew's Go is installed but too old: no upgrade, tells the user how"
  new_sandbox s20c; present_tools
  EXTRA_ENV=(FAKE_KEGS="go" FAKE_GO_VERSION=1.22.0)
  run_install --skip-build --yes
  expect_rc 1
  expect_no_log "^brew (install|upgrade|reinstall)"
  expect_out "install.sh never upgrades installed packages"
  expect_out "brew upgrade go"
  finish
}
sc_s20d() {
  begin S20d "every brew call runs with HOMEBREW_NO_INSTALL_UPGRADE/AUTO_UPDATE/ENV_HINTS set"
  new_sandbox s20d
  run_install --skip-build --yes
  expect_rc 0
  expect_log "^brew install go node ffmpeg$"
  expect_env "^brew install go node ffmpeg \\| .*HOMEBREW_NO_INSTALL_UPGRADE=1 HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_ENV_HINTS=1$"
  [ "$(grep -c '^brew ' "$SB/stub.log.env")" -ge 3 ] || fail "expected several brew calls (shellenv, list, install)"
  if grep '^brew ' "$SB/stub.log.env" | grep -qv 'HOMEBREW_NO_INSTALL_UPGRADE=1 HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_ENV_HINTS=1$'; then
    fail "a brew call ran without the no-upgrade environment: $(grep '^brew ' "$SB/stub.log.env" | grep -v 'HOMEBREW_NO_INSTALL_UPGRADE=1 HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_ENV_HINTS=1$' | head -n1)"
  fi
  finish
}
sc_s21a() {
  begin S21a "pinned Go toolchain for go install, wails doctor, wails build, go vet and go test"
  new_sandbox s21a
  rm -f "$SB/gopath/bin/wails" # force the wails install
  fake go node npm ffmpeg ffprobe
  ln -sf "$STUBS/stub" "$SB/prefix/bin/brew"
  EXTRA_ENV=(FAKE_GO_VERSION=1.27.3)
  run_install --run-tests --yes
  expect_rc 0
  expect_env "^go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0 \\| GOTOOLCHAIN=go1.25.6 "
  expect_env "^wails doctor \\| GOTOOLCHAIN=go1.25.6 "
  expect_env "^wails build -platform darwin/arm64 \\| GOTOOLCHAIN=go1.25.6 "
  expect_env "^go vet \\./\\.\\.\\. \\| GOTOOLCHAIN=go1.25.6 "
  expect_env "^go test \\./\\.\\.\\. \\| GOTOOLCHAIN=go1.25.6 "
  expect_no_env "^(go install|wails doctor|wails build|go vet|go test) .*GOTOOLCHAIN= "
  expect_out "Go toolchain pinned to go1.25.6"
  expect_out "proxy.golang.org"
  expect_out "go 1.27.3"
  finish
}
sc_s21b() {
  begin S21b "version probes use the local go: an old local go is detected even when GOTOOLCHAIN is already set"
  new_sandbox s21b; present_tools
  EXTRA_ENV=(FAKE_GO_VERSION=1.22.0 GOTOOLCHAIN=go1.25.6) # e.g. the user followed the README and exported the pin
  run_install --skip-build --yes
  expect_rc 1
  expect_out "Go >= 1.24.0 is required"
  expect_env "^go version \\| GOTOOLCHAIN=local "
  expect_no_env "^go version \\| GOTOOLCHAIN=go1"
  finish
}
sc_s21d() {
  begin S21d "scripts/build-mac.sh run on its own pins the Go toolchain too"
  new_sandbox s21d; present_tools
  RUN_MODE=build
  EXTRA_ENV=(FAKE_GO_VERSION=1.27.3)
  run_install
  expect_rc 0
  expect_env "^wails build -platform darwin/arm64 \\| GOTOOLCHAIN=go1.25.6 "
  expect_env "^go env GOPATH \\| GOTOOLCHAIN=local "
  finish
}
sc_s21c() {
  begin S21c "the pin is defined once (scripts/toolchain.env) and read by install.sh and build-mac.sh"
  new_sandbox s21c
  local v
  v=$(sed -n 's/^GO_TOOLCHAIN=\(go[0-9.]*\).*/\1/p' "$ROOT/scripts/toolchain.env")
  [ -n "$v" ] || fail "GO_TOOLCHAIN not defined in scripts/toolchain.env"
  [ "$(grep -c 'go1\.[0-9]*\.[0-9]' "$ROOT/install.sh" "$ROOT/scripts/build-mac.sh" | awk -F: '{n+=$2} END {print n+0}')" = 0 ] || fail "a Go version is hard-coded outside scripts/toolchain.env"
  grep -qF 'toolchain.env' "$ROOT/install.sh" || fail "install.sh does not read toolchain.env"
  grep -qF 'toolchain.env' "$ROOT/scripts/build-mac.sh" || fail "build-mac.sh does not read toolchain.env"
  OUT=/dev/null
  finish
}

SCENARIOS="s01a s01b s01c s02a s02b s03 s04a s04b s05a s05b s06a s06b s07 s08a s08b s08c s08d s08e s09a s09b s09c s09d s09e s09f s09g s10a s10b s11 s12a s12b s12c s13a s13b s14 s15 s16a s16b s17 s18a s18b s19 s20a s20b s20c s20d s21a s21b s21c s21d"

if [ "${1:-}" = --list ]; then
  for s in $SCENARIOS; do echo "$s"; done
  exit 0
fi

selected() {
  local s=$1 a
  [ "$#" -ge 1 ] || return 0
  [ "${#FILTERS[@]}" -gt 0 ] || return 0
  for a in "${FILTERS[@]}"; do
    case "$s" in "$(printf '%s' "$a" | tr '[:upper:]' '[:lower:]')"*) return 0 ;; esac
  done
  return 1
}
FILTERS=()
[ "$#" -gt 0 ] && FILTERS=("$@")

START=$SECONDS
for s in $SCENARIOS; do
  if selected "$s"; then "sc_$s"; fi
done

printf '\nstubbed install tests (real install.sh against fake tools)\n'
printf '%s\n' "$RESULTS"
printf '\n%s passed, %s failed, %ss\n' "$PASS" "$FAIL" "$((SECONDS - START))"
[ "$FAIL" = 0 ]
