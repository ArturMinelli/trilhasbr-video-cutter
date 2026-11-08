# Research: Export People Videos

## Decisions

- **Reuse `PersonCutAction` and `CutExecuteAction.ExecutePlanned`**. *Rationale*: one clip per person, padding, clamp, overlap merge, and scratch-then-rename already exist. Export only changes where the finished file is moved and what is checked beforehand. *Alternatives*: a second export pipeline (two cutters to keep correct); shelling out to a new ffmpeg recipe (duplicates copy-then-reencode).

- **Preview is a synchronous Wails call, not a job**. *Rationale*: the 3-second budget is a SQL read of track ranges (embeddings are already not loaded) plus `os.Stat` per distinct source. No frame is decoded. Start runs the same gate again so a stale dialog cannot bypass it. *Alternatives*: estimating inside the job after the user confirms (files could be created before the refusal); probing with ffmpeg (too slow, and it reads media).

- **Expected bytes**: for each planned segment, `round(fileSize × segmentDuration / videoDuration)`. Sum per person, then sum people. Ranges come from `BuildPersonCutPlan`, so overlaps are not counted twice and padding is already clamped. A source that cannot be stat'd blocks the preview (a zero estimate would under-block). *Alternatives*: bitrate from ffprobe (another process per file); duration-only guess (ignores the file the user actually has).

- **Operation bytes**: `(expected × 3 / 2) + (largestPerson × 3 / 2)` in integer arithmetic. *Rationale*: this is the spec's 1.5 factors without a float rounding mode. The first term allows every finished file to land half again larger than the source share. The second term is one temporary copy of the largest person at that same larger size, which matches writing that person's segments and then the merged file before the scratch directory is deleted. Truncation of the half-byte is downward and stable. *Alternatives*: a flat 50% on the total only (misses the second copy of a single-person export); measuring a sample encode (slow, and still wrong for the rest of the footage).

- **Gigabytes are decimal**. 1 GB = 1,000,000,000 bytes. Values ≥ 0.1 GB render as `%.1f GB`; smaller values render as whole MB. *Rationale*: matches the spec wording and macOS Finder. The gate compares the int64 byte counts, never these strings. *Alternative*: GiB (would disagree with Finder and with the word "gigabyte").

- **Free space**: `syscall.Statfs` on the destination folder, `Bavail × Bsize`. Block only when `free < operation`. Equal free space is allowed. *Rationale*: standard library, no new module. `Bavail` is what an unprivileged process can still allocate. *Alternatives*: `golang.org/x/sys/unix` (already indirect, but a direct require for one call); letting the user start and failing mid-write (rejected in the spec).

- **Writable**: `syscall.Access(path, W_OK)` during the gate. Do not create a probe file. *Rationale*: SC-003 forbids any destination file before a refusal. *Alternative*: create-and-delete a sentinel (leaves a file if the process dies, and races the "no file created" check).

- **Collisions**: `os.Stat` of `person-<id>.mp4` for each person who would be written. Any hit blocks the whole export. Other names in the folder are ignored. *Rationale*: spec. The check is repeated inside `Execute` before the first scratch directory is created.

- **One person at a time stays**. Segment workers inside one person stay capped as they are today. *Rationale*: the disk formula counts one person's scratch, not N. Parallel people would make the published cost a lie. Memory stays flat because ffmpeg is already `exec.CommandContext` with stream copy (re-encode only as fallback) and Go never reads the source bytes. *Alternative*: parallel people for speed (needs a new cost formula first; out of scope).

- **Cancel**: store a `context.CancelFunc` on the existing cut job, same pattern as detection jobs. `Execute` uses that context. On `context.Canceled`, delete the current scratch directory, keep finished `person-<id>.mp4` files, and emit `cut:cancelled` instead of `cut:failed`. *Rationale*: ffmpeg already dies when the context is cancelled, so the 5-second bound is the process kill, not a poll. A separate event avoids the manual-cut toast that says the cut failed. *Alternatives*: a new job manager (duplicate); treating cancel as `cut:failed` (wrong message, and the page watcher would show an error).

- **Replace `StartPeopleCut`**. The method and the `name` field go away. Callers pass `outputDirectory`. *Rationale*: FR-001 is one people action. Leaving the old method would keep writing `<workspace>/exports/<name>/`. *Alternative*: keep the old method as a wrapper (two destinations, easy to call by mistake).

- **Dialog owns the run**. The export modal stays open, shows `cut:progress` (`completed/total` plus the current person), and offers Cancel. Success opens the folder. The generic workspace cut watcher is not subscribed for this job. *Rationale*: that watcher only toasts and cannot cancel. Progress events already exist; the people UI never subscribed.

- **A failed person is named**. Error text uses the group label when it is non-empty, otherwise `person-<id>`. Later people are not started. Partial `CutResult` is returned with the error so finished paths are not dropped on the floor. *Rationale*: today's `Execute` returns an empty result on the first error, which hides work that is already on disk.

## Open risks

- Recreated clips (the ffmpeg fallback when stream copy fails validation) can exceed the 1.5 source-share allowance. The spec accepts that and says to raise the factors if real exports do. Do not sample-encode during preview to "fix" it.
- `syscall.Statfs` / `Access` are the darwin implementation. A Windows build needs a different free-space call before this feature can ship there. The app's current desktop target is macOS.
- Free space can still be taken by another program after the gate. The export then fails on the write, keeps finished files, and removes the current scratch directory. The preview does not pretend to reserve the space.
