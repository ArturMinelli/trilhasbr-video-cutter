package sqlite

import (
	"context"
	"database/sql"
	"encoding/binary"
	"math"
	"sort"
	"strings"
	"time"

	"trilhasbr-video-cutter/internal/apperrors"
	"trilhasbr-video-cutter/internal/domain"
)

type DetectionRepository struct {
	db *sql.DB
}

func NewDetectionRepository(db *sql.DB) *DetectionRepository {
	return &DetectionRepository{db: db}
}

func dbErr(msg string, err error) error {
	return apperrors.Wrap(apperrors.CodeDB, msg, err, nil)
}

func (r *DetectionRepository) CreateRun(ctx context.Context, workspaceID int64, sampleFPS float64, modelVersion string, videosTotal int) (domain.DetectionRun, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO detection_runs (workspace_id, sample_fps, model_version, status, started_at, videos_total) VALUES (?, ?, ?, 'RUNNING', ?, ?)`,
		workspaceID, sampleFPS, modelVersion, toRFC3339UTC(now), videosTotal)
	if err != nil {
		return domain.DetectionRun{}, dbErr("failed to create detection run", err)
	}
	id, _ := res.LastInsertId()
	return domain.DetectionRun{ID: id, WorkspaceID: workspaceID, SampleFPS: sampleFPS, ModelVer: modelVersion,
		Status: domain.DetectionRunning, StartedAt: now, VideosTotal: videosTotal}, nil
}

func (r *DetectionRepository) FinishRun(ctx context.Context, runID int64, status domain.DetectionStatus, videosDone int, tracksFound int) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE detection_runs SET status = ?, finished_at = ?, videos_done = ?, tracks_found = ? WHERE id = ?`,
		string(status), toRFC3339UTC(time.Now()), videosDone, tracksFound, runID)
	if err != nil {
		return dbErr("failed to finish detection run", err)
	}
	return nil
}

func (r *DetectionRepository) GetVideoAnalysis(ctx context.Context, videoID int64) (*domain.VideoAnalysis, error) {
	var a domain.VideoAnalysis
	err := r.db.QueryRowContext(ctx,
		`SELECT video_id, sample_fps, model_version, file_size, file_mtime, status, error FROM video_analyses WHERE video_id = ?`, videoID).
		Scan(&a.VideoID, &a.SampleFPS, &a.ModelVersion, &a.FileSize, &a.FileMTime, &a.Status, &a.Error)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr("failed to read video analysis", err)
	}
	return &a, nil
}

func (r *DetectionRepository) UpsertVideoAnalysis(ctx context.Context, a domain.VideoAnalysis) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO video_analyses (video_id, sample_fps, model_version, file_size, file_mtime, status, error)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(video_id) DO UPDATE SET sample_fps=excluded.sample_fps, model_version=excluded.model_version,
		   file_size=excluded.file_size, file_mtime=excluded.file_mtime, status=excluded.status, error=excluded.error`,
		a.VideoID, a.SampleFPS, a.ModelVersion, a.FileSize, a.FileMTime, a.Status, a.Error)
	if err != nil {
		return dbErr("failed to upsert video analysis", err)
	}
	return nil
}

func encodeEmbedding(e []float32) (any, error) {
	if len(e) == 0 {
		return nil, nil
	}
	if len(e) != domain.EmbeddingDim {
		return nil, apperrors.New(apperrors.CodeValidation, "embedding must have 512 floats", map[string]any{"len": len(e)})
	}
	b := make([]byte, 4*len(e))
	for i, f := range e {
		binary.LittleEndian.PutUint32(b[4*i:], math.Float32bits(f))
	}
	return b, nil
}

// boxFloats is the number of float32 values per stored box row: t, x1, y1, x2, y2.
const boxFloats = 5

func encodeBoxes(boxes []domain.TrackBox) any {
	if len(boxes) == 0 {
		return nil
	}
	b := make([]byte, 4*boxFloats*len(boxes))
	for i, bx := range boxes {
		for j, f := range [boxFloats]float32{bx.T, bx.X1, bx.Y1, bx.X2, bx.Y2} {
			binary.LittleEndian.PutUint32(b[4*(boxFloats*i+j):], math.Float32bits(f))
		}
	}
	return b
}

func decodeBoxes(b []byte) ([]domain.TrackBox, error) {
	if len(b)%(4*boxFloats) != 0 {
		return nil, apperrors.New(apperrors.CodeDB, "stored track boxes are corrupt", map[string]any{"bytes": len(b)})
	}
	out := make([]domain.TrackBox, len(b)/(4*boxFloats))
	f := func(i, j int) float32 { return math.Float32frombits(binary.LittleEndian.Uint32(b[4*(boxFloats*i+j):])) }
	for i := range out {
		out[i] = domain.TrackBox{T: f(i, 0), X1: f(i, 1), Y1: f(i, 2), X2: f(i, 3), Y2: f(i, 4)}
	}
	return out, nil
}

func decodeEmbedding(b []byte) []float32 {
	if len(b) == 0 {
		return nil
	}
	e := make([]float32, len(b)/4)
	for i := range e {
		e[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return e
}

// deleteEmptyGroups sweeps every empty group. ponytail: global sweep, scope by workspace if it ever gets slow.
func deleteEmptyGroups(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM person_groups WHERE id NOT IN (SELECT group_id FROM person_group_tracks)`)
	return err
}

func (r *DetectionRepository) ReplaceTracksForVideo(ctx context.Context, videoID int64, tracks []domain.Track) ([]domain.Track, error) {
	blobs := make([]any, len(tracks))
	for i, t := range tracks {
		b, err := encodeEmbedding(t.Embedding)
		if err != nil {
			return nil, err
		}
		blobs[i] = b
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, dbErr("failed to begin tx", err)
	}
	defer func() { _ = tx.Rollback() }()

	var workspaceID int64
	if err = tx.QueryRowContext(ctx, `SELECT workspace_id FROM videos WHERE id = ?`, videoID).Scan(&workspaceID); err != nil {
		return nil, dbErr("video not found for tracks", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM tracks WHERE video_id = ?`, videoID); err != nil {
		return nil, dbErr("failed to delete old tracks", err)
	}

	out := make([]domain.Track, len(tracks))
	for i, t := range tracks {
		res, execErr := tx.ExecContext(ctx,
			`INSERT INTO tracks (video_id, start_sec, end_sec, avg_confidence, detection_count, best_bbox, thumbnail_path, embedding, boxes)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			videoID, t.StartSec, t.EndSec, t.AvgConfidence, t.DetectionCount, t.BestBBox, t.ThumbnailPath, blobs[i], encodeBoxes(t.Boxes))
		if execErr != nil {
			return nil, dbErr("failed to insert track", execErr)
		}
		t.ID, _ = res.LastInsertId()
		t.VideoID = videoID
		gres, execErr := tx.ExecContext(ctx, `INSERT INTO person_groups (workspace_id) VALUES (?)`, workspaceID)
		if execErr != nil {
			return nil, dbErr("failed to insert group", execErr)
		}
		gid, _ := gres.LastInsertId()
		if _, execErr = tx.ExecContext(ctx, `INSERT INTO person_group_tracks (group_id, track_id) VALUES (?, ?)`, gid, t.ID); execErr != nil {
			return nil, dbErr("failed to link track to group", execErr)
		}
		out[i] = t
	}
	if err = deleteEmptyGroups(ctx, tx); err != nil {
		return nil, dbErr("failed to delete empty groups", err)
	}
	if err = tx.Commit(); err != nil {
		return nil, dbErr("failed to commit tracks", err)
	}
	return out, nil
}

// ListVideoTrackBoxes returns the stored paths of a video's visible tracks (not removed, with boxes), by start time.
func (r *DetectionRepository) ListVideoTrackBoxes(ctx context.Context, videoID int64) ([]domain.VideoTrackBoxes, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT t.id, gt.group_id, COALESCE(g.label, ''), t.start_sec, t.end_sec, t.boxes
		FROM tracks t
		JOIN person_group_tracks gt ON gt.track_id = t.id
		JOIN person_groups g ON g.id = gt.group_id
		WHERE t.video_id = ? AND t.removed = 0 AND t.boxes IS NOT NULL AND length(t.boxes) > 0
		ORDER BY t.start_sec, t.id`, videoID)
	if err != nil {
		return nil, dbErr("failed to list track boxes", err)
	}
	defer rows.Close()

	out := []domain.VideoTrackBoxes{}
	for rows.Next() {
		var item domain.VideoTrackBoxes
		var blob []byte
		if err = rows.Scan(&item.TrackID, &item.GroupID, &item.Label, &item.StartSec, &item.EndSec, &blob); err != nil {
			return nil, dbErr("failed to scan track boxes", err)
		}
		if item.Boxes, err = decodeBoxes(blob); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err = rows.Err(); err != nil {
		return nil, dbErr("failed to iterate track boxes", err)
	}
	return out, nil
}

func (r *DetectionRepository) ListGroups(ctx context.Context, f domain.GroupFilter) ([]domain.PersonGroup, int, error) {
	page, size := f.Page, f.PageSize
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 50
	}

	const base = `
		SELECT g.id AS id,
		       COALESCE(g.label, '') AS label,
		       SUM(t.end_sec - t.start_sec) AS dur,
		       COALESCE(SUM(t.avg_confidence * t.detection_count) / NULLIF(SUM(t.detection_count), 0), 0) AS conf
		FROM person_groups g
		JOIN person_group_tracks gt ON gt.group_id = g.id
		JOIN tracks t ON t.id = gt.track_id AND t.removed = 0
		WHERE g.workspace_id = ?
		GROUP BY g.id
		HAVING dur >= ? AND conf >= ?`

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM (`+base+`)`, f.WorkspaceID, f.MinDurationSec, f.MinConfidence).Scan(&total); err != nil {
		return nil, 0, dbErr("failed to count groups", err)
	}

	rows, err := r.db.QueryContext(ctx, base+` ORDER BY g.id LIMIT ? OFFSET ?`,
		f.WorkspaceID, f.MinDurationSec, f.MinConfidence, size, (page-1)*size)
	if err != nil {
		return nil, 0, dbErr("failed to list groups", err)
	}
	var groups []domain.PersonGroup
	index := map[int64]int{}
	var ids []any
	for rows.Next() {
		g := domain.PersonGroup{WorkspaceID: f.WorkspaceID, Tracks: []domain.Track{}}
		if err = rows.Scan(&g.ID, &g.Label, &g.TotalDurationSec, &g.AvgConfidence); err != nil {
			_ = rows.Close()
			return nil, 0, dbErr("failed to scan group", err)
		}
		index[g.ID] = len(groups)
		ids = append(ids, g.ID)
		groups = append(groups, g)
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		return nil, 0, dbErr("failed to iterate groups", err)
	}
	_ = rows.Close() // single-conn pool: must close before the next query
	if len(groups) == 0 {
		return []domain.PersonGroup{}, total, nil
	}

	trows, err := r.db.QueryContext(ctx,
		`SELECT gt.group_id, t.id, t.video_id, v.name, t.start_sec, t.end_sec, t.avg_confidence, t.detection_count,
		        t.best_bbox, t.thumbnail_path, t.embedding
		 FROM person_group_tracks gt
		 JOIN tracks t ON t.id = gt.track_id AND t.removed = 0
		 JOIN videos v ON v.id = t.video_id
		 WHERE gt.group_id IN (`+strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")+`)
		 ORDER BY t.start_sec, t.id`, ids...)
	if err != nil {
		return nil, 0, dbErr("failed to list group tracks", err)
	}
	defer trows.Close()
	videos := map[int64]map[int64]bool{}
	for trows.Next() {
		var gid int64
		var t domain.Track
		var emb []byte
		if err = trows.Scan(&gid, &t.ID, &t.VideoID, &t.VideoName, &t.StartSec, &t.EndSec, &t.AvgConfidence,
			&t.DetectionCount, &t.BestBBox, &t.ThumbnailPath, &emb); err != nil {
			return nil, 0, dbErr("failed to scan track", err)
		}
		t.Embedding = decodeEmbedding(emb)
		i := index[gid]
		groups[i].Tracks = append(groups[i].Tracks, t)
		if videos[gid] == nil {
			videos[gid] = map[int64]bool{}
		}
		videos[gid][t.VideoID] = true
	}
	if err = trows.Err(); err != nil {
		return nil, 0, dbErr("failed to iterate tracks", err)
	}
	for gid, i := range index {
		groups[i].VideoCount = len(videos[gid])
	}
	return groups, total, nil
}

func (r *DetectionRepository) MergeGroups(ctx context.Context, groupIDs []int64) (int64, error) {
	uniq := map[int64]bool{}
	for _, id := range groupIDs {
		uniq[id] = true
	}
	if len(uniq) < 2 {
		return 0, apperrors.New(apperrors.CodeValidation, "merge needs at least two groups", nil)
	}
	ids := make([]int64, 0, len(uniq))
	args := make([]any, 0, len(uniq))
	for id := range uniq {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		args = append(args, id)
	}
	in := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, dbErr("failed to begin tx", err)
	}
	defer func() { _ = tx.Rollback() }()

	var found, workspaces int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*), COUNT(DISTINCT workspace_id) FROM person_groups WHERE id IN (`+in+`)`, args...).Scan(&found, &workspaces); err != nil {
		return 0, dbErr("failed to check groups", err)
	}
	if found != len(ids) || workspaces != 1 {
		return 0, apperrors.New(apperrors.CodeValidation, "groups must exist and belong to one workspace", map[string]any{"groupIds": ids})
	}
	target := ids[0]
	// Keep the target's name; if it has none, adopt the first name among the merged groups (lowest id first).
	if _, err = tx.ExecContext(ctx, `
		UPDATE person_groups SET label = (
			SELECT label FROM person_groups WHERE id IN (`+in+`) AND label IS NOT NULL AND label <> '' ORDER BY id LIMIT 1)
		WHERE id = ? AND (label IS NULL OR label = '')`, append(append([]any{}, args...), target)...); err != nil {
		return 0, dbErr("failed to merge group names", err)
	}
	upd := append([]any{target}, args[1:]...)
	if _, err = tx.ExecContext(ctx, `UPDATE person_group_tracks SET group_id = ? WHERE group_id IN (`+strings.TrimSuffix(strings.Repeat("?,", len(ids)-1), ",")+`)`, upd...); err != nil {
		return 0, dbErr("failed to merge groups", err)
	}
	if err = deleteEmptyGroups(ctx, tx); err != nil {
		return 0, dbErr("failed to delete empty groups", err)
	}
	if err = tx.Commit(); err != nil {
		return 0, dbErr("failed to commit merge", err)
	}
	return target, nil
}

func (r *DetectionRepository) SplitTrack(ctx context.Context, groupID int64, trackID int64) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, dbErr("failed to begin tx", err)
	}
	defer func() { _ = tx.Rollback() }()

	var workspaceID int64
	err = tx.QueryRowContext(ctx,
		`SELECT g.workspace_id FROM person_groups g JOIN person_group_tracks gt ON gt.group_id = g.id
		 WHERE g.id = ? AND gt.track_id = ?`, groupID, trackID).Scan(&workspaceID)
	if err == sql.ErrNoRows {
		return 0, apperrors.New(apperrors.CodeValidation, "track is not in that group", map[string]any{"groupId": groupID, "trackId": trackID})
	}
	if err != nil {
		return 0, dbErr("failed to check track group", err)
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO person_groups (workspace_id) VALUES (?)`, workspaceID)
	if err != nil {
		return 0, dbErr("failed to insert group", err)
	}
	newID, _ := res.LastInsertId()
	if _, err = tx.ExecContext(ctx, `UPDATE person_group_tracks SET group_id = ? WHERE track_id = ?`, newID, trackID); err != nil {
		return 0, dbErr("failed to move track", err)
	}
	if err = deleteEmptyGroups(ctx, tx); err != nil {
		return 0, dbErr("failed to delete empty groups", err)
	}
	if err = tx.Commit(); err != nil {
		return 0, dbErr("failed to commit split", err)
	}
	return newID, nil
}

func (r *DetectionRepository) RemoveTrack(ctx context.Context, trackID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return dbErr("failed to begin tx", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `UPDATE tracks SET removed = 1 WHERE id = ?`, trackID)
	if err != nil {
		return dbErr("failed to remove track", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperrors.New(apperrors.CodeValidation, "track not found", map[string]any{"trackId": trackID})
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM person_group_tracks WHERE track_id = ?`, trackID); err != nil {
		return dbErr("failed to unlink track", err)
	}
	if err = deleteEmptyGroups(ctx, tx); err != nil {
		return dbErr("failed to delete empty groups", err)
	}
	if err = tx.Commit(); err != nil {
		return dbErr("failed to commit remove", err)
	}
	return nil
}

// SetGroupLabel names a group; an empty label clears the name (stored as NULL).
func (r *DetectionRepository) SetGroupLabel(ctx context.Context, groupID int64, label string) error {
	var value any
	if label != "" {
		value = label
	}
	res, err := r.db.ExecContext(ctx, `UPDATE person_groups SET label = ? WHERE id = ?`, value, groupID)
	if err != nil {
		return dbErr("failed to set group label", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperrors.New(apperrors.CodeValidation, "person not found", map[string]any{"groupId": groupID})
	}
	return nil
}
