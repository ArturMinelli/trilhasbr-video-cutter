package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/domain"
)

type VideoRepository struct {
	db *sql.DB
}

func NewVideoRepository(db *sql.DB) *VideoRepository {
	return &VideoRepository{db: db}
}

func (r *VideoRepository) ReplaceWorkspaceVideos(ctx context.Context, workspaceID int64, videos []domain.VideoInsert) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeDB, "failed to begin replace videos transaction", err, map[string]any{"workspaceId": workspaceID})
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	// Upsert by path (not delete-all) so video IDs stay stable: detection results
	// cascade from videos and must survive a re-import of the same folder.
	keep := make(map[string]bool, len(videos))
	for _, video := range videos {
		if video.DurationSeconds > 0 {
			keep[video.Path] = true
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, path FROM videos WHERE workspace_id = ?`, workspaceID)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeDB, "failed to list workspace videos", err, map[string]any{"workspaceId": workspaceID})
	}
	var stale []int64
	for rows.Next() {
		var id int64
		var path string
		if err = rows.Scan(&id, &path); err != nil {
			rows.Close()
			return apperrors.Wrap(apperrors.CodeDB, "failed to scan workspace video", err, map[string]any{"workspaceId": workspaceID})
		}
		if !keep[path] {
			stale = append(stale, id)
		}
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return apperrors.Wrap(apperrors.CodeDB, "failed to iterate workspace videos", err, map[string]any{"workspaceId": workspaceID})
	}
	for _, id := range stale {
		if _, err = tx.ExecContext(ctx, `DELETE FROM videos WHERE id = ?`, id); err != nil {
			return apperrors.Wrap(apperrors.CodeDB, "failed to delete stale video", err, map[string]any{"workspaceId": workspaceID, "videoId": id})
		}
	}

	insertStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO videos (
			workspace_id,
			name,
			path,
			started_at,
			finished_at,
			duration_seconds,
			created_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workspace_id, path) DO UPDATE SET
			name = excluded.name,
			started_at = excluded.started_at,
			finished_at = excluded.finished_at,
			duration_seconds = excluded.duration_seconds,
			updated_at = excluded.updated_at
	`)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeDB, "failed to prepare video insert statement", err, map[string]any{"workspaceId": workspaceID})
	}
	defer insertStmt.Close()

	nowRaw := toRFC3339UTC(time.Now().UTC())

	for _, video := range videos {
		if video.DurationSeconds <= 0 {
			continue
		}

		name := strings.TrimSpace(video.Name)
		if name == "" {
			name = filepath.Base(video.Path)
		}

		var startedAt any
		if video.StartedAt == nil {
			startedAt = nil
		} else {
			startedAt = toRFC3339UTC(video.StartedAt.UTC())
		}

		var finishedAt any
		if video.FinishedAt == nil {
			finishedAt = nil
		} else {
			finishedAt = toRFC3339UTC(video.FinishedAt.UTC())
		}

		if _, err = insertStmt.ExecContext(ctx,
			workspaceID,
			name,
			video.Path,
			startedAt,
			finishedAt,
			video.DurationSeconds,
			nowRaw,
			nowRaw,
		); err != nil {
			return apperrors.Wrap(apperrors.CodeDB, "failed to insert video", err, map[string]any{"workspaceId": workspaceID, "path": video.Path})
		}
	}

	if err = tx.Commit(); err != nil {
		return apperrors.Wrap(apperrors.CodeDB, "failed to commit replace videos transaction", err, map[string]any{"workspaceId": workspaceID})
	}
	tx = nil

	return nil
}

func (r *VideoRepository) ListByWorkspace(ctx context.Context, workspaceID int64, page int, pageSize int) ([]domain.Video, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 500 {
		pageSize = 500
	}
	offset := (page - 1) * pageSize

	var total int
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM videos
		WHERE workspace_id = ?
	`, workspaceID).Scan(&total); err != nil {
		return nil, 0, apperrors.Wrap(apperrors.CodeDB, "failed to count workspace videos", err, map[string]any{"workspaceId": workspaceID})
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			id,
			workspace_id,
			name,
			path,
			started_at,
			finished_at,
			duration_seconds,
			created_at,
			updated_at
		FROM videos
		WHERE workspace_id = ?
		ORDER BY
			CASE WHEN started_at IS NULL THEN 1 ELSE 0 END,
			started_at ASC,
			path ASC
		LIMIT ? OFFSET ?
	`, workspaceID, pageSize, offset)
	if err != nil {
		return nil, 0, apperrors.Wrap(apperrors.CodeDB, "failed to list workspace videos", err, map[string]any{"workspaceId": workspaceID})
	}
	defer rows.Close()

	videos := make([]domain.Video, 0, pageSize)
	for rows.Next() {
		video, scanErr := scanVideoRow(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		videos = append(videos, video)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, apperrors.Wrap(apperrors.CodeDB, "failed while iterating videos rows", err, map[string]any{"workspaceId": workspaceID})
	}

	return videos, total, nil
}

func (r *VideoRepository) ListByWorkspaceAll(ctx context.Context, workspaceID int64) ([]domain.Video, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			id,
			workspace_id,
			name,
			path,
			started_at,
			finished_at,
			duration_seconds,
			created_at,
			updated_at
		FROM videos
		WHERE workspace_id = ?
		ORDER BY
			CASE WHEN started_at IS NULL THEN 1 ELSE 0 END,
			started_at ASC,
			path ASC
	`, workspaceID)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeDB, "failed to list all workspace videos", err, map[string]any{"workspaceId": workspaceID})
	}
	defer rows.Close()

	videos := make([]domain.Video, 0, 256)
	for rows.Next() {
		video, scanErr := scanVideoRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		videos = append(videos, video)
	}

	if err = rows.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.CodeDB, "failed while iterating all workspace videos", err, map[string]any{"workspaceId": workspaceID})
	}

	return videos, nil
}

func (r *VideoRepository) GetByID(ctx context.Context, videoID int64) (domain.Video, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			id,
			workspace_id,
			name,
			path,
			started_at,
			finished_at,
			duration_seconds,
			created_at,
			updated_at
		FROM videos
		WHERE id = ?
	`, videoID)

	video, err := scanVideoRow(row)
	if err != nil {
		if appErr := apperrors.As(err); appErr != nil && appErr.Code == apperrors.CodeValidation {
			return domain.Video{}, apperrors.New(apperrors.CodeValidation, "video not found", map[string]any{"videoId": videoID})
		}
		return domain.Video{}, apperrors.Wrap(apperrors.CodeDB, "failed to get video by id", err, map[string]any{"videoId": videoID})
	}
	return video, nil
}

func (r *VideoRepository) DeleteWorkspaceExportVideos(ctx context.Context, workspaceID int64, workspaceDirectory string) (int, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, path
		FROM videos
		WHERE workspace_id = ?
		  AND (path LIKE '%/exports/%' OR path LIKE '%\exports\%')
	`, workspaceID)
	if err != nil {
		return 0, apperrors.Wrap(apperrors.CodeDB, "failed to query workspace videos for export cleanup", err, map[string]any{"workspaceId": workspaceID})
	}
	defer rows.Close()

	type rowItem struct {
		id   int64
		path string
	}
	candidates := make([]rowItem, 0)
	for rows.Next() {
		var item rowItem
		if scanErr := rows.Scan(&item.id, &item.path); scanErr != nil {
			return 0, apperrors.Wrap(apperrors.CodeDB, "failed to scan workspace video during export cleanup", scanErr, map[string]any{"workspaceId": workspaceID})
		}
		if pathContainsExportsSegment(item.path, workspaceDirectory) {
			candidates = append(candidates, item)
		}
	}
	if err = rows.Err(); err != nil {
		return 0, apperrors.Wrap(apperrors.CodeDB, "failed while iterating videos during export cleanup", err, map[string]any{"workspaceId": workspaceID})
	}

	if len(candidates) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, apperrors.Wrap(apperrors.CodeDB, "failed to begin export cleanup transaction", err, map[string]any{"workspaceId": workspaceID})
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `DELETE FROM videos WHERE workspace_id = ? AND id = ?`)
	if err != nil {
		return 0, apperrors.Wrap(apperrors.CodeDB, "failed to prepare export cleanup delete statement", err, map[string]any{"workspaceId": workspaceID})
	}
	defer stmt.Close()

	deleted := 0
	for _, item := range candidates {
		result, execErr := stmt.ExecContext(ctx, workspaceID, item.id)
		if execErr != nil {
			return 0, apperrors.Wrap(apperrors.CodeDB, "failed deleting export video row", execErr, map[string]any{"workspaceId": workspaceID, "videoId": item.id})
		}
		affected, affErr := result.RowsAffected()
		if affErr != nil {
			return 0, apperrors.Wrap(apperrors.CodeDB, "failed reading affected rows for export cleanup", affErr, map[string]any{"workspaceId": workspaceID, "videoId": item.id})
		}
		if affected > 0 {
			deleted++
		}
	}

	if err = tx.Commit(); err != nil {
		return 0, apperrors.Wrap(apperrors.CodeDB, "failed to commit export cleanup transaction", err, map[string]any{"workspaceId": workspaceID})
	}
	tx = nil

	return deleted, nil
}

func pathContainsExportsSegment(videoPath string, workspaceDirectory string) bool {
	cleanVideoPath := filepath.Clean(videoPath)
	cleanWorkspace := filepath.Clean(workspaceDirectory)

	if cleanWorkspace != "" {
		rel, err := filepath.Rel(cleanWorkspace, cleanVideoPath)
		if err == nil && rel != "." && rel != "" && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." {
			return hasExportsSegment(rel)
		}
	}

	return hasExportsSegment(cleanVideoPath)
}

func hasExportsSegment(rawPath string) bool {
	normalized := strings.ReplaceAll(rawPath, "\\", "/")
	segments := strings.Split(normalized, "/")
	for _, segment := range segments {
		if strings.EqualFold(segment, "exports") {
			return true
		}
	}
	return false
}

func (r *VideoRepository) FindOverlapping(ctx context.Context, workspaceID int64, windowStartUTC time.Time, windowEndUTC time.Time) ([]domain.Video, error) {
	windowEndRaw := toRFC3339UTC(windowEndUTC)
	windowStartRaw := toRFC3339UTC(windowStartUTC)

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			id,
			workspace_id,
			name,
			path,
			started_at,
			finished_at,
			duration_seconds,
			created_at,
			updated_at
		FROM videos
		WHERE workspace_id = ?
		  AND started_at IS NOT NULL
		  AND finished_at IS NOT NULL
		  AND started_at < ?
		  AND finished_at > ?
		ORDER BY started_at ASC
	`, workspaceID, windowEndRaw, windowStartRaw)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeDB, "failed to query overlapping videos", err, map[string]any{"workspaceId": workspaceID})
	}
	defer rows.Close()

	videos := make([]domain.Video, 0)
	for rows.Next() {
		video, scanErr := scanVideoRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		videos = append(videos, video)
	}

	if err = rows.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.CodeDB, "failed while iterating overlapping videos", err, map[string]any{"workspaceId": workspaceID})
	}

	return videos, nil
}

func (r *VideoRepository) ShiftVideoTimes(ctx context.Context, workspaceID int64, videoIDs []int64, shiftBy time.Duration) (int, error) {
	if len(videoIDs) == 0 || shiftBy == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, apperrors.Wrap(apperrors.CodeDB, "failed to begin shift video times transaction", err, map[string]any{"workspaceId": workspaceID})
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	selectStmt, err := tx.PrepareContext(ctx, `
		SELECT started_at, finished_at
		FROM videos
		WHERE workspace_id = ? AND id = ?
	`)
	if err != nil {
		return 0, apperrors.Wrap(apperrors.CodeDB, "failed to prepare shift select statement", err, map[string]any{"workspaceId": workspaceID})
	}
	defer selectStmt.Close()

	updateStmt, err := tx.PrepareContext(ctx, `
		UPDATE videos
		SET started_at = ?, finished_at = ?, updated_at = ?
		WHERE workspace_id = ? AND id = ?
	`)
	if err != nil {
		return 0, apperrors.Wrap(apperrors.CodeDB, "failed to prepare shift update statement", err, map[string]any{"workspaceId": workspaceID})
	}
	defer updateStmt.Close()

	updatedAtRaw := toRFC3339UTC(time.Now().UTC())
	updatedCount := 0

	for _, videoID := range videoIDs {
		var startedAtRaw, finishedAtRaw sql.NullString
		queryErr := selectStmt.QueryRowContext(ctx, workspaceID, videoID).Scan(&startedAtRaw, &finishedAtRaw)
		if queryErr != nil {
			if errors.Is(queryErr, sql.ErrNoRows) {
				return 0, apperrors.New(apperrors.CodeValidation, "video not found in workspace", map[string]any{"workspaceId": workspaceID, "videoId": videoID})
			}
			return 0, apperrors.Wrap(apperrors.CodeDB, "failed to load video timestamps before shift", queryErr, map[string]any{"workspaceId": workspaceID, "videoId": videoID})
		}

		if !startedAtRaw.Valid || !finishedAtRaw.Valid {
			return 0, apperrors.New(apperrors.CodeValidation, "video has missing timestamps and cannot be shifted", map[string]any{"workspaceId": workspaceID, "videoId": videoID})
		}

		startedAt, parseErr := parseRFC3339UTC(startedAtRaw.String)
		if parseErr != nil {
			return 0, apperrors.Wrap(apperrors.CodeDB, "failed to parse video started_at before shift", parseErr, map[string]any{"workspaceId": workspaceID, "videoId": videoID})
		}
		finishedAt, parseErr := parseRFC3339UTC(finishedAtRaw.String)
		if parseErr != nil {
			return 0, apperrors.Wrap(apperrors.CodeDB, "failed to parse video finished_at before shift", parseErr, map[string]any{"workspaceId": workspaceID, "videoId": videoID})
		}

		shiftedStartedAt := toRFC3339UTC(startedAt.Add(shiftBy))
		shiftedFinishedAt := toRFC3339UTC(finishedAt.Add(shiftBy))

		result, execErr := updateStmt.ExecContext(ctx, shiftedStartedAt, shiftedFinishedAt, updatedAtRaw, workspaceID, videoID)
		if execErr != nil {
			return 0, apperrors.Wrap(apperrors.CodeDB, "failed to update shifted video timestamps", execErr, map[string]any{"workspaceId": workspaceID, "videoId": videoID})
		}
		affected, affErr := result.RowsAffected()
		if affErr != nil {
			return 0, apperrors.Wrap(apperrors.CodeDB, "failed to read shift update affected rows", affErr, map[string]any{"workspaceId": workspaceID, "videoId": videoID})
		}
		if affected == 0 {
			return 0, apperrors.New(apperrors.CodeValidation, "video not found in workspace", map[string]any{"workspaceId": workspaceID, "videoId": videoID})
		}
		updatedCount += int(affected)
	}

	if err = tx.Commit(); err != nil {
		return 0, apperrors.Wrap(apperrors.CodeDB, "failed to commit shift video times transaction", err, map[string]any{"workspaceId": workspaceID})
	}
	tx = nil

	return updatedCount, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanVideoRow(s scanner) (domain.Video, error) {
	var (
		video                       domain.Video
		startedAtRaw, finishedAtRaw sql.NullString
		nameRaw                     sql.NullString
		createdAtRaw, updatedAtRaw  string
	)
	if err := s.Scan(
		&video.ID,
		&video.WorkspaceID,
		&nameRaw,
		&video.Path,
		&startedAtRaw,
		&finishedAtRaw,
		&video.DurationSeconds,
		&createdAtRaw,
		&updatedAtRaw,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Video{}, apperrors.New(apperrors.CodeValidation, "video not found", nil)
		}
		return domain.Video{}, apperrors.Wrap(apperrors.CodeDB, "failed to scan video row", err, nil)
	}

	if nameRaw.Valid {
		video.Name = nameRaw.String
	}
	if video.Name == "" {
		video.Name = filepath.Base(video.Path)
	}

	createdAt, err := parseRFC3339UTC(createdAtRaw)
	if err != nil {
		return domain.Video{}, apperrors.Wrap(apperrors.CodeDB, "failed to parse video created_at", err, map[string]any{"path": video.Path})
	}
	updatedAt, err := parseRFC3339UTC(updatedAtRaw)
	if err != nil {
		return domain.Video{}, apperrors.Wrap(apperrors.CodeDB, "failed to parse video updated_at", err, map[string]any{"path": video.Path})
	}

	if startedAtRaw.Valid {
		startedAt, parseErr := parseRFC3339UTC(startedAtRaw.String)
		if parseErr != nil {
			return domain.Video{}, apperrors.Wrap(apperrors.CodeDB, "failed to parse video started_at", parseErr, map[string]any{"path": video.Path})
		}
		video.StartedAt = &startedAt
	}
	if finishedAtRaw.Valid {
		finishedAt, parseErr := parseRFC3339UTC(finishedAtRaw.String)
		if parseErr != nil {
			return domain.Video{}, apperrors.Wrap(apperrors.CodeDB, "failed to parse video finished_at", parseErr, map[string]any{"path": video.Path})
		}
		video.FinishedAt = &finishedAt
	}
	video.CreatedAt = createdAt
	video.UpdatedAt = updatedAt

	return video, nil
}

func (r *VideoRepository) String() string {
	return fmt.Sprintf("VideoRepository{%p}", r)
}
