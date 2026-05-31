package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/seeu/backend/internal/domain"
)

type StoryRepository struct {
	db *DB
}

func NewStoryRepository(db *DB) *StoryRepository {
	return &StoryRepository{db: db}
}

func (r *StoryRepository) Create(ctx context.Context, story *domain.Story) error {
	// STORY-1: text-сторис без media_url — кладём NULL чтобы соответствовать
	// nullable-схеме после migration 046. Pgx сам конвертит "" → "" для NOT NULL
	// колонок, но media_url теперь nullable. Поэтому при пустой строке передаём
	// явный nil через pointer-trick.
	var mediaURLArg interface{} = story.MediaURL
	if story.MediaURL == "" {
		mediaURLArg = nil
	}

	// STORY-3: poll сериализуем в JSON (или NULL если отсутствует).
	var pollArg interface{}
	if story.Poll != nil {
		b, err := json.Marshal(story.Poll)
		if err != nil {
			return fmt.Errorf("marshal poll: %w", err)
		}
		pollArg = b
	}

	query := `
		INSERT INTO stories (user_id, media_url, media_type, duration, text_overlay, audio_track_id, shared_post_id, audio_start_seconds, bg_color, poll, is_close_friends_only)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, views_count, likes_count, expires_at, created_at`

	err := r.db.Pool.QueryRow(ctx, query,
		story.UserID,
		mediaURLArg,
		story.MediaType,
		story.Duration,
		story.TextOverlay,
		story.AudioTrackID,
		story.SharedPostID,
		story.AudioStartSeconds,
		story.BgColor,
		pollArg,
		story.IsCloseFriendsOnly,
	).Scan(&story.ID, &story.ViewsCount, &story.LikesCount, &story.ExpiresAt, &story.CreatedAt)

	if err != nil {
		return fmt.Errorf("create story: %w", err)
	}

	return nil
}

func (r *StoryRepository) GetByID(ctx context.Context, id string) (*domain.Story, error) {
	query := `
		SELECT s.id, s.user_id, COALESCE(s.media_url, ''), s.media_type, s.duration, s.text_overlay,
		       s.views_count, s.likes_count, s.audio_track_id, s.shared_post_id, s.audio_start_seconds, COALESCE(s.bg_color, ''), s.poll, COALESCE(s.is_close_friends_only, false), s.expires_at, s.created_at,
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM stories s
		JOIN users u ON u.id = s.user_id
		WHERE s.id = $1`

	story := &domain.Story{User: &domain.UserShort{}}
	var pollRaw []byte
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&story.ID, &story.UserID, &story.MediaURL, &story.MediaType,
		&story.Duration, &story.TextOverlay, &story.ViewsCount, &story.LikesCount,
		&story.AudioTrackID, &story.SharedPostID, &story.AudioStartSeconds, &story.BgColor, &pollRaw, &story.IsCloseFriendsOnly, &story.ExpiresAt, &story.CreatedAt,
		&story.User.ID, &story.User.Username, &story.User.FullName,
		&story.User.AvatarURL, &story.User.IsVerified,
	)
	if err == nil && len(pollRaw) > 0 {
		var p domain.StoryPoll
		if err2 := json.Unmarshal(pollRaw, &p); err2 == nil {
			story.Poll = &p
		}
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrStoryNotFound
		}
		return nil, fmt.Errorf("get story by id: %w", err)
	}

	return story, nil
}

func (r *StoryRepository) Delete(ctx context.Context, id, userID string) error {
	result, err := r.db.Pool.Exec(ctx,
		`DELETE FROM stories WHERE id = $1 AND user_id = $2`,
		id, userID)
	if err != nil {
		return fmt.Errorf("delete story: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrStoryNotFound
	}
	return nil
}

// GetByUsername — stories автора username. PROFILE-3 CF: is_close_friends_only
// stories видны только автору + close_friends. viewerID == "" → анон (CF
// невидимы по умолчанию).
func (r *StoryRepository) GetByUsername(ctx context.Context, username, viewerID string) ([]*domain.Story, error) {
	query := `
		SELECT s.id, s.user_id, COALESCE(s.media_url, ''), s.media_type, s.duration, s.text_overlay,
		       s.views_count, s.likes_count, s.audio_track_id, s.shared_post_id, s.audio_start_seconds, COALESCE(s.bg_color, ''), s.poll, COALESCE(s.is_close_friends_only, false), s.expires_at, s.created_at,
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM stories s
		JOIN users u ON u.id = s.user_id
		WHERE u.username = $1 AND s.expires_at > NOW()
		  AND (
		    NOT COALESCE(s.is_close_friends_only, false)
		    OR s.user_id = $2
		    OR EXISTS (SELECT 1 FROM close_friends cf WHERE cf.owner_id = s.user_id AND cf.friend_id = $2)
		  )
		ORDER BY s.created_at ASC`

	return r.scanStories(ctx, query, username, viewerID)
}

// GetAllByUsername — все сторис автора включая истёкшие (для highlights picker).
// Возвращает только собственные сторис владельца — CF-фильтр не применяется,
// вызывается только когда viewerID == authorID (проверяется в сервисе).
func (r *StoryRepository) GetAllByUsername(ctx context.Context, username string) ([]*domain.Story, error) {
	query := `
		SELECT s.id, s.user_id, COALESCE(s.media_url, ''), s.media_type, s.duration, s.text_overlay,
		       s.views_count, s.likes_count, s.audio_track_id, s.shared_post_id, s.audio_start_seconds, COALESCE(s.bg_color, ''), s.poll, COALESCE(s.is_close_friends_only, false), s.expires_at, s.created_at,
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM stories s
		JOIN users u ON u.id = s.user_id
		WHERE u.username = $1
		ORDER BY s.created_at DESC`

	return r.scanStories(ctx, query, username)
}

func (r *StoryRepository) GetFeed(ctx context.Context, userID string) ([]*domain.Story, error) {
	// PROFILE-3: CF-stories видны только автору + close_friends. WHERE
	// расширен дополнительной OR-веткой через EXISTS(close_friends).
	query := `
		SELECT s.id, s.user_id, COALESCE(s.media_url, ''), s.media_type, s.duration, s.text_overlay,
		       s.views_count, s.likes_count, s.audio_track_id, s.shared_post_id, s.audio_start_seconds, COALESCE(s.bg_color, ''), s.poll, COALESCE(s.is_close_friends_only, false), s.expires_at, s.created_at,
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM stories s
		JOIN users u ON u.id = s.user_id
		WHERE s.expires_at > NOW()
		  AND (s.user_id = $1 OR s.user_id IN (SELECT following_id FROM follows WHERE follower_id = $1))
		  AND (
		    NOT COALESCE(s.is_close_friends_only, false)
		    OR s.user_id = $1
		    OR EXISTS (SELECT 1 FROM close_friends cf WHERE cf.owner_id = s.user_id AND cf.friend_id = $1)
		  )
		ORDER BY s.created_at DESC`

	return r.scanStories(ctx, query, userID)
}

func (r *StoryRepository) AddView(ctx context.Context, storyID, userID string) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var alreadyViewed bool
	err = tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM story_views WHERE story_id = $1 AND user_id = $2)`,
		storyID, userID).Scan(&alreadyViewed)
	if err != nil {
		return err
	}

	if !alreadyViewed {
		_, err = tx.Exec(ctx,
			`INSERT INTO story_views (story_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			storyID, userID)
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx,
			`UPDATE stories SET views_count = views_count + 1 WHERE id = $1`,
			storyID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *StoryRepository) GetViewers(ctx context.Context, storyID string, limit, offset int) ([]*domain.StoryViewer, error) {
	query := `
		SELECT u.id, u.username, u.full_name, u.avatar_url, u.is_verified, sv.viewed_at
		FROM story_views sv
		JOIN users u ON u.id = sv.user_id
		WHERE sv.story_id = $1
		ORDER BY sv.viewed_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Pool.Query(ctx, query, storyID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get story viewers: %w", err)
	}
	defer rows.Close()

	var viewers []*domain.StoryViewer
	for rows.Next() {
		v := &domain.StoryViewer{User: &domain.UserShort{}}
		if err := rows.Scan(
			&v.User.ID, &v.User.Username, &v.User.FullName,
			&v.User.AvatarURL, &v.User.IsVerified, &v.ViewedAt,
		); err != nil {
			return nil, fmt.Errorf("scan story viewer: %w", err)
		}
		viewers = append(viewers, v)
	}

	return viewers, rows.Err()
}

func (r *StoryRepository) IsViewedByUser(ctx context.Context, storyID, userID string) (bool, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM story_views WHERE story_id = $1 AND user_id = $2)`,
		storyID, userID).Scan(&exists)
	return exists, err
}

func (r *StoryRepository) IncrementLikesCount(ctx context.Context, storyID string, delta int) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE stories SET likes_count = likes_count + $1 WHERE id = $2`,
		delta, storyID)
	return err
}

// SetReaction upserts the viewer's emoji reaction on a story — one emoji per
// (story, user), replaced on conflict.
func (r *StoryRepository) SetReaction(ctx context.Context, storyID, userID, emoji string) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO story_reactions (story_id, user_id, emoji)
		VALUES ($1, $2, $3)
		ON CONFLICT (story_id, user_id)
		DO UPDATE SET emoji = EXCLUDED.emoji, created_at = NOW()`,
		storyID, userID, emoji)
	if err != nil {
		return fmt.Errorf("set story reaction: %w", err)
	}
	return nil
}

// RemoveReaction deletes the viewer's reaction. No-op if it didn't exist.
func (r *StoryRepository) RemoveReaction(ctx context.Context, storyID, userID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM story_reactions WHERE story_id = $1 AND user_id = $2`,
		storyID, userID)
	return err
}

// CountReactions returns aggregated counts for one story.
func (r *StoryRepository) CountReactions(ctx context.Context, storyID string) (map[string]int, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT emoji, COUNT(*) FROM story_reactions
		 WHERE story_id = $1 GROUP BY emoji`,
		storyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var emoji string
		var n int
		if err := rows.Scan(&emoji, &n); err != nil {
			return nil, err
		}
		out[emoji] = n
	}
	return out, nil
}

// AttachReactions hydrates Reactions/MyReaction on a batch of stories in a
// single roundtrip. Pass empty currentUserID for anon viewers.
func (r *StoryRepository) AttachReactions(ctx context.Context, stories []*domain.Story, currentUserID string) error {
	if len(stories) == 0 {
		return nil
	}
	ids := make([]string, len(stories))
	for i, s := range stories {
		ids[i] = s.ID
	}
	rows, err := r.db.Pool.Query(ctx,
		`SELECT story_id, emoji, user_id
		 FROM story_reactions
		 WHERE story_id = ANY($1::uuid[])`,
		ids)
	if err != nil {
		return err
	}
	defer rows.Close()

	type bucket struct {
		counts map[string]int
		mine   string
	}
	buckets := make(map[string]*bucket, len(stories))
	for rows.Next() {
		var storyID, emoji, userID string
		if err := rows.Scan(&storyID, &emoji, &userID); err != nil {
			return err
		}
		b, ok := buckets[storyID]
		if !ok {
			b = &bucket{counts: map[string]int{}}
			buckets[storyID] = b
		}
		b.counts[emoji]++
		if userID == currentUserID && currentUserID != "" {
			b.mine = emoji
		}
	}
	for i := range stories {
		if b, ok := buckets[stories[i].ID]; ok {
			stories[i].Reactions = b.counts
			stories[i].MyReaction = b.mine
		}
	}
	return nil
}

// VotePoll (STORY-3) — записать голос viewer'а на poll-overlay.
// optionIndex: 0 = A, 1 = B. Перезаписывает previous vote (один голос на user).
// Возвращает agg counts + my_vote после голосования.
func (r *StoryRepository) VotePoll(
	ctx context.Context, storyID, userID string, optionIndex int,
) (votesA, votesB int, err error) {
	if optionIndex != 0 && optionIndex != 1 {
		return 0, 0, fmt.Errorf("invalid option index: %d", optionIndex)
	}
	_, err = r.db.Pool.Exec(ctx, `
		INSERT INTO story_poll_votes (story_id, user_id, option_index)
		VALUES ($1, $2, $3)
		ON CONFLICT (story_id, user_id)
		DO UPDATE SET option_index = EXCLUDED.option_index, voted_at = NOW()`,
		storyID, userID, optionIndex,
	)
	if err != nil {
		return 0, 0, fmt.Errorf("vote poll: %w", err)
	}
	err = r.db.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE option_index = 0),
			COUNT(*) FILTER (WHERE option_index = 1)
		FROM story_poll_votes WHERE story_id = $1`,
		storyID,
	).Scan(&votesA, &votesB)
	return votesA, votesB, err
}

// AttachPollVotes (STORY-3) — гидратирует Poll.VotesA/VotesB/MyVote на batch
// сторис в один запрос. Не делает ничего для сторис без poll'я.
func (r *StoryRepository) AttachPollVotes(
	ctx context.Context, stories []*domain.Story, viewerID string,
) error {
	if len(stories) == 0 {
		return nil
	}
	ids := make([]string, 0, len(stories))
	for _, s := range stories {
		if s.Poll != nil {
			ids = append(ids, s.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT story_id, option_index, user_id
		FROM story_poll_votes
		WHERE story_id = ANY($1::uuid[])`,
		ids,
	)
	if err != nil {
		return fmt.Errorf("attach poll votes: %w", err)
	}
	defer rows.Close()

	type pollAgg struct {
		votesA, votesB int
		myVote         int // -1 = не голосовал
	}
	aggs := make(map[string]*pollAgg, len(ids))
	for rows.Next() {
		var storyID, userID string
		var idx int
		if err := rows.Scan(&storyID, &idx, &userID); err != nil {
			return err
		}
		a, ok := aggs[storyID]
		if !ok {
			a = &pollAgg{myVote: -1}
			aggs[storyID] = a
		}
		if idx == 0 {
			a.votesA++
		} else {
			a.votesB++
		}
		if userID == viewerID && viewerID != "" {
			a.myVote = idx
		}
	}
	for _, s := range stories {
		if s.Poll == nil {
			continue
		}
		if a, ok := aggs[s.ID]; ok {
			s.Poll.VotesA = a.votesA
			s.Poll.VotesB = a.votesB
			s.Poll.MyVote = a.myVote
		} else {
			s.Poll.MyVote = -1
		}
	}
	return nil
}

func (r *StoryRepository) DeleteExpired(ctx context.Context) (int64, error) {
	result, err := r.db.Pool.Exec(ctx,
		`DELETE FROM stories WHERE expires_at < $1`,
		time.Now())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

func (r *StoryRepository) scanStories(ctx context.Context, query string, args ...interface{}) ([]*domain.Story, error) {
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query stories: %w", err)
	}
	defer rows.Close()

	var stories []*domain.Story
	for rows.Next() {
		s := &domain.Story{User: &domain.UserShort{}}
		var pollRaw []byte
		if err := rows.Scan(
			&s.ID, &s.UserID, &s.MediaURL, &s.MediaType,
			&s.Duration, &s.TextOverlay, &s.ViewsCount, &s.LikesCount,
			&s.AudioTrackID, &s.SharedPostID, &s.AudioStartSeconds, &s.BgColor, &pollRaw, &s.IsCloseFriendsOnly, &s.ExpiresAt, &s.CreatedAt,
			&s.User.ID, &s.User.Username, &s.User.FullName,
			&s.User.AvatarURL, &s.User.IsVerified,
		); err != nil {
			return nil, fmt.Errorf("scan story: %w", err)
		}
		if len(pollRaw) > 0 {
			var p domain.StoryPoll
			if err2 := json.Unmarshal(pollRaw, &p); err2 == nil {
				s.Poll = &p
			}
		}
		stories = append(stories, s)
	}

	return stories, rows.Err()
}
