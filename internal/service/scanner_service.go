package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	"github.com/seeu/backend/internal/ws"
	"go.uber.org/zap"
)

type ScannerService struct {
	scannerRepo *postgres.ScannerRepository
	userRepo    *postgres.UserRepository
	chatRepo    *postgres.ChatRepository
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

// SetChatRepo sets the chat repo (call after construction to avoid circular deps).
func (s *ScannerService) SetChatRepo(chatRepo *postgres.ChatRepository) {
	s.chatRepo = chatRepo
}


// ── Resolve ─────────────────────────────────────────────────────────────────

func (s *ScannerService) ResolveScanProfile(ctx context.Context, publicIDHex string) (*domain.ScanProfile, error) {
	return s.scannerRepo.ResolveScanProfile(ctx, publicIDHex)
}

func (s *ScannerService) ResolveScanProfiles(ctx context.Context, publicIDHexes []string) ([]*domain.ScanProfile, error) {
	return s.scannerRepo.ResolveScanProfiles(ctx, publicIDHexes)
}

// ── Like ────────────────────────────────────────────────────────────────────

// PostWave sends a like from likerID to the device with publicIDHex.
func (s *ScannerService) PostWave(ctx context.Context, likerID, publicIDHex string) error {
	targetUserID, err := s.scannerRepo.GetUserByDeviceHash(ctx, publicIDHex)
	if err != nil {
		return err
	}

	if targetUserID == likerID {
		return domain.ErrSelfAction
	}

	// Daily limit: 20 likes/day
	dailyCount, err := s.scannerRepo.DailyWavesCount(ctx, likerID)
	if err == nil && dailyCount >= 20 {
		return domain.ErrRateLimited
	}

	// Per-target cooldown: 1 like/hour
	lastWave, err := s.scannerRepo.LastWaveAt(ctx, likerID, targetUserID)
	if err == nil && !lastWave.IsZero() && time.Since(lastWave) < time.Hour {
		return domain.ErrRateLimited
	}

	if err := s.scannerRepo.InsertWave(ctx, likerID, targetUserID); err != nil {
		return fmt.Errorf("insert like: %w", err)
	}

	// Social score
	if err := s.statsRepo.IncrementLikes(ctx, targetUserID, "scanner_likes"); err != nil {
		s.logger.Warn("increment scanner_likes", zap.Error(err))
	}

	// Check if this creates a match
	isMutual, _ := s.scannerRepo.HasMutualLike(ctx, likerID, targetUserID)

	// Notification
	notifID := uuid.New().String()
	msg := "лайкнул(а) тебя в сканере"
	if isMutual {
		msg = "У вас мэтч! Подойдите друг к другу"
	}
	n := &domain.Notification{
		ID:         notifID,
		UserID:     targetUserID,
		FromUserID: &likerID,
		Type:       domain.NotificationTypeScannerLike,
		Message:    msg,
	}
	if err := s.notifRepo.Create(ctx, n); err != nil {
		s.logger.Warn("create like notification", zap.Error(err))
	} else {
		pushNotif(s.wsHub, n)
	}

	// If match, also notify the liker
	if isMutual {
		matchNotifID := uuid.New().String()
		mn := &domain.Notification{
			ID:         matchNotifID,
			UserID:     likerID,
			FromUserID: &targetUserID,
			Type:       domain.NotificationTypeScannerLike,
			Message:    "У вас мэтч! Подойдите друг к другу",
		}
		if err := s.notifRepo.Create(ctx, mn); err != nil {
			s.logger.Warn("create match notification", zap.Error(err))
		} else {
			pushNotif(s.wsHub, mn)
		}
	}

	return nil
}

// ── Matches ─────────────────────────────────────────────────────────────────

// GetMatches returns mutual likes.
func (s *ScannerService) GetMatches(ctx context.Context, userID string, limit, offset int) ([]*postgres.WaveRow, error) {
	return s.scannerRepo.GetMatches(ctx, userID, limit, offset)
}

// ── Connect QR ──────────────────────────────────────────────────────────────

const connectTokenTTL = 30 * time.Second

// GenerateConnectQR creates a short-lived QR token.
func (s *ScannerService) GenerateConnectQR(ctx context.Context, userID string) (*domain.ConnectToken, error) {
	token, expiresAt, err := s.scannerRepo.CreateConnectToken(ctx, userID, connectTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("generate connect QR: %w", err)
	}
	return &domain.ConnectToken{
		Token:     token,
		QRValue:   "seeu://connect/" + token,
		ExpiresAt: expiresAt,
	}, nil
}

// AcceptConnect validates token, checks mutual like + proximity, creates chat.
func (s *ScannerService) AcceptConnect(ctx context.Context, scannerUserID, token string) (string, error) {
	// 1. Validate token
	qrOwnerID, err := s.scannerRepo.ValidateConnectToken(ctx, token)
	if err != nil {
		return "", fmt.Errorf("invalid or expired QR code")
	}

	if qrOwnerID == scannerUserID {
		return "", domain.ErrSelfAction
	}

	// 2. Check mutual like
	isMutual, err := s.scannerRepo.HasMutualLike(ctx, scannerUserID, qrOwnerID)
	if err != nil {
		return "", fmt.Errorf("check mutual like: %w", err)
	}
	if !isMutual {
		return "", fmt.Errorf("mutual like required")
	}

	// 3. Check proximity (at least one direction)
	nearbyAB, _ := s.scannerRepo.AreNearby(ctx, scannerUserID, qrOwnerID)
	nearbyBA, _ := s.scannerRepo.AreNearby(ctx, qrOwnerID, scannerUserID)
	if !nearbyAB && !nearbyBA {
		return "", fmt.Errorf("you must be physically nearby")
	}

	// 4. Check if chat already exists
	hasChat, _ := s.scannerRepo.HasChatBetween(ctx, scannerUserID, qrOwnerID)
	if hasChat {
		return "", fmt.Errorf("chat already exists")
	}

	// 5. Mark token as used
	if err := s.scannerRepo.MarkConnectTokenUsed(ctx, token, scannerUserID); err != nil {
		s.logger.Warn("mark connect token used", zap.Error(err))
	}

	// 6. Create 1-on-1 chat
	if s.chatRepo == nil {
		return "", fmt.Errorf("chat service not available")
	}
	chatID, err := s.chatRepo.GetOrCreateConversation(ctx, scannerUserID, qrOwnerID)
	if err != nil {
		return "", fmt.Errorf("create chat: %w", err)
	}

	// 7. Notify both
	for _, pair := range [][2]string{{scannerUserID, qrOwnerID}, {qrOwnerID, scannerUserID}} {
		nid := uuid.New().String()
		fromID := pair[1]
		n := &domain.Notification{
			ID:         nid,
			UserID:     pair[0],
			FromUserID: &fromID,
			Type:       domain.NotificationTypeScannerLike,
			Message:    "Общение началось! Напишите друг другу",
		}
		_ = s.notifRepo.Create(ctx, n)
		pushNotif(s.wsHub, n)
	}

	return chatID, nil
}

// ── Heartbeat ───────────────────────────────────────────────────────────────

func (s *ScannerService) ReportHeartbeat(ctx context.Context, userID string, visibleHashes []string) error {
	return s.scannerRepo.ReportHeartbeat(ctx, userID, visibleHashes)
}

// ── Lists ───────────────────────────────────────────────────────────────────

func (s *ScannerService) GetReceivedWaves(ctx context.Context, userID string, limit, offset int) ([]*postgres.WaveRow, int, error) {
	rows, err := s.scannerRepo.GetReceivedWaves(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	count, err := s.scannerRepo.CountReceivedWaves(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	return rows, count, nil
}

func (s *ScannerService) GetSentWaves(ctx context.Context, userID string, limit, offset int) ([]*domain.ScanProfile, error) {
	return s.scannerRepo.GetSentWaveTargets(ctx, userID, limit, offset)
}

func (s *ScannerService) UnseenLikesCount(ctx context.Context, userID string) (int, error) {
	return s.scannerRepo.UnseenWavesCount(ctx, userID)
}

func (s *ScannerService) MarkLikesSeen(ctx context.Context, userID string) error {
	return s.scannerRepo.MarkWavesSeen(ctx, userID)
}

// ── Legacy aliases ──────────────────────────────────────────────────────────

func (s *ScannerService) PostLike(ctx context.Context, likerID, publicIDHex string) error {
	return s.PostWave(ctx, likerID, publicIDHex)
}

func (s *ScannerService) RemoveLike(ctx context.Context, likerID, publicIDHex string) error {
	return nil
}

func (s *ScannerService) GetReceivedLikes(ctx context.Context, userID string, limit, offset int) ([]*postgres.ScannerLikeRow, int, error) {
	return s.GetReceivedWaves(ctx, userID, limit, offset)
}

func (s *ScannerService) GetSentLikes(ctx context.Context, userID string, limit, offset int) ([]*domain.ScanProfile, error) {
	return s.GetSentWaves(ctx, userID, limit, offset)
}
