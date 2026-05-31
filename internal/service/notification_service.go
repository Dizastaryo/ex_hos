package service

import (
	"context"
	"fmt"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	redisRepo "github.com/seeu/backend/internal/repository/redis"
	"github.com/seeu/backend/pkg/pagination"
	"go.uber.org/zap"
)

type NotificationService struct {
	notifRepo *postgres.NotificationRepository
	cache     *redisRepo.Cache
	logger    *zap.Logger
}

func NewNotificationService(
	notifRepo *postgres.NotificationRepository,
	cache *redisRepo.Cache,
	logger *zap.Logger,
) *NotificationService {
	return &NotificationService{
		notifRepo: notifRepo,
		cache:     cache,
		logger:    logger,
	}
}

func (s *NotificationService) GetByUserID(ctx context.Context, userID string, page, limit int) ([]*domain.Notification, pagination.Meta, error) {
	offset := pagination.Offset(page, limit)
	// Перебираем raw чуть с запасом (limit*3) чтобы хватило сырых строк после
	// батчинга. Это компромисс — pagination становится приблизительной, но
	// "100 лайков" не растянется на 5 пустых страниц.
	rawLimit := limit*3 + 1
	notifications, err := s.notifRepo.GetByUserID(ctx, userID, rawLimit, offset)
	if err != nil {
		return nil, pagination.Meta{}, fmt.Errorf("get notifications: %w", err)
	}

	batched := batchNotifications(notifications)

	hasNext := len(batched) > limit
	if hasNext {
		batched = batched[:limit]
	}

	meta := pagination.Meta{
		Page:        page,
		Limit:       limit,
		HasNextPage: hasNext,
	}

	return batched, meta, nil
}

// batchNotifications сжимает поток похожих нотификаций в одну запись
// с агрегатом OthersCount + OtherUsers. Группа = (type, entity_id, 1hr-bucket).
// Follow и mention не батчатся — они per-user-specific.
//
// Список приходит уже отсортированным DESC по created_at, представителем
// батча становится самая свежая нотификация (первая встретившаяся).
func batchNotifications(in []*domain.Notification) []*domain.Notification {
	const maxPreviewUsers = 3
	seen := make(map[string]int) // batch-key -> index in out
	out := make([]*domain.Notification, 0, len(in))

	for _, n := range in {
		// Нельзя батчить если нет entity (follow всегда такой) или тип
		// "follow"/"mention" — там обязательно конкретный from_user.
		if n.EntityID == nil || *n.EntityID == "" ||
			n.Type == domain.NotificationTypeFollow ||
			n.Type == domain.NotificationTypeMention {
			out = append(out, n)
			continue
		}

		hourBucket := n.CreatedAt.Unix() / 3600
		key := fmt.Sprintf("%s|%s|%d", n.Type, *n.EntityID, hourBucket)

		if idx, exists := seen[key]; exists {
			rep := out[idx]
			rep.OthersCount++
			// Кладём аватарки других юзеров в preview (с дедупом).
			if n.FromUser != nil && len(rep.OtherUsers) < maxPreviewUsers {
				dup := rep.FromUser != nil && rep.FromUser.ID == n.FromUser.ID
				for _, ou := range rep.OtherUsers {
					if ou.ID == n.FromUser.ID {
						dup = true
						break
					}
				}
				if !dup {
					rep.OtherUsers = append(rep.OtherUsers, n.FromUser)
				}
			}
		} else {
			seen[key] = len(out)
			out = append(out, n)
		}
	}

	return out
}

func (s *NotificationService) MarkAsRead(ctx context.Context, notifID, userID string) error {
	return s.notifRepo.MarkAsRead(ctx, notifID, userID)
}

func (s *NotificationService) MarkAllAsRead(ctx context.Context, userID string) error {
	if err := s.notifRepo.MarkAllAsRead(ctx, userID); err != nil {
		return fmt.Errorf("mark all notifications as read: %w", err)
	}

	// Invalidate unread count cache
	cacheKey := redisRepo.NotifCountKey(userID)
	if err := s.cache.Delete(ctx, cacheKey); err != nil {
		s.logger.Warn("invalidate notification count cache", zap.Error(err))
	}

	return nil
}

func (s *NotificationService) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	cacheKey := redisRepo.NotifCountKey(userID)

	var count int
	if err := s.cache.Get(ctx, cacheKey, &count); err == nil {
		return count, nil
	}

	count, err := s.notifRepo.GetUnreadCount(ctx, userID)
	if err != nil {
		return 0, err
	}

	if err := s.cache.Set(ctx, cacheKey, count, cacheTTL); err != nil {
		s.logger.Warn("cache notification count", zap.Error(err))
	}

	return count, nil
}
