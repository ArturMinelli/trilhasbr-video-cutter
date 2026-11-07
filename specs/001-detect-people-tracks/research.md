# Research: Detect People and Cut by Track

## Decisions

- **Inference runtime**: ONNX Runtime via `onnxruntime_go` (cgo), CoreML execution provider with CPU fallback. *Rationale*: user decision; the models are already available as ONNX, so no re-export is needed. *Alternatives*: sidecar binary (extra artifact to sign), pure-Go runtime (too slow).
- **Platforms (v1)**: macOS arm64 only. *Rationale*: user decision; wrapper sits behind `inference.Engine` so Windows (DirectML) / Linux can be added later.
- **Frame decode**: bundled FFmpeg subprocess `-vf fps=N -f rawvideo -pix_fmt rgb24 -` piped to Go; letterbox done in Go. *Rationale*: reuses existing binary resolution; no new dep. Frame time = index/fps (+ start offset), exact enough at 5 fps. *Alternative*: libav bindings (heavy), temp JPEGs (I/O heavy).
- **Detector**: `person-yolo11n.onnx`, fixed 1280×1280 input, output 1×84×33600 (4 box + 80 classes); keep class 0 (person), confidence ≥0.35, NMS IoU 0.5. The thresholds are constants in one file and can be tuned.
- **Re-ID**: `osnet_x1_0_msmt17.onnx`, 256×128 crops, 512-d, L2-normalized, batched per frame. Embeddings computed for confident detections only (conf ≥0.5, box height ≥64 px), at most every 2nd sampled frame per track, to save time.
- **Tracker**: ByteTrack-style (high/low confidence two-stage IoU association, Kalman constant-velocity) with a blended cost = IoU cost + cosine embedding cost for lost-track recovery; hand-written Hungarian. Track kept lost for ~2 s of video time. No reference tracker source was available, so it is rebuilt from the published algorithm; parameters are tunable constants.
- **Parallelism**: worker pool over videos (bounded, default max(2, NumCPU/3)); a shared pool of ORT sessions (default 2) feeds inference so memory stays bounded; FFmpeg decode runs ahead through a small channel buffer.
- **Persistence**: per `(video, sample_fps, model_version)` run record; video re-analyzed only if file mtime/size or settings change (FR-007, FR-014). Mean track embedding stored as BLOB for a future automatic cross-video clustering feature.
- **Crops/thumbnails**: best-scoring crop per track saved as JPEG in `<appdata>/people/<workspace>/<track>.jpg`; served to the UI via Wails asset path or base64 DTO.
- **Cut**: a person group's tracks are mapped to `[start - before, end + after]` per video (clamped), then run through existing `CutSegment` + `MergeConcat`; ordered by video `started_at`, falling back to filename.
- **Model/ORT distribution**: `resources/models/` and `resources/onnxruntime/<goos>-<goarch>/`, resolved bundled-first like FFmpeg; sha256 manifest verified at startup of the first run (`MODEL_NOT_FOUND` otherwise).
- **Open risk**: tracker quality and CoreML op coverage for YOLO at 1280 must be measured early (quickstart step 3); if CoreML falls back to CPU, throughput targets (SC-002) may need a lower sampling default.

## Measured throughput (2026-11-07, Apple Silicon, 7 portrait 480x854 videos, 423 s total)

Provider: YOLO on CoreML, OSNet on CPU (CoreML recompiles OSNet per batch size). Run with `DETECT_E2E_DIR=<folder> go test ./internal/app/actions -run E2E -v`.

| Setting | Wall | Speed | 1 h of footage | Tracks |
|---|---|---|---|---|
| 5 fps, 3 workers (default), 4 runs | 49.6-63.6 s | 6.7-8.5x realtime | 7.0-9.0 min | 62 |
| 2 fps | 32 s | 13.2x | 4.5 min | 43 |
| 10 fps | 133 s | 3.2x | 18.9 min | 70 |
| 5 fps, 1 worker | 72 s | 5.9x | 10.2 min | 62 |
| 5 fps, 6 workers | 61 s | 6.9x | 8.7 min | 62 |

- SC-002 (1 h in <15 min) is met at the 5 fps default, missed at 10 fps.
- Workers 1 -> 3/6 gain only ~15%: inference (shared sessions) is the bottleneck, not decode. Worth investigating before raising the worker default.
- SC-005: re-run on unchanged workspace skipped 7/7 videos in ~1 ms.
- Extrapolations come from ~7 min of 480x854 footage; 1080p input was not measured.
