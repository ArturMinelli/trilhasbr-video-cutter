# Wails API Contract

People export replaces `StartPeopleCut`. Manual `StartCut` and the spreadsheet cut methods are unchanged.

Responses use the existing `{ ..., error? }` envelope. Byte fields are JSON numbers (int64). Labels are display-only.

| Method | Input | Output |
|---|---|---|
| `PickExportDirectory` | none | `{ path?, cancelled?, error? }`. Native folder dialog titled "Select export folder". `cancelled: true` and an empty path when the user dismisses it. |
| `PreviewPeopleExport` | `{ workspaceId, groupIds, outputDirectory?, secondsBefore, secondsAfter }` | `{ expectedBytes, operationBytes, expectedLabel, operationLabel, freeBytes?, freeLabel?, exportableCount, skippedCount, collisions?, blockers?, canExport, error? }` |
| `StartPeopleExport` | same shape; `outputDirectory` required | `{ jobId?, error? }`. Re-runs the preview gate. On failure returns `error` and creates no job and no file. On success, events below. |
| `CancelPeopleExport` | `jobId` string | `ActionResponse`. False / validation error when the job is unknown or already finished. |

`StartPeopleCut` is removed from `App` and the service.

## Preview fields

- `outputDirectory` empty: sizes and counts are filled, `freeBytes` is omitted, `canExport` is false, `blockers` includes that a folder is required.
- `collisions`: base names such as `person-12.mp4`. Any entry forces `canExport` false.
- `blockers`: short sentences the dialog can show (no folder, not a directory, not writable, name already present, not enough free space, nothing to export, a source size could not be read).
- `canExport` is the gate. The UI must not decide by comparing labels.
- Missing group id: `error` with `VALIDATION_ERROR`, `canExport` false. Same code the old cut used ("one or more selected people no longer exist").
- Padding outside 0…86,400, empty `groupIds`, or `workspaceId <= 0`: `VALIDATION_ERROR`.

`expectedLabel` / `operationLabel` / `freeLabel`: `0.1 GB` and up as one decimal; below that, whole megabytes. 1 GB = 1,000,000,000 bytes.

## Events

Existing, still emitted for this job:

- `cut:progress` `{ jobId, totalSegments, completedSegments, currentFile, currentPerson? }`
- `cut:finished` `{ jobId, result }` where `result.outputDirectory` is the chosen folder and `result.perSourceOutputs` lists each `person-<id>.mp4`
- `cut:failed` `{ jobId, error }`. `error.message` names the person (group label, or `person-<id>` when unlabeled). `error.details.groupId` is set.

New:

- `cut:cancelled` `{ jobId, outputDirectory, finishedCount }`

Cancel does not also emit `cut:failed`. Manual cuts do not emit `cut:cancelled`.

## Job status

`GetCutJob` may return `status: "CANCELLED"` for this job. Other cut types do not produce that status.

## What the dialog does

1. On open, and whenever padding changes, call `PreviewPeopleExport` (folder may be empty).
2. Folder button calls `PickExportDirectory`, then preview again with that path.
3. Confirm calls `StartPeopleExport` only when the latest preview has `canExport`. The modal stays open.
4. Subscribe to `cut:progress` for this `jobId` and show completed/total and `currentPerson`.
5. Cancel calls `CancelPeopleExport`.
6. On `cut:finished`, close and open the folder. On `cut:failed`, show `error.message` and leave finished files. On `cut:cancelled`, say it was cancelled and how many files finished.

Do not subscribe this job to the workspace page's generic cut watcher (that watcher toasts "Cut finished" / "Cut failed" and cannot cancel).
