package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/service"
	"go.uber.org/zap"
)

type LikeHandler struct {
	likeService *service.LikeService
	logger      *zap.Logger
}

func NewLikeHandler(likeService *service.LikeService, logger *zap.Logger) *LikeHandler {
	return &LikeHandler{
		likeService: likeService,
		logger:      logger,
	}
}

// LikePost godoc
// POST /api/v1/posts/:id/like
func (h *LikeHandler) LikePost(c *fiber.Ctx) error {
	postID := c.Params("id")
	userID := middleware.GetUserID(c)

	if err := h.likeService.LikePost(c.Context(), postID, userID); err != nil {
		if err == domain.ErrPostNotFound {
			return respondError(c, fiber.StatusNotFound, "post not found")
		}
		if err == domain.ErrAlreadyLiked {
			return respondError(c, fiber.StatusConflict, "already liked")
		}
		h.logger.Error("like post", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to like post")
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "post liked"}, nil)
}

// UnlikePost godoc
// DELETE /api/v1/posts/:id/like
func (h *LikeHandler) UnlikePost(c *fiber.Ctx) error {
	postID := c.Params("id")
	userID := middleware.GetUserID(c)

	if err := h.likeService.UnlikePost(c.Context(), postID, userID); err != nil {
		if err == domain.ErrNotLiked {
			return respondError(c, fiber.StatusConflict, "not liked")
		}
		h.logger.Error("unlike post", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to unlike post")
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "post unliked"}, nil)
}

// LikeStory godoc
// POST /api/v1/stories/:id/like
func (h *LikeHandler) LikeStory(c *fiber.Ctx) error {
	storyID := c.Params("id")
	userID := middleware.GetUserID(c)

	if err := h.likeService.LikeStory(c.Context(), storyID, userID); err != nil {
		if err == domain.ErrStoryNotFound {
			return respondError(c, fiber.StatusNotFound, "story not found")
		}
		if err == domain.ErrAlreadyLiked {
			return respondError(c, fiber.StatusConflict, "already liked")
		}
		h.logger.Error("like story", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to like story")
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "story liked"}, nil)
}

// UnlikeStory godoc
// DELETE /api/v1/stories/:id/like
func (h *LikeHandler) UnlikeStory(c *fiber.Ctx) error {
	storyID := c.Params("id")
	userID := middleware.GetUserID(c)

	if err := h.likeService.UnlikeStory(c.Context(), storyID, userID); err != nil {
		if err == domain.ErrNotLiked {
			return respondError(c, fiber.StatusConflict, "not liked")
		}
		h.logger.Error("unlike story", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to unlike story")
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "story unliked"}, nil)
}

// LikeComment godoc
// POST /api/v1/comments/:id/like
func (h *LikeHandler) LikeComment(c *fiber.Ctx) error {
	commentID := c.Params("id")
	userID := middleware.GetUserID(c)

	if err := h.likeService.LikeComment(c.Context(), commentID, userID); err != nil {
		if err == domain.ErrCommentNotFound {
			return respondError(c, fiber.StatusNotFound, "comment not found")
		}
		if err == domain.ErrAlreadyLiked {
			return respondError(c, fiber.StatusConflict, "already liked")
		}
		h.logger.Error("like comment", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to like comment")
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "comment liked"}, nil)
}

// UnlikeComment godoc
// DELETE /api/v1/comments/:id/like
func (h *LikeHandler) UnlikeComment(c *fiber.Ctx) error {
	commentID := c.Params("id")
	userID := middleware.GetUserID(c)

	if err := h.likeService.UnlikeComment(c.Context(), commentID, userID); err != nil {
		if err == domain.ErrNotLiked {
			return respondError(c, fiber.StatusConflict, "not liked")
		}
		h.logger.Error("unlike comment", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to unlike comment")
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "comment unliked"}, nil)
}

// SavePost godoc
// POST /api/v1/posts/:id/save
func (h *LikeHandler) SavePost(c *fiber.Ctx) error {
	postID := c.Params("id")
	userID := middleware.GetUserID(c)

	if err := h.likeService.SavePost(c.Context(), postID, userID); err != nil {
		if err == domain.ErrPostNotFound {
			return respondError(c, fiber.StatusNotFound, "post not found")
		}
		if err == domain.ErrAlreadySaved {
			return respondError(c, fiber.StatusConflict, "already saved")
		}
		h.logger.Error("save post", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to save post")
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "post saved"}, nil)
}

// UnsavePost godoc
// DELETE /api/v1/posts/:id/save
func (h *LikeHandler) UnsavePost(c *fiber.Ctx) error {
	postID := c.Params("id")
	userID := middleware.GetUserID(c)

	if err := h.likeService.UnsavePost(c.Context(), postID, userID); err != nil {
		if err == domain.ErrNotSaved {
			return respondError(c, fiber.StatusConflict, "not saved")
		}
		h.logger.Error("unsave post", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to unsave post")
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "post unsaved"}, nil)
}
