package sqlite

import (
	"context"
	"database/sql"
	"strings"

	"trilhasbr-video-cutter/internal/apperrors"
)

func RunMigrations(ctx context.Context, db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS workspaces (
			id INTEGER PRIMARY KEY,
			directory TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS videos (
			id INTEGER PRIMARY KEY,
			workspace_id INTEGER NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			name TEXT NOT NULL DEFAULT '',
			path TEXT NOT NULL,
			started_at TEXT NULL,
			finished_at TEXT NULL,
			duration_seconds REAL NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(workspace_id, path)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_videos_workspace_started_at ON videos(workspace_id, started_at);`,
		`CREATE INDEX IF NOT EXISTS idx_videos_workspace_finished_at ON videos(workspace_id, finished_at);`,
		`CREATE INDEX IF NOT EXISTS idx_videos_workspace_path ON videos(workspace_id, path);`,
		// detection (specs/001-detect-people-tracks/data-model.md)
		`CREATE TABLE IF NOT EXISTS detection_runs (
			id INTEGER PRIMARY KEY,
			workspace_id INTEGER NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			sample_fps REAL NOT NULL DEFAULT 5 CHECK (sample_fps >= 1 AND sample_fps <= 15),
			model_version TEXT NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('RUNNING','SUCCEEDED','FAILED','CANCELLED')),
			started_at TEXT NOT NULL,
			finished_at TEXT NULL,
			videos_total INTEGER NOT NULL DEFAULT 0,
			videos_done INTEGER NOT NULL DEFAULT 0,
			tracks_found INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS video_analyses (
			video_id INTEGER PRIMARY KEY REFERENCES videos(id) ON DELETE CASCADE,
			sample_fps REAL NOT NULL CHECK (sample_fps >= 1 AND sample_fps <= 15),
			model_version TEXT NOT NULL,
			file_size INTEGER NOT NULL,
			file_mtime INTEGER NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('DONE','FAILED')),
			error TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE TABLE IF NOT EXISTS tracks (
			id INTEGER PRIMARY KEY,
			video_id INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
			start_sec REAL NOT NULL,
			end_sec REAL NOT NULL,
			avg_confidence REAL NOT NULL,
			detection_count INTEGER NOT NULL,
			best_bbox TEXT NOT NULL DEFAULT '',
			thumbnail_path TEXT NOT NULL DEFAULT '',
			embedding BLOB NULL CHECK (embedding IS NULL OR length(embedding) = 2048),
			removed INTEGER NOT NULL DEFAULT 0 CHECK (removed IN (0,1))
		);`,
		`CREATE INDEX IF NOT EXISTS idx_tracks_video ON tracks(video_id);`,
		`CREATE TABLE IF NOT EXISTS person_groups (
			id INTEGER PRIMARY KEY,
			workspace_id INTEGER NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			label TEXT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_person_groups_workspace ON person_groups(workspace_id);`,
		`CREATE TABLE IF NOT EXISTS person_group_tracks (
			group_id INTEGER NOT NULL REFERENCES person_groups(id) ON DELETE CASCADE,
			track_id INTEGER NOT NULL UNIQUE REFERENCES tracks(id) ON DELETE CASCADE,
			PRIMARY KEY (group_id, track_id)
		);`,
	}

	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return apperrors.Wrap(apperrors.CodeDB, "failed to run sqlite migration", err, map[string]any{"statement": statement})
		}
	}

	compatStatements := []string{
		`ALTER TABLE videos ADD COLUMN name TEXT`,
		`ALTER TABLE videos ADD COLUMN started_at TEXT`,
		`ALTER TABLE videos ADD COLUMN finished_at TEXT`,
		// Packed little-endian float32 rows [t,x1,y1,x2,y2]; NULL for tracks stored before paths were recorded.
		`ALTER TABLE tracks ADD COLUMN boxes BLOB`,
	}
	for _, statement := range compatStatements {
		if _, err := db.ExecContext(ctx, statement); err != nil && !isDuplicateColumnError(err) {
			return apperrors.Wrap(apperrors.CodeDB, "failed to alter sqlite table", err, map[string]any{"statement": statement})
		}
	}

	backfillStatements := []string{
		`UPDATE videos SET name = path WHERE name IS NULL OR name = ''`,
		`UPDATE videos SET started_at = taken_at WHERE started_at IS NULL AND taken_at IS NOT NULL`,
		`UPDATE videos
		 SET finished_at = strftime('%Y-%m-%dT%H:%M:%fZ', julianday(started_at) + (duration_seconds / 86400.0))
		 WHERE finished_at IS NULL AND started_at IS NOT NULL`,
	}
	for _, statement := range backfillStatements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "no such column: taken_at") {
				continue
			}
			return apperrors.Wrap(apperrors.CodeDB, "failed to backfill sqlite videos data", err, map[string]any{"statement": statement})
		}
	}

	return nil
}

func isDuplicateColumnError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate column name")
}
