package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/seeu/backend/internal/domain"
)

type PlaylistRepository struct {
	db *DB
}

func NewPlaylistRepository(db *DB) *PlaylistRepository {
	return &PlaylistRepository{db: db}
}

func (r *PlaylistRepository) Create(ctx context.Context, userID, name string) (*domain.Playlist, error) {
	p := &domain.Playlist{}
	err := r.db.Pool.QueryRow(ctx, `
		INSERT INTO playlists (user_id, name)
		VALUES ($1, $2)
		RETURNING id, user_id, name, cover_url, tracks_count, created_at, updated_at`,
		userID, name,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.CoverURL, &p.TracksCount, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create playlist: %w", err)
	}
	return p, nil
}

func (r *PlaylistRepository) ListByUser(ctx context.Context, userID string) ([]*domain.Playlist, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, user_id, name, cover_url, tracks_count, created_at, updated_at
		FROM playlists WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list playlists: %w", err)
	}
	defer rows.Close()

	var out []*domain.Playlist
	for rows.Next() {
		p := &domain.Playlist{}
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.CoverURL, &p.TracksCount,
			&p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (r *PlaylistRepository) GetByID(ctx context.Context, id string) (*domain.Playlist, error) {
	p := &domain.Playlist{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, user_id, name, cover_url, tracks_count, created_at, updated_at
		FROM playlists WHERE id = $1`, id).Scan(
		&p.ID, &p.UserID, &p.Name, &p.CoverURL, &p.TracksCount, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPlaylistNotFound
		}
		return nil, err
	}
	return p, nil
}

func (r *PlaylistRepository) Update(ctx context.Context, id string, name, coverURL *string) error {
	if name == nil && coverURL == nil {
		return nil
	}
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE playlists SET
			name = COALESCE($2, name),
			cover_url = COALESCE($3, cover_url),
			updated_at = NOW()
		WHERE id = $1`, id, name, coverURL)
	if err != nil {
		return fmt.Errorf("update playlist: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrPlaylistNotFound
	}
	return nil
}

func (r *PlaylistRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM playlists WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete playlist: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrPlaylistNotFound
	}
	return nil
}

func (r *PlaylistRepository) GetTracks(ctx context.Context, playlistID string) ([]*domain.AudioTrack, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT t.id, t.title, t.artist, t.cover_url, t.audio_url,
		       t.duration_seconds, t.uses_count, t.genre, t.created_at
		FROM playlist_tracks pt
		JOIN audio_tracks t ON t.id = pt.track_id
		WHERE pt.playlist_id = $1
		ORDER BY pt.position, pt.added_at`, playlistID)
	if err != nil {
		return nil, fmt.Errorf("get playlist tracks: %w", err)
	}
	defer rows.Close()

	var out []*domain.AudioTrack
	for rows.Next() {
		t := &domain.AudioTrack{}
		if err := rows.Scan(&t.ID, &t.Title, &t.Artist, &t.CoverURL, &t.AudioURL,
			&t.DurationSeconds, &t.UsesCount, &t.Genre, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// AddTrack inserts a (playlist, track) row idempotently and refreshes
// tracks_count + cover_url (if cover was empty) inside one transaction.
func (r *PlaylistRepository) AddTrack(ctx context.Context, playlistID, trackID string) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var nextPos int
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(position) + 1, 0) FROM playlist_tracks WHERE playlist_id = $1`,
		playlistID).Scan(&nextPos)
	if err != nil {
		return fmt.Errorf("compute position: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO playlist_tracks (playlist_id, track_id, position)
		VALUES ($1, $2, $3)
		ON CONFLICT (playlist_id, track_id) DO NOTHING`,
		playlistID, trackID, nextPos,
	); err != nil {
		return fmt.Errorf("insert playlist track: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE playlists p SET
			tracks_count = (SELECT COUNT(*) FROM playlist_tracks WHERE playlist_id = p.id),
			cover_url = CASE
				WHEN p.cover_url = '' THEN
					COALESCE((SELECT t.cover_url FROM audio_tracks t WHERE t.id = $2), '')
				ELSE p.cover_url
			END,
			updated_at = NOW()
		WHERE p.id = $1`, playlistID, trackID,
	); err != nil {
		return fmt.Errorf("refresh playlist counters: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *PlaylistRepository) RemoveTrack(ctx context.Context, playlistID, trackID string) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		DELETE FROM playlist_tracks WHERE playlist_id = $1 AND track_id = $2`,
		playlistID, trackID,
	); err != nil {
		return fmt.Errorf("delete playlist track: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE playlists p SET
			tracks_count = (SELECT COUNT(*) FROM playlist_tracks WHERE playlist_id = p.id),
			updated_at = NOW()
		WHERE p.id = $1`, playlistID,
	); err != nil {
		return fmt.Errorf("refresh playlist counters: %w", err)
	}

	return tx.Commit(ctx)
}
