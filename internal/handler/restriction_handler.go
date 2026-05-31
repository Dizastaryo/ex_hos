package handler

import (
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/service"
)

// RestrictionHandler — PROFILE-4. POST/DELETE /users/:username/restrict
// + GET /users/me/restrictions для UI «список ограниченных».
type RestrictionHandler struct {
	service *service.RestrictionService
	logger  *zap.Logger
}

func NewRestrictionHandler(s *service.RestrictionService, l *zap.Logger) *RestrictionHandler {
	return &RestrictionHandler{service: s, logger: l}
}

func (h *RestrictionHandler) Restrict(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	target := c.Params("username")
	if err := h.service.Restrict(c.Context(), userID, target); err != nil {
		switch err {
		case domain.ErrUserNotFound:
			return respondError(c, fiber.StatusNotFound, "user not found")
		case domain.ErrInvalidInput:
			return respondError(c, fiber.StatusBadRequest, "cannot restrict self")
		default:
			h.logger.Error("restrict user", zap.Error(err))
			return respondError(c, fiber.StatusInternalServerError, "failed to restrict")
		}
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *RestrictionHandler) Unrestrict(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	target := c.Params("username")
	if err := h.service.Unrestrict(c.Context(), userID, target); err != nil {
		if err == domain.ErrUserNotFound {
			return respondError(c, fiber.StatusNotFound, "user not found")
		}
		h.logger.Error("unrestrict user", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to unrestrict")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *RestrictionHandler) ListRestricted(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	items, err := h.service.List(c.Context(), userID, limit, offset)
	if err != nil {
		h.logger.Error("list restricted", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to list restricted")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"items": items}, nil)
}
