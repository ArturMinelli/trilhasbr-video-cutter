#!/usr/bin/env bash
# TrilhasBR Video Cutter - one-shot installer for Apple Silicon Macs.
#
# Takes a machine with nothing installed (no Xcode CLT, Homebrew, Go, Node, ffmpeg) to a built
# macOS app: build/bin/trilhasbr-video-cutter.app. Safe to re-run; installed tools are detected and skipped.
#
#   ./install.sh                 install dependencies and build the app
#   ./install.sh --install-app   ... and copy the app into /Applications
#   ./install.sh --run-tests     ... and run go vet, go test and the frontend build
#   ./install.sh --skip-build    dependencies only
#   INSTALL_DRY_RUN=1 ./install.sh   print what would be done, change nothing
#   INSTALL_SKIP_PROFILE=1           do not edit ~/.zprofile
#   INSTALL_BREW=<path>              Homebrew binary to use (default /opt/homebrew/bin/brew; a test hook)
#
# Compatible with the bash 3.2 that ships with macOS.

# `sh install.sh` runs bash in POSIX mode, which cannot parse this script: re-run it with plain bash.
if [ -z "${BASH_VERSION:-}" ] || shopt -qo posix 2>/dev/null; then
  if [ -f "$0" ]; then exec /bin/bash "$0" "$@"; fi
  echo "install.sh must be run with bash from a checkout:  git clone <repo> && cd <repo> && ./install.sh" >&2
  exit 2
fi
set -Eeuo pipefail

SCRIPT="${BASH_SOURCE[0]:-}"
if [ -z "$SCRIPT" ] || [ ! -f "$SCRIPT" ]; then
  echo "install.sh cannot be piped from curl; it needs the whole repository. Run:  git clone <repo> && cd <repo> && ./install.sh" >&2
  exit 2
fi
ROOT="$(cd "$(dirname "$SCRIPT")" && pwd)"
cd "$ROOT"
SCRIPT="$ROOT/$(basename "$SCRIPT")" # absolute: the Rosetta re-exec below runs after the cd
# shellcheck source=scripts/toolchain.env
. "$ROOT/scripts/toolchain.env" # GO_TOOLCHAIN, the Go version used for building

APP_NAME=trilhasbr-video-cutter
APP_BUNDLE="build/bin/$APP_NAME.app"
APP_INSTALLED="/Applications/$APP_NAME.app"
LOG="$ROOT/install.log"
MIN_NODE_MAJOR=18
CLT_PLACEHOLDER=/tmp/.com.apple.dt.CommandLineTools.installondemand.in-progress-file
PROFILE_MARK="# >>> $APP_NAME install >>>"
PROFILE_END="# <<< $APP_NAME install <<<"

INSTALL_APP=0
RUN_TESTS=0
SKIP_BUILD=0
YES=0
DRY="${INSTALL_DRY_RUN:-0}"

usage() {
  cat <<'USAGE'
Usage: ./install.sh [options]

Installs everything needed to build and run TrilhasBR Video Cutter on an Apple Silicon Mac
(Xcode Command Line Tools, Homebrew, Go, Node, ffmpeg, the Wails CLI, ONNX Runtime) and builds the app.

Options:
  --install-app   copy the built app into /Applications (replaces an existing copy; quits it gracefully first)
  --run-tests     also run go vet, go test ./... and the frontend build
  --skip-build    install dependencies only; do not build the app
  --yes           never prompt (fail instead of asking for a sudo password)
  -h, --help      show this help

Environment:
  INSTALL_DRY_RUN=1      print what each step would do without changing anything
  INSTALL_SKIP_PROFILE=1 do not add Homebrew/Go to PATH in ~/.zprofile

Everything is also logged to install.log in the repository root.
USAGE
}

for arg in "$@"; do
  case "$arg" in
    --install-app) INSTALL_APP=1 ;;
    --run-tests) RUN_TESTS=1 ;;
    --skip-build) SKIP_BUILD=1 ;;
    --yes) YES=1 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $arg" >&2; usage >&2; exit 2 ;;
  esac
done
if [ "$SKIP_BUILD" = 1 ] && [ "$INSTALL_APP" = 1 ]; then
  echo "--install-app needs a build; it cannot be combined with --skip-build" >&2
  exit 2
fi

# Apple Silicon only. A Rosetta (x86_64) shell on an M-series Mac still reports hw.optional.arm64=1, so re-run natively.
if [ "$(uname -s)" = Darwin ] && [ "$(uname -m)" != arm64 ] && [ -z "${INSTALL_REEXEC:-}" ] \
  && [ "$(sysctl -n hw.optional.arm64 2>/dev/null || echo 0)" = 1 ]; then
  echo "x86_64 (Rosetta) shell detected on Apple Silicon; re-running natively"
  INSTALL_REEXEC=1 exec arch -arm64 /bin/bash "$SCRIPT" "$@"
fi

# Colors only on a terminal; then mirror everything into install.log.
if [ -t 1 ]; then
  BOLD=$'\033[1m'; RED=$'\033[31m'; GREEN=$'\033[32m'; YELLOW=$'\033[33m'; RESET=$'\033[0m'
else
  BOLD=''; RED=''; GREEN=''; YELLOW=''; RESET=''
fi
INTERACTIVE=0
if [ -t 0 ] && [ "$YES" = 0 ]; then INTERACTIVE=1; fi
# A fifo (not >(...)) lets cleanup wait for tee, so the last lines appear before the shell prompt returns.
TEE_PID=""
TEE_DIR="$(mktemp -d)"
mkfifo "$TEE_DIR/log"
tee -a "$LOG" < "$TEE_DIR/log" &
TEE_PID=$!
exec 3>&1 4>&2
exec > "$TEE_DIR/log" 2>&1
rm -rf "$TEE_DIR"

log() { printf '%s\n' "$*"; }
ok() { printf '%s%s%s\n' "$GREEN" "$*" "$RESET"; }
warn() { printf '%sWARNING: %s%s\n' "$YELLOW" "$*" "$RESET" >&2; }
die() { printf '%sERROR: %s%s\n' "$RED" "$*" "$RESET" >&2; exit 1; }
dry() { [ "$DRY" = 1 ]; }
# run CMD...: execute, or only print under INSTALL_DRY_RUN=1
run() {
  if dry; then log "[dry-run] $*"; else "$@"; fi
}

STEP_NAME=""
STEP_START=0
TIMINGS=()
end_step() {
  if [ -n "$STEP_NAME" ]; then
    TIMINGS+=("$(printf '%5ss  %s' $((SECONDS - STEP_START)) "$STEP_NAME")")
  fi
  STEP_NAME=""
}
step() {
  end_step
  STEP_NAME="$1"
  STEP_START=$SECONDS
  printf '\n%s==> %s%s\n' "$BOLD" "$1" "$RESET"
}

SUDO_KEEPALIVE_PID=""
cleanup() {
  local i
  [ -n "$SUDO_KEEPALIVE_PID" ] && kill "$SUDO_KEEPALIVE_PID" 2>/dev/null || true
  rm -f "$CLT_PLACEHOLDER" 2>/dev/null || true
  if [ -n "$TEE_PID" ]; then
    exec 1>&3 2>&4 # close our end of the fifo so tee sees EOF and flushes
    for ((i = 0; i < 50; i++)); do kill -0 "$TEE_PID" 2>/dev/null || break; sleep 0.1; done
  fi
}
on_err() {
  local code=$? line=$1
  trap - ERR
  printf '\n%sFAILED%s during step "%s" (install.sh line %s, exit %s)\n' "$RED" "$RESET" "${STEP_NAME:-startup}" "$line" "$code" >&2
  printf 'Full log: %s\nFix the problem and re-run ./install.sh (it resumes where it can).\n' "$LOG" >&2
  exit "$code"
}
trap cleanup EXIT
trap 'on_err $LINENO' ERR

# Retry a flaky network command: retry NAME CMD...
retry() {
  local name=$1 attempt
  shift
  for attempt in 1 2 3; do
    if "$@"; then return 0; fi
    warn "$name failed (attempt $attempt/3)"
    sleep $((attempt * 5))
  done
  return 1
}

# ver_ge A B: true when version A >= B (dotted numbers)
ver_ge() { [ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -n1)" = "$2" ]; }

log "$APP_NAME installer  (log: $LOG)"
dry && log "DRY RUN: nothing will be changed."

# ---------------------------------------------------------------- preflight
step "Preflight checks"
[ "$(id -u)" != 0 ] || die "do not run install.sh as root or with sudo; it asks for sudo itself when needed."
[ "$(uname -s)" = Darwin ] || die "macOS is required (found $(uname -s))."
[ "$(uname -m)" = arm64 ] || die "an Apple Silicon (arm64) Mac is required; this machine reports $(uname -m)."
log "macOS $(sw_vers -productVersion) on $(uname -m)"

free_kb=$(df -k "$ROOT" | awk 'NR==2 {print $4}')
if [ "${free_kb:-0}" -lt 10485760 ]; then
  warn "less than 10 GB free disk space ($((free_kb / 1048576)) GB); the toolchain and build need several GB."
fi
if ! curl -fsSI --max-time 20 https://github.com >/dev/null 2>&1; then
  die "cannot reach https://github.com; check your network or proxy settings and re-run."
fi
[ -f go.mod ] && [ -f frontend/package.json ] || die "run install.sh from a complete checkout of the repository."

need_clt=0; xcode-select -p >/dev/null 2>&1 || need_clt=1
BREW="${INSTALL_BREW:-/opt/homebrew/bin/brew}"
export INSTALL_BREW="$BREW" # scripts/build-mac.sh uses the same Homebrew
# Never upgrade, reinstall or auto-update anything that is already installed (set before the first brew call).
export HOMEBREW_NO_INSTALL_UPGRADE=1 HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_ENV_HINTS=1 HOMEBREW_NO_INSTALL_CLEANUP=1
need_brew=0; [ -x "$BREW" ] || need_brew=1
if [ "$need_clt" = 1 ] || [ "$need_brew" = 1 ]; then
  # Fail early (before any long step) if we cannot get administrator rights.
  if sudo -n true 2>/dev/null; then
    log "sudo: passwordless"
  elif [ "$INTERACTIVE" = 1 ]; then
    log "Administrator rights are needed to install the Command Line Tools / Homebrew."
    dry || sudo -v || die "sudo authentication failed."
    if ! dry; then
      ( while true; do sudo -n true; sleep 50; kill -0 "$$" 2>/dev/null || exit; done ) >/dev/null 2>&1 3>&- 4>&- &
      SUDO_KEEPALIVE_PID=$!
    fi
  else
    die "administrator rights are needed (Command Line Tools / Homebrew) but sudo needs a password and this session is non-interactive. Run ./install.sh in a Terminal, or configure passwordless sudo for this user."
  fi
fi

# ------------------------------------------------- Xcode Command Line Tools
step "Xcode Command Line Tools"
install_clt() {
  touch "$CLT_PLACEHOLDER"
  local label="" attempt
  for attempt in 1 2 3; do
    label=$(softwareupdate -l 2>&1 | sed -n 's/^[[:space:]]*\* Label: \(Command Line Tools.*\)$/\1/p' | sort -V | tail -n1 || true)
    [ -n "$label" ] && break
    sleep 5
  done
  if [ -z "$label" ]; then
    rm -f "$CLT_PLACEHOLDER"
    xcode-select --install >/dev/null 2>&1 || true
    die "could not find the Command Line Tools in Software Update. A system dialog may have opened: finish that installation, then re-run ./install.sh."
  fi
  log "installing: $label"
  sudo softwareupdate -i "$label" --verbose
  rm -f "$CLT_PLACEHOLDER"
  sudo xcode-select --switch /Library/Developer/CommandLineTools >/dev/null 2>&1 || true
}
if [ "$need_clt" = 0 ]; then
  log "already installed: $(xcode-select -p)"
else
  run install_clt
  if ! dry; then
    xcode-select -p >/dev/null 2>&1 || die "Command Line Tools installation did not complete; re-run ./install.sh."
    clang --version | head -n1
  fi
fi

# ------------------------------------------------------------------ Homebrew
step "Homebrew"
install_brew() {
  local tmp
  tmp=$(mktemp)
  retry "downloading the Homebrew installer" curl -fsSL -o "$tmp" https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh
  [ -s "$tmp" ] || die "the Homebrew installer download was empty."
  NONINTERACTIVE=1 /bin/bash "$tmp"
  rm -f "$tmp"
}
if [ "$need_brew" = 0 ]; then
  log "already installed: $BREW"
else
  run install_brew
fi
if [ -x "$BREW" ]; then eval "$("$BREW" shellenv)"; fi
if ! dry; then
  command -v brew >/dev/null 2>&1 || die "Homebrew is not available after installation."
  brew --version | head -n1
fi

# ------------------------------------------------- Go, Node and ffmpeg
step "Go, Node.js and ffmpeg"
GO_MIN=$(awk '/^go /{print $2; exit}' go.mod)
# Version checks use the LOCAL go (GOTOOLCHAIN=local): the pinned toolchain below is only for building.
local_go_version() { GOTOOLCHAIN=local go version 2>/dev/null | awk '{print $3}' | sed 's/^go//'; }
go_ok() { command -v go >/dev/null 2>&1 && ver_ge "$(local_go_version)" "$GO_MIN"; }
node_ok() { command -v node >/dev/null 2>&1 && [ "$(node -p 'process.versions.node.split(".")[0]')" -ge "$MIN_NODE_MAJOR" ] && command -v npm >/dev/null 2>&1; }
ffmpeg_ok() { command -v ffmpeg >/dev/null 2>&1 && command -v ffprobe >/dev/null 2>&1; }
# have_keg NAME: Homebrew already has this formula installed (even if outdated or not linked onto PATH)
have_keg() { [ -n "$(brew list --formula --versions "$1" 2>/dev/null || true)" ]; }

formulae=()
for f in go node ffmpeg; do
  "${f}_ok" && continue
  if have_keg "$f"; then
    # Installed by Homebrew but not usable from PATH: use the keg. Never reinstall or upgrade it.
    log "$f is installed by Homebrew but was not found on PATH; using $(brew --prefix "$f")/bin"
    PATH="$(brew --prefix "$f")/bin:$PATH"
    hash -r
  else
    formulae+=("$f")
  fi
done
if [ "${#formulae[@]}" -gt 0 ]; then
  log "installing with Homebrew: ${formulae[*]}"
  run brew install "${formulae[@]}"
  hash -r
else
  log "go, node and ffmpeg are already installed"
fi
if ! dry; then
  go_ok || die "Go >= $GO_MIN is required (found: $(GOTOOLCHAIN=local go version 2>/dev/null || echo none)). install.sh never upgrades installed packages; if Homebrew installed Go, run: brew upgrade go"
  node_ok || die "Node.js >= $MIN_NODE_MAJOR with npm is required (found: $(node -v 2>/dev/null || echo none)). install.sh never upgrades installed packages; if Homebrew installed Node, run: brew upgrade node"
  ffmpeg_ok || die "ffmpeg and ffprobe are required but were not found on PATH."
  log "go $(local_go_version)"
  log "node $(node -v), npm $(npm -v)"
  log "$(ffmpeg -version | head -n1)"
fi
# Build with a pinned Go toolchain: Homebrew's newest Go breaks `wails build` (see scripts/toolchain.env).
export GOTOOLCHAIN="$GO_TOOLCHAIN"
log "Go toolchain pinned to $GO_TOOLCHAIN for installing wails, wails doctor, the build and the tests: Homebrew's newest Go breaks wails $(awk '$1=="github.com/wailsapp/wails/v2" {print $2; exit}' go.mod). Go downloads it on first use (needs network access to proxy.golang.org)."

# ----------------------------------------------------------------- Wails CLI
step "Wails CLI"
WAILS_VERSION=$(awk '$1=="github.com/wailsapp/wails/v2" {print $2; exit}' go.mod)
[ -n "$WAILS_VERSION" ] || die "could not read the Wails version from go.mod."
if ! dry || command -v go >/dev/null 2>&1; then
  GOBIN_DIR="$(GOTOOLCHAIN=local go env GOPATH)/bin"
  export PATH="$GOBIN_DIR:$PATH"
fi
wails_ver() { wails version 2>&1 | grep -Eo 'v[0-9]+\.[0-9]+\.[0-9]+' | head -n1 || true; }
if command -v wails >/dev/null 2>&1 && [ "$(wails_ver)" = "$WAILS_VERSION" ]; then
  log "wails $WAILS_VERSION already installed"
else
  log "installing wails $WAILS_VERSION"
  run bash -c "cd \"\$HOME\" && go install github.com/wailsapp/wails/v2/cmd/wails@$WAILS_VERSION"
fi
if ! dry; then
  command -v wails >/dev/null 2>&1 || die "wails is not on PATH after installation (expected in $GOBIN_DIR)."
  doctor=$(wails doctor 2>&1 || true)
  if printf '%s' "$doctor" | grep -q "Required package(s) installation details"; then
    printf '%s\n' "$doctor"
    die "wails doctor reports missing required dependencies (see above)."
  fi
  if printf '%s' "$doctor" | grep -q "ready for Wails development"; then
    ok "wails doctor: ready ($(wails_ver))"
  else
    warn "wails doctor gave no clear verdict; continuing (the build will fail loudly if something is missing)."
  fi
fi

# ---------------------------------------------------- shell profile (PATH)
step "Shell profile"
PROFILE="$HOME/.zprofile"
if [ "${INSTALL_SKIP_PROFILE:-0}" = 1 ]; then
  log "INSTALL_SKIP_PROFILE=1: leaving $PROFILE untouched"
elif grep -qF "$PROFILE_MARK" "$PROFILE" 2>/dev/null; then
  log "$PROFILE already configured"
else
  gobin="${GOBIN_DIR:-$HOME/go/bin}"
  log "adding Homebrew and Go to PATH in $PROFILE"
  if ! dry; then
    # shellcheck disable=SC2016 # the profile lines must stay literal ($PATH expands when the profile runs)
    {
      printf '\n%s\n' "$PROFILE_MARK"
      printf 'eval "$(/opt/homebrew/bin/brew shellenv)"\n'
      printf 'export PATH="%s:$PATH"\n' "$gobin"
      printf '%s\n' "$PROFILE_END"
    } >> "$PROFILE"
  fi
fi

# ------------------------------------------------------ ONNX Runtime + models
step "ONNX Runtime and detection models"
run "$ROOT/scripts/fetch-onnxruntime.sh"
run "$ROOT/scripts/fetch-models.sh" # downloads from the GitHub release, verified against resources/models/manifest.json
manifest_sha() { sed -n "/\"$1\"/,/sha256/s/.*\"sha256\": *\"\([0-9a-f]*\)\".*/\1/p" resources/models/manifest.json; }
if ! dry; then
  for model in person-yolo11n.onnx osnet_x1_0_msmt17.onnx; do
    want=$(manifest_sha "$model")
    [ -n "$want" ] || die "no sha256 for $model in resources/models/manifest.json."
    [ -f "resources/models/$model" ] || die "resources/models/$model is missing; run scripts/fetch-models.sh (it downloads the models from the GitHub release)."
    got=$(shasum -a 256 "resources/models/$model" | awk '{print $1}')
    [ "$got" = "$want" ] || die "resources/models/$model is corrupt (sha256 $got, expected $want). Delete it and run scripts/fetch-models.sh."
    log "model ok: $model"
  done
fi

# --------------------------------------------------------------- frontend deps
step "Frontend dependencies (npm)"
if [ -f frontend/package-lock.json ]; then
  run bash -c "cd frontend && npm ci --no-audit --no-fund" || die "npm ci failed; see $LOG."
else
  run bash -c "cd frontend && npm install --no-audit --no-fund" || die "npm install failed; see $LOG."
fi

# ------------------------------------------------------------------- tests
if [ "$RUN_TESTS" = 1 ]; then
  step "Tests (frontend build, go vet, go test)"
  # The Go app embeds frontend/dist, so the frontend must be built before go vet compiles it.
  run bash -c "cd frontend && npm run build"
  run go vet ./...
  run go test ./...
fi

# ------------------------------------------------------------------- build
if [ "$SKIP_BUILD" = 0 ]; then
  step "Building the app (wails build)"
  run "$ROOT/scripts/build-mac.sh"
  if ! dry; then
    [ -d "$APP_BUNDLE" ] || die "build finished but $APP_BUNDLE is missing."
    codesign --verify --deep --strict "$APP_BUNDLE" || die "the built app has an invalid signature."
    for f in models/person-yolo11n.onnx models/osnet_x1_0_msmt17.onnx models/manifest.json onnxruntime/darwin-arm64/libonnxruntime.dylib; do
      [ -f "$APP_BUNDLE/Contents/Resources/resources/$f" ] || die "the built app is missing bundled resource: $f"
    done
    ok "built $APP_BUNDLE (signature and bundled models verified)"
  fi
fi

# ------------------------------------------------------------ install the app
if [ "$INSTALL_APP" = 1 ]; then
  step "Installing into /Applications"
  install_app() {
    if pgrep -f "$APP_INSTALLED/Contents/MacOS/$APP_NAME" >/dev/null 2>&1; then
      log "quitting the running app"
      osascript -e "tell application \"$APP_NAME\" to quit" >/dev/null 2>&1 || true
      local i
      for i in 1 2 3 4 5 6 7 8 9 10; do
        pgrep -f "$APP_INSTALLED/Contents/MacOS/$APP_NAME" >/dev/null 2>&1 || break
        sleep 1
      done
      if pgrep -f "$APP_INSTALLED/Contents/MacOS/$APP_NAME" >/dev/null 2>&1; then
        die "$APP_NAME is still running; quit it and re-run with --install-app."
      fi
    fi
    rm -rf "$APP_INSTALLED"
    ditto "$APP_BUNDLE" "$APP_INSTALLED" || die "could not write to /Applications (needs an administrator account)."
    codesign --verify --deep --strict "$APP_INSTALLED" || die "installed app has an invalid signature."
  }
  run install_app
  dry || ok "installed $APP_INSTALLED"
fi

# ----------------------------------------------------------------- summary
end_step
printf '\n%sDone.%s Step timings:\n' "$GREEN$BOLD" "$RESET"
for ((i = 0; i < ${#TIMINGS[@]}; i++)); do printf '  %s\n' "${TIMINGS[$i]}"; done
printf '  total %ss\n' "$SECONDS"

printf '\nNext steps:\n'
if [ "$SKIP_BUILD" = 1 ]; then
  printf '  - Build the app:    scripts/build-mac.sh\n'
elif [ "$INSTALL_APP" = 1 ]; then
  printf '  - Open the app:     open "%s"\n' "$APP_INSTALLED"
else
  printf '  - Open the app:     open "%s"\n' "$ROOT/$APP_BUNDLE"
  printf '  - Install it:       ./install.sh --install-app   (or drag the app into /Applications)\n'
fi
printf '  - Develop with live reload: open a NEW terminal (so PATH picks up Homebrew and Go), then:  cd "%s" && wails dev\n' "$ROOT"
printf '  - Log of this run:  %s\n' "$LOG"
