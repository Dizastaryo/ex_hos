package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/service"
	"go.uber.org/zap"
)

type ScannerHandler struct {
	scannerService *service.ScannerService
	logger         *zap.Logger
}

func NewScannerHandler(scannerService *service.ScannerService, logger *zap.Logger) *ScannerHandler {
	return &ScannerHandler{scannerService: scannerService, logger: logger}
}

// ── Resolve (real profiles) ──────────────────────────────────────────────────

// ResolveScanProfile GET /scanner/resolve/:deviceHash
func (h *ScannerHandler) ResolveScanProfile(c *fiber.Ctx) error {
	deviceHash := c.Params("deviceHash")
	if deviceHash == "" {
		return respondError(c, fiber.StatusBadRequest, "deviceHash is required")
	}
	profile, err := h.scannerService.ResolveScanProfile(c.Context(), deviceHash)
	if err != nil {
		if err == domain.ErrNotFound {
			return respondError(c, fiber.StatusNotFound, "device not found or visibility is off")
		}
		h.logger.Error("resolve scan profile", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to resolve")
	}
	return respondSuccess(c, fiber.StatusOK, profile, nil)
}

// ResolveScanProfiles POST /scanner/resolve
func (h *ScannerHandler) ResolveScanProfiles(c *fiber.Ctx) error {
	var body struct {
		DeviceHashes []string `json:"device_hashes"`
	}
	if err := c.BodyParser(&body); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid body")
	}
	if len(body.DeviceHashes) == 0 {
		return respondSuccess(c, fiber.StatusOK, fiber.Map{"profiles": []any{}}, nil)
	}
	if len(body.DeviceHashes) > 50 {
		return respondError(c, fiber.StatusBadRequest, "max 50 device_hashes per request")
	}
	profiles, err := h.scannerService.ResolveScanProfiles(c.Context(), body.DeviceHashes)
	if err != nil {
		h.logger.Error("batch resolve", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to resolve")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"profiles": profiles}, nil)
}

// ── Connect QR (will be replaced by Access system in Sprint 3) ──────────────

// GenerateConnectQR POST /connect/qr
func (h *ScannerHandler) GenerateConnectQR(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	token, err := h.scannerService.GenerateConnectQR(c.Context(), userID)
	if err != nil {
		h.logger.Error("generate connect QR", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to generate QR")
	}
	return respondSuccess(c, fiber.StatusOK, token, nil)
}

// AcceptConnect POST /connect/accept
func (h *ScannerHandler) AcceptConnect(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	var body struct {
		Token string `json:"token"`
	}
	if err := c.BodyParser(&body); err != nil || body.Token == "" {
		return respondError(c, fiber.StatusBadRequest, "token is required")
	}
	chatID, err := h.scannerService.AcceptConnect(c.Context(), userID, body.Token)
	if err != nil {
		if err == domain.ErrSelfAction {
			return respondError(c, fiber.StatusBadRequest, "cannot connect with yourself")
		}
		return respondError(c, fiber.StatusBadRequest, err.Error())
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"chat_id": chatID}, nil)
}
