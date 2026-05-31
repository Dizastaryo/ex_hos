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

type PostHandler struct {
	postService *service.PostService
	validate    *validator.Validate
	logger      *zap.Logger
}

func NewPostHandler(postService *service.PostService, validate *validator.Validate, logger *zap.Logger) *PostHandler {
	return &PostHandler{
		postService: postService,
		validate:    validate,
		logger:      logger,
	}
}

// CreatePost godoc
// POST /api/v1/posts
func (h *PostHandler) CreatePost(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req domain.CreatePostRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}

	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}

	post, err := h.postService.Create(c.Context(), userID, &req)
	if err != nil {
		h.logger.Error("create post", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to create post")
	}

	return respondSuccess(c, fiber.StatusCreated, post, nil)
}

// GetPost godoc
// GET /api/v1/posts/:id
func (h *PostHandler) GetPost(c *fiber.Ctx) error {
	postID := c.Params("id")
	viewerID := middleware.GetUserID(c)

	post, err := h.postService.GetByID(c.Context(), postID, viewerID)
	if err != nil {
		if err == domain.ErrPostNotFound {
			return respondError(c, fiber.StatusNotFound, "post not found")
		}
		h.logger.Error("get post", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get post")
	}

	return respondSuccess(c, fiber.StatusOK, post, nil)
}

// DeletePost godoc
// DELETE /api/v1/posts/:id
func (h *PostHandler) DeletePost(c *fiber.Ctx) error {
	postID := c.Params("id")
	userID := middleware.GetUserID(c)

	if err := h.postService.Delete(c.Context(), postID, userID); err != nil {
		if err == domain.ErrPostNotFound {
			return respondError(c, fiber.StatusNotFound, "post not found")
		}
		if err == domain.ErrForbidden {
			return respondError(c, fiber.StatusForbidden, "access denied")
		}
		h.logger.Error("delete post", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to delete post")
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "post deleted"}, nil)
}

// MarkViewed godoc
// POST /api/v1/posts/:id/view
//
// FEED-5: frontend вызывает после ~5 сек просмотра в viewport. Backend
// записывает (post_id, user_id) → post_views; следующие feed-запросы юзера
// этот пост уже не вернут (dedup).
func (h *PostHandler) MarkViewed(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == "" {
		return respondError(c, fiber.StatusUnauthorized, "auth required")
	}
	postID := c.Params("id")
	if postID == "" {
		return respondError(c, fiber.StatusBadRequest, "post id required")
	}
	if err := h.postService.MarkViewed(c.Context(), postID, userID); err != nil {
		h.logger.Warn("mark post viewed", zap.Error(err))
		// Non-fatal — view-tracking лучшая-усилие.
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}

// GetFeed godoc
// GET /api/v1/feed?cursor=<opaque>&limit=20
//   OR ?page=1&limit=20 — legacy/backward-compat
//   OR ?sort=smart&page=1&limit=20 — FEED-2 умная лента (score-based ranking)
//
// FEED-1: cursor → стабильная pagination без skip/dup при insert'ах между
// страницами. Response meta включает `next_cursor` если есть еще данные.
// FEED-2: при sort=smart переключаем на score-based offset-pagination.
func (h *PostHandler) GetFeed(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	sort := c.Query("sort", "")
	cursor := c.Query("cursor", "")
	_, limit := pagination.ParsePage("1", c.Query("limit", "20"))

	// FEED-2: smart-ranking mode. Offset-paginated (cursor по float-score'у
	// fragile). Frontend переключает toggle'ом.
	if sort == "smart" {
		page, _ := pagination.ParsePage(c.Query("page", "1"), c.Query("limit", "20"))
		posts, meta, err := h.postService.GetFeedSmart(c.Context(), userID, page, limit)
		if err != nil {
			h.logger.Error("get feed smart", zap.Error(err))
			return respondError(c, fiber.StatusInternalServerError, "failed to get feed")
		}
		return respondSuccess(c, fiber.StatusOK, posts, meta)
	}

	// Cursor flow (default для новых клиентов).
	if cursor != "" || c.Query("page", "") == "" && c.Query("cursor", "") == "" {
		posts, nextCursor, err := h.postService.GetFeedByCursor(c.Context(), userID, cursor, limit)
		if err != nil {
			h.logger.Error("get feed cursor", zap.Error(err))
			return respondError(c, fiber.StatusInternalServerError, "failed to get feed")
		}
		meta := fiber.Map{
			"limit":         limit,
			"next_cursor":   nextCursor,
			"has_next_page": nextCursor != "",
		}
		return respondSuccess(c, fiber.StatusOK, posts, meta)
	}

	// Legacy page-based.
	page, _ := pagination.ParsePage(c.Query("page", "1"), c.Query("limit", "20"))
	posts, meta, err := h.postService.GetFeed(c.Context(), userID, page, limit)
	if err != nil {
		h.logger.Error("get feed", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get feed")
	}
	return respondSuccess(c, fiber.StatusOK, posts, meta)
}

// GetExplore godoc
// GET /api/v1/explore
func (h *PostHandler) GetExplore(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	page, limit := pagination.ParsePage(c.Query("page", "1"), c.Query("limit", "20"))
	mediaType := c.Query("media_type") // "video" → only video posts (reels)

	posts, meta, err := h.postService.GetExplore(c.Context(), userID, page, limit, mediaType)
	if err != nil {
		h.logger.Error("get explore", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get explore")
	}

	return respondSuccess(c, fiber.StatusOK, posts, meta)
}

// React godoc
// POST /api/v1/posts/:id/react   body: {"emoji": "👍"}
func (h *PostHandler) React(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	postID := c.Params("id")

	var req struct {
		Emoji string `json:"emoji"`
	}
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	emoji := req.Emoji
	if emoji == "" {
		return respondError(c, fiber.StatusBadRequest, "emoji is required")
	}
	// Match chat-reactions cap: typical emoji codepoint = 4 bytes, with VS16
	// modifier 7-8. Anything bigger is abuse.
	if len(emoji) > 16 {
		return respondError(c, fiber.StatusBadRequest, "emoji too long")
	}

	counts, err := h.postService.SetReaction(c.Context(), postID, userID, emoji)
	if err != nil {
		if err == domain.ErrPostNotFound {
			return respondError(c, fiber.StatusNotFound, "post not found")
		}
		h.logger.Error("react post", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to react")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"post_id":     postID,
		"reactions":   counts,
		"my_reaction": emoji,
	}, nil)
}

// Unreact godoc
// DELETE /api/v1/posts/:id/react
func (h *PostHandler) Unreact(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	postID := c.Params("id")

	counts, err := h.postService.RemoveReaction(c.Context(), postID, userID)
	if err != nil {
		if err == domain.ErrPostNotFound {
			return respondError(c, fiber.StatusNotFound, "post not found")
		}
		h.logger.Error("unreact post", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to unreact")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"post_id":   postID,
		"reactions": counts,
	}, nil)
}
