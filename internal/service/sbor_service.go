package service

import (
	"context"
	"fmt"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	"github.com/seeu/backend/internal/ws"
	"github.com/seeu/backend/pkg/pagination"
	"go.uber.org/zap"
)

type SborService struct {
	repo     *postgres.SborRepository
	chatRepo *postgres.ChatRepository
	hub      *ws.Hub
	logger   *zap.Logger
}

func NewSborService(repo *postgres.SborRepository, chatRepo *postgres.ChatRepository, hub *ws.Hub, logger *zap.Logger) *SborService {
	return &SborService{repo: repo, chatRepo: chatRepo, hub: hub, logger: logger}
}

func (s *SborService) Create(ctx context.Context, hostID string, req *domain.CreateSborRequest) (*domain.Sbor, error) {
	city := req.City
	if city == "" {
		city = "Алматы"
	}
	sbor := &domain.Sbor{
		HostID:       hostID,
		Type:         req.Type,
		Category:     req.Category,
		Title:        req.Title,
		Place:        req.Place,
		City:         city,
		CoverUrl:     req.CoverUrl,
		Price:        req.Price,
		Description:  req.Description,
		ScheduledAt:  req.ScheduledAt,
		FlexibleTime: req.FlexibleTime,
		MaxSlots:     req.MaxSlots,
	}
	if err := s.repo.Create(ctx, sbor); err != nil {
		return nil, fmt.Errorf("create sbor: %w", err)
	}

	// Автоматически создаём group-чат сбора. Название и обложка берутся из сбора.
	chatID, err := s.chatRepo.CreateGroupConversation(ctx, hostID, req.Title, req.CoverUrl, []string{})
	if err != nil {
		s.logger.Warn("create sbor group chat failed", zap.String("sbor_id", sbor.ID), zap.Error(err))
	} else {
		if err := s.repo.SetChatID(ctx, sbor.ID, chatID); err != nil {
			s.logger.Warn("set sbor chat_id failed", zap.String("sbor_id", sbor.ID), zap.Error(err))
		} else {
			sbor.ChatID = &chatID
		}
	}

	sbor.MyRole = "organizer"
	sbor.IsJoined = true
	sbor.Joined = 1
	return sbor, nil
}

func (s *SborService) GetByID(ctx context.Context, id, viewerID string) (*domain.Sbor, error) {
	return s.repo.GetByID(ctx, id, viewerID)
}

func (s *SborService) List(ctx context.Context, viewerID, typeFilter, catFilter, cityFilter string, page, limit int) ([]*domain.Sbor, pagination.Meta, error) {
	offset := pagination.Offset(page, limit)
	items, err := s.repo.List(ctx, viewerID, typeFilter, catFilter, cityFilter, limit+1, offset)
	if err != nil {
		return nil, pagination.Meta{}, err
	}
	hasNext := len(items) > limit
	if hasNext {
		items = items[:limit]
	}
	meta := pagination.Meta{Page: page, Limit: limit, HasNextPage: hasNext}
	return items, meta, nil
}

func (s *SborService) ListMine(ctx context.Context, userID string, page, limit int) ([]*domain.Sbor, pagination.Meta, error) {
	offset := pagination.Offset(page, limit)
	items, err := s.repo.ListMine(ctx, userID, limit+1, offset)
	if err != nil {
		return nil, pagination.Meta{}, err
	}
	hasNext := len(items) > limit
	if hasNext {
		items = items[:limit]
	}
	meta := pagination.Meta{Page: page, Limit: limit, HasNextPage: hasNext}
	return items, meta, nil
}

func (s *SborService) Join(ctx context.Context, sborID, userID string) (*domain.Sbor, error) {
	sbor, err := s.repo.GetByID(ctx, sborID, userID)
	if err != nil {
		return nil, err
	}
	if sbor.IsJoined {
		// Уже в сборе — убеждаемся что и в чате (idempotent)
		if sbor.ChatID != nil {
			_ = s.chatRepo.AddParticipant(ctx, *sbor.ChatID, userID)
		} else {
			// Ретроспективно создаём group-чат если его не было
			chatID, err := s.chatRepo.CreateGroupConversation(ctx, sbor.HostID, sbor.Title, "", []string{})
			if err == nil {
				if err := s.repo.SetChatID(ctx, sbor.ID, chatID); err == nil {
					sbor.ChatID = &chatID
				}
			}
		}
		return sbor, nil
	}
	if sbor.MaxSlots != nil && sbor.Joined >= *sbor.MaxSlots {
		return nil, domain.ErrSborFull
	}
	if err := s.repo.Join(ctx, sborID, userID); err != nil {
		return nil, fmt.Errorf("join sbor: %w", err)
	}
	// Создаём group-чат если его ещё нет, затем добавляем участника
	if sbor.ChatID == nil {
		chatID, err := s.chatRepo.CreateGroupConversation(ctx, sbor.HostID, sbor.Title, "", []string{})
		if err == nil {
			if err := s.repo.SetChatID(ctx, sbor.ID, chatID); err == nil {
				sbor.ChatID = &chatID
			}
		}
	}
	if sbor.ChatID != nil {
		if err := s.chatRepo.AddParticipant(ctx, *sbor.ChatID, userID); err != nil {
			s.logger.Warn("add participant to sbor chat failed",
				zap.String("chat_id", *sbor.ChatID), zap.String("user_id", userID), zap.Error(err))
		}
	}
	return s.repo.GetByID(ctx, sborID, userID)
}

func (s *SborService) Leave(ctx context.Context, sborID, userID string) error {
	sbor, err := s.repo.GetByID(ctx, sborID, userID)
	if err != nil {
		return err
	}
	if sbor.HostID == userID {
		return fmt.Errorf("host cannot leave: %w", domain.ErrForbidden)
	}
	if err := s.repo.Leave(ctx, sborID, userID); err != nil {
		return err
	}
	// Удаляем из group-чата
	if sbor.ChatID != nil {
		if err := s.chatRepo.RemoveParticipant(ctx, *sbor.ChatID, userID); err != nil {
			s.logger.Warn("remove participant from sbor chat failed",
				zap.String("chat_id", *sbor.ChatID), zap.String("user_id", userID), zap.Error(err))
		}
	}
	return nil
}

func (s *SborService) Update(ctx context.Context, id, userID string, req *domain.UpdateSborRequest) (*domain.Sbor, error) {
	sbor, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if sbor.HostID != userID {
		return nil, domain.ErrForbidden
	}
	if err := s.repo.Update(ctx, id, userID, req); err != nil {
		return nil, fmt.Errorf("update sbor: %w", err)
	}
	updated, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	// Синхронизируем название и обложку в связанный групповой чат.
	if updated.ChatID != nil && (req.Title != nil || req.CoverUrl != nil) {
		if err := s.chatRepo.UpdateGroupMeta(ctx, *updated.ChatID, updated.Title, updated.CoverUrl); err != nil {
			s.logger.Warn("sync sbor update to chat failed",
				zap.String("sbor_id", id),
				zap.String("chat_id", *updated.ChatID),
				zap.Error(err))
		}
	}
	return updated, nil
}

func (s *SborService) ToggleBookmark(ctx context.Context, userID, sborID string) (bool, error) {
	return s.repo.ToggleBookmark(ctx, userID, sborID)
}

func (s *SborService) ListBookmarked(ctx context.Context, userID string, page, limit int) ([]*domain.Sbor, pagination.Meta, error) {
	offset := pagination.Offset(page, limit)
	items, err := s.repo.ListBookmarked(ctx, userID, limit+1, offset)
	if err != nil {
		return nil, pagination.Meta{}, err
	}
	hasNext := len(items) > limit
	if hasNext {
		items = items[:limit]
	}
	return items, pagination.Meta{Page: page, Limit: limit, HasNextPage: hasNext}, nil
}

func (s *SborService) Cancel(ctx context.Context, id, userID string) error {
	sbor, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return err
	}
	if sbor.HostID != userID {
		return domain.ErrForbidden
	}

	// Собираем ID участников до удаления чата, чтобы разослать уведомления.
	var memberIDs []string
	if sbor.ChatID != nil {
		if participants, pErr := s.chatRepo.GetParticipants(ctx, *sbor.ChatID); pErr == nil {
			for _, p := range participants {
				if p.User != nil {
					memberIDs = append(memberIDs, p.User.ID)
				}
			}
		}
	}

	if err := s.repo.Cancel(ctx, id, userID); err != nil {
		return err
	}

	// Уведомляем всех участников через WS о том, что сбор отменён.
	if len(memberIDs) > 0 {
		s.hub.SendToUsers(memberIDs, "sbor.cancelled", map[string]any{
			"sbor_id": id,
			"title":   sbor.Title,
		})
	}

	// Удаляем связанный групповой чат — он больше не нужен.
	if sbor.ChatID != nil {
		if err := s.chatRepo.DeleteConversation(ctx, *sbor.ChatID); err != nil {
			s.logger.Warn("delete sbor group chat failed",
				zap.String("sbor_id", id),
				zap.String("chat_id", *sbor.ChatID),
				zap.Error(err))
		}
	}
	return nil
}
