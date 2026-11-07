#!/usr/bin/env bash
# Local clean-room test of install.sh: a hermetic run on THIS Mac, no VM.
#
#   tests/install/run-local.sh [--models-dir DIR] [--videos DIR] [--skip-e2e] [--keep] [--timeout SECS] [-h]
#
# The detection models are not in git: install.sh downloads them from the GitHub release. Pass --models-dir DIR
# (a folder holding the two .onnx release assets) to serve them from file:// instead, so this test does not
# depend on the release existing yet or on the network for them.
#
# What it does: copies a clean snapshot of the project (tracked + new files, none of the ignored build
# artifacts) into a temp dir, then runs install.sh there under `env -i` with a throwaway HOME, GOPATH,
# Go/npm caches and a stripped PATH. Go modules, npm packages, the Wails CLI and ONNX Runtime are therefore
# downloaded fresh. Homebrew (/opt/homebrew), the Command Line Tools and the brew formulae already on this Mac
# are reused, so their first-time installation is NOT tested here (that needs a bare machine or a VM).
# It never touches your real ~/.zprofile, /Applications or app data (it checks that at the end).
#
# Stage failures are collected (no `set -e`) so one run shows every problem.
# shellcheck disable=SC2015 # record() always succeeds, so `cond && record PASS || record FAIL` is a real if/else here
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
APP_NAME=trilhasbr-video-cutter
TIMEOUT=1800
KEEP=0
SKIP_E2E=0
VIDEOS=""
MODELS_URL=""
TS="$(date +%Y%m%d-%H%M%S)"

usage() { sed -n '2,17p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; }
while [ $# -gt 0 ]; do
  case "$1" in
    --keep) KEEP=1; shift ;;
    --skip-e2e) SKIP_E2E=1; shift ;;
    --models-dir) MODELS_DIR="${2:?--models-dir needs a folder}"; shift 2 ;;
    --videos) VIDEOS="${2:?--videos needs a folder}"; shift 2 ;;
    --timeout) TIMEOUT="${2:?--timeout needs seconds}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done
if [ -n "${MODELS_DIR:-}" ]; then
  for m in person-yolo11n.onnx osnet_x1_0_msmt17.onnx; do
    [ -f "$MODELS_DIR/$m" ] || { echo "--models-dir: $MODELS_DIR/$m not found" >&2; exit 2; }
  done
  MODELS_URL="file://$(cd "$MODELS_DIR" && pwd)"
fi
[ -z "$VIDEOS" ] || [ -d "$VIDEOS" ] || { echo "--videos: not a folder: $VIDEOS" >&2; exit 2; }
[ "$(uname -m)" = arm64 ] || { echo "run-local.sh needs an Apple Silicon Mac" >&2; exit 2; }

WORK="$(mktemp -d "${TMPDIR:-/tmp}/trilhas-install.XXXXXX")"
REPO="$WORK/repo"
ART="$ROOT/tests/install/artifacts/$TS"
RES="$WORK/results.tsv"
mkdir -p "$REPO" "$WORK/home" "$WORK/tmp" "$ART"
: >"$RES"
exec > >(tee -a "$WORK/run.log") 2>&1

echo "== run-local $TS  work=$WORK"
echo "   NOTE: stage S6 starts the app for ~12 s; a window will flash on your screen. Nothing is written to your"
echo "   real HOME, /Applications or ~/.zprofile."

FAILS=0
record() { # STATUS STAGE NAME [DETAIL]
  printf '%s\t%s\t%s\t%s\n' "$1" "$2" "$3" "${4:-}" >>"$RES"
  printf '[%s] %s: %s %s\n' "$1" "$2" "$3" "${4:-}"
  case "$1" in PASS|INFO) ;; *) FAILS=$((FAILS + 1)) ;; esac
}
check() { # STAGE NAME CMD...  (PASS when the command succeeds)
  local stage="$1" name="$2"
  shift 2
  if "$@" >>"$WORK/check.log" 2>&1; then record PASS "$stage" "$name"; else record FAIL "$stage" "$name" "(rc=$?)"; fi
}
secs_since() { echo $((SECONDS - $1)); }

kill_tree() { # PID: SIGTERM the process and all descendants
  local c
  for c in $(pgrep -P "$1" 2>/dev/null); do kill_tree "$c"; done
  kill -TERM "$1" 2>/dev/null || true
}
run_timeout() { # SECS LOGFILE CMD... -> exit code of CMD, 124 on timeout
  local secs="$1" log="$2" pid t=0
  shift 2
  "$@" >"$log" 2>&1 </dev/null &
  pid=$!
  while kill -0 "$pid" 2>/dev/null; do
    if [ "$t" -ge "$secs" ]; then
      kill_tree "$pid"
      wait "$pid" 2>/dev/null
      return 124
    fi
    sleep 2
    t=$((t + 2))
    [ $((t % 60)) -ne 0 ] || echo "   ... ${t}s"
  done
  wait "$pid"
}

STRIPPED_PATH=/usr/bin:/bin:/usr/sbin:/sbin
AFTER_PATH="$WORK/gopath/bin:/opt/homebrew/bin:$STRIPPED_PATH" # what the install leaves behind
henv() { # PATH CMD...: run CMD in the hermetic environment
  local p="$1"
  shift
  env -i HOME="$WORK/home" USER="${USER:-user}" LOGNAME="${USER:-user}" PATH="$p" TMPDIR="$WORK/tmp" \
    GOPATH="$WORK/gopath" GOCACHE="$WORK/gocache" GOMODCACHE="$WORK/gopath/pkg/mod" GOFLAGS=-modcacherw \
    npm_config_cache="$WORK/npm-cache" XDG_CONFIG_HOME="$WORK/xdg-config" XDG_CACHE_HOME="$WORK/xdg-cache" \
    HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_ENV_HINTS=1 HOMEBREW_NO_INSTALL_UPGRADE=1 INSTALL_SKIP_PROFILE=1 ${MODELS_URL:+"MODELS_BASE_URL=$MODELS_URL"} "$@"
}

# Hard guard: a test must never change the real machine. Snapshot the user's real Homebrew formulae and the Go/Node
# they get on their normal PATH (this script runs in that environment), plus the files install.sh could touch.
touch_stamp() { stat -f '%m %N' "$@" 2>/dev/null || echo "absent $*"; }
real_state() {
  echo "## brew list --formula --versions"
  HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_ENV_HINTS=1 brew list --formula --versions 2>/dev/null | sort || echo "brew unavailable"
  echo "## go version";   go version 2>&1 || echo "go unavailable"
  echo "## node --version"; node --version 2>&1 || echo "node unavailable"
  echo "## files"; touch_stamp "$HOME/.zprofile" "/Applications/$APP_NAME.app"
}
REAL_BEFORE="$(real_state)"
GUARD_DONE=0
guard_check() { # compare real state now vs before; runs once, also from the EXIT trap (failure paths)
  [ "$GUARD_DONE" = 0 ] || return 0
  GUARD_DONE=1
  local after changed
  after="$(real_state)"
  if [ "$REAL_BEFORE" = "$after" ]; then
    record PASS S8 "real machine unchanged (brew formulae+versions, go, node, ~/.zprofile, /Applications app)"
  else
    changed="$(diff <(echo "$REAL_BEFORE") <(echo "$after") | grep '^[<>]' | sed 's/^< /was: /; s/^> /now: /')"
    record FAIL S8 "REAL MACHINE WAS MODIFIED BY THE TEST"
    printf '\n!!!!!!!!!!!!!!!! TEST MUTATED THE REAL MACHINE !!!!!!!!!!!!!!!!\n%s\n!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!\n' "$changed"
  fi
}
on_exit() {
  local rc=$?
  guard_check # a failure path (early exit, Ctrl-C, crash) still gets compared
  [ "$FAILS" -eq 0 ] || [ "$rc" -ne 0 ] || rc=1
  [ -z "${WORK:-}" ] || chmod -R u+w "$WORK" 2>/dev/null || true
  exit "$rc"
}
trap on_exit EXIT
trap 'exit 130' INT TERM

# ---------------------------------------------------------------- snapshot
T=$SECONDS
(cd "$ROOT" && git ls-files -z --cached --others --exclude-standard |
  while IFS= read -r -d '' f; do [ -e "$f" ] && printf '%s\0' "$f"; done |
  tar --null -T - -cf -) | tar -xf - -C "$REPO"
record INFO S0 "snapshot copied" "$(find "$REPO" -type f | wc -l | tr -d ' ') files in $(secs_since "$T")s"

check S0 "snapshot has install.sh" test -x "$REPO/install.sh"
check S0 "snapshot has the manifest + release.env but NOT the models (they are downloaded)" test -f "$REPO/resources/models/manifest.json" -a -f "$REPO/scripts/release.env" -a ! -e "$REPO/resources/models/person-yolo11n.onnx" -a ! -e "$REPO/resources/models/osnet_x1_0_msmt17.onnx"
for ignored in node_modules build/bin frontend/dist frontend/node_modules resources/onnxruntime/darwin-arm64/libonnxruntime.dylib install.log; do
  check S0 "snapshot has no $ignored" test ! -e "$REPO/$ignored"
done
for tool in go node npm ffmpeg ffprobe wails; do
  if henv "$STRIPPED_PATH" /bin/sh -c "command -v $tool" >/dev/null 2>&1; then
    record FAIL S0 "$tool not on the stripped PATH" "found"
  else record PASS S0 "$tool not on the stripped PATH"; fi
done
[ -x /opt/homebrew/bin/brew ] && record INFO S0 "Homebrew present (reused, not tested)" || record INFO S0 "Homebrew absent (install.sh will install it)"
xcode-select -p >/dev/null 2>&1 && record INFO S0 "Command Line Tools present (reused, not tested)" || record INFO S0 "Command Line Tools absent"

# ---------------------------------------------------------------- S1 CLI behaviour
cd "$REPO" || exit 1
henv "$STRIPPED_PATH" /bin/bash ./install.sh --help >"$WORK/s1-help.log" 2>&1; rc=$?
[ "$rc" -eq 0 ] && grep -q -- "--install-app" "$WORK/s1-help.log" && record PASS S1 "--help exits 0 and lists options" || record FAIL S1 "--help exits 0 and lists options" "rc=$rc"
henv "$STRIPPED_PATH" /bin/bash ./install.sh --bogus >"$WORK/s1-bogus.log" 2>&1; rc=$?
[ "$rc" -eq 2 ] && record PASS S1 "unknown flag exits 2" || record FAIL S1 "unknown flag exits 2" "rc=$rc"
tree_sum() { find "$REPO" -type f ! -name install.log | sort | shasum | awk '{print $1}'; }
before="$(tree_sum)"
henv "$STRIPPED_PATH" env INSTALL_DRY_RUN=1 /bin/bash ./install.sh --yes --run-tests >"$WORK/s1-dry.log" 2>&1 </dev/null; rc=$?
after="$(tree_sum)"
[ "$rc" -eq 0 ] && record PASS S1 "dry run exits 0" || record FAIL S1 "dry run exits 0" "rc=$rc"
grep -q "\[dry-run\]" "$WORK/s1-dry.log" && record PASS S1 "dry run only prints actions" || record FAIL S1 "dry run only prints actions"
[ "$before" = "$after" ] && record PASS S1 "dry run left the tree unchanged" || record FAIL S1 "dry run left the tree unchanged"
[ ! -d "$WORK/gopath/bin" ] && record PASS S1 "dry run installed nothing (no wails)" || record FAIL S1 "dry run installed nothing (no wails)"

# ---------------------------------------------------------------- S2 real install
echo "== S2: ./install.sh --yes --run-tests (downloads Go modules, npm packages, wails, ONNX Runtime; builds the app)"
T=$SECONDS
run_timeout "$TIMEOUT" "$WORK/s2.log" henv "$STRIPPED_PATH" /bin/bash ./install.sh --yes --run-tests; rc=$?
S2_SECS=$(secs_since "$T")
if [ "$rc" -eq 0 ]; then record PASS S2 "install.sh --yes --run-tests exits 0" "${S2_SECS}s"
elif [ "$rc" -eq 124 ]; then record FAIL S2 "install.sh --yes --run-tests" "timed out after ${TIMEOUT}s"
else record FAIL S2 "install.sh --yes --run-tests" "rc=$rc after ${S2_SECS}s"; tail -n 25 "$WORK/s2.log"; fi
grep -q "downloading onnxruntime" "$WORK/s2.log" && record PASS S2 "ONNX Runtime downloaded fresh" || record FAIL S2 "ONNX Runtime downloaded fresh"
[ "$(grep -c "^installed .*resources/models/.*\.onnx" "$WORK/s2.log")" = 2 ] && record PASS S2 "both models downloaded and verified" || record FAIL S2 "both models downloaded and verified"
grep -q "ready for Wails development\|wails doctor: ready" "$WORK/s2.log" && record PASS S2 "wails doctor ready" || record FAIL S2 "wails doctor ready"
grep -q "Done\." "$WORK/s2.log" && record PASS S2 "installer printed its summary" || record FAIL S2 "installer printed its summary"

# ---------------------------------------------------------------- S3 artifacts
APP="$REPO/build/bin/$APP_NAME.app"
RESDIR="$APP/Contents/Resources/resources"
check S3 "app bundle exists" test -d "$APP"
check S3 "codesign --verify --deep --strict" codesign --verify --deep --strict "$APP"
check S3 "bundle has both models + manifest" test -f "$RESDIR/models/person-yolo11n.onnx" -a -f "$RESDIR/models/osnet_x1_0_msmt17.onnx" -a -f "$RESDIR/models/manifest.json"
check S3 "bundle has libonnxruntime.dylib" test -f "$RESDIR/onnxruntime/darwin-arm64/libonnxruntime.dylib"
DYLIB="$RESDIR/onnxruntime/darwin-arm64/libonnxruntime.dylib"
dylib_kind="$(file -b "$DYLIB" 2>&1)" # -b: the path itself contains "arm64"
case "$dylib_kind" in *arm64*) record PASS S3 "dylib is arm64" ;; *) record FAIL S3 "dylib is arm64" "$dylib_kind" ;; esac
manifest_sha() { sed -n "/\"$1\"/,/sha256/s/.*\"sha256\": *\"\([0-9a-f]*\)\".*/\1/p" "$REPO/resources/models/manifest.json"; }
for m in person-yolo11n.onnx osnet_x1_0_msmt17.onnx; do
  want="$(manifest_sha "$m")"
  got="$(shasum -a 256 "$RESDIR/models/$m" 2>/dev/null | awk '{print $1}')"
  [ -n "$want" ] && [ "$want" = "$got" ] && record PASS S3 "bundled $m matches manifest sha256" || record FAIL S3 "bundled $m matches manifest sha256"
done
check S3 "wails installed in the fresh GOPATH" test -x "$WORK/gopath/bin/wails"
want_wails="$(awk '$1=="github.com/wailsapp/wails/v2" {print $2; exit}' "$REPO/go.mod")"
got_wails="$(henv "$AFTER_PATH" wails version 2>&1)" # capture first: `| grep -q` under pipefail can SIGPIPE the producer
case "$got_wails" in *"$want_wails"*) record PASS S3 "wails is $want_wails (go.mod)" ;; *) record FAIL S3 "wails is $want_wails (go.mod)" ;; esac
[ -n "$(ls -A "$WORK/gopath/pkg/mod" 2>/dev/null)" ] && record PASS S3 "Go modules downloaded fresh" || record FAIL S3 "Go modules downloaded fresh"
[ -n "$(ls -A "$WORK/npm-cache" 2>/dev/null)" ] && record PASS S3 "npm packages downloaded fresh" || record FAIL S3 "npm packages downloaded fresh"
check S3 "frontend/node_modules installed" test -d "$REPO/frontend/node_modules"
check S3 "frontend/dist built" test -f "$REPO/frontend/dist/index.html"
want_go="$(sed -n 's/^GO_TOOLCHAIN=//p' "$REPO/scripts/toolchain.env")"
got_go="$(henv "$AFTER_PATH" go version -m "$APP/Contents/MacOS/$APP_NAME" 2>/dev/null | sed -n '1s/.*: //p')"
[ -n "$want_go" ] && [ "$got_go" = "$want_go" ] && record PASS S3 "app binary compiled with the pinned $want_go" || record FAIL S3 "app binary compiled with the pinned $want_go" "got '$got_go'"
if grep -q -E "installing with Homebrew:|\[dry-run\] brew install|^==> Pouring" "$WORK/s2.log" "$REPO/install.log" 2>/dev/null; then
  record FAIL S3 "no brew install ran (everything is already installed here)" "$(grep -h -E "installing with Homebrew:|brew install" "$WORK/s2.log" | head -1)"
else record PASS S3 "no brew install ran (everything is already installed here)"; fi
grep -q -E "^(--- FAIL|FAIL[[:space:]])" "$WORK/s2.log" && record FAIL S3 "go test output has no failures" || record PASS S3 "go test output has no failures"

# ---------------------------------------------------------------- S4 idempotency
echo "== S4: second run (--yes --skip-build)"
T=$SECONDS
run_timeout 300 "$WORK/s4.log" henv "$STRIPPED_PATH" /bin/bash ./install.sh --yes --skip-build; rc=$?
S4_SECS=$(secs_since "$T")
[ "$rc" -eq 0 ] && record PASS S4 "second run exits 0" "${S4_SECS}s" || { record FAIL S4 "second run exits 0" "rc=$rc"; tail -n 15 "$WORK/s4.log"; }
[ "$S4_SECS" -lt 120 ] && record PASS S4 "second run under 120 s" "${S4_SECS}s" || record FAIL S4 "second run under 120 s" "${S4_SECS}s"
grep -q "already installed" "$WORK/s4.log" && record PASS S4 "second run reports tools already installed" || record FAIL S4 "second run reports tools already installed"
grep -q "already present" "$WORK/s4.log" && ! grep -q "downloading onnxruntime" "$WORK/s4.log" && record PASS S4 "ONNX Runtime not re-downloaded" || record FAIL S4 "ONNX Runtime not re-downloaded"

# ---------------------------------------------------------------- S5 native runtime
echo "== S5: native runtime (ONNX Runtime + models) loads and runs"
run_timeout "$TIMEOUT" "$WORK/s5.log" henv "$AFTER_PATH" go test -count=1 ./internal/inference -run TestRuntimeLoadsAndRuns -v; rc=$?
if [ "$rc" -eq 0 ] && grep -q -- "--- PASS: TestRuntimeLoadsAndRuns" "$WORK/s5.log" && ! grep -q "SKIP" "$WORK/s5.log"; then
  record PASS S5 "TestRuntimeLoadsAndRuns passes (not skipped)"
else record FAIL S5 "TestRuntimeLoadsAndRuns passes (not skipped)" "rc=$rc"; tail -n 15 "$WORK/s5.log"; fi

# ---------------------------------------------------------------- S6 launch the built app
echo "== S6: launching the built app for 12 s (isolated HOME)"
BIN="$APP/Contents/MacOS/$APP_NAME"
DATA="$WORK/home/Library/Application Support/$APP_NAME"
if [ -x "$BIN" ]; then
  henv "$STRIPPED_PATH" "$BIN" >"$WORK/s6.log" 2>&1 </dev/null &
  APP_PID=$!
  sleep 12
  if kill -0 "$APP_PID" 2>/dev/null; then record PASS S6 "app still running after 12 s"; else
    wait "$APP_PID" 2>/dev/null; record FAIL S6 "app still running after 12 s" "exited rc=$?"; tail -n 15 "$WORK/s6.log"
  fi
  [ -f "$DATA/video-cutter.sqlite" ] && record PASS S6 "app created its database (migrations ran)" || record FAIL S6 "app created its database (migrations ran)" "missing $DATA/video-cutter.sqlite"
  kill_tree "$APP_PID"
  for _ in 1 2 3 4 5 6 7 8 9 10; do kill -0 "$APP_PID" 2>/dev/null || break; sleep 1; done
  if kill -0 "$APP_PID" 2>/dev/null; then kill -KILL "$APP_PID" 2>/dev/null; record FAIL S6 "app exits on SIGTERM" "needed SIGKILL"; else record PASS S6 "app exits on SIGTERM"; fi
  wait "$APP_PID" 2>/dev/null
  pgrep -f "$REPO/build/bin/$APP_NAME.app" >/dev/null 2>&1 && record FAIL S6 "no leftover app process" || record PASS S6 "no leftover app process"
else record FAIL S6 "app binary exists" "$BIN"; fi

# ---------------------------------------------------------------- S7 full pipeline on real videos
if [ "$SKIP_E2E" = 1 ] || [ -z "$VIDEOS" ]; then
  record INFO S7 "full pipeline skipped" "$([ "$SKIP_E2E" = 1 ] && echo --skip-e2e || echo 'no --videos DIR')"
else
  echo "== S7: detect -> list -> cut on real videos (cloned copy, your folder is not modified)"
  mkdir -p "$WORK/videos"
  find "$VIDEOS" -maxdepth 1 -iname '*.mp4' -exec cp -c {} "$WORK/videos/" \;
  nvid="$(find "$WORK/videos" -iname '*.mp4' | wc -l | tr -d ' ')"
  if [ "$nvid" -eq 0 ]; then record FAIL S7 "videos found in --videos" "none"; else
    T=$SECONDS
    run_timeout "$TIMEOUT" "$WORK/s7.log" henv "$AFTER_PATH" env DETECT_E2E_DIR="$WORK/videos" \
      go test -count=1 ./internal/app/actions -run E2E -v -timeout 20m; rc=$?
    if [ "$rc" -eq 0 ] && grep -q -- "--- PASS: TestE2EDetectRealVideos" "$WORK/s7.log"; then
      record PASS S7 "e2e on $nvid videos (index, detect, rerun skips, list, cut)" "$(secs_since "$T")s"
      grep -E "DETECT fps|RERUN|PEOPLE groups" "$WORK/s7.log" | sed 's/^ *//' | cut -c1-200
    else record FAIL S7 "e2e on $nvid videos" "rc=$rc"; tail -n 20 "$WORK/s7.log"; fi
  fi
fi

# ---------------------------------------------------------------- S8 real machine untouched
guard_check

# ---------------------------------------------------------------- summary + artifacts
echo
echo "================ RESULTS ================"
awk -F'\t' '{printf "%-5s %-3s %s %s\n", $1, $2, $3, $4}' "$RES"
echo "========================================="
[ "$FAILS" -eq 0 ] && echo "ALL STAGES PASSED (install ${S2_SECS:-?}s, rerun ${S4_SECS:-?}s)" || echo "$FAILS FAILURE(S)"
cp "$RES" "$ART/results.tsv"
for f in run.log s1-dry.log s2.log s4.log s5.log s6.log s7.log; do [ -f "$WORK/$f" ] && cp "$WORK/$f" "$ART/"; done
[ -f "$REPO/install.log" ] && cp "$REPO/install.log" "$ART/install.log"
echo "artifacts: $ART"

if [ "$FAILS" -ne 0 ] || [ "$KEEP" = 1 ]; then
  echo "work dir kept: $WORK"
else
  cd "$ROOT" && chmod -R u+w "$WORK" 2>/dev/null; rm -rf "$WORK"
fi
[ "$FAILS" -eq 0 ]
