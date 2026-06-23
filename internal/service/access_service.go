package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	"go.uber.org/zap"
)

const accessQRTTL = 5 * time.Minute

type AccessService struct {
	accessRepo  *postgres.AccessRepository
	scannerRepo *postgres.ScannerRepository
	logger      *zap.Logger
}

func NewAccessService(
	accessRepo *postgres.AccessRepository,
	scannerRepo *postgres.ScannerRepository,
	logger *zap.Logger,
) *AccessService {
	return &AccessService{
		accessRepo:  accessRepo,
		scannerRepo: scannerRepo,
		logger:      logger,
	}
}

// GenerateQR creates a 5-minute token and returns QR data for the current user.
func (s *AccessService) GenerateQR(ctx context.Context, userID string) (*domain.ConnectToken, error) {
	token, expiresAt, err := s.scannerRepo.CreateConnectToken(ctx, userID, accessQRTTL)
	if err != nil {
		return nil, fmt.Errorf("generate access qr: %w", err)
	}
	return &domain.ConnectToken{
		Token:     token,
		QRValue:   "seeu://access/" + token,
		ExpiresAt: expiresAt,
	}, nil
}

// ScanQR validates a token and grants mutual access between the scanner and the QR owner.
func (s *AccessService) ScanQR(ctx context.Context, scannerID, token string) error {
	ownerID, err := s.scannerRepo.ValidateConnectToken(ctx, token)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("validate token: %w", err)
	}
	if ownerID == scannerID {
		return domain.ErrInvalidInput
	}

	if err := s.accessRepo.GrantAccess(ctx, scannerID, ownerID); err != nil {
		return fmt.Errorf("grant access: %w", err)
	}

	// Mark token as used (non-fatal).
	if err := s.scannerRepo.MarkConnectTokenUsed(ctx, token, scannerID); err != nil {
		s.logger.Warn("mark connect token used", zap.Error(err))
	}
	return nil
}

// CheckAccess reports whether two users have mutual access.
func (s *AccessService) CheckAccess(ctx context.Context, userAID, userBID string) (bool, error) {
	return s.accessRepo.HasAccess(ctx, userAID, userBID)
}

// RevokeAccess removes mutual access between two users.
func (s *AccessService) RevokeAccess(ctx context.Context, userAID, userBID string) error {
	return s.accessRepo.RevokeAccess(ctx, userAID, userBID)
}

// ListAccessPartners returns all users who have access with userID.
func (s *AccessService) ListAccessPartners(ctx context.Context, userID string, limit, offset int) ([]*domain.AccessPartner, error) {
	partners, err := s.accessRepo.ListAccessPartners(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	if partners == nil {
		partners = []*domain.AccessPartner{}
	}
	return partners, nil
}
