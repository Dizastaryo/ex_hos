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

// ── Resolve (real profiles) ──────────────────────────────────────────────────

// ResolveScanProfile GET /scanner/resolve/:deviceHash
func (h *ScannerHandler) ResolveScanProfile(c *fiber.Ctx) error {
	deviceHash := c.Params("deviceHash")
	if deviceHash == "" {
		return respondError(c, fiber.StatusBadRequest, "deviceHash is required")
	}
	profile, err := h.scannerService.ResolveScanProfile(c.Context(), deviceHash)
	if err != nil {
		if err == domain.ErrNotFound {
			return respondError(c, fiber.StatusNotFound, "device not found or visibility is off")
		}
		h.logger.Error("resolve scan profile", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to resolve")
	}
	return respondSuccess(c, fiber.StatusOK, profile, nil)
}

// ResolveScanProfiles POST /scanner/resolve
func (h *ScannerHandler) ResolveScanProfiles(c *fiber.Ctx) error {
	var body struct {
		DeviceHashes []string `json:"device_hashes"`
	}
	if err := c.BodyParser(&body); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid body")
	}
	if len(body.DeviceHashes) == 0 {
		return respondSuccess(c, fiber.StatusOK, fiber.Map{"profiles": []any{}}, nil)
	}
	if len(body.DeviceHashes) > 50 {
		return respondError(c, fiber.StatusBadRequest, "max 50 device_hashes per request")
	}
	profiles, err := h.scannerService.ResolveScanProfiles(c.Context(), body.DeviceHashes)
	if err != nil {
		h.logger.Error("batch resolve", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to resolve")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"profiles": profiles}, nil)
}

// ── Like ─────────────────────────────────────────────────────────────────────

// PostWave POST /scanner/wave (like someone nearby)
func (h *ScannerHandler) PostWave(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	var body struct {
		DeviceHash string `json:"device_hash"`
	}
	if err := c.BodyParser(&body); err != nil || body.DeviceHash == "" {
		return respondError(c, fiber.StatusBadRequest, "device_hash is required")
	}
	if err := h.scannerService.PostWave(c.Context(), userID, body.DeviceHash); err != nil {
		switch err {
		case domain.ErrNotFound:
			return respondError(c, fiber.StatusNotFound, "device not found or visibility is off")
		case domain.ErrSelfAction:
			return respondError(c, fiber.StatusBadRequest, "cannot like yourself")
		case domain.ErrRateLimited:
			return respondError(c, fiber.StatusTooManyRequests, "like cooldown active (1/hour per person, 20/day total)")
		}
		h.logger.Error("post like", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to send like")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"liked": true}, nil)
}

// ── Matches ──────────────────────────────────────────────────────────────────

// GetMatches GET /scanner/matches
func (h *ScannerHandler) GetMatches(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	limit := c.QueryInt("limit", 20)
	offset := c.QueryInt("offset", 0)
	if limit > 100 {
		limit = 100
	}
	rows, err := h.scannerService.GetMatches(c.Context(), userID, limit, offset)
	if err != nil {
		h.logger.Error("get matches", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get matches")
	}
	type matchItem struct {
		UserID   string `json:"user_id"`
		Username string `json:"username"`
		FullName string `json:"full_name"`
		Avatar   string `json:"avatar_url"`
		Verified bool   `json:"is_verified"`
		MatchedAt string `json:"matched_at"`
	}
	items := make([]matchItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, matchItem{
			UserID:   r.WaverID,
			Username: r.WaverUsername,
			FullName: r.WaverFullName,
			Avatar:   r.WaverAvatar,
			Verified: r.WaverVerified,
			MatchedAt: r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"items": items}, nil)
}

// ── Connect QR ───────────────────────────────────────────────────────────────

// GenerateConnectQR POST /connect/qr
func (h *ScannerHandler) GenerateConnectQR(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	token, err := h.scannerService.GenerateConnectQR(c.Context(), userID)
	if err != nil {
		h.logger.Error("generate connect QR", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to generate QR")
	}
	return respondSuccess(c, fiber.StatusOK, token, nil)
}

// AcceptConnect POST /connect/accept
func (h *ScannerHandler) AcceptConnect(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	var body struct {
		Token string `json:"token"`
	}
	if err := c.BodyParser(&body); err != nil || body.Token == "" {
		return respondError(c, fiber.StatusBadRequest, "token is required")
	}
	chatID, err := h.scannerService.AcceptConnect(c.Context(), userID, body.Token)
	if err != nil {
		if err == domain.ErrSelfAction {
			return respondError(c, fiber.StatusBadRequest, "cannot connect with yourself")
		}
		return respondError(c, fiber.StatusBadRequest, err.Error())
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"chat_id": chatID}, nil)
}

// ── Heartbeat ────────────────────────────────────────────────────────────────

// PostHeartbeat POST /scanner/heartbeat
func (h *ScannerHandler) PostHeartbeat(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	var body struct {
		VisibleDevices []string `json:"visible_devices"`
	}
	if err := c.BodyParser(&body); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid body")
	}
	if len(body.VisibleDevices) > 100 {
		body.VisibleDevices = body.VisibleDevices[:100]
	}
	if err := h.scannerService.ReportHeartbeat(c.Context(), userID, body.VisibleDevices); err != nil {
		h.logger.Warn("heartbeat", zap.Error(err))
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}

// ── Received / Sent ──────────────────────────────────────────────────────────

func (h *ScannerHandler) GetReceivedWaves(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	limit := c.QueryInt("limit", 20)
	offset := c.QueryInt("offset", 0)
	if limit > 100 {
		limit = 100
	}
	rows, total, err := h.scannerService.GetReceivedWaves(c.Context(), userID, limit, offset)
	if err != nil {
		h.logger.Error("get received likes", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get likes")
	}
	type likeItem struct {
		UserID   string `json:"user_id"`
		Username string `json:"username"`
		FullName string `json:"full_name"`
		Avatar   string `json:"avatar_url"`
		Verified bool   `json:"is_verified"`
		WavedAt  string `json:"waved_at"`
	}
	items := make([]likeItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, likeItem{
			UserID:   r.WaverID,
			Username: r.WaverUsername,
			FullName: r.WaverFullName,
			Avatar:   r.WaverAvatar,
			Verified: r.WaverVerified,
			WavedAt:  r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"items": items, "total": total}, nil)
}

func (h *ScannerHandler) GetSentWaves(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	limit := c.QueryInt("limit", 20)
	offset := c.QueryInt("offset", 0)
	if limit > 100 {
		limit = 100
	}
	profiles, err := h.scannerService.GetSentWaves(c.Context(), userID, limit, offset)
	if err != nil {
		h.logger.Error("get sent likes", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get sent likes")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"items": profiles}, nil)
}

// ── Legacy endpoints ─────────────────────────────────────────────────────────

func (h *ScannerHandler) PostLike(c *fiber.Ctx) error       { return h.PostWave(c) }
func (h *ScannerHandler) DeleteLike(c *fiber.Ctx) error     { return c.SendStatus(fiber.StatusNoContent) }
func (h *ScannerHandler) GetReceivedLikes(c *fiber.Ctx) error { return h.GetReceivedWaves(c) }
func (h *ScannerHandler) GetSentLikes(c *fiber.Ctx) error   { return h.GetSentWaves(c) }

func (h *ScannerHandler) UnseenLikesCount(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	count, err := h.scannerService.UnseenLikesCount(c.Context(), userID)
	if err != nil {
		count = 0
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"count": count}, nil)
}

func (h *ScannerHandler) MarkLikesSeen(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	_ = h.scannerService.MarkLikesSeen(c.Context(), userID)
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}
