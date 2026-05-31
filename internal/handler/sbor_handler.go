package handler

import (
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

	items, meta, err := h.svc.List(c.Context(), userID, typeFilter, catFilter, cityFilter, page, limit)
	if err != nil {
		h.logger.Error("list sbory", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to list sbory")
	}
	return respondSuccess(c, fiber.StatusOK, items, meta)
}

// GET /api/v1/sbory/me
func (h *SborHandler) ListMine(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	page, limit := pagination.ParsePage(c.Query("page", "1"), c.Query("limit", "20"))

	items, meta, err := h.svc.ListMine(c.Context(), userID, page, limit)
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
