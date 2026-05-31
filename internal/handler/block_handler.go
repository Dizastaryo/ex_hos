package handler

import (
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/service"
)

type BlockHandler struct {
	service *service.BlockService
	logger  *zap.Logger
}

func NewBlockHandler(s *service.BlockService, l *zap.Logger) *BlockHandler {
	return &BlockHandler{service: s, logger: l}
}

// Block godoc
// POST /api/v1/users/:username/block
func (h *BlockHandler) Block(c *fiber.Ctx) error {
	blockerID := middleware.GetUserID(c)
	target := c.Params("username")
	if err := h.service.Block(c.Context(), blockerID, target); err != nil {
		switch err {
		case domain.ErrUserNotFound:
			return respondError(c, fiber.StatusNotFound, "user not found")
		case domain.ErrInvalidInput:
			return respondError(c, fiber.StatusBadRequest, "cannot block self")
		default:
			h.logger.Error("block user", zap.Error(err))
			return respondError(c, fiber.StatusInternalServerError, "failed to block")
		}
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// Unblock godoc
// DELETE /api/v1/users/:username/block
func (h *BlockHandler) Unblock(c *fiber.Ctx) error {
	blockerID := middleware.GetUserID(c)
	target := c.Params("username")
	if err := h.service.Unblock(c.Context(), blockerID, target); err != nil {
		if err == domain.ErrUserNotFound {
			return respondError(c, fiber.StatusNotFound, "user not found")
		}
		h.logger.Error("unblock user", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to unblock")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ListBlocked godoc
// GET /api/v1/users/me/blocks
func (h *BlockHandler) ListBlocked(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	items, err := h.service.ListBlocked(c.Context(), userID, limit, offset)
	if err != nil {
		h.logger.Error("list blocked", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to list blocked")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"items": items}, nil)
}
