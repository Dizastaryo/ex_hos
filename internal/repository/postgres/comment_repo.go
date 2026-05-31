package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/seeu/backend/internal/domain"
)

type CommentRepository struct {
	db *DB
}

func NewCommentRepository(db *DB) *CommentRepository {
	return &CommentRepository{db: db}
}

func (r *CommentRepository) Create(ctx context.Context, comment *domain.Comment) error {
	query := `
		INSERT INTO comments (post_id, user_id, parent_id, text)
		VALUES ($1, $2, $3, $4)
		RETURNING id, likes_count, created_at, updated_at`

	err := r.db.Pool.QueryRow(ctx, query,
		comment.PostID,
		comment.UserID,
		comment.ParentID,
		comment.Text,
	).Scan(&comment.ID, &comment.LikesCount, &comment.CreatedAt, &comment.UpdatedAt)

	if err != nil {
		return fmt.Errorf("create comment: %w", err)
	}

	return nil
}

func (r *CommentRepository) GetByID(ctx context.Context, id string) (*domain.Comment, error) {
	query := `
		SELECT c.id, c.post_id, c.user_id, c.parent_id, c.text, c.likes_count, c.created_at, c.updated_at,
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM comments c
		JOIN users u ON u.id = c.user_id
		WHERE c.id = $1`

	comment := &domain.Comment{User: &domain.UserShort{}}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&comment.ID, &comment.PostID, &comment.UserID, &comment.ParentID,
		&comment.Text, &comment.LikesCount, &comment.CreatedAt, &comment.UpdatedAt,
		&comment.User.ID, &comment.User.Username, &comment.User.FullName,
		&comment.User.AvatarURL, &comment.User.IsVerified,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrCommentNotFound
		}
		return nil, fmt.Errorf("get comment by id: %w", err)
	}

	return comment, nil
}

func (r *CommentRepository) Delete(ctx context.Context, id, userID string) error {
	result, err := r.db.Pool.Exec(ctx,
		`DELETE FROM comments WHERE id = $1 AND (user_id = $2 OR EXISTS(SELECT 1 FROM posts WHERE id = (SELECT post_id FROM comments WHERE id = $1) AND user_id = $2))`,
		id, userID)
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrCommentNotFound
	}
	return nil
}

func (r *CommentRepository) GetByPostID(ctx context.Context, postID, viewerID string, limit, offset int) ([]*domain.Comment, error) {
	// PROFILE-4: фильтруем комменты от restricted-юзеров для всех зрителей
	// КРОМЕ самого commenter'а и автора поста. Restrict-source = author of post.
	// viewerID == "" — анонимный viewer; ему ограниченные комменты не видны.
	query := `
		WITH post_author AS (
			SELECT user_id FROM posts WHERE id = $1
		)
		SELECT c.id, c.post_id, c.user_id, c.parent_id, c.text, c.likes_count, c.created_at, c.updated_at,
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified,
		       (SELECT COUNT(*) FROM comments r WHERE r.parent_id = c.id) as replies_count
		FROM comments c
		JOIN users u ON u.id = c.user_id
		WHERE c.post_id = $1 AND c.parent_id IS NULL
		  AND (
		    -- комментер сам видит свои комменты (даже если ограничен)
		    c.user_id = $2
		    -- автор поста видит все комменты
		    OR (SELECT user_id FROM post_author) = $2
		    -- остальные видят только если commenter не ограничен автором поста
		    OR NOT EXISTS (
		        SELECT 1 FROM user_restrictions ur
		        WHERE ur.user_id = (SELECT user_id FROM post_author)
		          AND ur.restricted_user_id = c.user_id
		    )
		  )
		ORDER BY c.created_at DESC
		LIMIT $3 OFFSET $4`

	rows, err := r.db.Pool.Query(ctx, query, postID, viewerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query comments: %w", err)
	}
	defer rows.Close()

	var comments []*domain.Comment
	for rows.Next() {
		c := &domain.Comment{User: &domain.UserShort{}}
		if err := rows.Scan(
			&c.ID, &c.PostID, &c.UserID, &c.ParentID,
			&c.Text, &c.LikesCount, &c.CreatedAt, &c.UpdatedAt,
			&c.User.ID, &c.User.Username, &c.User.FullName,
			&c.User.AvatarURL, &c.User.IsVerified,
			&c.RepliesCount,
		); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		comments = append(comments, c)
	}

	return comments, rows.Err()
}

func (r *CommentRepository) GetReplies(ctx context.Context, parentID string, limit, offset int) ([]*domain.Comment, error) {
	query := `
		SELECT c.id, c.post_id, c.user_id, c.parent_id, c.text, c.likes_count, c.created_at, c.updated_at,
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified,
		       0 as replies_count
		FROM comments c
		JOIN users u ON u.id = c.user_id
		WHERE c.parent_id = $1
		ORDER BY c.created_at ASC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Pool.Query(ctx, query, parentID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query replies: %w", err)
	}
	defer rows.Close()

	var comments []*domain.Comment
	for rows.Next() {
		c := &domain.Comment{User: &domain.UserShort{}}
		if err := rows.Scan(
			&c.ID, &c.PostID, &c.UserID, &c.ParentID,
			&c.Text, &c.LikesCount, &c.CreatedAt, &c.UpdatedAt,
			&c.User.ID, &c.User.Username, &c.User.FullName,
			&c.User.AvatarURL, &c.User.IsVerified,
			&c.RepliesCount,
		); err != nil {
			return nil, fmt.Errorf("scan reply: %w", err)
		}
		comments = append(comments, c)
	}

	return comments, rows.Err()
}

func (r *CommentRepository) IncrementLikesCount(ctx context.Context, commentID string, delta int) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE comments SET likes_count = likes_count + $1 WHERE id = $2`,
		delta, commentID)
	return err
}

func (r *CommentRepository) IsLikedByUser(ctx context.Context, commentID, userID string) (bool, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM likes WHERE entity_id = $1 AND entity_type = 'comment' AND user_id = $2)`,
		commentID, userID).Scan(&exists)
	return exists, err
}
