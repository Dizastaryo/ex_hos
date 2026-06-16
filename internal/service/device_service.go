package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	"go.uber.org/zap"
)

type DeviceService struct {
	deviceRepo *postgres.DeviceRepository
	userRepo   *postgres.UserRepository
	secret     string // SEEU_DEVICE_SECRET из конфига
	logger     *zap.Logger
}

func NewDeviceService(
	deviceRepo *postgres.DeviceRepository,
	userRepo *postgres.UserRepository,
	secret string,
	logger *zap.Logger,
) *DeviceService {
	return &DeviceService{
		deviceRepo: deviceRepo,
		userRepo:   userRepo,
		secret:     secret,
		logger:     logger,
	}
}

// GenerateDevices создаёт count новых BLE-устройств.
// Для каждого:
//   - серийник SEEU_XXXXXXX (7 цифр, автоинкремент через uuid tail)
//   - public_id_hex = HMAC-SHA256(secret, serial+"_pub")[0:8]  → 16 hex символов
//   - private_id_hex = HMAC-SHA256(secret, serial+"_prv")[0:8] → 16 hex символов
//
// Возвращает список созданных устройств (с private_id_hex — только при генерации).
func (s *DeviceService) GenerateDevices(ctx context.Context, req *domain.GenerateDevicesRequest) ([]*domain.BleDeviceExportRow, error) {
	result := make([]*domain.BleDeviceExportRow, 0, req.Count)

	for i := 0; i < req.Count; i++ {
		id := uuid.New().String()
		// Используем первые 7 символов UUID hex как суффикс серийника
		serial := fmt.Sprintf("SEEU_%s", id[:7])

		pubHex := s.deriveHex(serial, "_pub")
		prvHex := s.deriveHex(serial, "_prv")

		d := &domain.BleDevice{
			ID:           id,
			SerialNumber: serial,
			PublicIDHex:  pubHex,
			PrivateIDHex: prvHex,
			IsActive:     true,
			Notes:        req.Notes,
		}
		if err := s.deviceRepo.Create(ctx, d); err != nil {
			return nil, fmt.Errorf("generate device %d: %w", i+1, err)
		}
		result = append(result, &domain.BleDeviceExportRow{
			SerialNumber: serial,
			PublicIDHex:  pubHex,
			PrivateIDHex: prvHex,
		})
	}

	s.logger.Info("generated ble devices",
		zap.Int("count", req.Count),
		zap.String("notes", req.Notes),
	)
	return result, nil
}

// BindDeviceToUser привязывает браслет (по серийнику с QR) к аккаунту юзера.
// Если браслет уже привязан к другому юзеру — ErrAlreadyExists.
// Если серийник не найден или устройство деактивировано — ErrNotFound.
func (s *DeviceService) BindDeviceToUser(ctx context.Context, userID, serialNumber string) error {
	device, err := s.deviceRepo.GetBySerial(ctx, serialNumber)
	if err != nil {
		return err // ErrNotFound прокидываем как есть
	}

	taken, err := s.deviceRepo.IsPublicIDTaken(ctx, device.PublicIDHex)
	if err != nil {
		return fmt.Errorf("check device taken: %w", err)
	}
	if taken {
		// Проверяем — может это тот же юзер перепривязывает
		existing, _ := s.userRepo.GetByID(ctx, userID)
		if existing != nil && existing.DevicePublicID == device.PublicIDHex {
			return nil // уже привязан к нему же — ок
		}
		return domain.ErrAlreadyExists
	}

	if err := s.userRepo.SetDeviceIDs(ctx, userID, device.PublicIDHex, device.PrivateIDHex); err != nil {
		return fmt.Errorf("set device ids: %w", err)
	}
	return nil
}

// UnbindDevice отвязывает браслет от юзера (очищает device_public_id и device_private_id).
func (s *DeviceService) UnbindDevice(ctx context.Context, userID string) error {
	return s.userRepo.SetDeviceIDs(ctx, userID, "", "")
}

// ── Private Whitelist ─────────────────────────────────────────────────────────

// GetPrivateWhitelist возвращает текущий whitelist пользователя.
func (s *DeviceService) GetPrivateWhitelist(ctx context.Context, ownerID string) ([]*domain.PrivateWhitelistEntry, error) {
	return s.deviceRepo.GetPrivateWhitelist(ctx, ownerID)
}

// SetPrivateWhitelist заменяет whitelist на userIDs.
// Фильтрует: в whitelist попадают только взаимные подписчики.
// Пустой userIDs = очистить whitelist (никто не видит в private mode).
func (s *DeviceService) SetPrivateWhitelist(ctx context.Context, ownerID string, userIDs []string) error {
	if len(userIDs) == 0 {
		return s.deviceRepo.SetPrivateWhitelist(ctx, ownerID, nil)
	}
	// Оставляем только взаимных подписчиков — нельзя добавить чужого
	mutuals, err := s.deviceRepo.FilterMutualFollowers(ctx, ownerID, userIDs)
	if err != nil {
		return fmt.Errorf("filter mutuals: %w", err)
	}
	return s.deviceRepo.SetPrivateWhitelist(ctx, ownerID, mutuals)
}

// deriveHex вычисляет HMAC-SHA256(secret, serial+suffix) и возвращает первые 8 байт в hex (16 символов).
func (s *DeviceService) deriveHex(serial, suffix string) string {
	mac := hmac.New(sha256.New, []byte(s.secret))
	mac.Write([]byte(serial + suffix))
	return hex.EncodeToString(mac.Sum(nil)[:8])
}
