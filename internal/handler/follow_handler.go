package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/service"
	"go.uber.org/zap"
)

type FollowHandler struct {
	followService *service.FollowService
	logger        *zap.Logger
}

func NewFollowHandler(followService *service.FollowService, logger *zap.Logger) *FollowHandler {
	return &FollowHandler{
		followService: followService,
		logger:        logger,
	}
}

// Follow godoc
// POST /api/v1/users/:username/follow
//
// Response payload:
//
//	{ "status": "following" }   // public account or already-mutual private
//	{ "status": "requested" }   // private account, queued for approval
func (h *FollowHandler) Follow(c *fiber.Ctx) error {
	username := c.Params("username")
	followerID := middleware.GetUserID(c)

	res, err := h.followService.Follow(c.Context(), followerID, username)
	if err != nil {
		if err == domain.ErrUserNotFound {
			return respondError(c, fiber.StatusNotFound, "user not found")
		}
		if err == domain.ErrAlreadyFollowing {
			return respondError(c, fiber.StatusConflict, "already following")
		}
		if err == domain.ErrSelfFollow {
			return respondError(c, fiber.StatusBadRequest, "cannot follow yourself")
		}
		if err == domain.ErrForbidden {
			return respondError(c, fiber.StatusForbidden, "blocked")
		}
		h.logger.Error("follow user", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to follow user")
	}

	return respondSuccess(c, fiber.StatusOK, res, nil)
}

// Unfollow godoc
// DELETE /api/v1/users/:username/follow
func (h *FollowHandler) Unfollow(c *fiber.Ctx) error {
	username := c.Params("username")
	followerID := middleware.GetUserID(c)

	if err := h.followService.Unfollow(c.Context(), followerID, username); err != nil {
		if err == domain.ErrUserNotFound {
			return respondError(c, fiber.StatusNotFound, "user not found")
		}
		if err == domain.ErrNotFollowing {
			return respondError(c, fiber.StatusConflict, "not following")
		}
		h.logger.Error("unfollow user", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to unfollow user")
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "unfollowed"}, nil)
}

// ListMyFollowRequests godoc
// GET /api/v1/users/me/follow-requests
func (h *FollowHandler) ListMyFollowRequests(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)

	rows, err := h.followService.ListPendingRequests(c.Context(), userID, limit, offset)
	if err != nil {
		h.logger.Error("list follow requests", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to list requests")
	}
	if rows == nil {
		return respondSuccess(c, fiber.StatusOK, []struct{}{}, nil)
	}
	return respondSuccess(c, fiber.StatusOK, rows, nil)
}

// AcceptFollowRequest godoc
// POST /api/v1/follow-requests/:id/accept
func (h *FollowHandler) AcceptFollowRequest(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	if err := h.followService.AcceptRequest(c.Context(), id, userID); err != nil {
		if err == domain.ErrNotFound {
			return respondError(c, fiber.StatusNotFound, "request not found")
		}
		if err == domain.ErrForbidden {
			return respondError(c, fiber.StatusForbidden, "not your request")
		}
		h.logger.Error("accept follow request", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to accept")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// DeclineFollowRequest godoc
// POST /api/v1/follow-requests/:id/decline
func (h *FollowHandler) DeclineFollowRequest(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	if err := h.followService.DeclineRequest(c.Context(), id, userID); err != nil {
		if err == domain.ErrNotFound {
			return respondError(c, fiber.StatusNotFound, "request not found")
		}
		if err == domain.ErrForbidden {
			return respondError(c, fiber.StatusForbidden, "not your request")
		}
		h.logger.Error("decline follow request", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to decline")
	}
	return c.SendStatus(fiber.StatusNoContent)
}
