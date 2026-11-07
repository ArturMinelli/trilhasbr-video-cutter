# Implementation Plan: Detect People and Cut by Track

**Branch**: `001-detect-people-tracks` | **Date**: 2026-11-06 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/001-detect-people-tracks/spec.md`

## Summary

Add a **Detect people** action to the workspace page. A new in-process Go inference package decodes sampled frames (bundled FFmpeg piped as raw RGB), runs YOLO11n person detection and OSNet re-ID embeddings through ONNX Runtime (cgo), and links detections into per-video tracks with a ByteTrack-style tracker that also uses embedding similarity. Videos are processed in parallel by a bounded worker pool. Tracks, embeddings and crops persist in SQLite plus the app data folder. The UI shows a gallery of tracks (grouped manually into person groups), and selected groups are cut with the existing cut/merge pipeline. Decisions from the grill session: ONNX Runtime via cgo, macOS arm64 only for v1, FFmpeg pipe for decode, per-video tracks + manual merge (no automatic cross-video clustering).

## Technical Context

**Language/Version**: Go 1.24 (toolchain 1.25.6), TypeScript/React (existing)

**Primary Dependencies**: existing Wails v2, `modernc.org/sqlite`. New: `github.com/yalue/onnxruntime_go` (cgo bindings) with bundled `libonnxruntime.dylib` (arm64). Models bundled: `person-yolo11n.onnx` (11 MB, 1×3×1280×1280 → 1×84×33600), `osnet_x1_0_msmt17.onnx` (8.7 MB, b×3×256×128 → b×512), obtained as the stock Ultralytics YOLO11n checkpoint exported to ONNX and the OSNet model.

**Storage**: SQLite (new migrations: `detection_runs`, `tracks`, `person_groups`, `person_group_tracks`); track embeddings as BLOB; crops as JPEG files under the app data directory.

**Testing**: `go test ./...` (tracker, NMS/decoder, grouping/cut-plan unit tests with synthetic data; inference integration test gated on the model files being present); `npm run build` for the frontend.

**Target Platform**: macOS arm64 (Wails desktop). Other OSes deferred behind the same `inference.Engine` interface.

**Project Type**: desktop-app (Wails, Go backend + React frontend)

**Performance Goals**: 1 h of footage in <15 min on an Apple Silicon laptop (SC-002); re-run on unchanged workspace <10 s (SC-005); UI stays responsive.

**Constraints**: offline; bounded parallelism (default = min(videos, max(2, NumCPU/3))); one shared ONNX session pool, not one model load per video; progress events; cancellable.

**Scale/Scope**: tens to low hundreds of videos per workspace, up to hours long each; tens to thousands of tracks.

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I Local-first | PASS | Models and runtime bundled; no network. |
| II Boundaries | PASS | New code under `internal/` by concern; Wails service stays a thin DTO mapper. |
| III Data integrity | PASS | Schema via migrations; typed errors (`MODEL_NOT_FOUND`, `INFERENCE_ERROR`, `DETECTION_ERROR`) surface to UI. |
| IV Media safety | PASS | Bundled-first resolution mirrored for models/ORT; FFmpeg work bounded; `detect:progress/finished/failed` events added alongside cut events. |
| V Minimal change | JUSTIFIED | One new dependency (onnxruntime_go + native lib) is unavoidable for local YOLO/OSNet; see Complexity Tracking. No other new libs (tracker, NMS, Hungarian are hand-written, small). |

Re-check after design: unchanged, PASS.

## Project Structure

### Documentation (this feature)

```text
specs/001-detect-people-tracks/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
└── contracts/wails-api.md
```

### Source Code (repository root)

```text
internal/
├── inference/                  # NEW: ONNX Runtime wrapper (cgo confined here)
│   ├── engine.go               # Engine interface: Detect(frame) / Embed(crops)
│   ├── ort.go                  # onnxruntime_go session pool, letterbox, batching
│   ├── yolo.go                 # output decode (84×33600) + NMS, person class only
│   └── osnet.go                # crop resize/normalize → 512-d L2-normalized
├── tracking/                   # NEW: pure Go, no cgo
│   ├── bytetrack.go            # two-stage association + embedding cost
│   ├── hungarian.go
│   └── tracker_test.go
├── media/ffmpeg/               # EXTEND: FrameReader (fps filter → rgb24 pipe), crop writer
├── app/actions/
│   ├── detect_run.go           # NEW: worker pool, per-video pipeline, resume/skip
│   ├── person_list.go          # NEW: list/filter groups, merge/split/remove
│   └── person_cut.go           # NEW: groups → CutSegment/Merge plan
├── app/jobs/                   # EXTEND: detection job state (reuse Manager pattern)
├── domain/detection.go         # NEW: Track, PersonGroup, DetectionRun
├── repository/sqlite/          # EXTEND: migrations + detection_repo.go
├── platform/                   # EXTEND: resolve models dir / libonnxruntime like FFmpegResolver
└── api/wails/                  # EXTEND: StartDetection, CancelDetection, ListPeople, ...
resources/
├── models/                     # NEW: person-yolo11n.onnx, osnet_x1_0_msmt17.onnx (+ manifest.json w/ sha256)
└── onnxruntime/darwin-arm64/libonnxruntime.dylib
frontend/src/
├── entities/person/            # NEW: model.ts (zod)
├── features/person-detection/  # NEW: DetectButton, DetectProgress
├── features/person-gallery/    # NEW: PersonCard, PersonGrid, filters, preview, CutSelectedModal
└── pages/WorkspaceDetailPage.tsx  # EXTEND: button + gallery tab
```

**Structure Decision**: follows the existing layered layout. `inference` is the only package with cgo, so the rest of the app (and its tests) stays CGO-free except the final link.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|---|---|---|
| New cgo dependency + native lib (Principle V) | Run YOLO11n/OSNet locally at usable speed | Pure-Go ONNX is far too slow for 1280px YOLO; a sidecar binary was rejected by the user in favor of in-process |
