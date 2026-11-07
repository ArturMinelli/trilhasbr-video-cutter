# Install tests

Tests for `install.sh`, with no VM. Run from the repository root.

| Script | Purpose |
|---|---|
| `tests/install/run-local.sh` | Hermetic clean-room run of the real `install.sh` on this Mac, then checks that the built app works. |
| `tests/install/run-stubbed.sh` | Runs the real `install.sh` against fake tools in throwaway sandboxes: 49 scenarios, about 30 s, for paths a configured Mac never reaches (first-time Command Line Tools and Homebrew install, brew installs, Go/Node too old, no sudo, bad checksums, profile writing, Rosetta). See its header. |
| `tests/install/lint.sh` | `bash -n` and `shellcheck` over `install.sh`, `scripts/*.sh` and every `*.sh` under `tests/install/`. |

## run-local.sh

```bash
tests/install/run-local.sh --videos ~/some-folder-with-mp4s    # everything, including the full pipeline
tests/install/run-local.sh --skip-e2e                           # no real videos needed
tests/install/run-local.sh --models-dir /path/to/release-assets # models from a local folder (before the release exists)
```

Options: `--models-dir DIR` (a folder with the two release `.onnx` files: they are served over `file://` instead of being downloaded from the GitHub release, so the test does not depend on the release existing or on the network for them), `--videos DIR` (folder with `.mp4` files for stage S7), `--skip-e2e`, `--keep` (keep the work dir),
`--timeout SECS` (per long stage, default 1800), `-h`.

Prerequisites: an Apple Silicon Mac with network access. Takes about 5 to 15 minutes, mostly downloads (the pinned Go toolchain,
Go modules, npm packages, the Wails CLI, ONNX Runtime) and the Wails build.

### How it stays clean

- It copies a snapshot of the project into a temp dir: tracked files plus new files that are not git-ignored.
  `node_modules`, `build/`, `frontend/dist`, the ONNX Runtime library, the model files and `install.log` are never copied, so
  everything `install.sh` produces is produced from scratch.
- It runs `install.sh` under `env -i` with a throwaway `HOME`, `GOPATH`, Go build and module caches, npm cache and
  a stripped `PATH` (`/usr/bin:/bin:/usr/sbin:/sbin`). Nothing from your own Go, npm or shell setup is reused.
- It never passes `--install-app`, and `INSTALL_SKIP_PROFILE=1` keeps it away from your real `~/.zprofile`.
  `HOMEBREW_NO_INSTALL_UPGRADE=1`, `HOMEBREW_NO_AUTO_UPDATE=1` and `HOMEBREW_NO_ENV_HINTS=1` are set so a brew call
  cannot upgrade anything.
- Hard guard against mutating your machine: before the run it records the full `brew list --formula --versions`,
  the `go version` and `node --version` you get on your normal PATH, and your `~/.zprofile` and
  `/Applications` app. After the run, on every path including failures, Ctrl-C and early exits, it compares them.
  Any difference fails the run (S8) with a loud message that lists what changed (for example `was: go 1.25.6`
  `now: go 1.27.3`).
- Stage S7 works on an APFS clone of your videos, so the `exports/` folder the test creates never lands in your folder.

### Stages

| Stage | What it asserts |
|---|---|
| S0 | The snapshot is clean (has `install.sh`, `manifest.json` and `scripts/release.env`; has neither the models, which are downloaded, nor any of the ignored artifacts). `go`, `node`, `npm`, `ffmpeg`, `ffprobe` and `wails` are not on the stripped `PATH`. |
| S1 | `--help` exits 0, an unknown flag exits 2, and `INSTALL_DRY_RUN=1` exits 0, only prints actions, changes no file and installs nothing. |
| S2 | `install.sh --yes --run-tests` exits 0 within the timeout, downloads ONNX Runtime fresh, reaches "wails doctor: ready" and prints its summary. |
| S3 | The app bundle exists and passes `codesign --verify --deep --strict`; both models, `manifest.json` and an arm64 `libonnxruntime.dylib` are inside it and match the manifest checksums; `wails` is the version in `go.mod`; the app binary was compiled with the Go toolchain pinned in `scripts/toolchain.env` (`go version -m`); no `brew install` ran; Go modules and npm packages were downloaded fresh; `go test` output has no failures. |
| S4 | A second `install.sh --yes --skip-build` exits 0 in under 120 s, reports tools as already installed and does not download ONNX Runtime again. |
| S5 | `TestRuntimeLoadsAndRuns` passes (not skipped): ONNX Runtime, CoreML and both models load and run. |
| S6 | The built app starts, is still running after 12 s, has created its SQLite database under the isolated `HOME`, exits on SIGTERM and leaves no process behind. A window briefly appears on screen. |
| S7 | With `--videos`: index, detect, re-run skips, list people and cut all pass on your videos (the gated `TestE2EDetectRealVideos`). |
| S8 | The real-machine guard: Homebrew formulae and versions, your Go and Node, `~/.zprofile` and the `/Applications` app are exactly as before. |

The run prints a PASS/FAIL table and exits non-zero on any failure. Logs go to `tests/install/artifacts/<timestamp>/`
(git-ignored): `results.tsv`, `run.log`, `install.log` and one log per stage. On failure (or with `--keep`) the temp
work dir is kept and its path is printed.

### What this proves, and what it cannot

Proves: from a clean checkout, `install.sh` runs non-interactively and idempotently, fetches and verifies what it
downloads, builds a signed app with the models and runtime bundled, and that app starts and runs the whole detection
pipeline.

Cannot prove: the first-time installation of the Xcode Command Line Tools, Homebrew, Go, Node and ffmpeg. They are
already installed on a developer Mac and are reused (`run-stubbed.sh` covers the script's logic for those steps against
fake tools, but not the real installers). Testing those needs a bare machine: a spare Mac, a fresh user on a
reset Mac, or a macOS VM. Docker cannot do it, because macOS does not run in a Docker container.
