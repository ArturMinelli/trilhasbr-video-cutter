# Third-party licenses and notices

TrilhasBR Video Cutter is licensed under the GNU Affero General Public License v3.0 or later (see `LICENSE` and `NOTICE`).
This file lists the components that ship in, are fetched by, or are run by the application. Each component keeps its own
license; the full texts are in the linked sources or in the module and package sources.

How this list was produced (verified 2026-11-07 against this repository): Go modules from
`GOOS=darwin GOARCH=arm64 go list -deps ./...` (main module excluded), licenses read from each module's `LICENSE` file
in the module cache; npm packages from the production dependency closure of `frontend/package.json` in
`frontend/package-lock.json` (license field); upstream licenses confirmed from each project's repository license
file. Re-run these checks when dependencies change.

## Detection models (release assets, not in the repository)

| Model | File | Upstream | License | SHA-256 |
|---|---|---|---|---|
| YOLO11n (COCO, exported at 1280, opset 12) | `person-yolo11n.onnx` | Ultralytics, https://github.com/ultralytics/ultralytics | AGPL-3.0 (the file's metadata says `license = AGPL-3.0 License (https://ultralytics.com/license)`), or an Ultralytics Enterprise license | `2c182881cc3d21614381e7c7b6d58feb1c6e49a509d14a67c6c5e2a34c62ee99` |
| OSNet x1.0 trained on MSMT17 (opset 17, dynamic batch) | `osnet_x1_0_msmt17.onnx` | torchreid, https://github.com/KaiyangZhou/deep-person-reid (code MIT); the Hugging Face repo https://huggingface.co/kaiyangzhou/osnet is tagged MIT | Code and Hugging Face repo: MIT. The training data, MSMT17, is distributed under a signed release agreement whose text we could not retrieve, so whether the weights can be used commercially is unconfirmed: treat as research use | `bad3a75234470f592b4c2b0a228b451347bba84e6181a6af7c5f93c4497cd564` |

Details and provenance: `MODELS.md`.

## Runtime and tools (fetched or installed, not committed)

| Component | License | Notes |
|---|---|---|
| ONNX Runtime 1.31.0 | MIT | Downloaded from https://github.com/microsoft/onnxruntime/releases by `scripts/fetch-onnxruntime.sh` with a pinned SHA-256. Its own third-party notices ship inside the release archive. |
| FFmpeg / FFprobe | LGPL-2.1+ or GPL-2.0+/3.0+, depending on the build | Not bundled (`resources/ffmpeg/` holds only a README). Installed by Homebrew and run as a separate process. If a future release bundles an FFmpeg binary, that binary's license and source-offer obligations apply. |
| Wails CLI v2.11.0 | MIT | Build tool installed by `install.sh`. |
| Spec Kit templates and scripts (`.specify/`, `.claude/skills`, `.cursor`, `.agents`) | MIT, Copyright GitHub, Inc. (github/spec-kit) | Development tooling committed in the repository. The upstream license text is reproduced at the end of this file. |

## Fonts and images

| Asset | License | Source |
|---|---|---|
| Nunito (`frontend/src/assets/fonts/nunito-v16-latin-regular.woff2`) | SIL Open Font License 1.1 | `frontend/src/assets/fonts/OFL.txt` is committed next to the font. |
| `build/appicon.png` | Wails project template asset (Wails is MIT) | Byte-identical (SHA-256 `28332a3b...`) to the file in the Wails v2.11.0 module (`pkg/buildassets/build/appicon.png`). Replace it for a branded release if you do not want the stock Wails icon. |
| `frontend/src/assets/images/logo-universal.png` | Wails project template asset (Wails is MIT) | Byte-identical to the Wails v2.11.0 Preact template copy. It is not referenced by the app code. |

## Go modules linked into the app (darwin/arm64): 18

| Module | Version | License |
|---|---|---|
| github.com/dustin/go-humanize | v1.0.1 | MIT |
| github.com/google/uuid | v1.6.0 | BSD-3-Clause |
| github.com/leaanthony/go-ansi-parser | v1.6.1 | MIT |
| github.com/leaanthony/slicer | v1.6.0 | MIT |
| github.com/leaanthony/u | v1.1.1 | MIT |
| github.com/mattn/go-isatty | v0.0.20 | MIT |
| github.com/ncruces/go-strftime | v1.0.0 | MIT |
| github.com/pkg/errors | v0.9.1 | BSD-2-Clause |
| github.com/remyoudompheng/bigfft | v0.0.0-20230129092748-24d4a6f8daec | BSD-3-Clause |
| github.com/rivo/uniseg | v0.4.7 | MIT |
| github.com/wailsapp/wails/v2 | v2.11.0 | MIT |
| github.com/yalue/onnxruntime_go | v1.38.0 | MIT |
| golang.org/x/exp | v0.0.0-20251023183803-a4bb9ffd2546 | BSD-3-Clause |
| golang.org/x/sys | v0.37.0 | BSD-3-Clause |
| modernc.org/libc | v1.67.6 | BSD-3-Clause |
| modernc.org/mathutil | v1.7.1 | BSD-3-Clause |
| modernc.org/memory | v1.11.0 | BSD-3-Clause |
| modernc.org/sqlite | v1.46.1 | BSD-3-Clause |

## npm packages bundled into the frontend (production closure): 11

| Package | Version | License |
|---|---|---|
| @remix-run/router | 1.23.0 | MIT |
| @tanstack/query-core | 5.90.20 | MIT |
| @tanstack/react-query | 5.90.21 | MIT |
| js-tokens | 4.0.0 | MIT |
| loose-envify | 1.4.0 | MIT |
| react | 18.3.1 | MIT |
| react-dom | 18.3.1 | MIT |
| react-router | 6.30.1 | MIT |
| react-router-dom | 6.30.1 | MIT |
| scheduler | 0.23.2 | MIT |
| zod | 3.23.8 | MIT |

Development-only packages (vite, typescript, tailwindcss, postcss and similar build tools) are not shipped. The one
development-only package with a non-permissive license is `caniuse-lite` (CC-BY-4.0, browser-support data used by the
build tooling).

## A note for binary distributions

The MIT and BSD licenses above require their copyright and permission notices to accompany copies of the software,
including binaries. This file names each license but does not reproduce every module's copyright line. If you
distribute built binaries, generate a full notices bundle (for example with `go-licenses` for the Go modules and your
package manager's license report for npm) and ship it with the app.

## Spec Kit license text (github/spec-kit)

    MIT License
    
    Copyright GitHub, Inc.
    
    Permission is hereby granted, free of charge, to any person obtaining a copy
    of this software and associated documentation files (the "Software"), to deal
    in the Software without restriction, including without limitation the rights
    to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
    copies of the Software, and to permit persons to whom the Software is
    furnished to do so, subject to the following conditions:
    
    The above copyright notice and this permission notice shall be included in all
    copies or substantial portions of the Software.
    
    THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
    IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
    FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
    AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
    LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
    OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
    SOFTWARE.
