package handler

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/repository/postgres"
)

type VideoCommentHandler struct {
	repo   *postgres.VideoCommentRepository
	logger *zap.Logger
}

func NewVideoCommentHandler(repo *postgres.VideoCommentRepository, logger *zap.Logger) *VideoCommentHandler {
	return &VideoCommentHandler{repo: repo, logger: logger}
}

// List godoc
// GET /api/v1/videos/:id/comments?limit=&offset=
func (h *VideoCommentHandler) List(c *fiber.Ctx) error {
	videoID := c.Params("id")
	limit := c.QueryInt("limit", 50)
	if limit > 200 {
		limit = 200
	}
	offset := c.QueryInt("offset", 0)

	items, err := h.repo.List(c.Context(), videoID, limit, offset)
	if err != nil {
		h.logger.Error("list video comments", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to list comments")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"items": items}, nil)
}

type createVideoCommentReq struct {
	Text string `json:"text"`
}

// Create godoc
// POST /api/v1/videos/:id/comments
func (h *VideoCommentHandler) Create(c *fiber.Ctx) error {
	videoID := c.Params("id")
	userID := middleware.GetUserID(c)

	var req createVideoCommentReq
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return respondError(c, fiber.StatusBadRequest, "text is required")
	}
	if len(text) > 1000 {
		return respondError(c, fiber.StatusBadRequest, "text too long")
	}

	cmt, err := h.repo.Create(c.Context(), videoID, userID, text)
	if err != nil {
		h.logger.Error("create video comment", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to create comment")
	}
	return respondSuccess(c, fiber.StatusCreated, cmt, nil)
}

// Delete godoc
// DELETE /api/v1/video-comments/:id
func (h *VideoCommentHandler) Delete(c *fiber.Ctx) error {
	commentID := c.Params("id")
	userID := middleware.GetUserID(c)
	if err := h.repo.Delete(c.Context(), commentID, userID); err != nil {
		if err == domain.ErrNotFound {
			return respondError(c, fiber.StatusNotFound, "comment not found")
		}
		h.logger.Error("delete video comment", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to delete comment")
	}
	return c.SendStatus(fiber.StatusNoContent)
}
