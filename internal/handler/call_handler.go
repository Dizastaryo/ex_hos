package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/repository/postgres"
	"go.uber.org/zap"
)

// CallHandler — endpoints для истории звонков (C-1).
type CallHandler struct {
	callRepo *postgres.CallRepository
	logger   *zap.Logger
}

func NewCallHandler(callRepo *postgres.CallRepository, logger *zap.Logger) *CallHandler {
	return &CallHandler{callRepo: callRepo, logger: logger}
}

// GetMyCalls godoc
// GET /api/v1/me/calls?limit=50&offset=0
//
// Возвращает list звонков current user'а (incoming + outgoing) hydrated
// peer-фillds (username/fullname/avatar). Свежие сверху.
func (h *CallHandler) GetMyCalls(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == "" {
		return respondError(c, fiber.StatusUnauthorized, "auth required")
	}
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	if limit < 1 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	calls, err := h.callRepo.GetForUser(c.Context(), userID, limit, offset)
	if err != nil {
		h.logger.Error("get calls", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get calls")
	}
	if calls == nil {
		return respondSuccess(c, fiber.StatusOK, []struct{}{}, nil)
	}
	return respondSuccess(c, fiber.StatusOK, calls, nil)
}

// GetPendingCalls godoc (BUG-5)
// GET /api/v1/me/calls/pending
//
// Возвращает active pending invitations (status=pending, не старше 60 сек)
// адресованные current user'у. Frontend дёргает после reconnect'а WS чтобы
// догнать звонок который пришёл когда соединение было разорвано — иначе
// caller думает что invite ушёл, а callee никогда не получил.
func (h *CallHandler) GetPendingCalls(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == "" {
		return respondError(c, fiber.StatusUnauthorized, "auth required")
	}
	calls, err := h.callRepo.GetPendingFor(c.Context(), userID)
	if err != nil {
		h.logger.Error("get pending calls", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get pending calls")
	}
	if calls == nil {
		return respondSuccess(c, fiber.StatusOK, []struct{}{}, nil)
	}
	return respondSuccess(c, fiber.StatusOK, calls, nil)
}
