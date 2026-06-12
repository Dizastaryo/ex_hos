package handler

import (
	"encoding/json"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/repository/postgres"
	"go.uber.org/zap"
)

type ReadingHandler struct {
	repo     *postgres.ReadingRepository
	validate *validator.Validate
	logger   *zap.Logger
}

func NewReadingHandler(repo *postgres.ReadingRepository, validate *validator.Validate, logger *zap.Logger) *ReadingHandler {
	return &ReadingHandler{repo: repo, validate: validate, logger: logger}
}

// PUT /api/v1/files/:id/progress
func (h *ReadingHandler) UpsertProgress(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	fileID := c.Params("id")

	var req domain.UpsertProgressRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}
	if !json.Valid(req.Position) {
		return respondError(c, fiber.StatusBadRequest, "position must be valid JSON")
	}

	if err := h.repo.UpsertProgress(c.Context(), userID, fileID, req.Position); err != nil {
		h.logger.Error("upsert progress", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to save progress")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}

// GET /api/v1/files/:id/progress
func (h *ReadingHandler) GetProgress(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	fileID := c.Params("id")

	p, err := h.repo.GetProgress(c.Context(), userID, fileID)
	if err != nil {
		h.logger.Error("get progress", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get progress")
	}
	if p == nil {
		return c.SendStatus(fiber.StatusNoContent)
	}
	return respondSuccess(c, fiber.StatusOK, p, nil)
}

// GET /api/v1/files/:id/bookmarks
func (h *ReadingHandler) GetBookmarks(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	fileID := c.Params("id")

	bookmarks, err := h.repo.GetBookmarks(c.Context(), userID, fileID)
	if err != nil {
		h.logger.Error("get bookmarks", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get bookmarks")
	}
	if bookmarks == nil {
		bookmarks = []*domain.FileBookmark{}
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"items": bookmarks}, nil)
}

// POST /api/v1/files/:id/bookmarks
func (h *ReadingHandler) CreateBookmark(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	fileID := c.Params("id")

	var req domain.CreateBookmarkRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}

	pos := req.Position
	if len(pos) == 0 {
		pos = json.RawMessage("{}")
	}

	b := &domain.FileBookmark{
		UserID:   userID,
		FileID:   fileID,
		Position: pos,
		Note:     req.Note,
	}
	if err := h.repo.CreateBookmark(c.Context(), b); err != nil {
		h.logger.Error("create bookmark", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to create bookmark")
	}
	return respondSuccess(c, fiber.StatusCreated, b, nil)
}

// DELETE /api/v1/files/bookmarks/:bookmarkId
func (h *ReadingHandler) DeleteBookmark(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	bookmarkID := c.Params("bookmarkId")

	if err := h.repo.DeleteBookmark(c.Context(), bookmarkID, userID); err != nil {
		return respondError(c, fiber.StatusInternalServerError, "failed to delete bookmark")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}

// GET /api/v1/files/:id/reading-status
func (h *ReadingHandler) GetReadingStatus(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	fileID := c.Params("id")

	s, err := h.repo.GetReadingStatus(c.Context(), userID, fileID)
	if err != nil {
		h.logger.Error("get reading status", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get status")
	}
	if s == nil {
		return c.SendStatus(fiber.StatusNoContent)
	}
	return respondSuccess(c, fiber.StatusOK, s, nil)
}

// PUT /api/v1/files/:id/reading-status
func (h *ReadingHandler) UpsertReadingStatus(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	fileID := c.Params("id")

	var req domain.UpsertReadingStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}

	if err := h.repo.UpsertReadingStatus(c.Context(), userID, fileID, req.Status); err != nil {
		h.logger.Error("upsert reading status", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to update status")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"status": req.Status}, nil)
}

// DELETE /api/v1/files/:id/reading-status
func (h *ReadingHandler) DeleteReadingStatus(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	fileID := c.Params("id")

	if err := h.repo.DeleteReadingStatus(c.Context(), userID, fileID); err != nil {
		h.logger.Error("delete reading status", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to delete status")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}

// GET /api/v1/users/me/reading-list?status=reading|want|done&cursor=...&limit=20
func (h *ReadingHandler) GetReadingList(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	status := c.Query("status", "reading")
	cursor := c.Query("cursor")
	limit := c.QueryInt("limit", 20)
	if limit > 100 {
		limit = 100
	}

	files, nextCursor, err := h.repo.GetUserReadingList(c.Context(), userID, status, cursor, limit)
	if err != nil {
		h.logger.Error("get reading list", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get reading list")
	}
	if files == nil {
		files = []*domain.File{}
	}
	return respondSuccess(c, fiber.StatusOK, files, fiber.Map{"next_cursor": nextCursor})
}
