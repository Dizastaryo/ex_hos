package handler

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/service"
	"go.uber.org/zap"
)

type AccessHandler struct {
	accessService *service.AccessService
	logger        *zap.Logger
}

func NewAccessHandler(accessService *service.AccessService, logger *zap.Logger) *AccessHandler {
	return &AccessHandler{accessService: accessService, logger: logger}
}

// GenerateQR godoc
// POST /api/v1/access/qr
// Генерирует 5-минутный QR-токен для текущего пользователя.
func (h *AccessHandler) GenerateQR(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	data, err := h.accessService.GenerateQR(c.Context(), userID)
	if err != nil {
		h.logger.Error("generate access qr", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to generate QR")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"token":      data.Token,
		"qr_value":   data.QRValue,
		"expires_at": data.ExpiresAt,
	}, nil)
}

// ScanQR godoc
// POST /api/v1/access/scan
// body: { "token": "..." }
// Валидирует токен и открывает взаимный доступ.
func (h *AccessHandler) ScanQR(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req domain.ScanQRRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if req.Token == "" {
		return respondError(c, fiber.StatusBadRequest, "token is required")
	}

	if err := h.accessService.ScanQR(c.Context(), userID, req.Token); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return respondError(c, fiber.StatusGone, "QR-код устарел или недействителен")
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			return respondError(c, fiber.StatusBadRequest, "нельзя сканировать собственный QR-код")
		}
		h.logger.Error("scan access qr", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to process QR")
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{"granted": true}, nil)
}

// CheckAccess godoc
// GET /api/v1/access/check/:userId
// Проверяет наличие взаимного доступа с указанным пользователем.
func (h *AccessHandler) CheckAccess(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	targetID := c.Params("userId")
	if targetID == "" {
		return respondError(c, fiber.StatusBadRequest, "userId is required")
	}

	has, err := h.accessService.CheckAccess(c.Context(), userID, targetID)
	if err != nil {
		h.logger.Error("check access", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to check access")
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{"has_access": has}, nil)
}

// RevokeAccess godoc
// DELETE /api/v1/access/:userId
// Отзывает взаимный доступ с указанным пользователем.
func (h *AccessHandler) RevokeAccess(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	targetID := c.Params("userId")
	if targetID == "" {
		return respondError(c, fiber.StatusBadRequest, "userId is required")
	}

	if err := h.accessService.RevokeAccess(c.Context(), userID, targetID); err != nil {
		h.logger.Error("revoke access", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to revoke access")
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ListAccessPartners godoc
// GET /api/v1/access/list
// Возвращает список пользователей, с которыми есть взаимный доступ.
func (h *AccessHandler) ListAccessPartners(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	if limit > 100 {
		limit = 100
	}

	partners, err := h.accessService.ListAccessPartners(c.Context(), userID, limit, offset)
	if err != nil {
		h.logger.Error("list access partners", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to list access partners")
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{"items": partners}, nil)
}
