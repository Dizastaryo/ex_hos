package handler

import (
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/service"
	"github.com/seeu/backend/pkg/pagination"
	"go.uber.org/zap"
)

type SborHandler struct {
	svc      *service.SborService
	validate *validator.Validate
	logger   *zap.Logger
}

func NewSborHandler(svc *service.SborService, validate *validator.Validate, logger *zap.Logger) *SborHandler {
	return &SborHandler{svc: svc, validate: validate, logger: logger}
}

// POST /api/v1/sbory
func (h *SborHandler) Create(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req domain.CreateSborRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}

	sbor, err := h.svc.Create(c.Context(), userID, &req)
	if err != nil {
		h.logger.Error("create sbor", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to create sbor")
	}
	return respondSuccess(c, fiber.StatusCreated, sbor, nil)
}

// GET /api/v1/sbory
func (h *SborHandler) List(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	page, limit := pagination.ParsePage(c.Query("page", "1"), c.Query("limit", "20"))
	typeFilter := c.Query("type")
	catFilter := c.Query("category")
	cityFilter := c.Query("city") // пустая строка = без фильтра по городу
	qFilter := strings.TrimSpace(c.Query("q"))

	var dateFrom, dateTo *time.Time
	if v := c.Query("date_from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			dateFrom = &t
		}
	}
	if v := c.Query("date_to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			dateTo = &t
		}
	}

	items, meta, err := h.svc.List(c.Context(), userID, typeFilter, catFilter, cityFilter, qFilter, dateFrom, dateTo, page, limit)
	if err != nil {
		h.logger.Error("list sbory", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to list sbory")
	}
	return respondSuccess(c, fiber.StatusOK, items, meta)
}

// GET /api/v1/sbory/me?past=true
func (h *SborHandler) ListMine(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	page, limit := pagination.ParsePage(c.Query("page", "1"), c.Query("limit", "50"))
	past := c.Query("past") == "true"

	items, meta, err := h.svc.ListMine(c.Context(), userID, past, page, limit)
	if err != nil {
		h.logger.Error("list my sbory", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to list sbory")
	}
	return respondSuccess(c, fiber.StatusOK, items, meta)
}

// GET /api/v1/sbory/:id
func (h *SborHandler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)

	sbor, err := h.svc.GetByID(c.Context(), id, userID)
	if err != nil {
		if err == domain.ErrSborNotFound {
			return respondError(c, fiber.StatusNotFound, "sbor not found")
		}
		h.logger.Error("get sbor", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get sbor")
	}
	return respondSuccess(c, fiber.StatusOK, sbor, nil)
}

// POST /api/v1/sbory/:id/requests  — подать заявку на вступление
func (h *SborHandler) SubmitRequest(c *fiber.Ctx) error {
	sborID := c.Params("id")
	userID := middleware.GetUserID(c)

	var body struct {
		Message string `json:"message"`
	}
	_ = c.BodyParser(&body)

	if err := h.svc.SubmitRequest(c.Context(), sborID, userID, body.Message); err != nil {
		switch err {
		case domain.ErrSborNotFound:
			return respondError(c, fiber.StatusNotFound, "sbor not found")
		case domain.ErrAlreadyJoined:
			return respondError(c, fiber.StatusConflict, "already a member")
		case domain.ErrAlreadyRequested:
			return respondError(c, fiber.StatusConflict, "request already pending")
		case domain.ErrForbidden:
			return respondError(c, fiber.StatusForbidden, "organizer cannot request to join")
		}
		h.logger.Error("submit sbor request", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to submit request")
	}
	return respondSuccess(c, fiber.StatusCreated, fiber.Map{"message": "request submitted"}, nil)
}

// DELETE /api/v1/sbory/:id/requests  — отозвать свою заявку
func (h *SborHandler) CancelRequest(c *fiber.Ctx) error {
	sborID := c.Params("id")
	userID := middleware.GetUserID(c)

	if err := h.svc.CancelRequest(c.Context(), sborID, userID); err != nil {
		if err == domain.ErrRequestNotFound {
			return respondError(c, fiber.StatusNotFound, "no pending request found")
		}
		h.logger.Error("cancel sbor request", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to cancel request")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "request cancelled"}, nil)
}

// GET /api/v1/sbory/:id/requests  — список заявок (только для организатора)
func (h *SborHandler) ListRequests(c *fiber.Ctx) error {
	sborID := c.Params("id")
	adminID := middleware.GetUserID(c)

	requests, err := h.svc.ListRequests(c.Context(), sborID, adminID)
	if err != nil {
		switch err {
		case domain.ErrSborNotFound:
			return respondError(c, fiber.StatusNotFound, "sbor not found")
		case domain.ErrForbidden:
			return respondError(c, fiber.StatusForbidden, "only host can view requests")
		}
		h.logger.Error("list sbor requests", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to list requests")
	}
	return respondSuccess(c, fiber.StatusOK, requests, nil)
}

// POST /api/v1/sbory/:id/requests/:reqID/approve
func (h *SborHandler) ApproveRequest(c *fiber.Ctx) error {
	reqID := c.Params("reqID")
	adminID := middleware.GetUserID(c)

	if err := h.svc.ApproveRequest(c.Context(), reqID, adminID); err != nil {
		switch err {
		case domain.ErrRequestNotFound:
			return respondError(c, fiber.StatusNotFound, "request not found")
		case domain.ErrSborFull:
			return respondError(c, fiber.StatusConflict, "sbor is full")
		case domain.ErrForbidden:
			return respondError(c, fiber.StatusForbidden, "only host can approve")
		}
		h.logger.Error("approve sbor request", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to approve request")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "request approved"}, nil)
}

// POST /api/v1/sbory/:id/requests/:reqID/reject
func (h *SborHandler) RejectRequest(c *fiber.Ctx) error {
	reqID := c.Params("reqID")
	adminID := middleware.GetUserID(c)

	if err := h.svc.RejectRequest(c.Context(), reqID, adminID); err != nil {
		switch err {
		case domain.ErrRequestNotFound:
			return respondError(c, fiber.StatusNotFound, "request not found")
		case domain.ErrForbidden:
			return respondError(c, fiber.StatusForbidden, "only host can reject")
		}
		h.logger.Error("reject sbor request", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to reject request")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "request rejected"}, nil)
}

// POST /api/v1/sbory/:id/join
func (h *SborHandler) Join(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)

	sbor, err := h.svc.Join(c.Context(), id, userID)
	if err != nil {
		switch err {
		case domain.ErrSborNotFound:
			return respondError(c, fiber.StatusNotFound, "sbor not found")
		case domain.ErrSborFull:
			return respondError(c, fiber.StatusConflict, "sbor is full")
		}
		h.logger.Error("join sbor", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to join sbor")
	}
	return respondSuccess(c, fiber.StatusOK, sbor, nil)
}

// DELETE /api/v1/sbory/:id/join
func (h *SborHandler) Leave(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)

	if err := h.svc.Leave(c.Context(), id, userID); err != nil {
		switch err {
		case domain.ErrSborNotFound:
			return respondError(c, fiber.StatusNotFound, "sbor not found")
		case domain.ErrNotJoined:
			return respondError(c, fiber.StatusConflict, "not a member")
		case domain.ErrForbidden:
			return respondError(c, fiber.StatusForbidden, "host cannot leave")
		}
		h.logger.Error("leave sbor", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to leave sbor")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "left sbor"}, nil)
}

// PATCH /api/v1/sbory/:id
func (h *SborHandler) Update(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)

	var req domain.UpdateSborRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}

	sbor, err := h.svc.Update(c.Context(), id, userID, &req)
	if err != nil {
		switch err {
		case domain.ErrSborNotFound:
			return respondError(c, fiber.StatusNotFound, "sbor not found")
		case domain.ErrForbidden:
			return respondError(c, fiber.StatusForbidden, "only host can update")
		case domain.ErrMaxSlotsConflict:
			return respondError(c, fiber.StatusConflict, "max slots cannot be less than current member count")
		}
		h.logger.Error("update sbor", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to update sbor")
	}
	return respondSuccess(c, fiber.StatusOK, sbor, nil)
}

// POST /api/v1/sbory/:id/bookmark  — добавить/убрать из закладок (toggle).
func (h *SborHandler) ToggleBookmark(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id := c.Params("id")
	saved, err := h.svc.ToggleBookmark(c.Context(), userID, id)
	if err != nil {
		if err == domain.ErrForbidden {
			return respondError(c, fiber.StatusForbidden, "host cannot bookmark own sbor")
		}
		h.logger.Error("toggle sbor bookmark", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to toggle bookmark")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"saved": saved}, nil)
}

// GET /api/v1/sbory/bookmarked  — список сохранённых сборов текущего юзера.
func (h *SborHandler) ListBookmarked(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	page, limit := pagination.ParsePage(c.Query("page", "1"), c.Query("limit", "20"))
	items, meta, err := h.svc.ListBookmarked(c.Context(), userID, page, limit)
	if err != nil {
		h.logger.Error("list bookmarked sbory", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to list bookmarked")
	}
	return respondSuccess(c, fiber.StatusOK, items, meta)
}

// DELETE /api/v1/sbory/:id
func (h *SborHandler) Cancel(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)

	if err := h.svc.Cancel(c.Context(), id, userID); err != nil {
		switch err {
		case domain.ErrSborNotFound:
			return respondError(c, fiber.StatusNotFound, "sbor not found")
		case domain.ErrForbidden:
			return respondError(c, fiber.StatusForbidden, "only host can cancel")
		}
		h.logger.Error("cancel sbor", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to cancel sbor")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "sbor cancelled"}, nil)
}

// GET /api/v1/sbory/:id/members
func (h *SborHandler) GetMembers(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	members, err := h.svc.ListMembers(c.Context(), id, userID)
	if err != nil {
		if err == domain.ErrSborNotFound {
			return respondError(c, fiber.StatusNotFound, "sbor not found")
		}
		if err == domain.ErrForbidden {
			return respondError(c, fiber.StatusForbidden, "access denied")
		}
		h.logger.Error("get sbor members", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get members")
	}
	return respondSuccess(c, fiber.StatusOK, members, nil)
}
