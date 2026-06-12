package handler

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/repository/postgres"
	"github.com/seeu/backend/internal/service"
	"github.com/seeu/backend/pkg/pagination"
	"go.uber.org/zap"
)

type UserHandler struct {
	userService   *service.UserService
	postService   *service.PostService
	followService *service.FollowService
	exportService *service.ExportService
	deviceService *service.DeviceService
	statsRepo     *postgres.UserStatsRepository
	validate      *validator.Validate
	logger        *zap.Logger
}

func NewUserHandler(
	userService *service.UserService,
	postService *service.PostService,
	followService *service.FollowService,
	exportService *service.ExportService,
	deviceService *service.DeviceService,
	statsRepo *postgres.UserStatsRepository,
	validate *validator.Validate,
	logger *zap.Logger,
) *UserHandler {
	return &UserHandler{
		userService:   userService,
		postService:   postService,
		followService: followService,
		exportService: exportService,
		deviceService: deviceService,
		statsRepo:     statsRepo,
		validate:      validate,
		logger:        logger,
	}
}

// GetMe godoc
// GET /api/v1/users/me
func (h *UserHandler) GetMe(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	user, err := h.userService.GetByID(c.Context(), userID)
	if err != nil {
		if err == domain.ErrUserNotFound {
			return respondError(c, fiber.StatusNotFound, "user not found")
		}
		h.logger.Error("get current user", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get user")
	}

	return respondSuccess(c, fiber.StatusOK, user, nil)
}

// UpdateMe godoc
// PUT /api/v1/users/me
func (h *UserHandler) UpdateMe(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req domain.UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}

	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}

	user, err := h.userService.UpdateProfile(c.Context(), userID, &req)
	if err != nil {
		if err == domain.ErrUserNotFound {
			return respondError(c, fiber.StatusNotFound, "user not found")
		}
		h.logger.Error("update user profile", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to update profile")
	}

	return respondSuccess(c, fiber.StatusOK, user, nil)
}

// ExportMe godoc
// GET /api/v1/users/me/export
//
// Returns a JSON file with all of the authenticated user's data: profile,
// posts, stories, comments, likes given, saved posts, follow lists,
// and messages they sent. Required by GDPR / KZ ПДн.
func (h *UserHandler) ExportMe(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	dump, err := h.exportService.BuildExport(c.Context(), userID)
	if err != nil {
		h.logger.Error("export account", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to build export")
	}

	c.Set("Content-Type", "application/json; charset=utf-8")
	c.Set("Content-Disposition", `attachment; filename="seeu-export.json"`)
	return c.JSON(dump)
}

// DeleteMe godoc
// DELETE /api/v1/users/me
//
// Permanently deletes the authenticated user. Cascades wipe posts, stories,
// comments, likes, follows, chats and library files. Required by App Store
// review guidelines (account deletion in-app, no support email).
func (h *UserHandler) DeleteMe(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	if err := h.userService.DeleteAccount(c.Context(), userID); err != nil {
		if err == domain.ErrUserNotFound {
			return respondError(c, fiber.StatusNotFound, "user not found")
		}
		h.logger.Error("delete account", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to delete account")
	}

	h.logger.Info("account deleted", zap.String("user_id", userID))
	return c.SendStatus(fiber.StatusNoContent)
}

type bindDeviceReq struct {
	// SerialNumber — серийный номер с QR-наклейки браслета (SEEU_XXXXXXX).
	// Backend резолвит серийник в public/private id и сохраняет хэши в аккаунт.
	SerialNumber string `json:"serial_number" validate:"required,min=4,max=40"`
}

// BindMyDevice godoc
// POST /api/v1/users/me/device
//
// Привязать браслет к аккаунту по серийному номеру с QR-наклейки.
// Если браслет уже привязан к другому юзеру — 409 Conflict.
// Если серийник не найден или деактивирован — 404.
func (h *UserHandler) BindMyDevice(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	var req bindDeviceReq
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}
	if err := h.deviceService.BindDeviceToUser(c.Context(), userID, req.SerialNumber); err != nil {
		switch err {
		case domain.ErrAlreadyExists:
			return respondError(c, fiber.StatusConflict, "device already bound to another user")
		case domain.ErrNotFound:
			return respondError(c, fiber.StatusNotFound, "device serial not found or inactive")
		}
		h.logger.Error("bind device", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to bind device")
	}
	// Инвалидируем кэш чтобы /me вернул свежий device_public_id
	_ = h.userService.InvalidateCache(c.Context(), userID)
	return c.SendStatus(fiber.StatusNoContent)
}

// UnbindMyDevice godoc
// DELETE /api/v1/users/me/device
func (h *UserHandler) UnbindMyDevice(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if err := h.userService.BindDevice(c.Context(), userID, ""); err != nil {
		h.logger.Error("unbind device", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to unbind")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// GetByDevicePrivate godoc (BUG-17)
// GET /api/v1/users/by-device-private/:privateId
//
// Резолвит приватный BLE-id (mode=0x01 packet) в одного из follow'ed
// юзеров viewer'а. Privacy guard: возвращает 404 если match не найден
// среди whitelist'а follow'ed (чтобы не leak'ать чужие private_id).
// Auth required.
func (h *UserHandler) GetByDevicePrivate(c *fiber.Ctx) error {
	privateID := c.Params("privateId")
	if privateID == "" {
		return respondError(c, fiber.StatusBadRequest, "privateId is required")
	}
	viewerID := middleware.GetUserID(c)
	user, err := h.userService.GetByDevicePrivateIDForViewer(
		c.Context(), viewerID, privateID,
	)
	if err != nil {
		if err == domain.ErrUserNotFound {
			return respondError(c, fiber.StatusNotFound, "no matching user among friends")
		}
		if err == domain.ErrUnauthorized {
			return respondError(c, fiber.StatusUnauthorized, "auth required")
		}
		h.logger.Error("get user by private device", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to lookup")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"id":          user.ID,
		"username":    user.Username,
		"full_name":   user.FullName,
		"avatar_url":  user.AvatarURL,
		"is_verified": user.IsVerified,
	}, nil)
}

// GetByDevice godoc
// GET /api/v1/users/by-device/:publicId
//
// Резолвит BLE device public_id_hex в анонимный scan-профиль.
// Возвращает ТОЛЬКО scan_alias + scan_avatar_url + device_hash.
// Реальный аккаунт (username, avatar, id) НЕ возвращается.
// scan_enabled=false → 404 (сервер-сайд privacy toggle).
func (h *UserHandler) GetByDevice(c *fiber.Ctx) error {
	publicID := c.Params("publicId")
	if publicID == "" {
		return respondError(c, fiber.StatusBadRequest, "publicId is required")
	}
	profile, err := h.userService.GetScanProfileByDeviceHash(c.Context(), publicID)
	if err != nil {
		if err == domain.ErrUserNotFound {
			return respondError(c, fiber.StatusNotFound, "no user with this device")
		}
		h.logger.Error("get scan profile by device", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to lookup")
	}
	return respondSuccess(c, fiber.StatusOK, profile, nil)
}

// GetPrivateWhitelist godoc
// GET /api/v1/users/me/private-whitelist
//
// Список пользователей которые видят тебя в private BLE-режиме.
// Только взаимные подписчики могут попасть в whitelist.
func (h *UserHandler) GetPrivateWhitelist(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	entries, err := h.deviceService.GetPrivateWhitelist(c.Context(), userID)
	if err != nil {
		h.logger.Error("get private whitelist", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get whitelist")
	}
	if entries == nil {
		entries = []*domain.PrivateWhitelistEntry{}
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"items": entries}, nil)
}

// SetPrivateWhitelist godoc
// PUT /api/v1/users/me/private-whitelist
// body: { "user_ids": ["uuid1", "uuid2"] }
//
// Заменяет whitelist — кто видит тебя в private BLE-режиме (mode=0x01).
// Не-взаимные подписчики из списка молча отбрасываются.
// Пустой массив [] = никто не видит.
func (h *UserHandler) SetPrivateWhitelist(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	var req domain.SetPrivateWhitelistRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if err := h.deviceService.SetPrivateWhitelist(c.Context(), userID, req.UserIDs); err != nil {
		h.logger.Error("set private whitelist", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to set whitelist")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// UpdateScanProfile godoc
// PUT /api/v1/users/me/scan-profile
//
// Обновить анонимный образ пользователя в BLE-сканере:
// scan_alias (псевдоним), scan_avatar_url, scan_enabled (toggle видимости).
func (h *UserHandler) UpdateScanProfile(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	var req domain.UpdateScanProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}
	if err := h.userService.UpdateScanProfile(c.Context(), userID, &req); err != nil {
		h.logger.Error("update scan profile", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to update scan profile")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// GetByUsername godoc
// GET /api/v1/users/:username
func (h *UserHandler) GetByUsername(c *fiber.Ctx) error {
	username := c.Params("username")
	viewerID := middleware.GetUserID(c)

	profile, err := h.userService.GetByUsername(c.Context(), username, viewerID)
	if err != nil {
		if err == domain.ErrUserNotFound {
			return respondError(c, fiber.StatusNotFound, "user not found")
		}
		h.logger.Error("get user by username", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get user")
	}

	return respondSuccess(c, fiber.StatusOK, profile, nil)
}

// GetUserPosts godoc
// GET /api/v1/users/:username/posts
func (h *UserHandler) GetUserPosts(c *fiber.Ctx) error {
	username := c.Params("username")
	viewerID := middleware.GetUserID(c)
	page, limit := pagination.ParsePage(c.Query("page", "1"), c.Query("limit", "20"))

	posts, meta, err := h.postService.GetByUsername(c.Context(), username, viewerID, page, limit)
	if err != nil {
		if err == domain.ErrUserNotFound {
			return respondError(c, fiber.StatusNotFound, "user not found")
		}
		if err == domain.ErrPrivateAccount {
			return respondError(c, fiber.StatusForbidden, "this account is private")
		}
		h.logger.Error("get user posts", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get posts")
	}

	return respondSuccess(c, fiber.StatusOK, posts, meta)
}

// GetSavedPosts godoc
// GET /api/v1/users/:username/saved
func (h *UserHandler) GetSavedPosts(c *fiber.Ctx) error {
	username := c.Params("username")
	viewerID := middleware.GetUserID(c)
	page, limit := pagination.ParsePage(c.Query("page", "1"), c.Query("limit", "20"))

	posts, meta, err := h.postService.GetSaved(c.Context(), viewerID, username, page, limit)
	if err != nil {
		if err == domain.ErrForbidden {
			return respondError(c, fiber.StatusForbidden, "access denied")
		}
		if err == domain.ErrUserNotFound {
			return respondError(c, fiber.StatusNotFound, "user not found")
		}
		h.logger.Error("get saved posts", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get saved posts")
	}

	return respondSuccess(c, fiber.StatusOK, posts, meta)
}

// GetFollowers godoc
// GET /api/v1/users/:username/followers
func (h *UserHandler) GetFollowers(c *fiber.Ctx) error {
	username := c.Params("username")
	viewerID := middleware.GetUserID(c)
	page, limit := pagination.ParsePage(c.Query("page", "1"), c.Query("limit", "20"))

	followers, err := h.userService.GetFollowers(c.Context(), username, viewerID, page, limit)
	if err != nil {
		if err == domain.ErrUserNotFound {
			return respondError(c, fiber.StatusNotFound, "user not found")
		}
		if err == domain.ErrPrivateAccount {
			return respondError(c, fiber.StatusForbidden, "this account is private")
		}
		h.logger.Error("get followers", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get followers")
	}

	meta := pagination.NewMeta(page, limit, len(followers))
	return respondSuccess(c, fiber.StatusOK, followers, meta)
}

// GET /api/v1/users/me/mutuals — mutual followers of the current user.
func (h *UserHandler) GetMutuals(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	mutuals, err := h.userService.GetMutuals(c.Context(), userID)
	if err != nil {
		h.logger.Error("get mutuals", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get mutuals")
	}
	if mutuals == nil {
		mutuals = []*domain.UserShort{}
	}
	return respondSuccess(c, fiber.StatusOK, mutuals, nil)
}

// GetFollowing godoc
// GET /api/v1/users/:username/following
func (h *UserHandler) GetFollowing(c *fiber.Ctx) error {
	username := c.Params("username")
	viewerID := middleware.GetUserID(c)
	page, limit := pagination.ParsePage(c.Query("page", "1"), c.Query("limit", "20"))

	following, err := h.userService.GetFollowing(c.Context(), username, viewerID, page, limit)
	if err != nil {
		if err == domain.ErrUserNotFound {
			return respondError(c, fiber.StatusNotFound, "user not found")
		}
		if err == domain.ErrPrivateAccount {
			return respondError(c, fiber.StatusForbidden, "this account is private")
		}
		h.logger.Error("get following", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get following")
	}

	meta := pagination.NewMeta(page, limit, len(following))
	return respondSuccess(c, fiber.StatusOK, following, meta)
}

// GET /api/v1/leaderboard?limit=50
func (h *UserHandler) Leaderboard(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 50)
	if limit < 1 || limit > 100 {
		limit = 50
	}
	entries, err := h.statsRepo.TopUsers(c.Context(), limit)
	if err != nil {
		h.logger.Error("leaderboard", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get leaderboard")
	}
	if entries == nil {
		entries = []*domain.LeaderboardEntry{}
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"items": entries}, nil)
}
