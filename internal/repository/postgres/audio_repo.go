package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/seeu/backend/internal/domain"
)

type AudioRepository struct {
	db *DB
}

func NewAudioRepository(db *DB) *AudioRepository {
	return &AudioRepository{db: db}
}

const audioSelect = `
	id, title, artist, cover_url, audio_url, duration_seconds, uses_count,
	genre, created_at, COALESCE(user_id::text, ''), status, rejection_reason,
	COALESCE(lyrics_lrc, ''), COALESCE(likes_count, 0)`

func scanAudio(rows pgx.Rows) (*domain.AudioTrack, error) {
	t := &domain.AudioTrack{}
	if err := rows.Scan(&t.ID, &t.Title, &t.Artist, &t.CoverURL, &t.AudioURL,
		&t.DurationSeconds, &t.UsesCount, &t.Genre, &t.CreatedAt,
		&t.UserID, &t.Status, &t.RejectionReason, &t.LyricsLRC, &t.LikesCount); err != nil {
		return nil, err
	}
	return t, nil
}

// RecordPlay (MUSIC-3) — записать прослушивание для smart-playlists и daily-mix.
// Frontend шлёт POST после ~5 сек воспроизведения. Multiple-ins одного и
// того же track'а в день = OK (нужны для recent-history + duration_played_sec).
func (r *AudioRepository) RecordPlay(
	ctx context.Context, userID, trackID string, durationSec int,
) error {
	if userID == "" || trackID == "" {
		return nil
	}
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO play_history (user_id, track_id, duration_played_sec)
		VALUES ($1, $2, $3)`,
		userID, trackID, durationSec,
	)
	if err != nil {
		return fmt.Errorf("record play: %w", err)
	}
	return nil
}

// RecentPlayed (MUSIC-3) — последние уникальные треки юзера.
// DISTINCT ON track_id чтобы один трек прослушанный N раз — один row.
func (r *AudioRepository) RecentPlayed(
	ctx context.Context, userID string, limit int,
) ([]*domain.AudioTrack, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT DISTINCT ON (t.id) `+audioSelect+`
		FROM audio_tracks t
		JOIN play_history h ON h.track_id = t.id
		WHERE h.user_id = $1 AND t.status = 'approved'
		ORDER BY t.id, h.played_at DESC
		LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("recent played: %w", err)
	}
	defer rows.Close()
	var tracks []*domain.AudioTrack
	for rows.Next() {
		t, err := scanAudio(rows)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

// LikedTracks (MUSIC-3) — треки на которые user положил heart-like.
// Использует polymorphic `likes` table (entity_type='audio_track').
func (r *AudioRepository) LikedTracks(
	ctx context.Context, userID string, limit int,
) ([]*domain.AudioTrack, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT `+audioSelect+`
		FROM audio_tracks t
		JOIN likes l ON l.entity_id = t.id AND l.entity_type = 'audio_track'
		WHERE l.user_id = $1 AND t.status = 'approved'
		ORDER BY l.created_at DESC
		LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("liked tracks: %w", err)
	}
	defer rows.Close()
	var tracks []*domain.AudioTrack
	for rows.Next() {
		t, err := scanAudio(rows)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

// DailyMix (MUSIC-4) — персональный mix на основе top-artists/genres юзера
// за последний месяц + случайные треки из тех же категорий.
//
// Алгоритм:
//  1. Top-N жанров юзера за 30 дней (по count of plays).
//  2. Random треки из этих жанров (исключая only-recently played чтобы не
//     повторяться). Если жанров мало — fall back на global popular.
//
// Deterministic seed по date чтобы один и тот же mix целый день — refresh
// в полночь автоматически (frontend не кэширует, бэк выдаёт новый).
func (r *AudioRepository) DailyMix(
	ctx context.Context, userID string, limit int,
) ([]*domain.AudioTrack, error) {
	rows, err := r.db.Pool.Query(ctx, `
		WITH user_genres AS (
			SELECT t.genre, COUNT(*) AS plays
			FROM play_history h
			JOIN audio_tracks t ON t.id = h.track_id
			WHERE h.user_id = $1
			  AND h.played_at > NOW() - INTERVAL '30 days'
			  AND t.genre IS NOT NULL AND t.genre <> ''
			GROUP BY t.genre
			ORDER BY plays DESC
			LIMIT 3
		),
		recent_track_ids AS (
			SELECT DISTINCT track_id FROM play_history
			WHERE user_id = $1
			  AND played_at > NOW() - INTERVAL '24 hours'
		)
		SELECT `+audioSelect+`
		FROM audio_tracks t
		WHERE t.status = 'approved'
		  AND t.id NOT IN (SELECT track_id FROM recent_track_ids)
		  AND (
		    (SELECT COUNT(*) FROM user_genres) = 0
		    OR t.genre IN (SELECT genre FROM user_genres)
		  )
		ORDER BY md5(t.id::text || CURRENT_DATE::text)
		LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("daily mix: %w", err)
	}
	defer rows.Close()
	var tracks []*domain.AudioTrack
	for rows.Next() {
		t, err := scanAudio(rows)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

// GetAll returns publicly visible (approved) tracks.
func (r *AudioRepository) GetAll(ctx context.Context, limit, offset int) ([]*domain.AudioTrack, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT `+audioSelect+`
		FROM audio_tracks
		WHERE status = 'approved'
		ORDER BY uses_count DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []*domain.AudioTrack
	for rows.Next() {
		t, err := scanAudio(rows)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, nil
}

// Search filters approved tracks by title/artist.
func (r *AudioRepository) Search(ctx context.Context, query string, limit, offset int) ([]*domain.AudioTrack, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT `+audioSelect+`
		FROM audio_tracks
		WHERE status = 'approved' AND (title ILIKE $1 OR artist ILIKE $1)
		ORDER BY uses_count DESC
		LIMIT $2 OFFSET $3`, "%"+query+"%", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []*domain.AudioTrack
	for rows.Next() {
		t, err := scanAudio(rows)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, nil
}

// Create inserts a user-uploaded track in 'pending' state. Pre-uploaded
// audio_url/cover_url come from /media/upload.
func (r *AudioRepository) Create(ctx context.Context, t *domain.AudioTrack) error {
	if t.Status == "" {
		t.Status = "pending"
	}
	err := r.db.Pool.QueryRow(ctx, `
		INSERT INTO audio_tracks (title, artist, cover_url, audio_url, duration_seconds,
		                          genre, user_id, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, uses_count`,
		t.Title, t.Artist, t.CoverURL, t.AudioURL, t.DurationSeconds, t.Genre,
		t.UserID, t.Status,
	).Scan(&t.ID, &t.CreatedAt, &t.UsesCount)
	if err != nil {
		return fmt.Errorf("create audio track: %w", err)
	}
	return nil
}

func (r *AudioRepository) GetByID(ctx context.Context, id string) (*domain.AudioTrack, error) {
	rows, err := r.db.Pool.Query(ctx, `SELECT `+audioSelect+` FROM audio_tracks WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, domain.ErrNotFound
	}
	return scanAudio(rows)
}

// ListByUser returns all tracks uploaded by a user (any status).
func (r *AudioRepository) ListByUser(ctx context.Context, userID string) ([]*domain.AudioTrack, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT `+audioSelect+`
		FROM audio_tracks
		WHERE user_id = $1
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.AudioTrack
	for rows.Next() {
		t, err := scanAudio(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// AdminList returns user-uploaded tracks filtered by status (used by moderator).
// Pass "" or "all" to include any status.
func (r *AudioRepository) AdminList(ctx context.Context, status string, limit, offset int) ([]*domain.AudioTrack, error) {
	if limit <= 0 {
		limit = 50
	}
	q := `
		SELECT ` + audioSelect + `
		FROM audio_tracks
		WHERE user_id IS NOT NULL`
	args := []any{}
	if status != "" && status != "all" {
		args = append(args, status)
		q += fmt.Sprintf(" AND status = $%d", len(args))
	}
	args = append(args, limit, offset)
	q += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := r.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.AudioTrack
	for rows.Next() {
		t, err := scanAudio(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// SetStatus moves a track between pending → approved/rejected. Records
// reviewer + reason. Returns ErrNotFound when the track doesn't exist.
func (r *AudioRepository) SetStatus(ctx context.Context, id, status, reason, reviewerID string) error {
	if status != "approved" && status != "rejected" && status != "pending" {
		return errors.New("invalid status")
	}
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE audio_tracks SET
			status = $2,
			rejection_reason = $3,
			reviewed_at = NOW(),
			reviewed_by = $4
		WHERE id = $1`, id, status, reason, reviewerID)
	if err != nil {
		return fmt.Errorf("set audio status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *AudioRepository) GetTrendingTags(ctx context.Context, limit int) ([]*domain.TrendingTag, error) {
	query := `
		SELECT tag, COUNT(*) as cnt
		FROM (
			SELECT DISTINCT p.id, unnest(regexp_matches(p.caption, '#(\w+)', 'g')) AS tag
			FROM posts p
			JOIN users u ON u.id = p.user_id
			WHERE u.is_private = false AND p.caption LIKE '%#%'
		) sub
		GROUP BY tag
		ORDER BY cnt DESC, tag
		LIMIT $1`

	rows, err := r.db.Pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []*domain.TrendingTag
	for rows.Next() {
		t := &domain.TrendingTag{}
		if err := rows.Scan(&t.Tag, &t.PostsCount); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, nil
}

// LikeTrack ставит лайк userID на трек trackID.
// Idempotent: ON CONFLICT DO NOTHING. Возвращает isNew=true если лайк новый.
func (r *AudioRepository) LikeTrack(ctx context.Context, trackID, userID string) (bool, error) {
	var isNew bool
	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		tag, txErr := tx.Exec(ctx, `
			INSERT INTO likes (user_id, entity_id, entity_type)
			VALUES ($1, $2, 'audio_track')
			ON CONFLICT (user_id, entity_id, entity_type) DO NOTHING`,
			userID, trackID)
		if txErr != nil {
			return fmt.Errorf("insert audio like: %w", txErr)
		}
		if tag.RowsAffected() == 0 {
			return nil // already liked
		}
		isNew = true
		_, txErr = tx.Exec(ctx,
			`UPDATE audio_tracks SET likes_count = likes_count + 1 WHERE id = $1`, trackID)
		return txErr
	})
	return isNew, err
}

// UnlikeTrack убирает лайк userID с трека trackID.
func (r *AudioRepository) UnlikeTrack(ctx context.Context, trackID, userID string) error {
	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			DELETE FROM likes WHERE user_id = $1 AND entity_id = $2 AND entity_type = 'audio_track'`,
			userID, trackID)
		if err != nil {
			return fmt.Errorf("delete audio like: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return nil // уже не лайкнут — ок
		}
		_, err = tx.Exec(ctx,
			`UPDATE audio_tracks SET likes_count = GREATEST(likes_count - 1, 0) WHERE id = $1`, trackID)
		return err
	})
}

// IsTrackLiked проверяет лайкнул ли userID трек trackID.
func (r *AudioRepository) IsTrackLiked(ctx context.Context, trackID, userID string) (bool, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM likes WHERE user_id = $1 AND entity_id = $2 AND entity_type = 'audio_track')`,
		userID, trackID,
	).Scan(&exists)
	return exists, err
}
