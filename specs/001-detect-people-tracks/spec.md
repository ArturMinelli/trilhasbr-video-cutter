# Feature Specification: Detect People and Cut by Track

**Feature Branch**: `001-detect-people-tracks`

**Created**: 2026-11-06

**Status**: Draft

**Input**: User description: "Let a user detect people across all videos of a workspace (local person detection + re-identification + tracking), present the people found, and let the user choose which tracks to cut videos from."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Detect people in a workspace (Priority: P1)

From a workspace page, the user presses **Detect people**. The app scans the frames of every video in the workspace, finds people, and groups each person's appearances into tracks. Progress is shown while it runs, and everything happens on the user's machine.

**Why this priority**: Nothing else works without detection results.

**Independent Test**: Import a folder of videos containing people, press Detect people, and confirm that a finished run reports a count of people/tracks found.

**Acceptance Scenarios**:

1. **Given** a workspace with indexed videos, **When** the user presses Detect people, **Then** progress (per video and overall) is shown and a completion summary lists videos processed, people found, and elapsed time.
2. **Given** a run is in progress, **When** the user cancels, **Then** processing stops promptly and already-finished videos keep their results.
3. **Given** a video was already analyzed with the same settings, **When** detection is run again, **Then** that video is skipped.
4. **Given** one video is corrupt, **When** detection runs, **Then** it is reported as failed while the others complete.

---

### User Story 2 - Browse people and select tracks (Priority: P1)

After detection, the user sees a gallery of people. Each card is one track (one person's continuous presence in one video) or a user-built group of tracks. Each card shows a representative crop, the time range, the number of appearances, and the videos involved. The user can preview a person's appearances, filter out short or low-confidence tracks, and tick the people they want.

**Why this priority**: Selection is the whole point of the detection step.

**Independent Test**: With completed detection results, open the gallery, preview a card, tick several, and confirm the selection count updates.

**Acceptance Scenarios**:

1. **Given** completed detection, **When** the user opens the people gallery, **Then** each person appears as a card with a thumbnail, time range, and video count.
2. **Given** the gallery, **When** the user clicks a card, **Then** the person's appearance plays in the source video at the right moment.
3. **Given** filters for minimum duration and confidence, **When** applied, **Then** the gallery shows only matching people.
4. **Given** one person appears as separate cards (for example in different videos), **When** the user merges them (or removes a false detection), **Then** the gallery updates and the change persists.

---

### User Story 3 - Cut clips for selected people (Priority: P1)

The user presses **Cut selected**. For each selected person the app produces one clip spanning their first to last appearance, plus configurable seconds before and after. If a person group contains tracks from several video files, the pieces are merged into a single clip, as in the existing cut flow. Output goes into a named folder.

**Why this priority**: This is the end deliverable.

**Independent Test**: Select two people, cut with 3 s padding, and confirm two clips exist whose timing matches the tracks.

**Acceptance Scenarios**:

1. **Given** selected people, **When** the user confirms a cut with a folder name and padding, **Then** one clip per person is written and progress is reported.
2. **Given** a person spans two videos, **When** cut, **Then** one merged clip is produced.
3. **Given** padding extends past a video boundary, **When** cut, **Then** it is clamped to the video.

---

### User Story 4 - Tune detection settings (Priority: P2)

Before running, the user may adjust frame sampling (default about 5 fps, range 1–15 fps) in an advanced section.

**Why this priority**: The default works for most cases, but speed vs. accuracy is situational.

**Independent Test**: Run at 1 fps and at 10 fps on the same video and confirm both complete, with the setting recorded with the results.

**Acceptance Scenarios**:

1. **Given** the advanced settings, **When** the user changes the sampling rate and runs, **Then** the run uses it and results are tied to it.
2. **Given** results exist at a different rate, **When** the user runs again at a new rate, **Then** the video is re-analyzed.

---

### Edge Cases

- Workspace has no videos, or no people are found: show an explicit empty state.
- Models or inference libraries are missing or fail to load: typed, user-visible error (no silent failure).
- App closed mid-run: partial results survive, and the next run resumes unfinished videos.
- A person leaves the frame and returns, or is briefly occluded: appearances are joined into one track when plausible.
- Same person seen in several videos: shown as separate cards until the user merges them (automatic cross-video grouping is out of scope for this feature).
- Videos without a reliable start time: merged clips order pieces by filename and the user is informed.
- Very large workspaces: parallelism is bounded so the machine stays usable.
- Different people who look alike may be wrongly merged; the user can split or remove them.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The workspace page MUST provide a **Detect people** action over all indexed videos in that workspace.
- **FR-002**: Detection MUST run entirely on the user's machine with no network access.
- **FR-003**: The system MUST find people in sampled frames and link detections into per-video tracks over time, using position and appearance similarity so that a person who is briefly occluded keeps one track. Each track MUST retain an appearance signature so automatic cross-video grouping can be added later without re-running detection.
- **FR-004**: Frame sampling MUST default to about 5 fps and be adjustable by the user (1–15 fps).
- **FR-005**: Videos MUST be processed in parallel with a bounded worker count, and progress events MUST be emitted (per video and overall).
- **FR-006**: Users MUST be able to cancel a run; completed videos retain their results.
- **FR-007**: Results MUST be persisted per workspace and video, with the settings used, so unchanged videos are skipped on re-run and results survive app restarts.
- **FR-008**: Failures (missing models, unreadable video, runtime error) MUST surface as typed, user-visible errors without aborting other videos.
- **FR-009**: The UI MUST present a gallery of people (thumbnail, time range, appearance count, video count) with duration/confidence filters.
- **FR-010**: Users MUST be able to preview a person's appearance in the source video.
- **FR-011**: Users MUST be able to select multiple people and cut them; each person yields one clip spanning first to last appearance plus configurable before/after padding, merged across videos when needed and clamped to video bounds.
- **FR-012**: Cuts MUST reuse the existing cut output conventions (named output folder, progress/finished/failed events).
- **FR-013**: Users MUST be able to merge cards into one person group, split a group, and remove a false detection, with changes persisted.
- **FR-014**: Re-running detection on a changed video or with different settings MUST replace that video's prior results and drop groups that no longer have any tracks.

### Key Entities

- **Detection Run**: One execution over a workspace, with settings (sampling rate), status, start/end times, and counts.
- **Track**: One person's continuous presence in a single video (start/end time, confidence, representative crop).
- **Person Group**: A user-built group of one or more tracks (default: one track per group) that the user treats as the same individual; the unit the user selects.
- **Person Cut**: A clip request derived from a Person Group plus padding, producing an output clip in a named folder.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can go from a workspace with indexed videos to a completed detection run with a single click (plus optional settings).
- **SC-002**: Detection on one hour of footage completes in under 15 minutes on a typical modern laptop.
- **SC-003**: At least 90% of people clearly visible for 2+ seconds appear as a person in the gallery.
- **SC-004**: A user can select people and produce their clips in under 2 minutes of interaction for a 20-person selection.
- **SC-005**: Re-running detection on an unchanged workspace completes in under 10 seconds.
- **SC-006**: The app remains responsive (UI interactions under 1 second) while detection runs.
- **SC-007**: The app works fully offline.

## Assumptions

- Detection models (person detector and re-identification) are bundled with the app; the user performs no download or setup.
- The existing video indexing and timestamp extraction (`started_at`) are reused for time proximity.
- Padding defaults mirror the existing cut flow's before/after seconds.
- Cropped or reframed output, live streams, and non-MP4 formats are out of scope.
- Supported platforms are those the app already supports; hardware acceleration is used when available but is not required.
- Automatic cross-video identity grouping is deferred to a later feature (decided during planning).
- Identification of named individuals (faces, bibs) is out of scope; people are anonymous identities.
