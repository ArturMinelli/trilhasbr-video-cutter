# Data Model

## detection_runs
| Field | Type | Notes |
|---|---|---|
| id | INTEGER PK | |
| workspace_id | INTEGER FK → workspaces (cascade) | |
| sample_fps | REAL | 1–15, default 5 |
| model_version | TEXT | from models manifest |
| status | TEXT | RUNNING / SUCCEEDED / FAILED / CANCELLED |
| started_at, finished_at | TEXT | UTC |
| videos_total, videos_done, tracks_found | INTEGER | summary counters |

## video_analyses
Records that a video was analyzed (enables skip/resume).
| Field | Type | Notes |
|---|---|---|
| video_id | INTEGER PK FK → videos (cascade) | |
| sample_fps, model_version | | cache key |
| file_size, file_mtime | | invalidation |
| status | TEXT | DONE / FAILED (+ error text) |

## tracks
| Field | Type | Notes |
|---|---|---|
| id | INTEGER PK | |
| video_id | INTEGER FK (cascade) | |
| start_sec, end_sec | REAL | offset within the video |
| avg_confidence | REAL | |
| detection_count | INTEGER | |
| best_bbox | TEXT (JSON) | box of the thumbnail frame |
| thumbnail_path | TEXT | JPEG |
| embedding | BLOB | 512×float32, mean, L2-normalized |
| removed | INTEGER | 1 = hidden false detection |

## person_groups / person_group_tracks
`person_groups(id, workspace_id, label NULL)`; `person_group_tracks(group_id, track_id UNIQUE)`. Every new track starts in its own group. Merge = move tracks into one group; split = move a track into a new group; deleting the last track deletes the group.

## Rules
- Re-analyzing a video deletes its tracks (cascade) and then removes empty groups (FR-014).
- Group time range / video count are derived at query time.
- Filters (min duration, min confidence) are query parameters, not stored.
