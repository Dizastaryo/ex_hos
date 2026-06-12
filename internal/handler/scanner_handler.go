package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/service"
	"go.uber.org/zap"
)

type ScannerHandler struct {
	scannerService *service.ScannerService
	logger         *zap.Logger
}

func NewScannerHandler(scannerService *service.ScannerService, logger *zap.Logger) *ScannerHandler {
	return &ScannerHandler{scannerService: scannerService, logger: logger}
}

// PostLike godoc
// POST /api/v1/scanner/like
// body: { "device_hash": "A3F5C2D1E4B6A7C8" }
//
// Поставить лайк пользователю по public_id_hex его браслета.
// Target получает уведомление с реальным аккаунтом лайкера.
// Лайкер видит только "Лайк отправлен" — без раскрытия реального аккаунта target'а.
func (h *ScannerHandler) PostLike(c *fiber.Ctx) error {
	likerID := middleware.GetUserID(c)

	var body struct {
		DeviceHash string `json:"device_hash"`
	}
	if err := c.BodyParser(&body); err != nil || body.DeviceHash == "" {
		return respondError(c, fiber.StatusBadRequest, "device_hash is required")
	}

	if err := h.scannerService.PostLike(c.Context(), likerID, body.DeviceHash); err != nil {
		switch err {
		case domain.ErrNotFound:
			return respondError(c, fiber.StatusNotFound, "device not found or visibility is off")
		case domain.ErrSelfAction:
			return respondError(c, fiber.StatusBadRequest, "cannot like yourself")
		case domain.ErrRateLimited:
			return respondError(c, fiber.StatusTooManyRequests, "daily like limit reached (100/day)")
		}
		h.logger.Error("post scanner like", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to post like")
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{"liked": true}, nil)
}

// DeleteLike godoc
// DELETE /api/v1/scanner/like/:deviceHash
//
// Убрать лайк.
func (h *ScannerHandler) DeleteLike(c *fiber.Ctx) error {
	likerID := middleware.GetUserID(c)
	deviceHash := c.Params("deviceHash")
	if deviceHash == "" {
		return respondError(c, fiber.StatusBadRequest, "deviceHash is required")
	}

	if err := h.scannerService.RemoveLike(c.Context(), likerID, deviceHash); err != nil {
		h.logger.Error("delete scanner like", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to remove like")
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// GetReceivedLikes godoc
// GET /api/v1/scanner/likes/received?limit=20&offset=0
//
// Список тех, кто лайкнул текущего юзера.
// Возвращает РЕАЛЬНЫЕ аккаунты лайкеров — только владелец браслета видит их.
func (h *ScannerHandler) GetReceivedLikes(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	limit := c.QueryInt("limit", 20)
	offset := c.QueryInt("offset", 0)
	if limit > 100 {
		limit = 100
	}

	rows, total, err := h.scannerService.GetReceivedLikes(c.Context(), userID, limit, offset)
	if err != nil {
		h.logger.Error("get received scanner likes", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get likes")
	}

	// Формируем ответ: реальный аккаунт лайкера
	type likerItem struct {
		UserID   string `json:"user_id"`
		Username string `json:"username"`
		FullName string `json:"full_name"`
		Avatar   string `json:"avatar_url"`
		Verified bool   `json:"is_verified"`
		LikedAt  string `json:"liked_at"`
	}
	items := make([]likerItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, likerItem{
			UserID:   r.LikerID,
			Username: r.LikerUsername,
			FullName: r.LikerFullName,
			Avatar:   r.LikerAvatar,
			Verified: r.LikerVerified,
			LikedAt:  r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"items": items,
		"total": total,
	}, nil)
}

// GetSentLikes godoc
// GET /api/v1/scanner/likes/sent?limit=20&offset=0
//
// Список тех, кому текущий юзер поставил лайк.
// Возвращает только scan-профиль (alias, avatar) — реальный аккаунт не раскрывается.
func (h *ScannerHandler) GetSentLikes(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	limit := c.QueryInt("limit", 20)
	offset := c.QueryInt("offset", 0)
	if limit > 100 {
		limit = 100
	}

	profiles, err := h.scannerService.GetSentLikes(c.Context(), userID, limit, offset)
	if err != nil {
		h.logger.Error("get sent scanner likes", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get sent likes")
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{"items": profiles}, nil)
}

// GET /api/v1/scanner/likes/unseen-count
func (h *ScannerHandler) UnseenLikesCount(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	count, err := h.scannerService.UnseenLikesCount(c.Context(), userID)
	if err != nil {
		h.logger.Warn("unseen likes count", zap.Error(err))
		count = 0
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"count": count}, nil)
}

// POST /api/v1/scanner/likes/mark-seen
func (h *ScannerHandler) MarkLikesSeen(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if err := h.scannerService.MarkLikesSeen(c.Context(), userID); err != nil {
		h.logger.Warn("mark likes seen", zap.Error(err))
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}
