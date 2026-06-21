package handler

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/repository/postgres"
	"github.com/seeu/backend/internal/ws"
)

type LiveStreamHandler struct {
	streamRepo *postgres.LiveStreamRepository
	followRepo *postgres.FollowRepository
	hub        *ws.Hub
	logger     *zap.Logger
}

func NewLiveStreamHandler(
	streamRepo *postgres.LiveStreamRepository,
	followRepo *postgres.FollowRepository,
	hub *ws.Hub,
	logger *zap.Logger,
) *LiveStreamHandler {
	return &LiveStreamHandler{
		streamRepo: streamRepo,
		followRepo: followRepo,
		hub:        hub,
		logger:     logger,
	}
}

// POST /api/v1/streams
// Starts a new live stream for the authenticated user. Fan-outs a
// live_stream.started event to all followers.
func (h *LiveStreamHandler) StartStream(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == "" {
		return respondError(c, fiber.StatusUnauthorized, "auth required")
	}

	var req struct {
		Title string `json:"title"`
	}
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid body")
	}

	stream, err := h.streamRepo.Create(c.Context(), userID, req.Title)
	if err != nil {
		if errors.Is(err, domain.ErrAlreadyStreaming) {
			return respondError(c, fiber.StatusConflict, "already streaming")
		}
		h.logger.Error("create stream", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to start stream")
	}

	// Fan-out to followers.
	go func() {
		followerIDs, err := h.followRepo.GetFollowerIDs(c.Context(), userID)
		if err != nil {
			h.logger.Warn("live_stream fan-out: get followers", zap.Error(err))
			return
		}
		payload := map[string]any{
			"stream_id":   stream.ID,
			"user_id":     stream.UserID,
			"username":    stream.Username,
			"full_name":   stream.FullName,
			"avatar_url":  stream.AvatarURL,
			"title":       stream.Title,
			"started_at":  stream.StartedAt,
		}
		h.hub.SendToUsers(followerIDs, "live_stream.started", payload)
	}()

	return respondSuccess(c, fiber.StatusCreated, stream, nil)
}

// DELETE /api/v1/streams/:id
// Ends the stream. Only the stream owner can call this.
func (h *LiveStreamHandler) EndStream(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == "" {
		return respondError(c, fiber.StatusUnauthorized, "auth required")
	}

	streamID := c.Params("id")
	if err := h.streamRepo.End(c.Context(), streamID, userID); err != nil {
		if errors.Is(err, domain.ErrStreamNotFound) {
			return respondError(c, fiber.StatusNotFound, "stream not found")
		}
		h.logger.Error("end stream", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to end stream")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}

// GET /api/v1/streams
// Lists all currently live streams.
func (h *LiveStreamHandler) GetActiveStreams(c *fiber.Ctx) error {
	streams, err := h.streamRepo.GetActive(c.Context())
	if err != nil {
		h.logger.Error("get active streams", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get streams")
	}
	if streams == nil {
		streams = []domain.LiveStream{}
	}
	return respondSuccess(c, fiber.StatusOK, streams, nil)
}

// GET /api/v1/streams/:id
// Returns stream details with viewer preview (first 5 avatars).
func (h *LiveStreamHandler) GetStream(c *fiber.Ctx) error {
	streamID := c.Params("id")
	stream, err := h.streamRepo.GetByID(c.Context(), streamID)
	if err != nil {
		if errors.Is(err, domain.ErrStreamNotFound) {
			return respondError(c, fiber.StatusNotFound, "stream not found")
		}
		h.logger.Error("get stream", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get stream")
	}

	viewers, _ := h.streamRepo.GetViewerPreview(c.Context(), streamID, 5)
	if viewers == nil {
		viewers = []domain.LiveStreamViewer{}
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"stream":  stream,
		"viewers": viewers,
	}, nil)
}

// POST /api/v1/streams/:id/join
// Records viewer join in DB; WebRTC signaling happens over WS.
func (h *LiveStreamHandler) JoinStream(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == "" {
		return respondError(c, fiber.StatusUnauthorized, "auth required")
	}

	streamID := c.Params("id")
	stream, err := h.streamRepo.GetByID(c.Context(), streamID)
	if err != nil {
		if errors.Is(err, domain.ErrStreamNotFound) {
			return respondError(c, fiber.StatusNotFound, "stream not found")
		}
		return respondError(c, fiber.StatusInternalServerError, "failed to get stream")
	}
	if stream.Status != "live" {
		return respondError(c, fiber.StatusGone, "stream ended")
	}

	viewerCount, err := h.streamRepo.AddViewer(c.Context(), streamID, userID)
	if err != nil {
		h.logger.Error("add viewer", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to join")
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"stream":       stream,
		"viewer_count": viewerCount,
	}, nil)
}

// DELETE /api/v1/streams/:id/join
// Records viewer leave in DB.
func (h *LiveStreamHandler) LeaveStream(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == "" {
		return respondError(c, fiber.StatusUnauthorized, "auth required")
	}

	streamID := c.Params("id")
	h.streamRepo.RemoveViewer(c.Context(), streamID, userID)
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}
