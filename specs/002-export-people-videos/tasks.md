---
description: "Task list for Export People Videos"
---

# Tasks: Export People Videos

**Input**: Design documents from `/specs/002-export-people-videos/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/wails-api.md, quickstart.md

**Tests**: Not TDD. The byte budget, the gate, collision, and cancel each land with a `go test` in the same task, because plan.md and quickstart.md require those checks. No new test framework.

**Organization**: One phase per user story. All three stories are P1. US2 and the rest of US3 edit the same files as US1, so they follow US1. The job-manager cancel task does not.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: different files, no dependency on an incomplete task
- **[Story]**: [US1], [US2], [US3] on story phases only
- Paths are relative to the repo root

## Path Conventions

- Backend: `internal/app/actions/`, `internal/app/jobs/`, `internal/api/wails/`, `app.go`
- Frontend: `frontend/src/features/person-gallery/`, `frontend/src/infrastructure/backend/`, `frontend/wailsjs/`

---

## Phase 1: Setup

No tasks. plan.md adds no dependency and no new package. Free space and the write check use the standard library `syscall` package. Do not add a direct require on `golang.org/x/sys`.

---

## Phase 2: Foundational (blocks all user stories)

**Purpose**: the cancel event the later phases emit. No story writes files yet.

**Critical**: finish this before the story phases. `RuntimeEventBridge` will not compile until every implementation has the new method.

- [ ] T001 Add `CurrentPerson string \`json:"currentPerson,omitempty"\`` on `CutProgressEvent` and add `CutCancelledEvent` with `JobID`, `OutputDirectory`, `FinishedCount` in `internal/app/actions/interfaces.go`. Add `PublishCancelled(ctx, CutCancelledEvent)` to `CutEventPublisher` in that file and to `RuntimeEventBridge` in `internal/api/wails/service.go`. Emit `cut:cancelled` from `internal/api/wails/bridge.go`. Add the method to the stub in `internal/api/wails/service_csv_test.go`.

**Checkpoint**: existing cuts still compile. Nothing emits `cut:cancelled` yet.

---

## Phase 3: User Story 1 - Export selected people to a chosen folder (Priority: P1) MVP

**Goal**: **Export** replaces **Cut selected**. The user picks a folder and gets one `person-<id>.mp4` per person in that folder, not under `<workspace>/exports/<name>/`.

**Independent Test**: Select two people, pick an empty folder, leave padding at 20 and 20, export, and confirm exactly those two files are in the chosen folder.

- [ ] T002 [US1] In `internal/app/actions/person_cut.go`, replace `StartPeopleCutInput.Name` with `OutputDirectory`. `Execute` requires `workspaceId > 0`, a non-empty `groupIds`, `secondsBefore` and `secondsAfter` each in `0 … 86,400`, and `OutputDirectory` cleaned with `filepath.Abs` to an existing directory. Write `person-<id>.mp4` directly in that directory (no generated subfolder, do not create the directory). Keep one person at a time and remove `<outputDirectory>/.work-person-<id>/` before the next person starts. Do not create that scratch directory until the input validation above has passed. Update callers in `internal/app/actions/person_cut_test.go` and `internal/app/actions/detect_e2e_test.go` so the test expects the given directory, not `<workspace>/exports/<name>/`.

- [ ] T003 [US1] Replace `StartPeopleCut` with `StartPeopleExport` in `internal/api/wails/people.go` and `app.go`. Request fields: `workspaceId`, `groupIds`, `outputDirectory`, `secondsBefore`, `secondsAfter`. Delete `StartPeopleCut`. On validation failure return the error and do not create a job. Add `PickExportDirectory` on `internal/api/wails/bridge.go` and `app.go`: native folder dialog titled `Select export folder`, response `{ path?, cancelled?, error? }`, `cancelled: true` and an empty path when dismissed.

- [ ] T004 [P] [US1] Replace the people-cut client with `pickExportDirectory`, `startPeopleExport`, and a `cut:cancelled` subscription in `frontend/src/infrastructure/backend/types.ts`, `frontend/src/infrastructure/backend/backendClient.ts`, `frontend/src/infrastructure/backend/wailsBackendClient.ts`, `frontend/wailsjs/go/main/App.d.ts`, `frontend/wailsjs/go/main/App.js`, and `frontend/wailsjs/go/models.ts`. Remove `StartPeopleCut` from those files.

- [ ] T005 [US1] Replace `frontend/src/features/person-gallery/CutSelectedModal.tsx` with an export dialog and point `frontend/src/features/person-gallery/PeopleGallery.tsx` at it. The gallery button label is **Export**. The dialog picks a folder via `pickExportDirectory` and edits seconds before and after (default 20 and 20, whole numbers from 0 through 86,400). Confirm calls `startPeopleExport`. Remove **Cut selected**.

**Checkpoint**: two selected people produce two files in the folder the user picked. Manual cut and spreadsheet cut are unchanged.

---

## Phase 4: User Story 2 - See the size and stop when the disk cannot hold it (Priority: P1)

**Goal**: the dialog shows expected output and operation disk cost, and Export cannot start when the drive is short, a target name exists, or nobody has footage.

**Independent Test**: Open Export, read both figures, raise the padding and confirm both increase, then aim at a drive with less free space than the operation cost and confirm Export cannot be confirmed and no file is created.

- [ ] T006 [US2] Add the budget and the gate in `internal/app/actions/person_cut.go`, tested from `internal/app/actions/person_cut_test.go`. Expected bytes are the sum over planned segments of `round(fileSize × segmentDuration / videoDuration)`, using `BuildPersonCutPlan` so overlaps are billed once and padding is already clamped. `operationBytes = (expectedBytes × 3 / 2) + (largestPersonBytes × 3 / 2)` in integer division. File size is `os.Stat` of the source; if stat fails, the gate blocks (do not estimate 0). Free space is `syscall.Statfs` `Bavail × Bsize` on the destination. Writable is `syscall.Access(path, W_OK)` with no probe file. Block only when `freeBytes < operationBytes` (equal is allowed). Collisions are existing `person-<id>.mp4` base names for people who would be written; any hit blocks the whole export; other names in the folder do not. Labels: `1 GB = 1,000,000,000` bytes; values `>= 100,000,000` bytes render as one decimal GB, smaller values as whole MB. `canExport` is false when the folder is empty, not a directory, not writable, a collision exists, every selected person is skipped, a source cannot be stat'd, a selected group no longer exists, or free space is below the operation cost. An empty `outputDirectory` still returns sizes and counts, omits `freeBytes`, and includes a blocker that a folder is required.

- [ ] T007 [US2] Add `PreviewPeopleExport` in `internal/api/wails/people.go` and `app.go` with the response fields in `contracts/wails-api.md` (`expectedBytes`, `operationBytes`, labels, `freeBytes?`, `freeLabel?`, `exportableCount`, `skippedCount`, `collisions?`, `blockers?`, `canExport`, `error?`). `StartPeopleExport` runs that same gate again before it creates a job or a scratch directory. A gate failure returns `error` and creates neither. A missing group returns `VALIDATION_ERROR` with the message `one or more selected people no longer exist`.

- [ ] T008 [US2] Show `expectedLabel`, `operationLabel`, `freeLabel` once a folder is chosen, `exportableCount`, `skippedCount`, and `blockers` in `frontend/src/features/person-gallery/ExportModal.tsx`. Call `PreviewPeopleExport` on open and again whenever padding or the folder changes, before confirm is possible. Confirm stays disabled unless the latest preview has `canExport`. Do not compare the rounded labels in the UI.

**Checkpoint**: a short drive or an existing `person-<id>.mp4` refuses the export before any destination file or scratch directory appears.

---

## Phase 5: User Story 3 - Finish long exports without exhausting the computer (Priority: P1)

**Goal**: cancel and a failed person keep finished files, drop the in-progress scratch directory, and leave memory independent of video length.

**Independent Test**: Cancel mid-export: new work stops within 5 seconds, finished `person-<id>.mp4` files remain, `.work-person-*` is gone, and the dialog says cancelled rather than failed. A missing source later in the selection names that person and does not start the people after them.

- [ ] T009 [P] [US3] In `internal/app/jobs/manager.go`, store `context.CancelFunc` on `cutJob` (not on `Snapshot`). Add `SetCancel`, `Cancel` (false when the job is unknown or already finished), and `SetCancelled` with status `CANCELLED`. `Create` stays valid for manual cuts that never set a cancel func. Cover cancel-vs-finished in `internal/app/jobs/manager_test.go`.

- [ ] T010 [US3] In `internal/app/actions/person_cut.go`, run `Execute` on the given context. On `context.Canceled`, delete the current `.work-person-<id>/`, keep finished files already moved, return the partial `CutResult` (finished paths included) with the cancel error, and do not start later people. On a person failure, do the same cleanup and return a typed error whose message is the group label when it is non-empty, otherwise `person-<id>`, with `details.groupId` set. Set `CutProgressEvent.CurrentPerson` to that same name. The estimate path must not read source bytes and must not run ffprobe; size stays `os.Stat` plus known durations. People stay a sequential loop. Do not change the segment parallelism cap in `internal/app/actions/cut_execute.go`.

- [ ] T011 [US3] In `internal/api/wails/people.go` and `app.go`, start the export with `context.WithCancel`, store the cancel func with `SetCancel`, and add `CancelPeopleExport(jobId)`. On cancel, `SetCancelled` and `PublishCancelled` (`jobId`, `outputDirectory`, `finishedCount`). Do not also emit `cut:failed`. On a person failure, `SetFailed` and `cut:failed` with the named error. `GetCutJob` returns `status: "CANCELLED"` only for this outcome.

- [ ] T012 [US3] Keep the dialog in `frontend/src/features/person-gallery/ExportModal.tsx` open after start. Show `completedSegments` / `totalSegments` and `currentPerson` from `cut:progress`. Cancel calls `CancelPeopleExport`. On `cut:cancelled`, say it was cancelled and how many files finished. On `cut:finished`, close and open the folder. On `cut:failed`, show `error.message`. In `frontend/src/features/person-gallery/PeopleGallery.tsx`, stop sending this job to `onCutStarted` on `frontend/src/pages/WorkspaceDetailPage.tsx`, so the generic watcher does not toast "Cut failed" for a cancel.

**Checkpoint**: cancel and a mid-list failure leave finished files, no scratch directory, and a message that matches the outcome.

---

## Phase 6: Polish

**Purpose**: prove the three stories together without touching manual or spreadsheet cuts.

- [ ] T013 Run `go test ./internal/app/actions/ ./internal/app/jobs/` and `npm run build` in `frontend`. Fix failures.
- [ ] T014 [P] Confirm `StartCut` and the spreadsheet cut methods in `internal/api/wails/service.go` are unchanged, and that `go.mod` has no new require.
- [ ] T015 Walk through `specs/002-export-people-videos/quickstart.md` sections 2 and 3 in `wails dev`. Section 4 (long video, memory within 2× a one-minute export, figures within 3 seconds, finished size within 15% when the original footage is reused) stays manual.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup**: nothing to do.
- **Foundational (T001)**: blocks every story. The bridge interface will not build until the stub is updated.
- **US1 (T002–T005)**: after T001. This is the first increment that writes files.
- **US2 (T006–T008)**: after US1. T006 and T007 edit `person_cut.go`, `people.go`, and `app.go`, which T002 and T003 already own.
- **US3 (T009–T012)**: T009 after T001 only. T010 after T002. T011 after T003, T009, and T010. T012 after T005 and T011.
- **Polish**: after the stories you intend to ship.

### User Story Dependencies

- **US1**: no other story. MVP.
- **US2**: needs US1's directory write path so the gate can refuse before scratch is created. Independently testable by refusing an export without waiting for a long encode.
- **US3**: T009 is independent of US1 and US2. The cancel behavior inside `Execute` needs US1's loop. The dialog needs US1's modal and US2's confirm flow if both are present; progress and cancel do not need the size figures to be correct.

### Parallel Opportunities

- T004 (frontend client and `wailsjs`) can run with T003 (Go methods). They meet at T005.
- T009 (`internal/app/jobs/manager.go`) can run with US1 and US2. It shares no file with them.
- T014 can run with T013.

---

## Parallel Example: User Story 1

```text
T003  StartPeopleExport and PickExportDirectory in internal/api/wails/people.go, bridge.go, and app.go
T004  Client and wailsjs bindings in frontend/src/infrastructure/backend/ and frontend/wailsjs/
```

Then T005, which imports the client.

## Parallel Example: User Story 3

```text
T009  Cancel on internal/app/jobs/manager.go
```

can run while US1 or US2 is in progress. T010 waits until T002 has landed.

---

## Implementation Strategy

### MVP First (User Story 1)

1. T001
2. T002–T005
3. Stop and export two people into a chosen folder.

US2 should land before a large real export. US1 alone can fill a drive.

### Incremental Delivery

1. US1 delivers the folder and one file per person.
2. US2 adds the figures and the refusal.
3. US3 adds cancel, named failure, and the memory bound (no source bytes in the Go process, people one at a time).
4. T013–T015 confirm the quickstart.

### Parallel Team Strategy

After T001: one person takes T002–T003, another takes T004, another takes T009. US2 starts when T002 and T003 are done. T012 starts when the modal and `CancelPeopleExport` both exist.

---

## Notes

- Do not leave `StartPeopleCut` beside `StartPeopleExport`.
- Do not overwrite an existing `person-<id>.mp4`, and do not offer "export anyway".
- Manual cuts must not emit `cut:cancelled`.
