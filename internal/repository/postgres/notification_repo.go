package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/seeu/backend/internal/domain"
)

type NotificationRepository struct {
	db *DB
}

func NewNotificationRepository(db *DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) Create(ctx context.Context, n *domain.Notification) error {
	query := `
		INSERT INTO notifications (user_id, from_user_id, type, entity_id, entity_type, comment_id, message)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`

	err := r.db.Pool.QueryRow(ctx, query,
		n.UserID,
		n.FromUserID,
		n.Type,
		n.EntityID,
		n.EntityType,
		n.CommentID,
		n.Message,
	).Scan(&n.ID, &n.CreatedAt)

	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}

	return nil
}

func (r *NotificationRepository) GetByUserID(ctx context.Context, userID string, limit, offset int) ([]*domain.Notification, error) {
	query := `
		SELECT n.id, n.user_id, n.from_user_id, n.type, n.entity_id, n.entity_type,
		       n.comment_id, n.message, n.is_read, n.created_at,
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM notifications n
		LEFT JOIN users u ON u.id = n.from_user_id
		WHERE n.user_id = $1
		ORDER BY n.created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get notifications: %w", err)
	}
	defer rows.Close()

	var notifications []*domain.Notification
	for rows.Next() {
		n := &domain.Notification{}
		fromUser := &domain.UserShort{}
		var fromUserID, fromUserIDStr, fromUserUsername, fromUserFullName, fromUserAvatar *string
		var fromUserVerified *bool

		if err := rows.Scan(
			&n.ID, &n.UserID, &n.FromUserID, &n.Type,
			&n.EntityID, &n.EntityType, &n.CommentID, &n.Message, &n.IsRead, &n.CreatedAt,
			&fromUserIDStr, &fromUserUsername, &fromUserFullName, &fromUserAvatar, &fromUserVerified,
		); err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}

		_ = fromUserID
		if fromUserIDStr != nil {
			fromUser.ID = *fromUserIDStr
			if fromUserUsername != nil {
				fromUser.Username = *fromUserUsername
			}
			if fromUserFullName != nil {
				fromUser.FullName = *fromUserFullName
			}
			if fromUserAvatar != nil {
				fromUser.AvatarURL = *fromUserAvatar
			}
			if fromUserVerified != nil {
				fromUser.IsVerified = *fromUserVerified
			}
			n.FromUser = fromUser
		}

		notifications = append(notifications, n)
	}

	return notifications, rows.Err()
}

func (r *NotificationRepository) GetByID(ctx context.Context, id string) (*domain.Notification, error) {
	query := `SELECT id, user_id, from_user_id, type, entity_id, entity_type, comment_id, message, is_read, created_at
		FROM notifications WHERE id = $1`

	n := &domain.Notification{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&n.ID, &n.UserID, &n.FromUserID, &n.Type,
		&n.EntityID, &n.EntityType, &n.CommentID, &n.Message, &n.IsRead, &n.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get notification: %w", err)
	}

	return n, nil
}

func (r *NotificationRepository) MarkAsRead(ctx context.Context, id, userID string) error {
	result, err := r.db.Pool.Exec(ctx,
		`UPDATE notifications SET is_read = true WHERE id = $1 AND user_id = $2`,
		id, userID)
	if err != nil {
		return fmt.Errorf("mark notification as read: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *NotificationRepository) MarkAllAsRead(ctx context.Context, userID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE notifications SET is_read = true WHERE user_id = $1 AND is_read = false`,
		userID)
	return err
}

func (r *NotificationRepository) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = false`,
		userID).Scan(&count)
	return count, err
}
