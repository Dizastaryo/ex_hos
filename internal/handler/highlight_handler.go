package handler

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/service"
	"go.uber.org/zap"
)

type HighlightHandler struct {
	highlightService *service.HighlightService
	validate         *validator.Validate
	logger           *zap.Logger
}

func NewHighlightHandler(highlightService *service.HighlightService, validate *validator.Validate, logger *zap.Logger) *HighlightHandler {
	return &HighlightHandler{
		highlightService: highlightService,
		validate:         validate,
		logger:           logger,
	}
}

// GetHighlights godoc
// GET /api/v1/highlights/:username
func (h *HighlightHandler) GetHighlights(c *fiber.Ctx) error {
	username := c.Params("username")
	viewerID := middleware.GetUserID(c)

	highlights, err := h.highlightService.GetByUsername(c.Context(), username, viewerID)
	if err != nil {
		if err == domain.ErrUserNotFound {
			return respondError(c, fiber.StatusNotFound, "user not found")
		}
		if err == domain.ErrPrivateAccount {
			return respondError(c, fiber.StatusForbidden, "this account is private")
		}
		h.logger.Error("get highlights", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get highlights")
	}

	return respondSuccess(c, fiber.StatusOK, highlights, nil)
}

// CreateHighlight godoc
// POST /api/v1/highlights
func (h *HighlightHandler) CreateHighlight(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req domain.CreateHighlightRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}

	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}

	highlight, err := h.highlightService.Create(c.Context(), userID, &req)
	if err != nil {
		h.logger.Error("create highlight", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to create highlight")
	}

	return respondSuccess(c, fiber.StatusCreated, highlight, nil)
}

// UpdateHighlight godoc
// PUT /api/v1/highlights/:id
func (h *HighlightHandler) UpdateHighlight(c *fiber.Ctx) error {
	highlightID := c.Params("id")
	userID := middleware.GetUserID(c)

	var req domain.UpdateHighlightRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}

	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}

	highlight, err := h.highlightService.Update(c.Context(), highlightID, userID, &req)
	if err != nil {
		if err == domain.ErrHighlightNotFound {
			return respondError(c, fiber.StatusNotFound, "highlight not found")
		}
		if err == domain.ErrForbidden {
			return respondError(c, fiber.StatusForbidden, "access denied")
		}
		h.logger.Error("update highlight", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to update highlight")
	}

	return respondSuccess(c, fiber.StatusOK, highlight, nil)
}

// DeleteHighlight godoc
// DELETE /api/v1/highlights/:id
func (h *HighlightHandler) DeleteHighlight(c *fiber.Ctx) error {
	highlightID := c.Params("id")
	userID := middleware.GetUserID(c)

	if err := h.highlightService.Delete(c.Context(), highlightID, userID); err != nil {
		if err == domain.ErrHighlightNotFound {
			return respondError(c, fiber.StatusNotFound, "highlight not found")
		}
		if err == domain.ErrForbidden {
			return respondError(c, fiber.StatusForbidden, "access denied")
		}
		h.logger.Error("delete highlight", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to delete highlight")
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "highlight deleted"}, nil)
}
