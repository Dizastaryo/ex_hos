package handler

import (
	"encoding/json"
	"fmt"

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

// GET /api/v1/users/me/reading-stats
func (h *ReadingHandler) GetReadingStats(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	stats, err := h.repo.GetReadingStats(c.Context(), userID)
	if err != nil {
		h.logger.Error("get reading stats", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get reading stats")
	}
	return respondSuccess(c, fiber.StatusOK, stats, nil)
}

// GET /api/v1/users/me/recently-read?limit=10
func (h *ReadingHandler) GetRecentlyRead(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	limit := c.QueryInt("limit", 10)
	if limit > 50 {
		limit = 50
	}
	files, err := h.repo.GetRecentlyRead(c.Context(), userID, limit)
	if err != nil {
		h.logger.Error("get recently read", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get recently read")
	}
	if files == nil {
		files = []*domain.File{}
	}
	return respondSuccess(c, fiber.StatusOK, files, nil)
}

// GET /api/v1/reading/leaderboard?metric=books|pages&limit=20
func (h *ReadingHandler) GetLeaderboard(c *fiber.Ctx) error {
	metric := c.Query("metric", "books")
	limit := c.QueryInt("limit", 20)
	if limit > 100 {
		limit = 100
	}
	board, err := h.repo.GetReadingLeaderboard(c.Context(), metric, limit)
	if err != nil {
		h.logger.Error("get reading leaderboard", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get leaderboard")
	}
	if board == nil {
		board = []map[string]interface{}{}
	}
	return respondSuccess(c, fiber.StatusOK, board, nil)
}

// GET /api/v1/reading/activity?days=7
func (h *ReadingHandler) GetReadingActivity(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	days := c.QueryInt("days", 7)
	if days > 365 {
		days = 365
	}
	activity, err := h.repo.GetReadingActivity(c.Context(), userID, days)
	if err != nil {
		h.logger.Error("get reading activity", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get activity")
	}
	return respondSuccess(c, fiber.StatusOK, activity, nil)
}

// GET /api/v1/users/me/reading-goal?year=2026
func (h *ReadingHandler) GetReadingGoal(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	year := c.QueryInt("year", 0)
	if year == 0 {
		year = c.Context().Time().Year()
	}
	goal, err := h.repo.GetReadingGoal(c.Context(), userID, year)
	if err != nil {
		h.logger.Error("get reading goal", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get reading goal")
	}
	if goal == nil {
		return respondSuccess(c, fiber.StatusOK, nil, fiber.Map{"year": year})
	}
	return respondSuccess(c, fiber.StatusOK, goal, nil)
}

// PUT /api/v1/users/me/reading-goal
func (h *ReadingHandler) UpsertReadingGoal(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	var req domain.UpsertReadingGoalRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid body")
	}
	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}
	year := c.QueryInt("year", 0)
	if year == 0 {
		year = c.Context().Time().Year()
	}
	goal, err := h.repo.UpsertReadingGoal(c.Context(), userID, year, req.GoalBooks)
	if err != nil {
		h.logger.Error("upsert reading goal", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to set reading goal")
	}
	return respondSuccess(c, fiber.StatusOK, goal, nil)
}

// DELETE /api/v1/users/me/reading-goal
func (h *ReadingHandler) DeleteReadingGoal(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	year := c.QueryInt("year", 0)
	if year == 0 {
		year = c.Context().Time().Year()
	}
	if err := h.repo.DeleteReadingGoal(c.Context(), userID, year); err != nil {
		return respondError(c, fiber.StatusInternalServerError, "failed to delete reading goal")
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

// GET /api/v1/files/:id/notes — get the user's private note for a file
func (h *ReadingHandler) GetFileNote(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	fileID := c.Params("id")
	content, err := h.repo.GetFileNote(c.Context(), userID, fileID)
	if err != nil {
		h.logger.Error("get file note", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get note")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"content": content}, nil)
}

// PUT /api/v1/files/:id/notes — upsert the user's private note for a file
func (h *ReadingHandler) UpsertFileNote(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	fileID := c.Params("id")
	var body struct {
		Content string `json:"content"`
	}
	if err := c.BodyParser(&body); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid body")
	}
	if len(body.Content) > 5000 {
		body.Content = body.Content[:5000]
	}
	if err := h.repo.UpsertFileNote(c.Context(), userID, fileID, body.Content); err != nil {
		h.logger.Error("upsert file note", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to save note")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}

// DELETE /api/v1/files/:id/notes — delete the user's private note for a file
func (h *ReadingHandler) DeleteFileNote(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	fileID := c.Params("id")
	if err := h.repo.DeleteFileNote(c.Context(), userID, fileID); err != nil {
		h.logger.Error("delete file note", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to delete note")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}

// ─── Page Reading Progress (honest tracker) ──────────────────────────────────

// GET /api/v1/files/:id/pages-progress
func (h *ReadingHandler) GetPageProgress(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	fileID := c.Params("id")
	pages, err := h.repo.GetPageProgress(c.Context(), userID, fileID)
	if err != nil {
		h.logger.Error("get page progress", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get page progress")
	}
	readCount, _ := h.repo.CountReadPages(c.Context(), userID, fileID)
	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"pages":          pages,
		"read_count":     readCount,
		"threshold_secs": postgres.PageReadThreshold,
	}, nil)
}

// PUT /api/v1/files/:id/pages-progress
// Body: {"pages": {"0": 15, "1": 8, "2": 30, ...}}
func (h *ReadingHandler) UpsertPageProgress(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	fileID := c.Params("id")

	var body struct {
		Pages map[string]int `json:"pages"`
	}
	if err := c.BodyParser(&body); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid body")
	}
	if len(body.Pages) == 0 {
		return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
	}
	if len(body.Pages) > 2000 {
		return respondError(c, fiber.StatusBadRequest, "too many pages (max 2000)")
	}

	// Convert string keys to int
	pages := make(map[int]int, len(body.Pages))
	for k, v := range body.Pages {
		var page int
		if _, err := fmt.Sscanf(k, "%d", &page); err != nil {
			continue
		}
		if page < 0 || v < 0 {
			continue
		}
		pages[page] = v
	}

	if err := h.repo.BatchUpsertPageProgress(c.Context(), userID, fileID, pages); err != nil {
		h.logger.Error("upsert page progress", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to save page progress")
	}

	readCount, _ := h.repo.CountReadPages(c.Context(), userID, fileID)
	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"ok":         true,
		"read_count": readCount,
	}, nil)
}
