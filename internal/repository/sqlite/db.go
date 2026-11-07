package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"trilhasbr-video-cutter/internal/apperrors"
)

func Open(ctx context.Context, databasePath string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o755); err != nil {
		return nil, apperrors.Wrap(apperrors.CodeIO, "failed to create database directory", err, map[string]any{"path": databasePath})
	}

	db, err := sql.Open("sqlite", databasePath+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeDB, "failed to open sqlite database", err, map[string]any{"path": databasePath})
	}

	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)

	// foreign_keys/busy_timeout are also in the DSN above: a recycled connection would otherwise
	// come back with foreign keys OFF and silently break ON DELETE CASCADE.
	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA foreign_keys=ON;",
		"PRAGMA busy_timeout=5000;",
	}
	for _, pragma := range pragmas {
		if _, execErr := db.ExecContext(ctx, pragma); execErr != nil {
			_ = db.Close()
			return nil, apperrors.Wrap(apperrors.CodeDB, "failed to configure sqlite pragmas", execErr, map[string]any{"pragma": pragma})
		}
	}

	if pingErr := db.PingContext(ctx); pingErr != nil {
		_ = db.Close()
		return nil, apperrors.Wrap(apperrors.CodeDB, "failed to ping sqlite database", pingErr, map[string]any{"path": databasePath})
	}

	return db, nil
}

func MustDBPath(appDataDir string) string {
	return filepath.Join(appDataDir, "video-cutter.sqlite")
}

func toRFC3339UTC(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseRFC3339UTC(raw string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse timestamp %q: %w", raw, err)
	}
	return parsed.UTC(), nil
}
