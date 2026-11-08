---
description: "Task list for Detect People and Cut by Track"
---

# Tasks: Detect People and Cut by Track

**Input**: Design documents from `/specs/001-detect-people-tracks/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/wails-api.md
**Tests**: Not requested as TDD. Small unit tests are included only for non-trivial logic named in plan.md (decoder/NMS, tracker, cut planning).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: parallelizable (different files, no dependency on incomplete tasks)
- Paths are relative to the repo root `~/projects/trilhasbr-video-cutter`

---

## Phase 1: Setup

- [X] T001 Add `person-yolo11n.onnx` (the stock Ultralytics YOLO11n checkpoint exported to ONNX) and `osnet_x1_0_msmt17.onnx` (the OSNet model) to `resources/models/` and write `resources/models/manifest.json` with sha256 and input/output shapes (values from the model export metadata)
- [X] T002 [P] Add `libonnxruntime.dylib` (arm64) under `resources/onnxruntime/darwin-arm64/` and a `resources/onnxruntime/README.md` stating the pinned version and source
- [X] T003 Add `github.com/yalue/onnxruntime_go` to `go.mod` (`go get`), then run `go mod tidy`
- [X] T004 [P] Add `.gitattributes`/`.gitignore` handling for `resources/models/*.onnx` and the dylib (use Git LFS or document the manual fetch in `README.md`), and add a Detection section to `README.md`

---

## Phase 2: Foundational (blocks all user stories)

- [X] T005 Add error codes `MODEL_NOT_FOUND`, `INFERENCE_ERROR`, `DETECTION_ERROR` in `internal/apperrors/errors.go`
- [X] T006 [P] Create domain types `Track`, `PersonGroup`, `DetectionRun` in `internal/domain/detection.go` per data-model.md
- [X] T007 Add migrations for `detection_runs`, `video_analyses`, `tracks`, `person_groups`, `person_group_tracks` in `internal/repository/sqlite/migrations.go` exactly per data-model.md: `sample_fps` REAL 1–15 default 5; `detection_runs.status` ∈ RUNNING/SUCCEEDED/FAILED/CANCELLED; `video_analyses.status` ∈ DONE/FAILED; `tracks.embedding` BLOB 512×float32; `tracks.removed` INTEGER 0/1; `person_group_tracks.track_id` UNIQUE; FKs to `videos`/`workspaces` with ON DELETE CASCADE
- [X] T008 Implement `internal/repository/sqlite/detection_repo.go` (create/finish run, upsert/get `video_analyses`, replace tracks for a video then delete empty groups, list groups with filters, merge/split/remove) and declare its interface in `internal/repository/contracts.go`
- [X] T009 [P] Extend `internal/platform/binaries.go` with a resolver for the models dir and `libonnxruntime` (bundled-first under `resources/`, then next to the executable), returning `MODEL_NOT_FOUND` and verifying sha256 from the manifest
- [X] T010 [P] Define the `Engine` interface (`Detect(frame) []Box`, `Embed(crops) [][]float32`) in `internal/inference/engine.go`
- [X] T011 [P] Add `FrameReader` to `internal/media/ffmpeg/` (new file `frames.go`): run bundled FFmpeg with `-vf fps=N -f rawvideo -pix_fmt rgb24 -`, yield frames with timestamp = index/fps, honor context cancellation, bounded channel buffer
- [X] T012 [P] Add detection job state (pending/running/succeeded/failed/cancelled, counters, cancel func) to `internal/app/jobs/` following the existing `Manager` pattern
- [X] T013 Add DTOs and event payloads (`detect:progress`, `detect:finished`, `detect:failed`) in `internal/api/wails/types.go` and `internal/app/actions/interfaces.go` per contracts/wails-api.md

**Checkpoint**: foundation ready.

---

## Phase 3: User Story 1 - Detect people in a workspace (P1) 🎯 MVP

**Goal**: pressing Detect people analyzes all videos and persists tracks, with progress and cancel.
**Independent Test**: run detection on a workspace, see a summary with tracks found; re-run skips unchanged videos.

- [X] T014 [P] [US1] Implement ONNX session pool, letterbox to 1280×1280 and CoreML-with-CPU-fallback in `internal/inference/ort.go`
- [X] T015 [P] [US1] Implement YOLO output decode (1×84×33600, class 0 only, conf ≥0.35) and NMS (IoU 0.5) in `internal/inference/yolo.go`, with a unit test using a synthetic tensor in `internal/inference/yolo_test.go`
- [X] T016 [P] [US1] Implement OSNet crop preprocessing (256×128, normalize) and L2-normalized 512-d output, batched, in `internal/inference/osnet.go`
- [X] T017 [P] [US1] Implement Hungarian assignment in `internal/tracking/hungarian.go`
- [X] T018 [US1] Implement ByteTrack-style tracker (constant-velocity Kalman, high/low confidence two-stage IoU association, blended IoU+cosine cost for lost tracks, lost tolerance ≈2 s video time) in `internal/tracking/bytetrack.go`, with a test of a crossing/occlusion scenario in `internal/tracking/tracker_test.go`
- [X] T019 [US1] Implement per-video pipeline in `internal/app/actions/detect_run.go`: frame reader → detect → embed (conf ≥0.5, height ≥64 px, every 2nd sample per track) → tracker → finalize tracks (mean embedding, best crop saved as JPEG under the app data dir `people/<workspace>/<track>.jpg`)
- [X] T020 [US1] Add the worker pool in `internal/app/actions/detect_run.go`: bounded workers (default `max(2, NumCPU/3)`), shared session pool, skip videos whose `video_analyses` key (sample_fps, model_version, file size, mtime) matches, per-video failure isolated and recorded, cancellation preserving finished videos, progress events
- [X] T021 [US1] Wire `StartDetection`, `CancelDetection`, `GetDetectionJob` in `internal/api/wails/service.go` and `internal/api/wails/bridge.go`; construct the engine and action in `internal/bootstrap/bootstrap.go`
- [X] T022 [P] [US1] Add Zod models in `frontend/src/entities/person/model.ts` and backend client methods in `frontend/src/infrastructure/backend/` (`backendClient.ts`, `types.ts`, `wailsBackendClient.ts`)
- [X] T023 [US1] Build `DetectButton` and `DetectProgress` (per-video + overall progress, cancel, completion summary, empty state when no people) in `frontend/src/features/person-detection/` and mount them in `frontend/src/pages/WorkspaceDetailPage.tsx`

**Checkpoint**: detection runs end to end and results persist.

---

## Phase 4: User Story 2 - Browse people and select tracks (P1)

**Goal**: gallery of cards with filters, preview, selection, merge/split/remove.
**Independent Test**: with stored results, open gallery, filter, preview, tick cards, merge two.

- [X] T024 [US2] Implement `internal/app/actions/person_list.go`: list groups with `minDurationSec`/`minConfidence` filters and pagination, plus merge (move tracks into one group), split (track into new group), remove (`removed`=1, delete empty group)
- [X] T025 [US2] Wire `ListPeople`, `MergePeople`, `SplitPerson`, `RemoveTrack` in `internal/api/wails/service.go`; thumbnails returned as data URLs in `PersonGroupDTO`
- [X] T026 [P] [US2] Build `PersonCard` and `PersonGrid` (thumbnail, time range, appearance count, video count, checkbox, selection count) in `frontend/src/features/person-gallery/`
- [X] T027 [P] [US2] Build duration/confidence filter controls and the gallery tab/section on `frontend/src/pages/WorkspaceDetailPage.tsx`
- [X] T028 [US2] Build the preview player that opens the source video at a track's start time (reuse existing video preview path from `internal/app/actions/video_preview.go`) in `frontend/src/features/person-gallery/PersonPreview.tsx`
- [X] T029 [US2] Add merge / split / remove actions to the cards with query invalidation in `frontend/src/features/person-gallery/`

**Checkpoint**: user can browse and curate people.

---

## Phase 5: User Story 3 - Cut clips for selected people (P1)

**Goal**: one clip per selected group, padded, merged across videos, clamped.
**Independent Test**: select two groups, cut with 3 s padding, verify two clips with matching timing.

- [X] T030 [US3] Implement `internal/app/actions/person_cut.go`: map each group's tracks to `[start−before, end+after]` clamped to video bounds, order by video `started_at` (fallback filename), feed existing `CutSegment` + `MergeConcat` with bounded parallelism; add a test for clamping and ordering in `internal/app/actions/person_cut_test.go`
- [X] T031 [US3] Wire `StartPeopleCut` in `internal/api/wails/service.go`, reusing the cut job manager and `cut:progress`/`cut:finished`/`cut:failed` events
- [X] T032 [US3] Build `CutSelectedModal` (folder name, seconds before/after defaulting to the existing cut flow, progress via `frontend/src/features/cut-progress`) in `frontend/src/features/person-gallery/CutSelectedModal.tsx` and add the **Cut selected** button to the gallery

**Checkpoint**: full flow import → detect → select → cut works.

---

## Phase 6: User Story 4 - Tune detection settings (P2)

**Goal**: adjustable sampling rate stored with results.
**Independent Test**: run at 1 fps and 10 fps; changing the rate forces re-analysis.

- [X] T033 [US4] Validate `sampleFps` in `internal/app/actions/detect_run.go` (reject outside 1–15 with `VALIDATION_ERROR`, default 5) and include it in the skip key
- [X] T034 [US4] Add an advanced section with an fps input (1–15, default 5) to `frontend/src/features/person-detection/DetectButton.tsx`

---

## Phase 7: Polish & Cross-Cutting

- [X] T035 [P] Add `slog` logging for decode, inference timing, per-video summaries and failures in `internal/app/actions/detect_run.go` and `internal/inference/ort.go`
- [X] T036 [P] Add a gated smoke test `internal/inference/smoke_test.go` (skips if models are missing): ≥1 person on a fixture frame, 512-d unit-norm embedding
- [X] T037 Run the throughput check from `quickstart.md` step 3 on a 10-minute sample; record fps and CoreML-vs-CPU in `specs/001-detect-people-tracks/research.md`, and adjust default workers/fps if SC-002 is missed
- [X] T038 Run `go test ./...` and `cd frontend && npm run build`; fix failures
- [X] T039 Walk through `quickstart.md` steps 5–7 in `wails dev` and fix gaps; update `README.md` features and structure sections
- [X] T040 Make `ReplaceWorkspaceVideos` in `internal/repository/sqlite/video_repo.go` upsert by path and delete only vanished videos, so re-importing a folder keeps video IDs and therefore detection results (found during implementation; test in `video_repo_test.go`)

---

## Dependencies & Execution Order

- Phase 1 → Phase 2 → stories. US1 must finish before US2/US3 (they read its results). US2 and US3 backend work (T024, T030) can proceed in parallel after US1; US3's UI needs the gallery from US2 (T032 after T026). US4 touches US1 files, so it follows US1.
- Within US1: T014–T017 parallel → T018 → T019 → T020 → T021 → T023 (T022 parallel with backend work).

## Parallel Example (US1)

T014, T015, T016, T017 and T022 can run together (different files).

## Implementation Strategy

- **MVP**: Phases 1–3 (US1) proves the riskiest part (inference speed, tracker quality) early; run T037 right after T020 if possible.
- Then US2 → US3 for the full user flow, US4 last.
