# Data Model

No new tables and no migration. An export is a request, a byte budget, and files in a folder the user already chose.

## Export request

In memory only. Not stored.

| Field | Type | Rules |
|---|---|---|
| workspaceId | int64 | > 0, must exist |
| groupIds | int64[] | non-empty; every id must still be a group in that workspace at preview and again at start |
| outputDirectory | string | absolute, cleaned path; must be an existing writable directory. Empty during the first preview, before the user picks a folder |
| secondsBefore, secondsAfter | int | 0 … 86,400. Default 20 and 20 |

The old `name` field (a folder under `<workspace>/exports/`) is not part of this request.

## Person clip plan

Built by the existing `BuildPersonCutPlan`. One plan per group that has at least one clamped, non-empty range.

| Field | Type | Rules |
|---|---|---|
| groupId | int64 | file name is `person-<groupId>.mp4` in `outputDirectory`. The group label is display-only |
| segments | segment[] | merged and clamped; ordered by video start time, then path, then start offset |
| segment.videoId, videoPath, videoDurationSec | | duration must be > 0 or the segment is absent |
| segment.startSec, durationSec | float | duration > 0, inside the video |

Groups with no segments are skipped. They do not get a file and do not add bytes. If every selected group is skipped, the export cannot start.

## Disk budget

Computed from the plans. Not stored.

| Field | Type | Rules |
|---|---|---|
| expectedBytes | int64 | sum over segments of `round(fileSize × duration / videoDuration)`. File size is `os.Stat` of the source, taken at calculation time |
| largestPersonBytes | int64 | max of the per-person expected bytes. 0 when nobody is exportable |
| operationBytes | int64 | `(expectedBytes × 3 / 2) + (largestPersonBytes × 3 / 2)` |
| freeBytes | int64 | `Statfs` available bytes on `outputDirectory`. Absent until a folder is chosen |
| collisions | string[] | base names `person-<id>.mp4` that already exist in the folder |

**Can start** only when all of these hold: a folder is set, it is a writable directory, at least one person is exportable, `collisions` is empty, every selected group still exists, every planned source could be stat'd, and `freeBytes >= operationBytes`.

The rounded label (≥ 0.1 GB as one decimal gigabyte, otherwise whole megabytes, decimal 10^9) is derived from the int64. It is not an input to the comparison.

## Files on disk

| Path | Lifetime |
|---|---|
| `<outputDirectory>/person-<id>.mp4` | finished clip. Left in place on later failure or cancel. Never overwritten; its presence blocks a new export of that id |
| `<outputDirectory>/.work-person-<id>/` | scratch for the person currently being written (segment files, then the merged file). At most one of these exists. Removed before the next person starts, and removed on failure or cancel |

Other files already in the folder are not read or deleted.

## Job

The existing in-memory cut job. One extra status and a cancel function that is not serialized.

| Status | Meaning |
|---|---|
| PENDING / RUNNING | unchanged |
| SUCCEEDED | every exportable person was written |
| FAILED | a person failed, or the gate failed before the job was created. Finished files stay. The error names that person |
| CANCELLED | the user cancelled. Finished files stay. Scratch for the in-progress person is gone. People not started are not started |

Manual cuts and spreadsheet cuts do not set CANCELLED and do not emit `cut:cancelled`.

## State

```text
dialog open
  → preview (no folder) → sizes shown, start blocked
  → folder chosen or padding changed → preview again
  → gate fails → start stays blocked, nothing is written
  → gate passes → job RUNNING
       → each person: scratch → person-<id>.mp4 → scratch deleted
       → SUCCEEDED | FAILED | CANCELLED
```

A second preview or a start that finds a missing group does not write files. The user refreshes the gallery and previews again.
