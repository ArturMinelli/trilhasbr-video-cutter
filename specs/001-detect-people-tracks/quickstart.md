# Quickstart / Validation

Prerequisites: macOS arm64, Go 1.24+, Node, models and `libonnxruntime.dylib` placed under `resources/` (see `resources/models/manifest.json`).

1. **Unit tests**: `go test ./...` — tracker, YOLO decode/NMS, group→cut-plan tests pass without models.
2. **Inference smoke test** (needs models): `go test ./internal/inference -run TestSmoke` — detects ≥1 person in a fixture frame and returns a 512-d unit vector.
3. **Throughput check**: run detection on a 10-minute sample at 5 fps; record fps and whether CoreML or CPU was used (target: whole hour in <15 min, SC-002).
4. **Frontend**: `cd frontend && npm run build`.
5. **End to end** (`wails dev`): import a folder → **Detect people** → watch progress → app switches to the **People** tab with cards → preview one → tick two → **Cut selected** (set before/after to 3 s if you want the quick check; default is 20 s) → two clips in the output folder; timing matches the track range.
6. **Re-run**: press Detect people again — completes in <10 s with all videos skipped; change fps → videos re-analyzed.
7. **Cancel / failure**: cancel mid-run (finished videos keep results); rename a model file → start detection fails with a toast whose body includes `MODEL_NOT_FOUND:`.
