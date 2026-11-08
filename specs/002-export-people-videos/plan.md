# Implementation Plan: Export People Videos

**Branch**: `002-export-people-videos` | **Date**: 2026-11-07 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/002-export-people-videos/spec.md`

## Summary

Replace the people gallery **Cut selected** action with **Export**. The user picks any folder; the app writes one `person-<id>.mp4` per person into that folder, still using the existing per-person cut and merge. Before anything is written, a preview computes the expected output from source file size × planned duration and an operation disk cost of `1.5×` that total plus `1.5×` the largest person. Export is refused when the chosen drive is smaller than that cost or when a target file name already exists. People stay one-at-a-time, temporary files stay inside one scratch directory that is removed before the next person, and cancel kills the running ffmpeg process through the context it already listens to.

## Technical Context

**Language/Version**: Go 1.25 (existing module), TypeScript/React (existing)

**Primary Dependencies**: existing Wails v2, SQLite, bundled FFmpeg. No new module. Free space and write permission come from the standard-library `syscall` package (`Statfs`, `Access`).

**Storage**: no schema change. Finished files are `person-<id>.mp4` in the user-chosen folder. Scratch for the person being written stays a hidden directory inside that folder and is deleted when that person finishes, fails, or is cancelled.

**Testing**: `go test ./internal/app/actions/ ./internal/app/jobs/` for the byte budget, the gate, collision, and cancel. `npm run build` in `frontend` when the dialog changes. Long-video memory and the 15% size check are manual (see [quickstart.md](quickstart.md)); CI does not need a multi-hour fixture.

**Target Platform**: the current Wails desktop app (macOS). `syscall.Statfs` and `syscall.Access` are the darwin path this app ships.

**Project Type**: desktop-app (Wails, Go backend + React frontend)

**Performance Goals**: size figures for a 10-hour selection within 3 seconds, with no video decode (SC-007). Cancel stops new ffmpeg work within 5 seconds (SC-008). Memory for a source longer than 2 hours stays within twice a 1-minute export of the same picture size (SC-006).

**Constraints**: offline; people exported strictly one at a time; segment parallelism inside one person stays the existing cap (`max(1, min(4, NumCPU-1))`); do not read source media into the Go process; comparison of free space uses exact byte counts, not the rounded label; no overwrite and no "export anyway".

**Scale/Scope**: one gallery action, one dialog, a mass selection of people (tens to low hundreds) drawn from videos that may be hours long.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|---|---|---|
| I Local-first | PASS | Folder, estimate, and cuts stay on the machine. No network. |
| II Boundaries | PASS | Budget and gate live in `internal/app/actions`. Wails service maps DTOs and events only. UI stays in `frontend/src/features/person-gallery`. |
| III Data integrity | PASS | No migration. Missing people, collisions, a short drive, and a failed person return the existing typed errors. Finished files are not deleted on a later failure. |
| IV Media safety | PASS | Same bundled FFmpeg path and the same bounded segment parallelism. Progress reuses `cut:progress`. Cancel is a context cancel, which `exec.CommandContext` already honors. |
| V Minimal change | PASS | No new dependency. The cutter is not rewritten. `StartPeopleCut`'s `name` → workspace-exports folder is removed rather than kept beside Export. |

Re-check after design: unchanged, PASS. The disk formula and the cancel event are behavior of the existing cut loop, not a second pipeline.

## Project Structure

### Documentation (this feature)

```text
specs/002-export-people-videos/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/wails-api.md
└── checklists/requirements.md
```

### Source Code (repository root)

```text
internal/app/actions/person_cut.go          # EXTEND: output directory, budget, gate, sequential cleanup
internal/app/actions/person_cut_test.go     # EXTEND: budget, collision, short drive, chosen folder
internal/app/jobs/manager.go                # EXTEND: cancel func + CANCELLED, same shape as detection jobs
internal/api/wails/people.go                # REPLACE StartPeopleCut with preview + start + cancel
internal/api/wails/bridge.go                # EXTEND: export-folder dialog title; cut:cancelled event
app.go                                      # EXTEND: bind the three methods, drop StartPeopleCut
frontend/src/features/person-gallery/       # REPLACE Cut selected dialog with Export
frontend/src/infrastructure/backend/        # EXTEND: client methods and event types
frontend/src/pages/WorkspaceDetailPage.tsx  # EXTEND: people export no longer shares the generic cut toast
```

**Structure Decision**: keep the people-cut action as the only writer. Preview and start both call one gate function so the dialog cannot start a job the action would reject. Manual cuts and spreadsheet cuts do not take the new request shape.

## Complexity Tracking

No constitution violations.
