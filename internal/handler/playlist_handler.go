package handler

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/repository/postgres"
	"go.uber.org/zap"
)

type PlaylistHandler struct {
	playlistRepo *postgres.PlaylistRepository
	logger       *zap.Logger
}

func NewPlaylistHandler(playlistRepo *postgres.PlaylistRepository, logger *zap.Logger) *PlaylistHandler {
	return &PlaylistHandler{playlistRepo: playlistRepo, logger: logger}
}

// GET /api/v1/playlists/me
func (h *PlaylistHandler) ListMine(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	list, err := h.playlistRepo.ListByUser(c.Context(), userID)
	if err != nil {
		h.logger.Error("list my playlists", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to list playlists")
	}
	if list == nil {
		list = []*domain.Playlist{}
	}
	return respondSuccess(c, fiber.StatusOK, list, nil)
}

// POST /api/v1/playlists  body: {name}
func (h *PlaylistHandler) Create(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	var req struct {
		Name string `json:"name"`
	}
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return respondError(c, fiber.StatusBadRequest, "name is required")
	}
	if len(name) > 120 {
		name = name[:120]
	}

	p, err := h.playlistRepo.Create(c.Context(), userID, name)
	if err != nil {
		h.logger.Error("create playlist", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to create playlist")
	}
	return respondSuccess(c, fiber.StatusCreated, p, nil)
}

// PATCH /api/v1/playlists/:id  body: {name?, cover_url?}
func (h *PlaylistHandler) Update(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id := c.Params("id")

	owned, err := h.assertOwner(c, id, userID)
	if !owned {
		return err
	}

	var req struct {
		Name     *string `json:"name"`
		CoverURL *string `json:"cover_url"`
	}
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		if trimmed == "" {
			return respondError(c, fiber.StatusBadRequest, "name cannot be empty")
		}
		if len(trimmed) > 120 {
			trimmed = trimmed[:120]
		}
		req.Name = &trimmed
	}

	if err := h.playlistRepo.Update(c.Context(), id, req.Name, req.CoverURL); err != nil {
		if errors.Is(err, domain.ErrPlaylistNotFound) {
			return respondError(c, fiber.StatusNotFound, "playlist not found")
		}
		h.logger.Error("update playlist", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to update playlist")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// DELETE /api/v1/playlists/:id
func (h *PlaylistHandler) Delete(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id := c.Params("id")

	owned, err := h.assertOwner(c, id, userID)
	if !owned {
		return err
	}

	if err := h.playlistRepo.Delete(c.Context(), id); err != nil {
		if errors.Is(err, domain.ErrPlaylistNotFound) {
			return respondError(c, fiber.StatusNotFound, "playlist not found")
		}
		h.logger.Error("delete playlist", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to delete playlist")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// GET /api/v1/playlists/:id  → playlist + tracks
func (h *PlaylistHandler) Get(c *fiber.Ctx) error {
	id := c.Params("id")

	p, err := h.playlistRepo.GetByID(c.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrPlaylistNotFound) {
			return respondError(c, fiber.StatusNotFound, "playlist not found")
		}
		h.logger.Error("get playlist", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get playlist")
	}

	tracks, err := h.playlistRepo.GetTracks(c.Context(), id)
	if err != nil {
		h.logger.Error("get playlist tracks", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get tracks")
	}
	if tracks == nil {
		tracks = []*domain.AudioTrack{}
	}

	return respondSuccess(c, fiber.StatusOK, &domain.PlaylistDetail{
		Playlist: *p,
		Tracks:   tracks,
	}, nil)
}

// POST /api/v1/playlists/:id/tracks  body: {track_id}
func (h *PlaylistHandler) AddTrack(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id := c.Params("id")

	owned, err := h.assertOwner(c, id, userID)
	if !owned {
		return err
	}

	var req struct {
		TrackID string `json:"track_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if strings.TrimSpace(req.TrackID) == "" {
		return respondError(c, fiber.StatusBadRequest, "track_id is required")
	}

	if err := h.playlistRepo.AddTrack(c.Context(), id, req.TrackID); err != nil {
		h.logger.Error("add track to playlist", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to add track")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// DELETE /api/v1/playlists/:id/tracks/:trackId
func (h *PlaylistHandler) RemoveTrack(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id := c.Params("id")
	trackID := c.Params("trackId")

	owned, err := h.assertOwner(c, id, userID)
	if !owned {
		return err
	}

	if err := h.playlistRepo.RemoveTrack(c.Context(), id, trackID); err != nil {
		h.logger.Error("remove track from playlist", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to remove track")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// assertOwner returns (true, nil) if the playlist exists and is owned by userID.
// Otherwise it writes the appropriate error response and returns (false, err).
func (h *PlaylistHandler) assertOwner(c *fiber.Ctx, playlistID, userID string) (bool, error) {
	p, err := h.playlistRepo.GetByID(c.Context(), playlistID)
	if err != nil {
		if errors.Is(err, domain.ErrPlaylistNotFound) {
			return false, respondError(c, fiber.StatusNotFound, "playlist not found")
		}
		h.logger.Error("get playlist for ownership check", zap.Error(err))
		return false, respondError(c, fiber.StatusInternalServerError, "failed to verify ownership")
	}
	if p.UserID != userID {
		return false, respondError(c, fiber.StatusForbidden, "access denied")
	}
	return true, nil
}
