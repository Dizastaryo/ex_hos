package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/repository/postgres"
	"github.com/seeu/backend/internal/service"
	"github.com/seeu/backend/pkg/pagination"
	"go.uber.org/zap"
)

type SearchHandler struct {
	searchService *service.SearchService
	historyRepo   *postgres.SearchHistoryRepository
	logger        *zap.Logger
}

func NewSearchHandler(
	searchService *service.SearchService,
	historyRepo *postgres.SearchHistoryRepository,
	logger *zap.Logger,
) *SearchHandler {
	return &SearchHandler{
		searchService: searchService,
		historyRepo:   historyRepo,
		logger:        logger,
	}
}

// Search godoc
// GET /api/v1/search?q=&type=users|posts|all&page=1&limit=20
//
// When the caller is authenticated, non-empty queries are recorded into the
// per-user history so they can be replayed on the search screen.
func (h *SearchHandler) Search(c *fiber.Ctx) error {
	q := c.Query("q", "")
	searchType := c.Query("type", "all")
	page, limit := pagination.ParsePage(c.Query("page", "1"), c.Query("limit", "20"))

	if searchType != "users" && searchType != "posts" && searchType != "all" {
		searchType = "all"
	}

	results, meta, err := h.searchService.Search(c.Context(), q, searchType, page, limit)
	if err != nil {
		h.logger.Error("search", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "search failed")
	}

	// Record into history (best-effort, async-style — don't block response).
	if userID := middleware.GetUserID(c); userID != "" && q != "" {
		if err := h.historyRepo.Record(c.Context(), userID, q); err != nil {
			h.logger.Warn("record search history", zap.Error(err))
		}
	}

	return respondSuccess(c, fiber.StatusOK, results, meta)
}

// SearchHistory godoc
// GET /api/v1/search/history?limit=10
func (h *SearchHandler) SearchHistory(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	items, err := h.historyRepo.List(c.Context(), userID, c.QueryInt("limit", 10))
	if err != nil {
		h.logger.Error("list search history", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to list history")
	}
	if items == nil {
		items = []postgres.SearchHistoryItem{}
	}
	return respondSuccess(c, fiber.StatusOK, items, nil)
}

// DeleteSearchHistory godoc
// DELETE /api/v1/search/history?q=foo  → remove one
// DELETE /api/v1/search/history        → clear all
func (h *SearchHandler) DeleteSearchHistory(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	q := c.Query("q", "")
	var err error
	if q == "" {
		err = h.historyRepo.Clear(c.Context(), userID)
	} else {
		err = h.historyRepo.DeleteOne(c.Context(), userID, q)
	}
	if err != nil {
		h.logger.Error("delete search history", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to clear history")
	}
	return c.SendStatus(fiber.StatusNoContent)
}
