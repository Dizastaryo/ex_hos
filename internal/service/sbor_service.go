package service

import (
	"context"
	"fmt"
	"time"

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

func (s *SborService) List(ctx context.Context, viewerID, typeFilter, catFilter, cityFilter, q string, dateFrom, dateTo *time.Time, page, limit int) ([]*domain.Sbor, pagination.Meta, error) {
	offset := pagination.Offset(page, limit)
	items, err := s.repo.List(ctx, viewerID, typeFilter, catFilter, cityFilter, q, dateFrom, dateTo, limit+1, offset)
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

func (s *SborService) ListMine(ctx context.Context, userID string, past bool, page, limit int) ([]*domain.Sbor, pagination.Meta, error) {
	offset := pagination.Offset(page, limit)
	items, err := s.repo.ListMine(ctx, userID, past, limit+1, offset)
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

// ─── Request flow ─────────────────────────────────────────────────

func (s *SborService) SubmitRequest(ctx context.Context, sborID, userID, message string) error {
	sbor, err := s.repo.GetByID(ctx, sborID, userID)
	if err != nil {
		return err
	}
	if sbor.HostID == userID {
		return domain.ErrForbidden // organizer can't request to join own sbor
	}
	return s.repo.SubmitRequest(ctx, sborID, userID, message)
}

func (s *SborService) CancelRequest(ctx context.Context, sborID, userID string) error {
	return s.repo.CancelRequest(ctx, sborID, userID)
}

func (s *SborService) ApproveRequest(ctx context.Context, requestID, adminID string) error {
	approvedUserID, sborID, err := s.repo.ApproveRequest(ctx, requestID, adminID)
	if err != nil {
		return err
	}
	// Add to group chat
	sbor, err := s.repo.GetByID(ctx, sborID, adminID)
	if err != nil {
		return nil // member added; chat sync failure is non-critical
	}
	if sbor.ChatID != nil {
		if err := s.chatRepo.AddParticipant(ctx, *sbor.ChatID, approvedUserID); err != nil {
			s.logger.Warn("add approved member to sbor chat",
				zap.String("sbor_id", sborID), zap.String("user_id", approvedUserID), zap.Error(err))
		}
	}
	// Notify the approved user via WS
	s.hub.SendToUsers([]string{approvedUserID}, "sbor.request.approved", map[string]any{
		"sbor_id": sborID,
		"title":   sbor.Title,
	})
	return nil
}

func (s *SborService) RejectRequest(ctx context.Context, requestID, adminID string) error {
	rejectedUserID, sborID, err := s.repo.RejectRequest(ctx, requestID, adminID)
	if err != nil {
		return err
	}
	// Notify the rejected user via WS
	sbor, _ := s.repo.GetByID(ctx, sborID, adminID)
	title := ""
	if sbor != nil {
		title = sbor.Title
	}
	s.hub.SendToUsers([]string{rejectedUserID}, "sbor.request.rejected", map[string]any{
		"sbor_id": sborID,
		"title":   title,
	})
	return nil
}

func (s *SborService) ListRequests(ctx context.Context, sborID, adminID string) ([]*domain.SborJoinRequest, error) {
	sbor, err := s.repo.GetByID(ctx, sborID, adminID)
	if err != nil {
		return nil, err
	}
	if sbor.HostID != adminID {
		return nil, domain.ErrForbidden
	}
	return s.repo.ListRequests(ctx, sborID, adminID)
}

// ─────────────────────────────────────────────────────────────────

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
			// Ретроспективно создаём group-чат и добавляем всех текущих участников.
			chatID, err := s.chatRepo.CreateGroupConversation(ctx, sbor.HostID, sbor.Title, "", sbor.MemberIDs)
			if err == nil {
				if err := s.repo.SetChatID(ctx, sbor.ID, chatID); err == nil {
					sbor.ChatID = &chatID
				}
			}
		}
		return sbor, nil
	}
	if err := s.repo.Join(ctx, sborID, userID); err != nil {
		return nil, fmt.Errorf("join sbor: %w", err)
	}
	// Создаём group-чат если его ещё нет, добавляем всех текущих участников + нового.
	if sbor.ChatID == nil {
		allMembers := append(sbor.MemberIDs, userID)
		chatID, err := s.chatRepo.CreateGroupConversation(ctx, sbor.HostID, sbor.Title, "", allMembers)
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
	// Если была pending заявка — убираем (idempotent, ошибку игнорируем)
	_ = s.repo.CancelRequest(ctx, sborID, userID)
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
	// Проверяем что новый max_slots не меньше текущего числа участников
	if req.MaxSlots != nil {
		count, err := s.repo.CountMembers(ctx, id)
		if err != nil {
			return nil, err
		}
		if *req.MaxSlots < count {
			return nil, domain.ErrMaxSlotsConflict
		}
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
	sbor, err := s.repo.GetByID(ctx, sborID, userID)
	if err != nil {
		return false, err
	}
	if sbor.HostID == userID {
		return false, domain.ErrForbidden
	}
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

	// Cancel sbor and delete linked chat atomically in the repo.
	if err := s.repo.Cancel(ctx, id, userID); err != nil {
		return err
	}

	// Notify all participants via WS after the atomic cancel+delete succeeds.
	if len(memberIDs) > 0 {
		s.hub.SendToUsers(memberIDs, "sbor.cancelled", map[string]any{
			"sbor_id": id,
			"title":   sbor.Title,
		})
	}
	return nil
}

// ListMembers returns all participants of a sbor. Any participant may call this.
func (s *SborService) ListMembers(ctx context.Context, sborID, viewerID string) ([]*postgres.SborMemberDTO, error) {
	// Verify sbor exists and viewer has access.
	_, err := s.repo.GetByID(ctx, sborID, viewerID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListMembers(ctx, sborID)
}
