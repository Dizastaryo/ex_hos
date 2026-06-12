package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	"github.com/seeu/backend/internal/ws"
	"go.uber.org/zap"
)

type ScannerService struct {
	scannerRepo *postgres.ScannerRepository
	userRepo    *postgres.UserRepository
	notifRepo   *postgres.NotificationRepository
	statsRepo   *postgres.UserStatsRepository
	wsHub       *ws.Hub
	logger      *zap.Logger
}

func NewScannerService(
	scannerRepo *postgres.ScannerRepository,
	userRepo *postgres.UserRepository,
	notifRepo *postgres.NotificationRepository,
	statsRepo *postgres.UserStatsRepository,
	wsHub *ws.Hub,
	logger *zap.Logger,
) *ScannerService {
	return &ScannerService{
		scannerRepo: scannerRepo,
		userRepo:    userRepo,
		notifRepo:   notifRepo,
		statsRepo:   statsRepo,
		wsHub:       wsHub,
		logger:      logger,
	}
}

// PostLike ставит лайк от likerID на устройство с publicIDHex.
// Флоу:
//  1. Резолвим publicIDHex → targetUserID (404 если scan_enabled=false)
//  2. Нельзя лайкать себя
//  3. INSERT scanner_likes (idempotent — ON CONFLICT DO NOTHING)
//  4. Если лайк новый → уведомление targetUser через DB + WS push
//
// Возвращает ErrNotFound если устройство не найдено или scan_enabled=false.
func (s *ScannerService) PostLike(ctx context.Context, likerID, publicIDHex string) error {
	targetUserID, err := s.scannerRepo.GetUserByDeviceHash(ctx, publicIDHex)
	if err != nil {
		return err // ErrNotFound прокидываем как есть
	}

	if targetUserID == likerID {
		return domain.ErrSelfAction
	}

	// Rate limit: не более 100 лайков в сутки (anti-spam).
	dailyCount, err := s.scannerRepo.DailyLikesCount(ctx, likerID)
	if err == nil && dailyCount >= 100 {
		return domain.ErrRateLimited
	}

	isNew, err := s.scannerRepo.UpsertLike(ctx, likerID, targetUserID)
	if err != nil {
		return fmt.Errorf("upsert like: %w", err)
	}

	// Уведомляем и обновляем статистику только если лайк новый (не дубль)
	if !isNew {
		return nil
	}

	// Social score: увеличиваем scanner_likes + total_likes целевого юзера.
	if err := s.statsRepo.IncrementLikes(ctx, targetUserID, "scanner_likes"); err != nil {
		s.logger.Warn("increment scanner_likes", zap.Error(err))
	}

	notifID := uuid.New().String()
	n := &domain.Notification{
		ID:         notifID,
		UserID:     targetUserID,
		FromUserID: &likerID,
		Type:       domain.NotificationTypeScannerLike,
		// Message без имени — frontend рендерит "X лайкнул(а) тебя в сканере"
		// используя FromUser.username. Имя лайкера видно только target'у.
		Message: "лайкнул(а) тебя в сканере",
	}
	if err := s.notifRepo.Create(ctx, n); err != nil {
		s.logger.Warn("create scanner like notification", zap.Error(err))
	} else {
		pushNotif(s.wsHub, n)
	}

	return nil
}

// RemoveLike убирает лайк likerID → устройство с publicIDHex.
func (s *ScannerService) RemoveLike(ctx context.Context, likerID, publicIDHex string) error {
	targetUserID, err := s.scannerRepo.GetUserByDeviceHash(ctx, publicIDHex)
	if err != nil {
		// Если устройство не найдено — лайка и так нет, ок
		if err == domain.ErrNotFound {
			return nil
		}
		return err
	}
	return s.scannerRepo.DeleteLike(ctx, likerID, targetUserID)
}

// GetReceivedLikes возвращает список тех, кто лайкнул targetUserID.
// Отдаёт реальные данные лайкеров — только target видит кто его лайкнул.
func (s *ScannerService) GetReceivedLikes(ctx context.Context, userID string, limit, offset int) ([]*postgres.ScannerLikeRow, int, error) {
	rows, err := s.scannerRepo.GetReceivedLikes(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("get received likes: %w", err)
	}
	count, err := s.scannerRepo.CountReceivedLikes(ctx, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("count received likes: %w", err)
	}
	return rows, count, nil
}

// GetSentLikes возвращает scan-профили тех, кому likerID поставил лайк.
// Реальный аккаунт не раскрывается — только scan_alias.
func (s *ScannerService) GetSentLikes(ctx context.Context, userID string, limit, offset int) ([]*domain.ScanProfile, error) {
	return s.scannerRepo.GetSentLikeTargets(ctx, userID, limit, offset)
}

func (s *ScannerService) UnseenLikesCount(ctx context.Context, userID string) (int, error) {
	return s.scannerRepo.UnseenLikesCount(ctx, userID)
}

func (s *ScannerService) MarkLikesSeen(ctx context.Context, userID string) error {
	return s.scannerRepo.MarkLikesSeen(ctx, userID)
}
