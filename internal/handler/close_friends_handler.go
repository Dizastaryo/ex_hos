package handler

import (
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/service"
)

type CloseFriendsHandler struct {
	service *service.CloseFriendsService
	logger  *zap.Logger
}

func NewCloseFriendsHandler(s *service.CloseFriendsService, l *zap.Logger) *CloseFriendsHandler {
	return &CloseFriendsHandler{service: s, logger: l}
}

// AddCloseFriend POST /api/v1/users/:username/close-friend
func (h *CloseFriendsHandler) Add(c *fiber.Ctx) error {
	ownerID := middleware.GetUserID(c)
	target := c.Params("username")
	if err := h.service.Add(c.Context(), ownerID, target); err != nil {
		switch err {
		case domain.ErrUserNotFound:
			return respondError(c, fiber.StatusNotFound, "user not found")
		case domain.ErrInvalidInput:
			return respondError(c, fiber.StatusBadRequest, "cannot add self")
		default:
			h.logger.Error("add close friend", zap.Error(err))
			return respondError(c, fiber.StatusInternalServerError, "failed")
		}
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// RemoveCloseFriend DELETE /api/v1/users/:username/close-friend
func (h *CloseFriendsHandler) Remove(c *fiber.Ctx) error {
	ownerID := middleware.GetUserID(c)
	target := c.Params("username")
	if err := h.service.Remove(c.Context(), ownerID, target); err != nil {
		if err == domain.ErrUserNotFound {
			return respondError(c, fiber.StatusNotFound, "user not found")
		}
		h.logger.Error("remove close friend", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ListCloseFriends GET /api/v1/users/me/close-friends
func (h *CloseFriendsHandler) List(c *fiber.Ctx) error {
	ownerID := middleware.GetUserID(c)
	limit := c.QueryInt("limit", 100)
	offset := c.QueryInt("offset", 0)
	items, err := h.service.List(c.Context(), ownerID, limit, offset)
	if err != nil {
		h.logger.Error("list close friends", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"items": items}, nil)
}
