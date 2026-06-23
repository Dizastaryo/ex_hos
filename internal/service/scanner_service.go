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
	wsHub       *ws.Hub
	logger      *zap.Logger
}

func NewScannerService(
	scannerRepo *postgres.ScannerRepository,
	userRepo *postgres.UserRepository,
	notifRepo *postgres.NotificationRepository,
	wsHub *ws.Hub,
	logger *zap.Logger,
) *ScannerService {
	return &ScannerService{
		scannerRepo: scannerRepo,
		userRepo:    userRepo,
		notifRepo:   notifRepo,
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

// ── Connect QR (will be replaced by Access system in Sprint 3) ──────────────

const connectTokenTTL = 5 * time.Minute

// GenerateConnectQR creates a short-lived QR token.
func (s *ScannerService) GenerateConnectQR(ctx context.Context, userID string) (*domain.ConnectToken, error) {
	token, expiresAt, err := s.scannerRepo.CreateConnectToken(ctx, userID, connectTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("generate connect QR: %w", err)
	}
	return &domain.ConnectToken{
		Token:     token,
		QRValue:   "seeu://access/" + token,
		ExpiresAt: expiresAt,
	}, nil
}

// AcceptConnect validates token and creates a 1-on-1 chat between the scanner and QR owner.
func (s *ScannerService) AcceptConnect(ctx context.Context, scannerUserID, token string) (string, error) {
	qrOwnerID, err := s.scannerRepo.ValidateConnectToken(ctx, token)
	if err != nil {
		return "", fmt.Errorf("invalid or expired QR code")
	}

	if qrOwnerID == scannerUserID {
		return "", domain.ErrSelfAction
	}

	hasChat, _ := s.scannerRepo.HasChatBetween(ctx, scannerUserID, qrOwnerID)
	if hasChat {
		return "", fmt.Errorf("chat already exists")
	}

	if err := s.scannerRepo.MarkConnectTokenUsed(ctx, token, scannerUserID); err != nil {
		s.logger.Warn("mark connect token used", zap.Error(err))
	}

	if s.chatRepo == nil {
		return "", fmt.Errorf("chat service not available")
	}
	chatID, err := s.chatRepo.GetOrCreateConversation(ctx, scannerUserID, qrOwnerID)
	if err != nil {
		return "", fmt.Errorf("create chat: %w", err)
	}

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
