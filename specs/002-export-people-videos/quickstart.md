# Quickstart: Export People Videos

Proves the export gate and the people-gallery flow. Contracts: [contracts/wails-api.md](contracts/wails-api.md). Fields: [data-model.md](data-model.md).

## 1. Unit tests

From the repo root:

```bash
go test ./internal/app/actions/ ./internal/app/jobs/
```

Expected:

- Budget uses merged, clamped ranges (an overlap is not billed twice).
- `operationBytes = (expected × 3 / 2) + (largest × 3 / 2)`.
- Labels: `100_000_000` bytes → `0.1 GB`; one byte under that → whole MB.
- Execute writes `person-<id>.mp4` into the directory given on the request, not `<workspace>/exports/<name>/`.
- An existing `person-<id>.mp4` or a free-space figure below `operationBytes` returns an error and creates no scratch directory.
- Cancel on a running cut job sets `CANCELLED` and does not set `FAILED`.

`npm run build` in `frontend` must pass after the dialog change.

## 2. Dialog (wails dev)

Prerequisite: a workspace that already has people detected.

1. Open the people tab, tick two people, press **Export**. There is no **Cut selected** button.
2. With no folder yet, both size figures are visible and Export cannot be confirmed.
3. Raise seconds-before. Both figures update before confirm is possible.
4. Pick a folder on a drive with plenty of space. Free space is shown. Confirm writes `person-<id>.mp4` for each exportable person and opens that folder. Scratch directories are gone.
5. Export the same people to the same folder again. Confirm stays blocked and the existing files are unchanged.
6. Point the folder at a volume whose free space is below the operation figure (or simulate it in the unit test in step 1). Confirm stays blocked and the folder gains no new file.

## 3. Cancel and failure

1. Start an export of several people and cancel while the first is running. New work stops within 5 seconds. Finished files, if any, remain. No `.work-person-*` directory remains. The dialog says the export was cancelled, not that the cut failed.
2. Remove a source file for a later person and export. People already written stay. The error names the person who failed. Later people are not started.

## 4. Long video (manual)

Not part of `go test`. Use a source longer than two hours and a one-minute clip of the same picture size.

- Activity Monitor: the long export's memory stays within twice the one-minute export.
- The size figures for a selection whose planned time totals about 10 hours appear within 3 seconds and the video does not start playing.
- When the clip reuses the original footage, the finished file is within 15% of the expected size shown beforehand. If a clip was recreated and lands far above the 1.5 allowance, that is the known gap in [research.md](research.md); raise the factors in the spec before changing the formula.
