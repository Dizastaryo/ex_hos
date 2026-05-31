package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/seeu/backend/internal/domain"
)

type VideoRepository struct {
	db *DB
}

func NewVideoRepository(db *DB) *VideoRepository {
	return &VideoRepository{db: db}
}

// CheckUserVisibility — cross-service privacy-check. cmd/video не имеет
// доступа к UserRepository/FollowRepository из cmd/api, но shared БД даёт
// прямой SQL-доступ. Возвращает:
//   - domain.ErrUserNotFound — owner не существует
//   - domain.ErrPrivateAccount — приватный + viewer не владелец и не подписчик
//   - nil — viewer может видеть
//
// Дублирует логику post_service.GetByUsername (см. social-api). Когда заведём
// inter-service contract, эту функцию можно будет выкинуть.
func (r *VideoRepository) CheckUserVisibility(ctx context.Context, ownerID, viewerID string) error {
	var isPrivate, isFollower bool
	err := r.db.Pool.QueryRow(ctx, `
		SELECT
			u.is_private,
			COALESCE((
				SELECT TRUE FROM follows
				WHERE follower_id = $2 AND following_id = u.id
				LIMIT 1
			), FALSE)
		FROM users u WHERE u.id = $1`,
		ownerID, viewerID,
	).Scan(&isPrivate, &isFollower)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrUserNotFound
		}
		return fmt.Errorf("check visibility: %w", err)
	}
	if isPrivate && ownerID != viewerID && !isFollower {
		return domain.ErrPrivateAccount
	}
	return nil
}

func (r *VideoRepository) CreateVideo(ctx context.Context, v *domain.Video) error {
	query := `
		INSERT INTO videos (user_id, title, description, video_url, thumbnail_url, duration_seconds, category_id, resolution, subtitles_url)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8, $9)
		RETURNING id, views_count, likes_count, comments_count, is_live, created_at, updated_at`

	return r.db.Pool.QueryRow(ctx, query,
		v.UserID, v.Title, v.Description, v.VideoURL, v.ThumbnailURL,
		v.DurationSeconds, v.CategoryID, v.Resolution, v.SubtitlesURL,
	).Scan(&v.ID, &v.ViewsCount, &v.LikesCount, &v.CommentsCount, &v.IsLive, &v.CreatedAt, &v.UpdatedAt)
}

func (r *VideoRepository) GetVideoByID(ctx context.Context, id string) (*domain.Video, error) {
	query := `
		SELECT v.id, v.user_id, v.title, v.description, v.video_url, v.thumbnail_url,
		       v.duration_seconds, COALESCE(v.category_id::text, ''), v.resolution,
		       v.views_count, v.likes_count, v.comments_count, v.is_live, COALESCE(v.subtitles_url, ''), v.created_at, v.updated_at,
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM videos v
		JOIN users u ON u.id = v.user_id
		WHERE v.id = $1`

	video := &domain.Video{User: &domain.UserShort{}}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&video.ID, &video.UserID, &video.Title, &video.Description, &video.VideoURL,
		&video.ThumbnailURL, &video.DurationSeconds, &video.CategoryID, &video.Resolution,
		&video.ViewsCount, &video.LikesCount, &video.CommentsCount, &video.IsLive, &video.SubtitlesURL,
		&video.CreatedAt, &video.UpdatedAt,
		&video.User.ID, &video.User.Username, &video.User.FullName,
		&video.User.AvatarURL, &video.User.IsVerified,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrVideoNotFound
		}
		return nil, fmt.Errorf("get video: %w", err)
	}
	return video, nil
}

func (r *VideoRepository) ListVideos(ctx context.Context, categoryID string, limit, offset int) ([]*domain.Video, int, error) {
	countQuery := `SELECT COUNT(*) FROM videos`
	listQuery := `
		SELECT v.id, v.user_id, v.title, v.description, v.video_url, v.thumbnail_url,
		       v.duration_seconds, COALESCE(v.category_id::text, ''), v.resolution,
		       v.views_count, v.likes_count, v.comments_count, v.is_live, COALESCE(v.subtitles_url, ''), v.created_at, v.updated_at,
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM videos v
		JOIN users u ON u.id = v.user_id`

	args := []interface{}{}
	argIdx := 1

	if categoryID != "" {
		countQuery += fmt.Sprintf(" WHERE category_id = $%d", argIdx)
		listQuery += fmt.Sprintf(" WHERE v.category_id = $%d", argIdx)
		args = append(args, categoryID)
		argIdx++
	}

	var total int
	err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count videos: %w", err)
	}

	listQuery += fmt.Sprintf(" ORDER BY v.created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list videos: %w", err)
	}
	defer rows.Close()

	var videos []*domain.Video
	for rows.Next() {
		v := &domain.Video{User: &domain.UserShort{}}
		if err := rows.Scan(
			&v.ID, &v.UserID, &v.Title, &v.Description, &v.VideoURL,
			&v.ThumbnailURL, &v.DurationSeconds, &v.CategoryID, &v.Resolution,
			&v.ViewsCount, &v.LikesCount, &v.CommentsCount, &v.IsLive, &v.SubtitlesURL,
			&v.CreatedAt, &v.UpdatedAt,
			&v.User.ID, &v.User.Username, &v.User.FullName,
			&v.User.AvatarURL, &v.User.IsVerified,
		); err != nil {
			return nil, 0, fmt.Errorf("scan video: %w", err)
		}
		videos = append(videos, v)
	}
	return videos, total, nil
}

func (r *VideoRepository) GetFeatured(ctx context.Context) (*domain.Video, error) {
	query := `
		SELECT v.id, v.user_id, v.title, v.description, v.video_url, v.thumbnail_url,
		       v.duration_seconds, COALESCE(v.category_id::text, ''), v.resolution,
		       v.views_count, v.likes_count, v.comments_count, v.is_live, COALESCE(v.subtitles_url, ''), v.created_at, v.updated_at,
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM videos v
		JOIN users u ON u.id = v.user_id
		ORDER BY v.views_count DESC, v.created_at DESC
		LIMIT 1`

	video := &domain.Video{User: &domain.UserShort{}}
	err := r.db.Pool.QueryRow(ctx, query).Scan(
		&video.ID, &video.UserID, &video.Title, &video.Description, &video.VideoURL,
		&video.ThumbnailURL, &video.DurationSeconds, &video.CategoryID, &video.Resolution,
		&video.ViewsCount, &video.LikesCount, &video.CommentsCount, &video.IsLive, &video.SubtitlesURL,
		&video.CreatedAt, &video.UpdatedAt,
		&video.User.ID, &video.User.Username, &video.User.FullName,
		&video.User.AvatarURL, &video.User.IsVerified,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrVideoNotFound
		}
		return nil, fmt.Errorf("get featured: %w", err)
	}
	return video, nil
}

func (r *VideoRepository) GetUserVideos(ctx context.Context, userID string, limit, offset int) ([]*domain.Video, int, error) {
	var total int
	err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM videos WHERE user_id = $1`, userID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT v.id, v.user_id, v.title, v.description, v.video_url, v.thumbnail_url,
		       v.duration_seconds, COALESCE(v.category_id::text, ''), v.resolution,
		       v.views_count, v.likes_count, v.comments_count, v.is_live, COALESCE(v.subtitles_url, ''), v.created_at, v.updated_at,
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM videos v
		JOIN users u ON u.id = v.user_id
		WHERE v.user_id = $1
		ORDER BY v.created_at DESC LIMIT $2 OFFSET $3`

	rows, err := r.db.Pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var videos []*domain.Video
	for rows.Next() {
		v := &domain.Video{User: &domain.UserShort{}}
		if err := rows.Scan(
			&v.ID, &v.UserID, &v.Title, &v.Description, &v.VideoURL,
			&v.ThumbnailURL, &v.DurationSeconds, &v.CategoryID, &v.Resolution,
			&v.ViewsCount, &v.LikesCount, &v.CommentsCount, &v.IsLive, &v.SubtitlesURL,
			&v.CreatedAt, &v.UpdatedAt,
			&v.User.ID, &v.User.Username, &v.User.FullName,
			&v.User.AvatarURL, &v.User.IsVerified,
		); err != nil {
			return nil, 0, err
		}
		videos = append(videos, v)
	}
	return videos, total, nil
}

func (r *VideoRepository) DeleteVideo(ctx context.Context, id, userID string) error {
	result, err := r.db.Pool.Exec(ctx, `DELETE FROM videos WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return domain.ErrVideoNotFound
	}
	return nil
}

func (r *VideoRepository) IncrementViews(ctx context.Context, videoID, userID string) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO video_views (video_id, user_id) VALUES ($1, $2)
		ON CONFLICT (video_id, user_id) DO UPDATE SET viewed_at = NOW()`, videoID, userID)
	if err != nil {
		return err
	}
	_, err = r.db.Pool.Exec(ctx, `UPDATE videos SET views_count = views_count + 1 WHERE id = $1`, videoID)
	return err
}

func (r *VideoRepository) LikeVideo(ctx context.Context, videoID, userID string) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO video_likes (video_id, user_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, videoID, userID)
	if err != nil {
		return err
	}
	_, err = r.db.Pool.Exec(ctx, `
		UPDATE videos SET likes_count = (SELECT COUNT(*) FROM video_likes WHERE video_id = $1) WHERE id = $1`, videoID)
	return err
}

func (r *VideoRepository) UnlikeVideo(ctx context.Context, videoID, userID string) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM video_likes WHERE video_id = $1 AND user_id = $2`, videoID, userID)
	if err != nil {
		return err
	}
	_, err = r.db.Pool.Exec(ctx, `
		UPDATE videos SET likes_count = (SELECT COUNT(*) FROM video_likes WHERE video_id = $1) WHERE id = $1`, videoID)
	return err
}

func (r *VideoRepository) IsVideoLiked(ctx context.Context, videoID, userID string) (bool, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM video_likes WHERE video_id = $1 AND user_id = $2)`, videoID, userID).Scan(&exists)
	return exists, err
}

func (r *VideoRepository) GetCategories(ctx context.Context) ([]*domain.VideoCategory, error) {
	rows, err := r.db.Pool.Query(ctx, `SELECT id, name, created_at FROM video_categories ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cats []*domain.VideoCategory
	for rows.Next() {
		c := &domain.VideoCategory{}
		if err := rows.Scan(&c.ID, &c.Name, &c.CreatedAt); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, nil
}
