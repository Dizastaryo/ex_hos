package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/service"
	"github.com/seeu/backend/pkg/pagination"
	"go.uber.org/zap"
)

type NotificationHandler struct {
	notifService *service.NotificationService
	logger       *zap.Logger
}

func NewNotificationHandler(notifService *service.NotificationService, logger *zap.Logger) *NotificationHandler {
	return &NotificationHandler{
		notifService: notifService,
		logger:       logger,
	}
}

// GetNotifications godoc
// GET /api/v1/notifications
func (h *NotificationHandler) GetNotifications(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	page, limit := pagination.ParsePage(c.Query("page", "1"), c.Query("limit", "20"))

	notifications, meta, err := h.notifService.GetByUserID(c.Context(), userID, page, limit)
	if err != nil {
		h.logger.Error("get notifications", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get notifications")
	}

	return respondSuccess(c, fiber.StatusOK, notifications, meta)
}

// MarkAllRead godoc
// PUT /api/v1/notifications/read
func (h *NotificationHandler) MarkAllRead(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	if err := h.notifService.MarkAllAsRead(c.Context(), userID); err != nil {
		h.logger.Error("mark all notifications as read", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to mark notifications as read")
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "all notifications marked as read"}, nil)
}

// MarkRead godoc
// PUT /api/v1/notifications/:id/read
func (h *NotificationHandler) MarkRead(c *fiber.Ctx) error {
	notifID := c.Params("id")
	userID := middleware.GetUserID(c)

	if err := h.notifService.MarkAsRead(c.Context(), notifID, userID); err != nil {
		h.logger.Error("mark notification as read", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to mark notification as read")
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "notification marked as read"}, nil)
}
