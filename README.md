# TrilhasBR Video Cutter

Desktop app (Wails: Go backend, React frontend) for working with large folders of MP4 footage. It indexes a
folder into a local SQLite database, cuts time-based clips with FFmpeg, and detects and tracks people in the
videos with local models so you can cut clips of the people you pick. Everything runs on your machine; no network
access is needed after installation.

## Contents

- [Features](#features)
- [Requirements](#requirements)
- [Quick start](#quick-start)
- [Manual install](#manual-install)
- [Using the app](#using-the-app)
- [Where data lives](#where-data-lives)
- [How binaries are resolved](#how-binaries-are-resolved)
- [Development](#development)
- [Architecture](#architecture)
- [Error codes](#error-codes)
- [Install testing](#install-testing)
- [Troubleshooting](#troubleshooting)
- [Known limitations](#known-limitations)
- [License](#license)
- [Spec Kit](#spec-kit)

## Features

**Workspaces and cuts**

- Workspaces from a folder: pick a folder with the native dialog; MP4 files are indexed recursively into SQLite
  (`exports` sub-folders are skipped).
- `started_at` comes from media metadata (`creation_time`, `com.apple.quicktime.creationdate`) with a
  filename-based fallback (for example `CAM_20260304_142530.mp4`). `finished_at` is `started_at + duration`.
  Videos with an unknown `started_at` are stored with `NULL` and excluded from time-window cuts.
- Time-based cut: a name (output folder), a target date/time, and seconds before and after. It produces one
  segment per matching source video plus a merged clip. FFmpeg work runs in parallel with a bounded worker count.
- CSV bulk cuts: import a CSV with the header `name,datetime,seconds_before,seconds_after` (see `import.csv`).
- Shift time: move the timestamps of selected videos forward or backward when a camera clock was wrong.
- Refresh a workspace to re-scan its folder, or delete a workspace from the app. Delete never touches your video
  files or your exported cuts.

**People detection**

- Local detection with YOLO11n (person class) and OSNet re-identification embeddings, both through ONNX Runtime
  (YOLO runs on CoreML where possible, with CPU fallback).
- Per-video tracking (ByteTrack-style, with appearance embeddings to recover people after occlusion).
- Adjustable sampling rate: 1 to 15 frames per second, default 5.
- Videos are processed in parallel, with progress, cancel, and skip-if-unchanged on re-runs.

**People gallery**

- Thumbnails, with filters for minimum duration and minimum confidence.
- Preview that plays the person's video with a bounding-box overlay (toggle "Show boxes").
- Rename a person (name or `Person N`), merge several cards into one person, split a track out of a person, or
  remove a false detection.
- Cut selected people: one clip per person, padded before and after, merged across videos when needed.

## Requirements

| Requirement | Detail |
|---|---|
| OS | macOS on Apple Silicon (arm64). This is the only supported platform in v1. |
| Why | The bundled ONNX Runtime library is `darwin-arm64` and YOLO uses CoreML. Other platforms need their own runtime library and are not set up. |
| Build tools | Xcode Command Line Tools (cgo needs a C compiler), Go 1.24+, Node.js with npm, FFmpeg/FFprobe, and the Wails v2 CLI. `install.sh` installs all of them. |
| Disk | A few GB for the toolchain and Homebrew packages; the built app is about 75 MB. |

## Quick start

```bash
git clone https://github.com/ArturMinelli/trilhasbr-video-cutter.git
cd trilhasbr-video-cutter
./install.sh
```

`install.sh` starts from a bare Mac and is idempotent: steps that are already done are detected and skipped, so
you can re-run it after a failure. In order, it:

1. Checks that you are on macOS arm64.
2. Installs the Xcode Command Line Tools if missing.
3. Installs Homebrew if missing.
4. Installs Go, Node.js and FFmpeg with Homebrew. It never upgrades or reinstalls anything already installed; if a
   Homebrew copy exists but is not on your `PATH`, it uses that copy.
5. Installs the Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0`, the version in `go.mod`).
   Go and the build are pinned to the `go1.25.6` toolchain (see [Go toolchain](#go-toolchain)); the first run
   downloads it from `proxy.golang.org`.
6. Fetches the ONNX Runtime library (version 1.31.0) into `resources/onnxruntime/darwin-arm64/` with
   `scripts/fetch-onnxruntime.sh`, checking a pinned SHA-256. Then downloads the two detection models from this
   project's GitHub release (`scripts/fetch-models.sh`; repository and tag are set in `scripts/release.env`) into
   `resources/models/`, checking each file against the SHA-256 pinned in `resources/models/manifest.json`. The models
   are not stored in git.
7. Installs the frontend dependencies.
8. Builds the app into `build/bin/trilhasbr-video-cutter.app` (via `scripts/build-mac.sh`), bundles the models and
   runtime into it, and ad-hoc signs it.

Everything is logged to `install.log`. Rough timing: a few minutes when the toolchain is already present; allow
20 to 40 minutes on a clean Mac, mostly for the Command Line Tools, Homebrew and FFmpeg.

| Option | Effect |
|---|---|
| `--help` | Show usage. |
| `--install-app` | Also copy the built app to `/Applications`. |
| `--skip-build` | Install dependencies only; do not build the app. |
| `--run-tests` | Also run `go vet`, `go test ./...` and the frontend build. |
| `--yes` | Never prompt: fail with a clear message instead of asking for a sudo password. |
| `INSTALL_DRY_RUN=1` | Environment variable: print what would be done without doing it. |
| `INSTALL_SKIP_PROFILE=1` | Environment variable: do not add Homebrew and Go to `PATH` in `~/.zprofile`. |

Open the app from `build/bin/trilhasbr-video-cutter.app` (or `/Applications` if you used `--install-app`). The app is
ad-hoc signed and not notarized; see [Troubleshooting](#troubleshooting) if macOS blocks it.

## Manual install

If you prefer not to run the script:

```bash
# 1. Xcode Command Line Tools (provides clang for cgo and git)
xcode-select --install

# 2. Homebrew: https://brew.sh, then:
brew install go node ffmpeg

# 3. Wails CLI (make sure $(go env GOPATH)/bin is on your PATH). Pin the Go toolchain first (see "Go toolchain" below).
export GOTOOLCHAIN=go1.25.6
go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0

# 4. ONNX Runtime 1.31.0 for darwin-arm64 and the two detection models (downloaded from the GitHub release),
#    each verified against a pinned SHA-256. Set MODELS_BASE_URL to use a mirror or a local folder (file:///dir).
./scripts/fetch-onnxruntime.sh
./scripts/fetch-models.sh

# 5. Build and bundle the app (runs `wails build`, copies models + runtime in, ad-hoc signs).
#    It also fetches ONNX Runtime first if step 4 was skipped.
./scripts/build-mac.sh
```

`scripts/build-mac.sh` runs the frontend install and build as part of `wails build`. The result is
`build/bin/trilhasbr-video-cutter.app`; copy it to `/Applications` if you want it installed.

### Go toolchain

The app builds with a pinned Go toolchain, `go1.25.6`, defined once in `scripts/toolchain.env`; `install.sh` and
`scripts/build-mac.sh` export it as `GOTOOLCHAIN`. Homebrew's newest Go (1.27.x at the time of writing) makes
`wails build` with Wails v2.11.0 fail with `internal error: package "time" without types was imported`. Any Go
1.24 or newer is fine as the installed `go`: Go downloads the pinned toolchain from `proxy.golang.org` the first time
it is needed (a few tens of MB, needs network access). When you run Wails or Go commands by hand, export it too:

```bash
export GOTOOLCHAIN=go1.25.6
wails dev        # or: wails build -platform darwin/arm64
```

## Using the app

1. **Import a folder.** On the home page click **Import** and choose a folder. The app scans it recursively for
   MP4 files and shows a summary (indexed, failed, and videos without a start time).
2. **Open the workspace.** The workspace page has two tabs: **Videos** (folder tree and video list) and **People**.
3. **Detect people.** Click **Detect people**. Open **Advanced** to change the sampling rate (1 to 15 fps, default
   5). Progress shows videos done, the current video and tracks found; **Cancel** stops the run and keeps the
   videos that already finished. Running it again skips videos that have not changed.
4. **Browse the People tab.** Each card is one person: a thumbnail, the time range, how many appearances and
   videos, total duration and confidence. Use the minimum duration (seconds) and minimum confidence (%) filters to
   hide short or weak tracks.
5. **Preview.** Click a card to play the person's video. Boxes are drawn over the video: the person you opened is
   highlighted and other people in the same video are shown in a muted color. Untick **Show boxes** to hide them.
   If a video has no box data yet, the preview says to run **Detect people** again.
6. **Rename.** Use the pencil on a card or in the preview header. The title shows the name, or `Person N` when
   unnamed (a named person also shows its `#N`). Names are trimmed and limited to 60 characters; an empty name
   clears it.
7. **Fix the grouping.** Tick two or more cards and use **Merge selected** when they are the same person. In the
   preview, **Split out** moves one appearance into its own person and **Not a person** removes a false detection.
8. **Cut selected.** Tick the people you want and click **Cut selected**. Choose a folder name and the seconds
   before and after (default 20 each). Each person becomes one clip.

Where cuts end up (inside the workspace folder):

| Action | Output |
|---|---|
| Time-based cut | `<workspace>/exports/<name>/segments/` (one clip per source video) and `<workspace>/exports/<name>/merged/final.mp4` |
| Cut selected people | `<workspace>/exports/<name>/person-<id>.mp4` (one per person; the folder must be new or empty) |
| CSV cuts | under `<workspace>/exports/` |

Other workspace actions:

- **Refresh** re-scans the stored folder. Unchanged videos keep their IDs, so their detected people survive. Videos
  that are gone from disk are removed along with their people. If the folder is missing, you get an error and
  nothing is changed. If FFmpeg cannot be found you get `FFMPEG_NOT_FOUND` instead of an empty folder.
- **Delete** removes the workspace from the app: its index, detected people and generated thumbnails. It does not
  touch your video files or your exports.
- Refresh and Delete are blocked while detection is running for that workspace; cancel detection first.
- **Shift time** and **Import CSV** are on the workspace page.

Datetime input is interpreted in your local time zone and stored as UTC.

## Where data lives

Application data is under `~/Library/Application Support/trilhasbr-video-cutter/`:

| Path | Contents |
|---|---|
| `video-cutter.sqlite` | Workspaces, videos, detection runs, tracks (with per-frame boxes), people groups and names |
| `thumbnails/<workspaceID>/` | Video preview images |
| `people/<workspaceID>/` | Person thumbnails (`<videoID>-<trackID>.jpg`) |

Deleting a workspace removes its rows (videos, runs, tracks and groups follow through foreign keys) and the
`thumbnails/<workspaceID>` and `people/<workspaceID>` folders. Your videos and `exports` folders are never deleted.

## How binaries are resolved

**FFmpeg and FFprobe**, first match wins:

1. Bundled: `resources/ffmpeg/<goos>-<goarch>/` (relative to the working directory, or `resources/ffmpeg/` or
   `ffmpeg/` next to the executable). No binaries are bundled by default.
2. `PATH`.
3. `/opt/homebrew/bin`, then `/usr/local/bin`. This matters because apps launched from Finder get a minimal `PATH`
   without Homebrew.

If none is found the app returns `FFMPEG_NOT_FOUND`.

**Detection models and ONNX Runtime**, first root that contains all three files wins (`models/person-yolo11n.onnx`,
`models/osnet_x1_0_msmt17.onnx`, `onnxruntime/darwin-arm64/libonnxruntime.dylib`):

1. `resources/` relative to the working directory.
2. `<executable dir>/resources/`.
3. `<app bundle>/Contents/Resources/resources/` (where `scripts/build-mac.sh` puts them).
4. The executable's own directory.

The models are not in git: `scripts/fetch-models.sh` downloads them from the project's GitHub release and verifies
them. At run time the two models are checked against the SHA-256 values in `resources/models/manifest.json`; a
missing file or a mismatch returns `MODEL_NOT_FOUND`. The engine is opened on the first **Detect people** run, so the rest of the app
works even if the models are missing.

## Development

```bash
export GOTOOLCHAIN=go1.25.6     # once per shell: see "Go toolchain" above (needed by wails dev / wails build)
wails dev                       # from the repo root: Go backend + Vite dev server
go test ./...                   # backend tests
cd frontend && npm run build    # type-check and build the frontend
./scripts/build-mac.sh          # full app build with bundled models and runtime
```

Some tests need real files and skip themselves when their input is not provided:

| Test | Enable with |
|---|---|
| End-to-end detect, list and cut on real videos (`internal/app/actions`, `TestE2EDetectRealVideos`) | `DETECT_E2E_DIR=<folder with .mp4>` (optional: `DETECT_E2E_FPS`, `DETECT_E2E_WORKERS`, `DETECT_E2E_THUMBS`), then `go test ./internal/app/actions -run E2E -v -timeout 30m` |
| Inference smoke test (`internal/inference`, `TestSmoke`) | `INFERENCE_SMOKE_FRAME=<png with a person>` (optional: `INFERENCE_SMOKE_PROVIDER=cpu\|coreml\|auto`) |
| FFmpeg frame reader (`internal/media/ffmpeg`) | `FRAMES_TEST_VIDEO=<short mp4>` |

These tests also need the models and `libonnxruntime.dylib` in `resources/` (run `./scripts/fetch-onnxruntime.sh` and
`./scripts/fetch-models.sh`; see [Manual install](#manual-install)).

Project conventions live in `.specify/memory/constitution.md` (local-first, layered boundaries, typed errors,
bounded FFmpeg work, smallest correct diff).

### Project structure

```text
app.go, main.go                  Wails entry points; app.go exposes the backend methods to the frontend
internal/
  api/wails/                     Wails service, DTOs, runtime event bridge, /media/<videoId> handler
  app/actions/                   Use cases: workspace load/refresh/delete, cuts, detection run, people list/cut
  app/csvimport/                 CSV cut parser
  app/jobs/                      In-memory job state (cuts and detection)
  apperrors/                     Typed errors surfaced to the UI
  bootstrap/                     Dependency wiring
  domain/                        Workspace, Video, Cut, Track and person types
  inference/                     ONNX Runtime wrapper: YOLO detector, OSNet embeddings (cgo lives here)
  media/ffmpeg/                  Probe, cut, merge, raw frame reader, JPEG crops
  media/timestamp/               Metadata and filename timestamp parsing
  platform/                      Binary and model resolution, app data paths, open-folder
  repository/sqlite/             SQLite repositories and migrations (modernc.org/sqlite, no cgo)
  tracking/                      ByteTrack-style tracker and Hungarian assignment
frontend/src/                    app, pages, features, entities, shared, infrastructure (layered React)
resources/                       models/, onnxruntime/, ffmpeg/ (see above)
scripts/                         fetch-onnxruntime.sh, fetch-models.sh, release.env (where the models are published), build-mac.sh, toolchain.env
specs/, .specify/                Spec Kit feature specs and project constitution
```

Stack: Go and Wails v2; React 18, TypeScript, Vite, TanStack Query, React Router, Zod and Tailwind CSS;
SQLite via `modernc.org/sqlite`; FFmpeg/FFprobe; ONNX Runtime through `github.com/yalue/onnxruntime_go`.

### Publishing the models (maintainers)

The model files are release assets, pinned by `resources/models/manifest.json`. To publish or replace them:

1. Put the new `.onnx` files in `resources/models/` and update the `sha256` values in `manifest.json`.
2. Choose a new tag in `scripts/release.env` (`MODELS_TAG`, for example `models-v2`).
3. Upload them: `gh release create models-v2 resources/models/*.onnx resources/models/manifest.json --title "Detection models v2" --notes-file RELEASE_NOTES.md`.
4. Commit `manifest.json` and `release.env` together. Old tags keep working for old commits.

## Architecture

The detection pipeline, per video (several videos run in parallel):

```text
 video.mp4
    |  ffmpeg: fps=N, raw rgb24 frames over a pipe (decoded up to 1920 px wide)
    v
 YOLO11n (1280x1280 input, person class, confidence >= 0.35, NMS)
    |  boxes per sampled frame
    v
 OSNet re-ID (256x128 crops, 512-d embeddings; boxes with confidence >= 0.5 and height >= 64 px)
    v
 ByteTrack-style tracker (IoU + appearance, recovers short occlusions)
    |  tracks with per-frame boxes, mean embedding, best crop
    v
 SQLite: tracks (boxes normalized 0..1) + person groups   --> gallery, overlay, cuts
```

- Each new track starts as its own person group; merge, split and remove edit the groups.
- Results are stored with the sampling rate, a model version and the file size and modification time. A video is
  skipped on re-run only when all of them match.
- Tracks are per video. The same person in two videos shows up as two cards until you merge them.
- Cuts of people reuse the same FFmpeg segment and merge code as time-based cuts.

## Error codes

Errors reach the UI as typed codes:

| Code | Typical cause | Fix |
|---|---|---|
| `FFMPEG_NOT_FOUND` | FFmpeg/FFprobe not on `PATH` or in the Homebrew directories | `brew install ffmpeg`, then retry |
| `MODEL_NOT_FOUND` | Models or `libonnxruntime.dylib` missing, or a model checksum mismatch | Run `./scripts/fetch-onnxruntime.sh` and `./scripts/fetch-models.sh`, then rebuild with `./scripts/build-mac.sh`; do not edit the model files |
| `INFERENCE_ERROR` | ONNX Runtime failed to load or run (for example a library that cannot be loaded) | Check the log; rebuild so the library is signed with the app |
| `DETECTION_ERROR` | Detection failed (for example every video in the run failed) | Check the per-video errors in the log |
| `VALIDATION_ERROR` | Bad input: fps outside 1 to 15, an unknown id, a non-empty output folder, an invalid name, or an action blocked while detection runs | Fix the input as the message says |
| `PROBE_ERROR` | FFprobe could not read a file | Check that the file is a valid MP4 |
| `CUT_ERROR`, `MERGE_ERROR` | FFmpeg failed while cutting or merging | Check the log; check disk space |
| `IO_ERROR` | A folder or file could not be read or written (for example a missing workspace folder on refresh) | Check that the path exists and is readable |
| `DB_ERROR` | SQLite failure | Check the log; the database is `video-cutter.sqlite` in the app data folder |
| `INTERNAL_ERROR` | A bug or missing wiring | Report it with the log |

## Install testing

`tests/install/run-local.sh` tests `install.sh` on this Mac without a VM. It copies a clean snapshot of the project
(tracked and new files only, none of your build artifacts) into a temp folder and runs the real `install.sh` there
under `env -i` with a throwaway `HOME`, `GOPATH`, Go and npm caches and a stripped `PATH`. The Go toolchain, Go modules,
npm packages, the Wails CLI and ONNX Runtime are therefore downloaded fresh, and your real `~/.zprofile`, `/Applications`
and app data are never touched. A hard guard compares your real Homebrew formulae and versions, Go, Node, `~/.zprofile`
and `/Applications` app before and after, on every path including failures, and fails the run if anything changed.

```bash
./tests/install/run-local.sh --videos ~/folder-with-mp4s   # everything, including the full pipeline on your videos
./tests/install/run-local.sh --skip-e2e                    # no videos needed
./tests/install/run-stubbed.sh                             # install.sh vs fake tools: 44 scenarios, ~20 s (first-time CLT/Homebrew/brew paths, error cases)
./tests/install/lint.sh                                    # bash -n + shellcheck for all the shell scripts
```

It takes roughly 5 to 15 minutes and prints a PASS/FAIL table (logs in `tests/install/artifacts/`). It asserts that
`install.sh` exits cleanly, that a dry run changes nothing, that the app bundle is built, signed and contains the models
and runtime, that a second run is fast and downloads nothing again, that ONNX Runtime and the models load and run, that
the built app starts and creates its database, and (with `--videos`) that index, detect, re-run, list and cut all work
on real videos. A window flashes on screen for about 12 seconds while the app is started. See `tests/install/README.md`
for the stage list.

What it cannot prove: Homebrew, the Xcode Command Line Tools, Go, Node and ffmpeg are already installed on a developer
Mac and are reused, so their first-time installation is not exercised. Testing that needs a bare machine, such as a
spare Mac, a new user on a reset Mac, or a macOS VM. Docker does not help: macOS cannot run in a Docker container (Docker
on a Mac runs a Linux VM, and this app only supports macOS).

## Troubleshooting

| Symptom | Cause and fix |
|---|---|
| Workspace shows no videos, or `FFMPEG_NOT_FOUND` | Finder-launched apps do not see Homebrew on `PATH`. The app also looks in `/opt/homebrew/bin` and `/usr/local/bin`; install FFmpeg with `brew install ffmpeg`, or start the app from a terminal. |
| macOS says the app is damaged or from an unidentified developer | The app is ad-hoc signed and not notarized. Right-click the app and choose Open, or run `xattr -dr com.apple.quarantine /Applications/trilhasbr-video-cutter.app`. |
| `MODEL_NOT_FOUND` | A model or `libonnxruntime.dylib` is missing, or a model does not match `manifest.json`. Run `./scripts/build-mac.sh` (it fetches the library and the models if missing), or `./scripts/fetch-models.sh` on its own; a damaged model file is replaced by a verified download. See [How binaries are resolved](#how-binaries-are-resolved). |
| ONNX Runtime fails to load | The library must be signed together with the app: build with `scripts/build-mac.sh`, which re-signs the bundle after copying it in. |
| The first detection run feels slow to start | The engine and CoreML compile on first use; later runs in the same session start faster. |
| First Detect people run after an update re-analyzes every video | The pipeline version is part of the stored analysis key. When the stored tracking output changes shape, old results are re-analyzed once. |
| Names and merges disappeared after re-running detection | Re-analyzing a video replaces its tracks and people, which resets names, merges, splits and removals for that video. Detection skips unchanged videos, so this only happens when the settings, models or pipeline version change, or the file changed. |
| Deleting a workspace left data behind | Foreign keys must be on for every database connection; the app sets this in the connection string. If you modified the database outside the app, check it with `sqlite3`. |
| Cut fails with "output folder already exists and is not empty" | Choose a new folder name; existing exports are never overwritten. |
| `wails: command not found` | Add `$(go env GOPATH)/bin` to your `PATH`. |
| `wails build` / `wails dev` fails with `package "time" without types was imported` | You are building with a Go that is too new for Wails v2.11.0 (Homebrew's Go 1.27.x). Run `export GOTOOLCHAIN=go1.25.6` in that shell, or build with `scripts/build-mac.sh`, which sets it. See [Go toolchain](#go-toolchain). |
| Go reports it cannot download `go1.25.6` | The first build needs network access to `proxy.golang.org` (check VPN/proxy settings, or set `GOPROXY`). |
| `install.sh` stops with "Go >= 1.24.0 is required" | It never upgrades installed packages. If Homebrew installed Go, run `brew upgrade go` yourself, then re-run `./install.sh`. The same applies to Node.js. |

## Known limitations

- macOS on Apple Silicon only.
- The app is ad-hoc signed, not notarized, so other Macs show a Gatekeeper warning.
- One detector class (person); tracks are per video, with manual merging. There is no automatic cross-video
  identity.
- Re-analysis replaces a video's tracks and people, including your manual edits.
- The gallery shows one page of up to 500 people.
- Speed depends on the machine: on an Apple M4, 7 minutes of 480x854 footage took about 1 minute at 5 fps (about
  7 to 8 times real time). Higher sampling rates are slower (10 fps was about 3 times real time).

## License

TrilhasBR Video Cutter is free software, licensed under the **GNU Affero General Public License v3.0 or later**
(`AGPL-3.0-or-later`). See [LICENSE](LICENSE) and [NOTICE](NOTICE). Copyright (C) 2026 Artur Peixoto.

It is a local desktop application. If you modify it and make that version available to users over a network, section 13
of the AGPL requires you to offer them the corresponding source.

Third-party components keep their own licenses: see [THIRD_PARTY_LICENSES.md](THIRD_PARTY_LICENSES.md).

### Models and licenses

The application downloads two ONNX models from the GitHub release `models-v1` (`scripts/fetch-models.sh`, verified by
SHA-256). They are separate works with their own terms; full details are in [MODELS.md](MODELS.md).

| Model | What it is | License |
|---|---|---|
| `person-yolo11n.onnx` | Ultralytics YOLO11n, stock COCO checkpoint exported to ONNX at 1280 px | AGPL-3.0 (https://ultralytics.com/license). Ultralytics also offers an Enterprise license. |
| `osnet_x1_0_msmt17.onnx` | OSNet x1.0 re-identification weights (torchreid family, trained on MSMT17) | torchreid code is MIT. The MSMT17 dataset terms could not be confirmed: treat these weights as research use and check the terms before any commercial use. |

The AGPL-3.0 license of this repository is chosen to be consistent with the Ultralytics weights. If you want to use the
code in a closed or commercial product, you need an Ultralytics Enterprise license (or a different detector) and you
need to check the OSNet and MSMT17 terms. This is a description of the licenses as we understand them, not legal advice.

## Spec Kit

Feature specs follow [Spec Kit](https://github.com/github/spec-kit). `specs/001-detect-people-tracks/` holds the
spec, plan, research notes, data model, API contract and task list for people detection. `.specify/` holds the Spec
Kit templates, scripts and the project constitution (`.specify/memory/constitution.md`).
