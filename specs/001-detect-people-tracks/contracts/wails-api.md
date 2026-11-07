# Wails API Contract (new methods on `Service`)

Responses use the existing `{ ..., error? }` envelope (field names below are the wired shapes).

| Method | Input | Output |
|---|---|---|
| `StartDetection` | `{ workspaceId, sampleFps? }` (0/absent = 5; else 1–15) | `{ jobId?, error? }` |
| `CancelDetection` | `{ jobId }` | `ActionResponse` |
| `GetDetectionJob` | `jobId` | `{ job?: {jobId, status, videosTotal, videosDone, currentVideo, tracksFound, error?}, error? }` |
| `ListPeople` | `{ workspaceId, minDurationSec?, minConfidence?, page, pageSize }` | `{ items, total, error? }` |
| `MergePeople` | `groupIds: number[]` | `{ group?: { id }, error? }` (id-only group) |
| `SplitPerson` | `groupId, trackId` | `{ group?: { id }, error? }` (id-only group) |
| `RemoveTrack` | `{ trackId }` | `ActionResponse` |
| `StartPeopleCut` | `{ workspaceId, groupIds, name, secondsBefore, secondsAfter }` | `{ jobId }` (reuses cut job + `cut:*` events) |

`PersonGroupDTO`: `{ id, thumbnailDataUrl, startSec?, trackCount, videoCount, totalDurationSec, avgConfidence, tracks: [{ id, videoId, videoName, startSec, endSec }] }`.

## Events
`detect:progress {jobId, videosTotal, videosDone, currentVideo, tracksFound}`, `detect:finished {jobId, tracksFound, elapsedMs}`, `detect:failed {jobId, error}`.

## Error codes (additions)
`MODEL_NOT_FOUND`, `INFERENCE_ERROR`, `DETECTION_ERROR`.

## Media
`GET /media/<videoId>` is served by the asset-server fallback handler with HTTP range support; it resolves the file from the database by video ID (never from a path in the URL).
