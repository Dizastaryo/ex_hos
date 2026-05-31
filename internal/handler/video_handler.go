package handler

import (
	"errors"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/service"
	"github.com/seeu/backend/pkg/pagination"
	"go.uber.org/zap"
)

type VideoHandler struct {
	videoService *service.VideoService
	validate     *validator.Validate
	logger       *zap.Logger
}

func NewVideoHandler(videoService *service.VideoService, validate *validator.Validate, logger *zap.Logger) *VideoHandler {
	return &VideoHandler{videoService: videoService, validate: validate, logger: logger}
}

// POST /api/v1/videos
func (h *VideoHandler) CreateVideo(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	var req domain.CreateVideoRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}
	video, err := h.videoService.CreateVideo(c.Context(), userID, &req)
	if err != nil {
		h.logger.Error("create video", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to create video")
	}
	return respondSuccess(c, fiber.StatusCreated, video, nil)
}

// GET /api/v1/videos
func (h *VideoHandler) ListVideos(c *fiber.Ctx) error {
	categoryID := c.Query("category_id")
	p := pagination.FromFiber(c.Query("page", "1"), c.Query("limit", "20"))
	videos, total, err := h.videoService.ListVideos(c.Context(), categoryID, p.Limit, p.Offset)
	if err != nil {
		h.logger.Error("list videos", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to list videos")
	}
	return respondSuccess(c, fiber.StatusOK, videos, pagination.MetaFromTotal(total, p))
}

// GET /api/v1/videos/featured
func (h *VideoHandler) GetFeatured(c *fiber.Ctx) error {
	video, err := h.videoService.GetFeatured(c.Context())
	if err != nil {
		if err == domain.ErrVideoNotFound {
			return respondSuccess(c, fiber.StatusOK, nil, nil)
		}
		h.logger.Error("get featured", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get featured video")
	}
	return respondSuccess(c, fiber.StatusOK, video, nil)
}

// GET /api/v1/videos/:id
func (h *VideoHandler) GetVideo(c *fiber.Ctx) error {
	id := c.Params("id")
	viewerID := middleware.GetUserID(c)
	video, err := h.videoService.GetVideo(c.Context(), id, viewerID)
	if err != nil {
		if err == domain.ErrVideoNotFound {
			return respondError(c, fiber.StatusNotFound, "video not found")
		}
		return respondError(c, fiber.StatusInternalServerError, "failed to get video")
	}
	return respondSuccess(c, fiber.StatusOK, video, nil)
}

// DELETE /api/v1/videos/:id
func (h *VideoHandler) DeleteVideo(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	if err := h.videoService.DeleteVideo(c.Context(), id, userID); err != nil {
		if err == domain.ErrVideoNotFound {
			return respondError(c, fiber.StatusNotFound, "video not found")
		}
		return respondError(c, fiber.StatusInternalServerError, "failed to delete video")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "video deleted"}, nil)
}

// POST /api/v1/videos/:id/view
func (h *VideoHandler) ViewVideo(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	_ = h.videoService.ViewVideo(c.Context(), id, userID)
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "view recorded"}, nil)
}

// POST /api/v1/videos/:id/like
func (h *VideoHandler) LikeVideo(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	if err := h.videoService.LikeVideo(c.Context(), id, userID); err != nil {
		return respondError(c, fiber.StatusInternalServerError, "failed to like video")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "liked"}, nil)
}

// DELETE /api/v1/videos/:id/like
func (h *VideoHandler) UnlikeVideo(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	if err := h.videoService.UnlikeVideo(c.Context(), id, userID); err != nil {
		return respondError(c, fiber.StatusInternalServerError, "failed to unlike video")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "unliked"}, nil)
}

// GET /api/v1/videos/categories
func (h *VideoHandler) GetCategories(c *fiber.Ctx) error {
	cats, err := h.videoService.GetCategories(c.Context())
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, "failed to get categories")
	}
	return respondSuccess(c, fiber.StatusOK, cats, nil)
}

// GET /api/v1/users/:id/videos
func (h *VideoHandler) GetUserVideos(c *fiber.Ctx) error {
	ownerID := c.Params("id") // user_id passed from social service lookup
	viewerID := middleware.GetUserID(c)
	p := pagination.FromFiber(c.Query("page", "1"), c.Query("limit", "20"))
	videos, total, err := h.videoService.GetUserVideos(c.Context(), ownerID, viewerID, p.Limit, p.Offset)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return respondError(c, fiber.StatusNotFound, "user not found")
		}
		if errors.Is(err, domain.ErrPrivateAccount) {
			return respondError(c, fiber.StatusForbidden, "this account is private")
		}
		return respondError(c, fiber.StatusInternalServerError, "failed to get user videos")
	}
	return respondSuccess(c, fiber.StatusOK, videos, pagination.MetaFromTotal(total, p))
}
