# Feature Specification: Export People Videos

**Feature Branch**: `002-export-people-videos`

**Created**: 2026-11-07

**Status**: Draft

**Input**: User description: "Let the user select people and export one cut video per person to a folder they choose. The export dialog must estimate how many gigabytes the output will be and how much disk the whole operation will use, including a mass export of many people. The export must stay reliable for very long videos and must not run out of memory. Refuse to start when the chosen drive does not have enough free space."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Export selected people to a chosen folder (Priority: P1)

On the people gallery, the user ticks the people they want and presses **Export**. The export dialog asks for a destination folder and for how many seconds to keep before and after each appearance. On confirm, the app writes one video file per selected person directly into that folder. A person who appears in several videos still produces one file. This replaces the previous **Cut selected** action, which only created a named folder inside the workspace exports area.

**Why this priority**: Choosing the people and the folder is the outcome the user asked for. Size checks and long-video safety exist to make this export trustworthy.

**Independent Test**: Select two people, pick an empty folder, keep the default padding, export, and confirm exactly two playable files exist in that folder and nowhere else was required as the destination.

**Acceptance Scenarios**:

1. **Given** at least one person is selected, **When** the user presses Export, **Then** a dialog opens where they choose a destination folder and the seconds before and after each appearance (default 20 and 20).
2. **Given** a destination folder and at least one person who has footage to export, **When** the user confirms, **Then** the app writes one file per exported person into that folder, named `person-<id>.mp4`, and reports progress until the export finishes.
3. **Given** a person appears in more than one video, **When** that person is exported, **Then** the result is one file covering their appearances in order, not one file per video.
4. **Given** padding would extend past the start or end of a video, **When** the clip is planned, **Then** the extra time is shortened to stay inside that video.
5. **Given** two appearances of the same person in the same video overlap once padding is applied, **When** the clip is planned, **Then** the overlap is kept once, not twice.
6. **Given** the export finishes, **When** the user is returned to the workspace, **Then** they can open the destination folder from the app.

---

### User Story 2 - See the size and stop when the disk cannot hold it (Priority: P1)

Before any cutting starts, the dialog shows two figures for the whole selection: the expected size of the finished files, and the disk the operation is allowed to occupy at its busiest moment. It also shows how much free space the chosen folder's drive has. The user can change padding or the folder and see the figures update. Export stays unavailable when the drive is short, when a selected person's file name is already in the folder, or when nobody in the selection has footage to export.

**Why this priority**: A mass export of long videos can fill a drive. The user must see the cost and be blocked before files start landing.

**Independent Test**: Open Export for a known selection, read the two figures, raise the padding and confirm both figures increase, then point at a drive with less free space than the operation cost and confirm Export cannot be confirmed.

**Acceptance Scenarios**:

1. **Given** the dialog is open, **When** the selection and padding are known, **Then** the user sees the expected output size, the operation disk cost, the number of people who will get a file, and the free space on the chosen drive once a folder is chosen.
2. **Given** the user changes the seconds before or after, or the destination folder, **When** the change is applied, **Then** the figures are recalculated before Export can be confirmed, and a stale figure is never used.
3. **Given** free space is lower than the operation disk cost, **When** the user looks at the dialog, **Then** Export cannot start and the dialog shows free space versus the space required.
4. **Given** the destination folder already contains `person-<id>.mp4` for any selected person, **When** the user tries to export, **Then** Export cannot start, those existing files are left untouched, and unrelated files in the folder are also left untouched.
5. **Given** some selected people have no footage that can be cut, **When** the dialog is shown, **Then** those people are excluded from the file count and from both size figures, and the dialog states how many were skipped. If none remain, Export cannot start.
6. **Given** the on-screen size is rounded, **When** free space is compared with the operation disk cost, **Then** the comparison uses the exact figures, so a cost that rounds to the same displayed value as the free space still blocks when the exact cost is higher.

---

### User Story 3 - Finish long exports without exhausting the computer (Priority: P1)

The user can export people taken from multi-hour videos, and can export many people in one action. The app keeps working through the footage without its memory growing with the length of the video or the number of people. Temporary working files for only one person exist at a time. The user can cancel. If a person fails, finished files stay, that person's temporary files disappear, and people not yet started are not started.

**Why this priority**: The export is only useful if a long, large selection completes on a normal machine instead of running out of memory or leaving a drive full of abandoned working files.

**Independent Test**: Export one person from a source longer than two hours and confirm the app's memory stays in the same band as a one-minute export of the same picture size, the finished file remains, and no temporary files are left behind. Repeat a cancel midway and confirm the same cleanup.

**Acceptance Scenarios**:

1. **Given** a source longer than two hours, **When** a person from that source is exported, **Then** the app completes or cancels without running out of memory, and memory use stays within twice the memory used to export one minute of the same picture size.
2. **Given** 50 people are exported, **When** the export runs, **Then** memory use stays within twice the memory used to export one of those people.
3. **Given** an export is running, **When** the user cancels, **Then** new work stops within 5 seconds, files for people already finished stay in the destination, the in-progress person's temporary files are removed, and later people are not started.
4. **Given** one person fails after others have finished, **When** the failure is reported, **Then** finished files stay, the failed person's temporary files are removed, later people are not started, and the message names the person who failed.
5. **Given** a person finishes, **When** the next person starts, **Then** the previous person's temporary files are already gone, so only one person's working files occupy disk at a time.
6. **Given** the size figures are requested for a selection whose planned clips total 10 hours, **When** the user opens the dialog or changes padding, **Then** the figures appear within 3 seconds and the videos are not played to produce them.

---

### Edge Cases

- No people are selected: Export is unavailable.
- The destination folder is missing, not writable, or is not a folder: Export cannot start, and the dialog says why.
- A selected person was removed after the dialog opened: Export cannot start until the figures are refreshed against the people who still exist.
- A source file is missing or unreadable: that person fails, finished files are kept, temporary files for the failed person are removed, and later people are not started.
- Free space shrinks after Export has started because another program writes to the same drive: the export stops at the failure, keeps finished files, removes the in-progress temporary files, and does not claim the original check protects against other programs.
- The destination already has files that are not `person-<id>.mp4` for the selection: those files stay, and they do not by themselves block the export.
- Padding is zero: the clip is exactly the merged appearance range, and both size figures include no extra time.
- Padding is very large: time is still clamped to each video, and the figures use the clamped ranges.
- One selected person accounts for almost all of the planned duration: the operation disk cost still adds that person's temporary working space on top of the finished-file allowance.
- The expected output is under 0.1 GB: the dialog shows megabytes instead of rounding the value to 0 GB. Larger values are shown in gigabytes with one decimal place.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The people gallery MUST offer a single **Export** action for the current selection and MUST NOT keep a separate **Cut selected** action for people.
- **FR-002**: Export MUST require a destination folder chosen by the user. Files MUST be written in that folder, not in a generated subfolder.
- **FR-003**: Each exported person MUST produce exactly one video file named `person-<id>.mp4`. People who appear in several videos MUST still produce one file. A person's display name MUST NOT be used as the file name.
- **FR-004**: Each file MUST cover that person's first appearance through their last appearance in each source video, plus the chosen seconds before and after, clamped to each video. Overlapping ranges in the same video MUST be included once. Pieces from different videos MUST be ordered by the video's start time when it is known.
- **FR-005**: Seconds before and seconds after MUST be editable in the dialog, MUST default to 20 and 20, and MUST accept whole numbers from 0 through 86,400.
- **FR-006**: Before cutting starts, the dialog MUST show the expected output size of the whole selection and the operation disk cost of the whole selection. Both MUST use the same merged, clamped time ranges as the files that will be written.
- **FR-007**: Expected output size MUST be the sum, over every planned piece, of the source file's size multiplied by the planned duration of that piece divided by the source duration. It MUST be computed from file sizes and known durations, without playing the videos.
- **FR-008**: Operation disk cost MUST equal 1.5 times the expected output size, plus 1.5 times the expected size of the single largest person's file. The 1.5 factor is the allowance for a clip that must be recreated and comes out larger than the source share. The second term is the one temporary working copy that exists while a person is being written.
- **FR-009**: The app MUST export people one at a time. Temporary working files MUST exist for at most the person currently being written, MUST live only for that person, and MUST be removed when that person finishes, fails, or is cancelled, before the next person starts.
- **FR-010**: While an export is running, and no other program is writing to the drive, the extra disk occupied by finished files plus temporary files MUST stay at or below the operation disk cost shown when the export started.
- **FR-011**: The dialog MUST show free space on the drive that contains the destination folder, in the same units as the cost. Export MUST be impossible while free space is below the operation disk cost. The user MUST NOT be offered a way to start anyway.
- **FR-012**: Export MUST be impossible while the destination already contains `person-<id>.mp4` for any person who would be written. Existing files MUST NOT be overwritten or deleted. Other files in the folder MUST be left as they are.
- **FR-013**: People with no cuttable footage MUST be omitted from the file count and from both size figures, and the dialog MUST say how many were skipped. If every selected person is skipped, Export MUST be impossible.
- **FR-014**: Changing padding, the destination folder, or the set of people still available MUST recalculate both figures and the collision and free-space checks before Export can be confirmed.
- **FR-015**: The comparison of free space to operation disk cost MUST use the exact calculated values, not the rounded text shown to the user.
- **FR-016**: Sizes of 0.1 GB or more MUST be shown in gigabytes with one decimal place. Sizes under 0.1 GB MUST be shown in megabytes so they are not displayed as zero.
- **FR-017**: Export MUST report progress across the whole selection and MUST let the user cancel. Cancel MUST stop new work within 5 seconds, keep finished person files, remove the in-progress person's temporary files, and skip people not yet started.
- **FR-018**: If one person fails, the app MUST stop the export, keep finished person files, remove that person's temporary files, skip people not yet started, and name the person who failed.
- **FR-019**: Memory use MUST NOT grow in proportion to source duration or to the number of people selected. A source longer than two hours MUST stay within twice the memory of a one-minute export at the same picture size. An export of 50 people MUST stay within twice the memory of exporting one of those people.
- **FR-020**: For a selection whose planned clips total 10 hours, both size figures MUST appear within 3 seconds of opening the dialog or changing padding.
- **FR-021**: When the finished clips reuse the original footage, their total size on disk MUST land within 15% of the expected output size shown before the export.
- **FR-022**: Other ways of cutting video in the app (manual cuts and cuts from a spreadsheet) MUST keep their current destination rules. This feature changes only the people-gallery export.

### Key Entities

- **Export request**: The selected people, the destination folder, the seconds before and after, which of those people can actually be exported, the expected output size, the operation disk cost, and the free space seen when the figures were calculated.
- **Person clip**: One finished video file for one person, planned from that person's appearances, padding, and the videos those appearances belong to.
- **Disk budget**: The expected output size, the operation disk cost derived from it, and the free space on the destination drive. The budget is what the dialog shows and what allows or blocks the start.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can select 20 people, choose a folder, review the two size figures, and start an export in under 2 minutes of interaction, not counting the time spent writing files.
- **SC-002**: In a trial of 10 exports where the clips reuse the original footage, every trial's finished files total within 15% of the expected output size shown beforehand.
- **SC-003**: In a trial where free space is below the operation disk cost, 100% of attempts are refused before any destination file or temporary file is created.
- **SC-004**: In a trial where a selected file name already exists, 100% of attempts are refused and the existing file's contents are unchanged.
- **SC-005**: While no other program writes to the drive, peak disk used by a completed or cancelled export stays at or below the operation disk cost shown at the start, and no temporary working files remain afterward.
- **SC-006**: Exporting from a source longer than 2 hours uses no more than twice the memory of exporting 1 minute at the same picture size. Exporting 50 people uses no more than twice the memory of exporting 1 of those people.
- **SC-007**: For a 10-hour selection, both size figures appear within 3 seconds, without the videos being played.
- **SC-008**: Cancel stops new work within 5 seconds. Finished person files remain. The cancelled person's temporary files are gone.

## Assumptions

- The gallery selection the user already has is the set of people to export. Export does not add a second picker.
- File names stay `person-<id>.mp4` so two people with the same display name cannot collide, and a repeat export into the same folder is blocked until the user moves the old file.
- Seconds before and after keep today's defaults and limits (20, 20, and 0 through 86,400).
- Expected output size assumes each planned second costs the same share of its source file. That matches clips that reuse the original footage. Some clips cannot reuse it and are recreated; those can be larger or smaller than the share. The operation disk cost therefore reserves half again on the finished files and half again on the largest person's temporary copy (the 1.5 factors in FR-008). Raise those factors if real recreated clips regularly exceed that allowance.
- People are written one at a time so the temporary-copy term stays a single person. A faster parallel export would need a new cost formula before it could be allowed.
- The free-space check is made against the drive that holds the destination folder, at calculation time. It does not reserve the space exclusively, and it does not protect against another program filling the drive afterward.
- On cancel or failure, finished person files are kept. They are not rolled back. Deleting multi-gigabyte successes would cost the user the work already paid for.
- A person with no cuttable footage is skipped and explained, matching the rule that an empty plan produces no file. The export does not invent a clip for them.
- Manual cuts and spreadsheet cuts are unchanged.
- The previous people action **Cut selected**, including its habit of creating `<workspace>/exports/<name>/`, is retired for this gallery. That older behavior is superseded by this specification for the people gallery only.
