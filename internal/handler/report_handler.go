package handler

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/service"
)

type ReportHandler struct {
	service  *service.ReportService
	validate *validator.Validate
	logger   *zap.Logger
}

func NewReportHandler(s *service.ReportService, v *validator.Validate, l *zap.Logger) *ReportHandler {
	return &ReportHandler{service: s, validate: v, logger: l}
}

// Create godoc
// POST /api/v1/reports
//
// Files a content moderation report. Required by App Store community guidelines:
// users must be able to report posts, comments, stories or other users.
func (h *ReportHandler) Create(c *fiber.Ctx) error {
	var req domain.CreateReportRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}

	reporterID := middleware.GetUserID(c)
	report, err := h.service.Submit(c.Context(), reporterID, &req)
	if err != nil {
		h.logger.Error("submit report", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to submit report")
	}
	return respondSuccess(c, fiber.StatusCreated, report, nil)
}
