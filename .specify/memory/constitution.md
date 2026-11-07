# TrilhasBR Video Cutter Constitution

## Core Principles

### I. Local-First Desktop Product

The application MUST run as a Wails desktop app with core workflows (workspace indexing, cuts,
progress) operating on local files and SQLite without requiring network access unless a change is
explicitly scoped and documented as networked. Rationale: user media and paths are sensitive; the
product promise is offline-capable tooling.

### II. Preserve Architectural Boundaries

Backend logic MUST live under `internal/` by concern (actions, repository, media, platform, API);
frontend MUST follow the existing layered layout (`app`, `pages`, `features`, `entities`, `shared`,
`infrastructure`). The Wails bridge MUST stay a thin mapping layer between use-cases and DTOs.
Rationale: boundaries keep FFmpeg and storage concerns testable and the UI replaceable.

### III. Data Integrity And Explicit Errors

Persistent state MUST use SQLite with migrations for schema changes. User-visible failures MUST use
the typed backend error model surfaced to the frontend—not silent failures or raw panics. Cut and
indexing rules (for example `started_at` NULL exclusion) MUST remain documented when behavior
affects user outcomes. Rationale: bad cuts or lost indexing erode trust in a media tool.

### IV. Media Pipeline Safety

FFmpeg/FFprobe resolution MUST follow the documented order: bundled binaries, then `PATH`. Parallel
FFmpeg work MUST stay bounded; long-running work MUST emit progress events (`cut:progress`,
`cut:finished`, `cut:failed`) so the UI can reflect state. Rationale: stability with large files and
clear operator feedback.

### V. Minimal Change And Observability

Changes MUST be the smallest correct diff; speculative abstractions and new dependencies require
clear justification. Logging and errors MUST be sufficient to diagnose probe, cut, merge, and DB
failures in support scenarios. Rationale: desktop multimedia stacks fail in the field—debuggability
is a feature.

## Stack & Trust Boundaries

- **Stack (fixed unless amended):** Go + Wails v2 backend; React + TypeScript + TanStack Query +
  React Router + Zod + Tailwind on the frontend; SQLite via `modernc.org/sqlite`; FFmpeg/FFprobe
  via the internal abstraction.
- **Trust boundaries:** Paths and workspace roots come from the user and native dialogs; treat them
  as untrusted input at filesystem and subprocess boundaries (validate, normalize, and fail closed).
- **Timezone:** Datetime input uses local system timezone and UTC internally; behavior changes
  MUST update user-facing docs.

## Development Workflow & Quality Gates

- **Tests:** Run `go test ./...` for backend changes; frontend/build checks MUST pass when UI or
  build scripts change (`npm run build` in `frontend` as appropriate).
- **Review:** Pull requests MUST describe user-visible behavior, risk (especially for media and
  concurrency), and any migration or bundled binary updates.
- **Compatibility:** Breaking changes to stored data, public Wails API shapes, or cut semantics
  require a migration or version note in the amendment or release documentation.

## Governance

This constitution supersedes ad-hoc conventions when they conflict. Amendments MUST be made via pull
request that updates `.specify/memory/constitution.md`, bumps **Version** per semantic versioning
(MAJOR: incompatible governance or removed principles; MINOR: new principle or materially expanded
section; PATCH: clarifications only), sets **Last Amended** to the merge date, and summarizes the
change in the PR description. Reviewers MUST confirm that merged work aligns with the principles
above; complexity MUST be justified in prose when it violates Principle V. Day-to-day coding
patterns in `README.md` and team rules remain guidance and MUST not contradict this document without
an amendment.

**Version**: 1.0.0 | **Ratified**: 2026-11-06 | **Last Amended**: 2026-11-06
