package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/seeu/backend/internal/domain"
)

type PostRepository struct {
	db *DB
}

func NewPostRepository(db *DB) *PostRepository {
	return &PostRepository{db: db}
}

func (r *PostRepository) Create(ctx context.Context, post *domain.Post) error {
	query := `
		INSERT INTO posts (user_id, caption, media_urls, media_types, location, thumbnail_url,
		                   audio_track_id)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, '')::uuid)
		RETURNING id, likes_count, comments_count, saves_count, created_at, updated_at`

	err := r.db.Pool.QueryRow(ctx, query,
		post.UserID,
		post.Caption,
		post.MediaURLs,
		post.MediaTypes,
		post.Location,
		post.ThumbnailURL,
		post.AudioTrackID,
	).Scan(&post.ID, &post.LikesCount, &post.CommentsCount, &post.SavesCount, &post.CreatedAt, &post.UpdatedAt)

	if err != nil {
		return fmt.Errorf("create post: %w", err)
	}

	return nil
}

func (r *PostRepository) GetByID(ctx context.Context, id string) (*domain.Post, error) {
	query := `
		SELECT p.id, p.user_id, p.caption, p.media_urls, p.media_types, p.location,
		       p.thumbnail_url, p.likes_count, p.comments_count, p.saves_count, p.created_at, p.updated_at,
		       COALESCE(p.audio_track_id::text, ''),
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM posts p
		JOIN users u ON u.id = p.user_id
		WHERE p.id = $1`

	post := &domain.Post{User: &domain.UserShort{}}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&post.ID, &post.UserID, &post.Caption, &post.MediaURLs, &post.MediaTypes,
		&post.Location, &post.ThumbnailURL, &post.LikesCount, &post.CommentsCount, &post.SavesCount,
		&post.CreatedAt, &post.UpdatedAt, &post.AudioTrackID,
		&post.User.ID, &post.User.Username, &post.User.FullName,
		&post.User.AvatarURL, &post.User.IsVerified,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPostNotFound
		}
		return nil, fmt.Errorf("get post by id: %w", err)
	}

	return post, nil
}

func (r *PostRepository) Delete(ctx context.Context, id, userID string) error {
	result, err := r.db.Pool.Exec(ctx,
		`DELETE FROM posts WHERE id = $1 AND user_id = $2`,
		id, userID)
	if err != nil {
		return fmt.Errorf("delete post: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrPostNotFound
	}
	return nil
}

func (r *PostRepository) GetByUserID(ctx context.Context, userID string, limit, offset int) ([]*domain.Post, error) {
	query := `
		SELECT p.id, p.user_id, p.caption, p.media_urls, p.media_types, p.location,
		       p.thumbnail_url, p.likes_count, p.comments_count, p.saves_count, p.created_at, p.updated_at,
		       COALESCE(p.audio_track_id::text, ''),
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM posts p
		JOIN users u ON u.id = p.user_id
		WHERE p.user_id = $1
		ORDER BY p.created_at DESC
		LIMIT $2 OFFSET $3`

	return r.scanPosts(ctx, query, userID, limit, offset)
}

func (r *PostRepository) GetFeed(ctx context.Context, userID string, limit, offset int) ([]*domain.Post, error) {
	// Hide posts from any user that is in a block relationship with the viewer
	// (in either direction) and from banned authors.
	query := `
		SELECT p.id, p.user_id, p.caption, p.media_urls, p.media_types, p.location,
		       p.thumbnail_url, p.likes_count, p.comments_count, p.saves_count, p.created_at, p.updated_at,
		       COALESCE(p.audio_track_id::text, ''),
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM posts p
		JOIN users u ON u.id = p.user_id
		WHERE u.is_banned = false
		  AND (p.user_id = $1 OR p.user_id IN (SELECT following_id FROM follows WHERE follower_id = $1))
		  AND NOT EXISTS (
		      SELECT 1 FROM user_blocks
		      WHERE (blocker_id = $1 AND blocked_id = p.user_id)
		         OR (blocker_id = p.user_id AND blocked_id = $1)
		  )
		ORDER BY p.created_at DESC
		LIMIT $2 OFFSET $3`

	return r.scanPosts(ctx, query, userID, limit, offset)
}

// MarkPostViewed (FEED-5) — записать или обновить post_views row для (postID,
// userID). Idempotent через ON CONFLICT — повторный view одного и того же
// поста just refresh'ит viewed_at (для будущей возможности «recently viewed»).
func (r *PostRepository) MarkPostViewed(
	ctx context.Context, postID, userID string,
) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO post_views (post_id, user_id, viewed_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (post_id, user_id) DO UPDATE SET viewed_at = NOW()`,
		postID, userID)
	if err != nil {
		return fmt.Errorf("mark post viewed: %w", err)
	}
	return nil
}

// GetFeedSmart (FEED-2) — score-based ranking для умной ленты.
//
// Score формула:
//   log10(likes+1) + 0.5*log10(comments+1) + 1.5*log10(saves+1) - 0.5*hours_age
//
// (Spec упоминал views_count, но его в схеме нет — saves используется как
// proxy. Bonus за followed-creator не нужен — feed уже фильтрует только
// follow'ов + сам сyузер).
//
// Использует offset-pagination (не cursor) — score не monotonic по времени,
// cursor по нему был бы fragile.
func (r *PostRepository) GetFeedSmart(
	ctx context.Context, userID string, limit, offset int,
) ([]*domain.Post, error) {
	query := `
		SELECT p.id, p.user_id, p.caption, p.media_urls, p.media_types, p.location,
		       p.thumbnail_url, p.likes_count, p.comments_count, p.saves_count, p.created_at, p.updated_at,
		       COALESCE(p.audio_track_id::text, ''),
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM posts p
		JOIN users u ON u.id = p.user_id
		WHERE u.is_banned = false
		  AND (p.user_id = $1 OR p.user_id IN (SELECT following_id FROM follows WHERE follower_id = $1))
		  AND NOT EXISTS (
		      SELECT 1 FROM user_blocks
		      WHERE (blocker_id = $1 AND blocked_id = p.user_id)
		         OR (blocker_id = p.user_id AND blocked_id = $1)
		  )
		ORDER BY (
			LOG(10, p.likes_count + 1)
			+ 0.5 * LOG(10, p.comments_count + 1)
			+ 1.5 * LOG(10, p.saves_count + 1)
			- 0.5 * EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 3600.0
		) DESC, p.created_at DESC
		LIMIT $2 OFFSET $3`
	return r.scanPosts(ctx, query, userID, limit, offset)
}

// GetFeedByCursor (FEED-1) — стабильная pagination через `(created_at, id)`
// composite key. Гарантирует no-dup / no-skip даже когда new posts падают
// между запросами. beforeTime/beforeID — последний элемент предыдущей страницы;
// для first page обе пустые/zero → query без cursor-фильтра.
//
// FEED-5: фильтрует уже-viewed посты через NOT EXISTS post_views — юзер не
// получает второй раз то что уже скроллил.
func (r *PostRepository) GetFeedByCursor(
	ctx context.Context, userID string,
	beforeTime time.Time, beforeID string,
	limit int,
) ([]*domain.Post, error) {
	cursorFilter := ""
	args := []any{userID}
	if !beforeTime.IsZero() && beforeID != "" {
		// `(a, b) < (c, d)` в postgres = lex compare → выбирает строго раньше
		// (created_at, id) тапла. Гарантирует stable ordering при дупликатах
		// timestamp'ов (один батч insert'ов с тем же ms).
		cursorFilter = "AND (p.created_at, p.id) < ($2::timestamptz, $3::uuid)"
		args = append(args, beforeTime, beforeID)
	}
	args = append(args, limit)
	limitParam := fmt.Sprintf("$%d", len(args))

	query := fmt.Sprintf(`
		SELECT p.id, p.user_id, p.caption, p.media_urls, p.media_types, p.location,
		       p.thumbnail_url, p.likes_count, p.comments_count, p.saves_count, p.created_at, p.updated_at,
		       COALESCE(p.audio_track_id::text, ''),
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM posts p
		JOIN users u ON u.id = p.user_id
		WHERE u.is_banned = false
		  AND (p.user_id = $1 OR p.user_id IN (SELECT following_id FROM follows WHERE follower_id = $1))
		  AND NOT EXISTS (
		      SELECT 1 FROM user_blocks
		      WHERE (blocker_id = $1 AND blocked_id = p.user_id)
		         OR (blocker_id = p.user_id AND blocked_id = $1)
		  )
		  AND NOT EXISTS (
		      SELECT 1 FROM post_views v
		      WHERE v.post_id = p.id AND v.user_id = $1
		  )
		  %s
		ORDER BY p.created_at DESC, p.id DESC
		LIMIT %s`, cursorFilter, limitParam)

	return r.scanPosts(ctx, query, args...)
}

func (r *PostRepository) GetExplore(ctx context.Context, userID string, limit, offset int, mediaType ...string) ([]*domain.Post, error) {
	// Explore = всё, что юзеру можно увидеть: посты (фото + видео-посты)
	// от всех публичных не-забаненных авторов, кроме своих и блокировок.
	//
	// mediaType (optional): "video" → only posts containing at least one video
	// media element. Used by Reels feed.
	var uid any
	if userID != "" {
		uid = userID
	}
	mt := ""
	if len(mediaType) > 0 {
		mt = mediaType[0]
	}
	mediaFilter := ""
	if mt == "video" {
		mediaFilter = "AND 'video' = ANY(p.media_types)"
	} else if mt == "image" {
		mediaFilter = "AND NOT 'video' = ANY(p.media_types)"
	}
	query := `
		SELECT p.id, p.user_id, p.caption, p.media_urls, p.media_types, p.location,
		       p.thumbnail_url, p.likes_count, p.comments_count, p.saves_count, p.created_at, p.updated_at,
		       COALESCE(p.audio_track_id::text, ''),
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM posts p
		JOIN users u ON u.id = p.user_id
		WHERE u.is_private = false
		  AND u.is_banned = false
		  AND ($1::uuid IS NULL OR p.user_id != $1::uuid)
		  AND NOT EXISTS (
		      SELECT 1 FROM user_blocks
		      WHERE $1::uuid IS NOT NULL AND (
		            (blocker_id = $1::uuid AND blocked_id = p.user_id)
		         OR (blocker_id = p.user_id AND blocked_id = $1::uuid)
		      )
		  )
		  ` + mediaFilter + `
		ORDER BY p.likes_count DESC, p.created_at DESC
		LIMIT $2 OFFSET $3`

	return r.scanPosts(ctx, query, uid, limit, offset)
}

func (r *PostRepository) GetSavedByUserID(ctx context.Context, userID string, limit, offset int) ([]*domain.Post, error) {
	query := `
		SELECT p.id, p.user_id, p.caption, p.media_urls, p.media_types, p.location,
		       p.thumbnail_url, p.likes_count, p.comments_count, p.saves_count, p.created_at, p.updated_at,
		       COALESCE(p.audio_track_id::text, ''),
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM posts p
		JOIN users u ON u.id = p.user_id
		JOIN saved_posts sp ON sp.post_id = p.id
		WHERE sp.user_id = $1
		ORDER BY sp.saved_at DESC
		LIMIT $2 OFFSET $3`

	return r.scanPosts(ctx, query, userID, limit, offset)
}

func (r *PostRepository) SearchByCaption(ctx context.Context, query string, limit, offset int) ([]*domain.Post, error) {
	q := `
		SELECT p.id, p.user_id, p.caption, p.media_urls, p.media_types, p.location,
		       p.thumbnail_url, p.likes_count, p.comments_count, p.saves_count, p.created_at, p.updated_at,
		       COALESCE(p.audio_track_id::text, ''),
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM posts p
		JOIN users u ON u.id = p.user_id
		WHERE u.is_private = false AND p.caption ILIKE $1
		ORDER BY p.likes_count DESC, p.created_at DESC
		LIMIT $2 OFFSET $3`

	return r.scanPosts(ctx, q, "%"+query+"%", limit, offset)
}

func (r *PostRepository) IsLikedByUser(ctx context.Context, postID, userID string) (bool, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM likes WHERE entity_id = $1 AND entity_type = 'post' AND user_id = $2)`,
		postID, userID).Scan(&exists)
	return exists, err
}

func (r *PostRepository) IsSavedByUser(ctx context.Context, postID, userID string) (bool, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM saved_posts WHERE post_id = $1 AND user_id = $2)`,
		postID, userID).Scan(&exists)
	return exists, err
}

func (r *PostRepository) IncrementLikesCount(ctx context.Context, postID string, delta int) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE posts SET likes_count = GREATEST(likes_count + $1, 0) WHERE id = $2`,
		delta, postID)
	return err
}

func (r *PostRepository) IncrementCommentsCount(ctx context.Context, postID string, delta int) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE posts SET comments_count = GREATEST(comments_count + $1, 0) WHERE id = $2`,
		delta, postID)
	return err
}

func (r *PostRepository) IncrementSavesCount(ctx context.Context, postID string, delta int) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE posts SET saves_count = GREATEST(saves_count + $1, 0) WHERE id = $2`,
		delta, postID)
	return err
}

func (r *PostRepository) SavePost(ctx context.Context, userID, postID string) (bool, error) {
	var id string
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO saved_posts (user_id, post_id) VALUES ($1, $2) ON CONFLICT DO NOTHING RETURNING user_id`,
		userID, postID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil // already saved, no row inserted
		}
		return false, fmt.Errorf("save post: %w", err)
	}
	return true, nil
}

func (r *PostRepository) UnsavePost(ctx context.Context, userID, postID string) error {
	result, err := r.db.Pool.Exec(ctx,
		`DELETE FROM saved_posts WHERE user_id = $1 AND post_id = $2`,
		userID, postID)
	if err != nil {
		return fmt.Errorf("unsave post: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotSaved
	}
	return nil
}

func (r *PostRepository) GetLikedPostIDs(ctx context.Context, userID string, postIDs []string) (map[string]bool, error) {
	result := make(map[string]bool)
	if len(postIDs) == 0 {
		return result, nil
	}
	rows, err := r.db.Pool.Query(ctx,
		`SELECT entity_id FROM likes WHERE user_id = $1 AND entity_type = 'post' AND entity_id = ANY($2)`,
		userID, postIDs)
	if err != nil {
		return nil, fmt.Errorf("get liked post ids: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result[id] = true
	}
	return result, rows.Err()
}

func (r *PostRepository) GetSavedPostIDs(ctx context.Context, userID string, postIDs []string) (map[string]bool, error) {
	result := make(map[string]bool)
	if len(postIDs) == 0 {
		return result, nil
	}
	rows, err := r.db.Pool.Query(ctx,
		`SELECT post_id FROM saved_posts WHERE user_id = $1 AND post_id = ANY($2)`,
		userID, postIDs)
	if err != nil {
		return nil, fmt.Errorf("get saved post ids: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result[id] = true
	}
	return result, rows.Err()
}

func (r *PostRepository) scanPosts(ctx context.Context, query string, args ...interface{}) ([]*domain.Post, error) {
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query posts: %w", err)
	}
	defer rows.Close()

	var posts []*domain.Post
	for rows.Next() {
		p := &domain.Post{User: &domain.UserShort{}}
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.Caption, &p.MediaURLs, &p.MediaTypes,
			&p.Location, &p.ThumbnailURL, &p.LikesCount, &p.CommentsCount, &p.SavesCount,
			&p.CreatedAt, &p.UpdatedAt, &p.AudioTrackID,
			&p.User.ID, &p.User.Username, &p.User.FullName,
			&p.User.AvatarURL, &p.User.IsVerified,
		); err != nil {
			return nil, fmt.Errorf("scan post: %w", err)
		}
		posts = append(posts, p)
	}

	return posts, rows.Err()
}

// SetReaction upserts the user's emoji reaction on a post — one emoji per
// (post, user), replaced on conflict.
func (r *PostRepository) SetReaction(ctx context.Context, postID, userID, emoji string) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO post_reactions (post_id, user_id, emoji)
		VALUES ($1, $2, $3)
		ON CONFLICT (post_id, user_id)
		DO UPDATE SET emoji = EXCLUDED.emoji, created_at = NOW()`,
		postID, userID, emoji)
	if err != nil {
		return fmt.Errorf("set post reaction: %w", err)
	}
	return nil
}

// RemoveReaction deletes the user's reaction. No-op if it didn't exist.
func (r *PostRepository) RemoveReaction(ctx context.Context, postID, userID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM post_reactions WHERE post_id = $1 AND user_id = $2`,
		postID, userID)
	return err
}

// CountReactions returns aggregated counts for one post — used by the
// realtime push so clients can update without an extra GET.
func (r *PostRepository) CountReactions(ctx context.Context, postID string) (map[string]int, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT emoji, COUNT(*) FROM post_reactions
		 WHERE post_id = $1 GROUP BY emoji`,
		postID)
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

// AttachReactions hydrates Reactions/MyReaction on a batch of posts in a
// single roundtrip. Pass empty currentUserID for anon (MyReaction stays "").
func (r *PostRepository) AttachReactions(ctx context.Context, posts []*domain.Post, currentUserID string) error {
	if len(posts) == 0 {
		return nil
	}
	ids := make([]string, len(posts))
	for i, p := range posts {
		ids[i] = p.ID
	}
	rows, err := r.db.Pool.Query(ctx,
		`SELECT post_id, emoji, user_id
		 FROM post_reactions
		 WHERE post_id = ANY($1::uuid[])`,
		ids)
	if err != nil {
		return err
	}
	defer rows.Close()

	type bucket struct {
		counts map[string]int
		mine   string
	}
	buckets := make(map[string]*bucket, len(posts))
	for rows.Next() {
		var postID, emoji, userID string
		if err := rows.Scan(&postID, &emoji, &userID); err != nil {
			return err
		}
		b, ok := buckets[postID]
		if !ok {
			b = &bucket{counts: map[string]int{}}
			buckets[postID] = b
		}
		b.counts[emoji]++
		if userID == currentUserID && currentUserID != "" {
			b.mine = emoji
		}
	}
	for i := range posts {
		if b, ok := buckets[posts[i].ID]; ok {
			posts[i].Reactions = b.counts
			posts[i].MyReaction = b.mine
		}
	}
	return nil
}

