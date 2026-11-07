package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/domain"
)

type WorkspaceRepository struct {
	db *sql.DB
}

func NewWorkspaceRepository(db *sql.DB) *WorkspaceRepository {
	return &WorkspaceRepository{db: db}
}

func (r *WorkspaceRepository) UpsertByDirectory(ctx context.Context, directory string, name string) (domain.Workspace, error) {
	now := time.Now().UTC()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Workspace{}, apperrors.Wrap(apperrors.CodeDB, "failed to begin workspace upsert transaction", err, nil)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	var workspaceID int64
	row := tx.QueryRowContext(ctx, `SELECT id FROM workspaces WHERE directory = ?`, directory)
	err = row.Scan(&workspaceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			insertResult, insertErr := tx.ExecContext(ctx, `
				INSERT INTO workspaces (directory, name, created_at, updated_at)
				VALUES (?, ?, ?, ?)
			`, directory, name, toRFC3339UTC(now), toRFC3339UTC(now))
			if insertErr != nil {
				return domain.Workspace{}, apperrors.Wrap(apperrors.CodeDB, "failed to insert workspace", insertErr, map[string]any{"directory": directory})
			}
			workspaceID, err = insertResult.LastInsertId()
			if err != nil {
				return domain.Workspace{}, apperrors.Wrap(apperrors.CodeDB, "failed to read inserted workspace id", err, map[string]any{"directory": directory})
			}
		} else {
			return domain.Workspace{}, apperrors.Wrap(apperrors.CodeDB, "failed to query workspace by directory", err, map[string]any{"directory": directory})
		}
	} else {
		if _, updateErr := tx.ExecContext(ctx, `
			UPDATE workspaces
			SET name = ?, updated_at = ?
			WHERE id = ?
		`, name, toRFC3339UTC(now), workspaceID); updateErr != nil {
			return domain.Workspace{}, apperrors.Wrap(apperrors.CodeDB, "failed to update workspace", updateErr, map[string]any{"workspaceId": workspaceID})
		}
	}

	workspace, fetchErr := r.getByIDTx(ctx, tx, workspaceID)
	if fetchErr != nil {
		return domain.Workspace{}, fetchErr
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return domain.Workspace{}, apperrors.Wrap(apperrors.CodeDB, "failed to commit workspace upsert transaction", commitErr, nil)
	}
	tx = nil

	return workspace, nil
}

func (r *WorkspaceRepository) GetByID(ctx context.Context, workspaceID int64) (domain.Workspace, error) {
	workspace, err := r.getByIDQuery(ctx, r.db, workspaceID)
	if err != nil {
		return domain.Workspace{}, err
	}
	return workspace, nil
}

func (r *WorkspaceRepository) Delete(ctx context.Context, workspaceID int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM workspaces WHERE id = ?`, workspaceID)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeDB, "failed to delete workspace", err, map[string]any{"workspaceId": workspaceID})
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return apperrors.New(apperrors.CodeValidation, "workspace not found", map[string]any{"workspaceId": workspaceID})
	}
	return nil
}

func (r *WorkspaceRepository) ListWithStats(ctx context.Context) ([]domain.WorkspaceSummary, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			w.id,
			w.directory,
			w.name,
			w.created_at,
			w.updated_at,
			COUNT(v.id) AS video_count,
			COALESCE(SUM(CASE WHEN v.started_at IS NULL THEN 1 ELSE 0 END), 0) AS null_started_at_count
		FROM workspaces w
		LEFT JOIN videos v ON v.workspace_id = w.id
		GROUP BY w.id
		ORDER BY w.updated_at DESC
	`)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeDB, "failed to list workspaces", err, nil)
	}
	defer rows.Close()

	results := make([]domain.WorkspaceSummary, 0)
	for rows.Next() {
		var (
			workspace                  domain.Workspace
			createdAtRaw, updatedAtRaw string
			summary                    domain.WorkspaceSummary
		)
		if scanErr := rows.Scan(
			&workspace.ID,
			&workspace.Directory,
			&workspace.Name,
			&createdAtRaw,
			&updatedAtRaw,
			&summary.VideoCount,
			&summary.NullStartedAtCount,
		); scanErr != nil {
			return nil, apperrors.Wrap(apperrors.CodeDB, "failed to scan workspace list row", scanErr, nil)
		}

		createdAt, parseErr := parseRFC3339UTC(createdAtRaw)
		if parseErr != nil {
			return nil, apperrors.Wrap(apperrors.CodeDB, "failed to parse workspace created_at", parseErr, map[string]any{"workspaceId": workspace.ID})
		}
		updatedAt, parseErr := parseRFC3339UTC(updatedAtRaw)
		if parseErr != nil {
			return nil, apperrors.Wrap(apperrors.CodeDB, "failed to parse workspace updated_at", parseErr, map[string]any{"workspaceId": workspace.ID})
		}
		workspace.CreatedAt = createdAt
		workspace.UpdatedAt = updatedAt

		summary.Workspace = workspace
		results = append(results, summary)
	}

	if err = rows.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.CodeDB, "failed while iterating workspace rows", err, nil)
	}

	return results, nil
}

type queryRowScanner interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (r *WorkspaceRepository) getByIDTx(ctx context.Context, tx *sql.Tx, workspaceID int64) (domain.Workspace, error) {
	return r.getByIDQuery(ctx, tx, workspaceID)
}

func (r *WorkspaceRepository) getByIDQuery(ctx context.Context, db queryRowScanner, workspaceID int64) (domain.Workspace, error) {
	var (
		workspace                  domain.Workspace
		createdAtRaw, updatedAtRaw string
	)

	row := db.QueryRowContext(ctx, `
		SELECT id, directory, name, created_at, updated_at
		FROM workspaces
		WHERE id = ?
	`, workspaceID)
	if err := row.Scan(&workspace.ID, &workspace.Directory, &workspace.Name, &createdAtRaw, &updatedAtRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Workspace{}, apperrors.New(apperrors.CodeValidation, "workspace not found", map[string]any{"workspaceId": workspaceID})
		}
		return domain.Workspace{}, apperrors.Wrap(apperrors.CodeDB, "failed to fetch workspace", err, map[string]any{"workspaceId": workspaceID})
	}

	createdAt, err := parseRFC3339UTC(createdAtRaw)
	if err != nil {
		return domain.Workspace{}, apperrors.Wrap(apperrors.CodeDB, "failed to parse workspace created_at", err, map[string]any{"workspaceId": workspaceID})
	}
	updatedAt, err := parseRFC3339UTC(updatedAtRaw)
	if err != nil {
		return domain.Workspace{}, apperrors.Wrap(apperrors.CodeDB, "failed to parse workspace updated_at", err, map[string]any{"workspaceId": workspaceID})
	}

	workspace.CreatedAt = createdAt
	workspace.UpdatedAt = updatedAt

	return workspace, nil
}

func (r *WorkspaceRepository) String() string {
	return fmt.Sprintf("WorkspaceRepository{%p}", r)
}
