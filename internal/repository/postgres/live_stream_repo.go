package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/seeu/backend/internal/domain"
)

type LiveStreamRepository struct {
	db *DB
}

func NewLiveStreamRepository(db *DB) *LiveStreamRepository {
	return &LiveStreamRepository{db: db}
}

// Create starts a new live stream. Returns ErrAlreadyStreaming if user already
// has one active.
func (r *LiveStreamRepository) Create(ctx context.Context, userID, title string) (*domain.LiveStream, error) {
	var existingID string
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id FROM live_streams WHERE user_id = $1 AND status = 'live' LIMIT 1`,
		userID,
	).Scan(&existingID)
	if err == nil {
		return nil, domain.ErrAlreadyStreaming
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// Insert then re-read with the user JOIN so the returned stream carries
	// username/full_name/avatar_url. Без этого fan-out `live_stream.started`
	// уходил подписчикам с пустым именем/аватаром (баннер «@ начал(а) эфир»).
	var s domain.LiveStream
	err = r.db.Pool.QueryRow(ctx,
		`WITH ins AS (
		     INSERT INTO live_streams (user_id, title)
		     VALUES ($1, $2)
		     RETURNING id, user_id, title, status, viewer_count, started_at
		 )
		 SELECT ins.id, ins.user_id::text, u.username,
		        COALESCE(u.full_name, ''), COALESCE(u.avatar_url, ''),
		        ins.title, ins.status, ins.viewer_count, ins.started_at
		 FROM ins JOIN users u ON u.id = ins.user_id`,
		userID, title,
	).Scan(&s.ID, &s.UserID, &s.Username, &s.FullName, &s.AvatarURL,
		&s.Title, &s.Status, &s.ViewerCount, &s.StartedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// End marks a stream as ended. Only the owner can end it.
func (r *LiveStreamRepository) End(ctx context.Context, streamID, userID string) error {
	tag, err := r.db.Pool.Exec(ctx,
		`UPDATE live_streams
		 SET status = 'ended', ended_at = NOW()
		 WHERE id = $1 AND user_id = $2 AND status = 'live'`,
		streamID, userID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrStreamNotFound
	}
	return nil
}

// GetActive returns all currently live streams enriched with user info.
func (r *LiveStreamRepository) GetActive(ctx context.Context) ([]domain.LiveStream, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT ls.id, ls.user_id::text, u.username, COALESCE(u.full_name,''), COALESCE(u.avatar_url,''),
		        ls.title, ls.status, ls.viewer_count, ls.started_at
		 FROM live_streams ls
		 JOIN users u ON u.id = ls.user_id
		 WHERE ls.status = 'live'
		 ORDER BY ls.viewer_count DESC, ls.started_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var streams []domain.LiveStream
	for rows.Next() {
		var s domain.LiveStream
		if err := rows.Scan(&s.ID, &s.UserID, &s.Username, &s.FullName, &s.AvatarURL,
			&s.Title, &s.Status, &s.ViewerCount, &s.StartedAt); err != nil {
			return nil, err
		}
		streams = append(streams, s)
	}
	return streams, rows.Err()
}

// GetByID returns a single stream enriched with user info.
func (r *LiveStreamRepository) GetByID(ctx context.Context, streamID string) (*domain.LiveStream, error) {
	var s domain.LiveStream
	err := r.db.Pool.QueryRow(ctx,
		`SELECT ls.id, ls.user_id::text, u.username, COALESCE(u.full_name,''), COALESCE(u.avatar_url,''),
		        ls.title, ls.status, ls.viewer_count, ls.started_at, ls.ended_at
		 FROM live_streams ls
		 JOIN users u ON u.id = ls.user_id
		 WHERE ls.id = $1`,
		streamID,
	).Scan(&s.ID, &s.UserID, &s.Username, &s.FullName, &s.AvatarURL,
		&s.Title, &s.Status, &s.ViewerCount, &s.StartedAt, &s.EndedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrStreamNotFound
	}
	return &s, err
}

// GetByUserID returns the active stream for a user, or ErrStreamNotFound.
func (r *LiveStreamRepository) GetByUserID(ctx context.Context, userID string) (*domain.LiveStream, error) {
	var s domain.LiveStream
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, user_id::text, title, status, viewer_count, started_at
		 FROM live_streams WHERE user_id = $1 AND status = 'live' LIMIT 1`,
		userID,
	).Scan(&s.ID, &s.UserID, &s.Title, &s.Status, &s.ViewerCount, &s.StartedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrStreamNotFound
	}
	return &s, err
}

// AddViewer inserts viewer and bumps viewer_count. Returns the new count.
func (r *LiveStreamRepository) AddViewer(ctx context.Context, streamID, userID string) (int, error) {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO live_stream_viewers (stream_id, user_id) VALUES ($1, $2)
		 ON CONFLICT (stream_id, user_id) DO NOTHING`,
		streamID, userID,
	)
	if err != nil {
		return 0, err
	}
	var count int
	err = r.db.Pool.QueryRow(ctx,
		`UPDATE live_streams SET viewer_count = (
		   SELECT COUNT(*) FROM live_stream_viewers WHERE stream_id = $1
		 ) WHERE id = $1 RETURNING viewer_count`,
		streamID,
	).Scan(&count)
	return count, err
}

// RemoveViewer deletes viewer and decrements viewer_count. Returns new count.
func (r *LiveStreamRepository) RemoveViewer(ctx context.Context, streamID, userID string) (int, error) {
	r.db.Pool.Exec(ctx, //nolint:errcheck
		`DELETE FROM live_stream_viewers WHERE stream_id = $1 AND user_id = $2`,
		streamID, userID,
	)
	var count int
	err := r.db.Pool.QueryRow(ctx,
		`UPDATE live_streams SET viewer_count = (
		   SELECT COUNT(*) FROM live_stream_viewers WHERE stream_id = $1
		 ) WHERE id = $1 RETURNING viewer_count`,
		streamID,
	).Scan(&count)
	return count, err
}

// GetViewerIDs returns all current viewer user_ids for WS fan-out.
func (r *LiveStreamRepository) GetViewerIDs(ctx context.Context, streamID string) ([]string, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT user_id::text FROM live_stream_viewers WHERE stream_id = $1`,
		streamID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetViewerPreview returns the first N viewers with user info (avatar stack).
func (r *LiveStreamRepository) GetViewerPreview(ctx context.Context, streamID string, limit int) ([]domain.LiveStreamViewer, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT v.user_id::text, u.username, COALESCE(u.full_name,''), COALESCE(u.avatar_url,'')
		 FROM live_stream_viewers v
		 JOIN users u ON u.id = v.user_id
		 WHERE v.stream_id = $1
		 ORDER BY v.joined_at DESC
		 LIMIT $2`,
		streamID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var viewers []domain.LiveStreamViewer
	for rows.Next() {
		var v domain.LiveStreamViewer
		if err := rows.Scan(&v.UserID, &v.Username, &v.FullName, &v.AvatarURL); err != nil {
			return nil, err
		}
		viewers = append(viewers, v)
	}
	return viewers, rows.Err()
}

// GetActiveIDsByUser returns ids of all currently-live streams owned by user.
// Used on WS disconnect to notify viewers their stream is gone.
func (r *LiveStreamRepository) GetActiveIDsByUser(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id FROM live_streams WHERE user_id = $1 AND status = 'live'`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// IsStreamParticipant reports whether userID is the broadcaster of the stream
// or one of its current viewers. Anti-spoof guard for WebRTC signal relay.
func (r *LiveStreamRepository) IsStreamParticipant(ctx context.Context, streamID, userID string) (bool, error) {
	var ok bool
	err := r.db.Pool.QueryRow(ctx,
		`SELECT EXISTS (
		     SELECT 1 FROM live_streams      WHERE id = $1 AND user_id = $2
		     UNION ALL
		     SELECT 1 FROM live_stream_viewers WHERE stream_id = $1 AND user_id = $2
		 )`,
		streamID, userID,
	).Scan(&ok)
	return ok, err
}

// EndAllByUser ends all active streams for a user (cleanup on reconnect).
func (r *LiveStreamRepository) EndAllByUser(ctx context.Context, userID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE live_streams SET status = 'ended', ended_at = NOW()
		 WHERE user_id = $1 AND status = 'live'`,
		userID,
	)
	return err
}
