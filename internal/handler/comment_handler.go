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

type CommentHandler struct {
	commentService *service.CommentService
	validate       *validator.Validate
	logger         *zap.Logger
}

func NewCommentHandler(commentService *service.CommentService, validate *validator.Validate, logger *zap.Logger) *CommentHandler {
	return &CommentHandler{
		commentService: commentService,
		validate:       validate,
		logger:         logger,
	}
}

// GetComments godoc
// GET /api/v1/posts/:id/comments
func (h *CommentHandler) GetComments(c *fiber.Ctx) error {
	postID := c.Params("id")
	viewerID := middleware.GetUserID(c)
	page, limit := pagination.ParsePage(c.Query("page", "1"), c.Query("limit", "20"))

	comments, meta, err := h.commentService.GetByPostID(c.Context(), postID, viewerID, page, limit)
	if err != nil {
		if err == domain.ErrPostNotFound {
			return respondError(c, fiber.StatusNotFound, "post not found")
		}
		h.logger.Error("get comments", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get comments")
	}

	return respondSuccess(c, fiber.StatusOK, comments, meta)
}

// CreateComment godoc
// POST /api/v1/posts/:id/comments
func (h *CommentHandler) CreateComment(c *fiber.Ctx) error {
	postID := c.Params("id")
	userID := middleware.GetUserID(c)

	var req domain.CreateCommentRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}

	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}

	comment, err := h.commentService.Create(c.Context(), postID, userID, &req)
	if err != nil {
		if err == domain.ErrPostNotFound {
			return respondError(c, fiber.StatusNotFound, "post not found")
		}
		h.logger.Error("create comment", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to create comment")
	}

	return respondSuccess(c, fiber.StatusCreated, comment, nil)
}

// DeleteComment godoc
// DELETE /api/v1/comments/:id
func (h *CommentHandler) DeleteComment(c *fiber.Ctx) error {
	commentID := c.Params("id")
	userID := middleware.GetUserID(c)

	if err := h.commentService.Delete(c.Context(), commentID, userID); err != nil {
		if err == domain.ErrCommentNotFound {
			return respondError(c, fiber.StatusNotFound, "comment not found")
		}
		if err == domain.ErrForbidden {
			return respondError(c, fiber.StatusForbidden, "access denied")
		}
		h.logger.Error("delete comment", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to delete comment")
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "comment deleted"}, nil)
}

// GetReplies godoc
// GET /api/v1/comments/:id/replies
func (h *CommentHandler) GetReplies(c *fiber.Ctx) error {
	parentID := c.Params("id")
	viewerID := middleware.GetUserID(c)
	page, limit := pagination.ParsePage(c.Query("page", "1"), c.Query("limit", "20"))

	replies, meta, err := h.commentService.GetReplies(c.Context(), parentID, viewerID, page, limit)
	if err != nil {
		h.logger.Error("get replies", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get replies")
	}

	return respondSuccess(c, fiber.StatusOK, replies, meta)
}
