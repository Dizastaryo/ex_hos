package handler

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/repository/postgres"
	"github.com/seeu/backend/pkg/pagination"
	"github.com/seeu/backend/pkg/probe"
	"go.uber.org/zap"
)

// extractEmbeddedCover (MUSIC-8) — извлекает embedded cover-art из ID3-тегов
// audio-файла через ffmpeg. Возвращает относительный URL (`/uploads/.../cover.jpg`)
// или пустую строку если cover'а нет либо ffmpeg недоступен.
//
// Принцип: `ffmpeg -i input.mp3 -an -c:v copy -map 0:v:0? cover.jpg`
//   -an     — без audio
//   -c:v copy — copy video stream (cover-art в mp3/m4a хранится как video stream)
//   -map 0:v:0? — берём первый video stream, ? = optional (no error если нет)
//
// Cover пишется рядом с audio-файлом с тем же basename + `.cover.jpg`.
func extractEmbeddedCover(audioURL string, logger *zap.Logger) string {
	if audioURL == "" || !strings.HasPrefix(audioURL, "/uploads/") {
		return ""
	}
	audioPath := strings.TrimPrefix(audioURL, "/")
	if _, err := os.Stat(audioPath); err != nil {
		return ""
	}
	ext := filepath.Ext(audioPath)
	base := strings.TrimSuffix(audioPath, ext)
	coverPath := base + ".cover.jpg"
	if _, err := os.Stat(coverPath); err == nil {
		// Already extracted earlier.
		return "/" + coverPath
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-y",          // overwrite (defensive, файла нет см. выше)
		"-i", audioPath,
		"-an",
		"-c:v", "copy",
		"-map", "0:v:0?",
		coverPath,
	)
	if err := cmd.Run(); err != nil {
		// Non-fatal: ffmpeg not installed, no embedded cover, или другая
		// ошибка. Просто пустая строка — caller fallback'нётся.
		if logger != nil {
			if ctx.Err() == context.DeadlineExceeded {
				logger.Warn("ffmpeg cover extract timeout",
					zap.String("audio", audioURL))
			} else {
				logger.Debug("ffmpeg cover extract failed", zap.Error(err),
					zap.String("audio", audioURL))
			}
		}
		// BUG-21: cleanup partial cover. При DeadlineExceeded ffmpeg killed
		// mid-write — файл может быть corrupt non-zero — всё равно remove.
		// При прочих ошибках — только если empty.
		if ctx.Err() == context.DeadlineExceeded {
			_ = os.Remove(coverPath)
		} else if info, statErr := os.Stat(coverPath); statErr == nil && info.Size() == 0 {
			_ = os.Remove(coverPath)
		}
		return ""
	}
	if info, err := os.Stat(coverPath); err != nil || info.Size() == 0 {
		_ = os.Remove(coverPath)
		return ""
	}
	return "/" + coverPath
}

type AudioHandler struct {
	audioRepo *postgres.AudioRepository
	logger    *zap.Logger
}

func NewAudioHandler(audioRepo *postgres.AudioRepository, logger *zap.Logger) *AudioHandler {
	return &AudioHandler{audioRepo: audioRepo, logger: logger}
}

// GET /api/v1/audio-tracks?q=&page=1&limit=20
func (h *AudioHandler) GetTracks(c *fiber.Ctx) error {
	q := c.Query("q", "")
	page, limit := pagination.ParsePage(c.Query("page", "1"), c.Query("limit", "20"))
	offset := pagination.Offset(page, limit)

	var err error
	if q != "" {
		tracks, err := h.audioRepo.Search(c.Context(), q, limit+1, offset)
		if err != nil {
			h.logger.Error("search audio", zap.Error(err))
			return respondError(c, fiber.StatusInternalServerError, "failed to search audio")
		}
		hasNext := len(tracks) > limit
		if hasNext {
			tracks = tracks[:limit]
		}
		return respondSuccess(c, fiber.StatusOK, tracks, pagination.Meta{
			Page: page, Limit: limit, HasNextPage: hasNext,
		})
	}

	tracks, err := h.audioRepo.GetAll(c.Context(), limit+1, offset)
	if err != nil {
		h.logger.Error("get audio tracks", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get audio tracks")
	}
	hasNext := len(tracks) > limit
	if hasNext {
		tracks = tracks[:limit]
	}
	return respondSuccess(c, fiber.StatusOK, tracks, pagination.Meta{
		Page: page, Limit: limit, HasNextPage: hasNext,
	})
}

// GET /api/v1/audio-tracks/:id
// Lazy fetch одного трека по ID. Используется story-viewer'ом для загрузки
// audio_track_id из Story.audio_track_id без выкачивания всего списка.
func (h *AudioHandler) GetTrackByID(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return respondError(c, fiber.StatusBadRequest, "missing id")
	}
	track, err := h.audioRepo.GetByID(c.Context(), id)
	if err != nil {
		h.logger.Error("get audio track", zap.Error(err))
		return respondError(c, fiber.StatusNotFound, "track not found")
	}
	return respondSuccess(c, fiber.StatusOK, track, nil)
}

// GET /api/v1/tags/trending?limit=30
func (h *AudioHandler) GetTrendingTags(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 30)
	if limit > 100 {
		limit = 100
	}

	tags, err := h.audioRepo.GetTrendingTags(c.Context(), limit)
	if err != nil {
		h.logger.Error("get trending tags", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get trending tags")
	}

	return respondSuccess(c, fiber.StatusOK, tags, nil)
}

// POST /api/v1/audio-tracks
// Body: {title, artist, genre, audio_url, cover_url, duration_seconds}
// audio_url and cover_url should already be uploaded via /media/upload.
func (h *AudioHandler) CreateTrack(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req struct {
		Title           string `json:"title"`
		Artist          string `json:"artist"`
		Genre           string `json:"genre"`
		AudioURL        string `json:"audio_url"`
		CoverURL        string `json:"cover_url"`
		DurationSeconds int    `json:"duration_seconds"`
	}
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}

	title := strings.TrimSpace(req.Title)
	artist := strings.TrimSpace(req.Artist)
	if title == "" {
		return respondError(c, fiber.StatusBadRequest, "title is required")
	}
	if artist == "" {
		return respondError(c, fiber.StatusBadRequest, "artist is required")
	}
	if strings.TrimSpace(req.AudioURL) == "" {
		return respondError(c, fiber.StatusBadRequest, "audio_url is required")
	}
	if len(title) > 200 {
		title = title[:200]
	}
	if len(artist) > 200 {
		artist = artist[:200]
	}
	if req.DurationSeconds < 0 {
		req.DurationSeconds = 0
	}
	// If frontend couldn't compute duration (slow `AudioPlayer.setUrl().duration`
	// await on submit), probe via ffprobe on the already-uploaded file. The
	// audio_url is "/uploads/.../<hash>.mp3" — strip leading `/` to make it
	// the local path the binary can resolve.
	if req.DurationSeconds == 0 && strings.HasPrefix(req.AudioURL, "/uploads/") {
		localPath := strings.TrimPrefix(req.AudioURL, "/")
		if d := probe.DurationSeconds(localPath); d > 0 {
			req.DurationSeconds = d
			h.logger.Info("audio duration probed via ffprobe",
				zap.String("url", req.AudioURL),
				zap.Int("seconds", d))
		}
	}

	coverURL := strings.TrimSpace(req.CoverURL)
	// MUSIC-8: если cover_url не указан, пытаемся extract embedded cover-art
	// из ID3-тегов через ffmpeg. Хранится рядом с audio-файлом как .jpg.
	// Failure non-fatal — track создаётся с пустым cover_url (фронт fallback'нётся
	// на gradient/initials).
	if coverURL == "" && strings.HasPrefix(req.AudioURL, "/uploads/") {
		if extracted := extractEmbeddedCover(req.AudioURL, h.logger); extracted != "" {
			coverURL = extracted
			h.logger.Info("audio cover extracted from ID3",
				zap.String("audio", req.AudioURL),
				zap.String("cover", extracted))
		}
	}

	t := &domain.AudioTrack{
		Title:           title,
		Artist:          artist,
		Genre:           strings.TrimSpace(req.Genre),
		AudioURL:        strings.TrimSpace(req.AudioURL),
		CoverURL:        coverURL,
		DurationSeconds: req.DurationSeconds,
		UserID:          userID,
		Status:          "pending",
	}
	if err := h.audioRepo.Create(c.Context(), t); err != nil {
		h.logger.Error("create audio track", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to create track")
	}
	return respondSuccess(c, fiber.StatusCreated, t, nil)
}

// POST /api/v1/audio-tracks/:id/play — MUSIC-3: запись прослушивания.
// body: {duration_played_sec?: int}
func (h *AudioHandler) RecordPlay(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == "" {
		return respondError(c, fiber.StatusUnauthorized, "auth required")
	}
	trackID := c.Params("id")
	if trackID == "" {
		return respondError(c, fiber.StatusBadRequest, "track id required")
	}
	var req struct {
		DurationPlayedSec int `json:"duration_played_sec"`
	}
	_ = c.BodyParser(&req)
	if req.DurationPlayedSec < 0 {
		req.DurationPlayedSec = 0
	}
	if err := h.audioRepo.RecordPlay(c.Context(), userID, trackID, req.DurationPlayedSec); err != nil {
		// Non-fatal logging only.
		h.logger.Warn("record play", zap.Error(err))
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}

// GET /api/v1/audio-tracks/recent — MUSIC-3: недавно прослушанные.
func (h *AudioHandler) ListRecent(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == "" {
		return respondError(c, fiber.StatusUnauthorized, "auth required")
	}
	limit := c.QueryInt("limit", 30)
	if limit < 1 || limit > 100 {
		limit = 30
	}
	tracks, err := h.audioRepo.RecentPlayed(c.Context(), userID, limit)
	if err != nil {
		h.logger.Error("recent played", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed")
	}
	if tracks == nil {
		return respondSuccess(c, fiber.StatusOK, []struct{}{}, nil)
	}
	return respondSuccess(c, fiber.StatusOK, tracks, nil)
}

// GET /api/v1/audio-tracks/liked — MUSIC-3: лайкнутые.
func (h *AudioHandler) ListLiked(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == "" {
		return respondError(c, fiber.StatusUnauthorized, "auth required")
	}
	limit := c.QueryInt("limit", 50)
	if limit < 1 || limit > 200 {
		limit = 50
	}
	tracks, err := h.audioRepo.LikedTracks(c.Context(), userID, limit)
	if err != nil {
		h.logger.Error("liked tracks", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed")
	}
	if tracks == nil {
		return respondSuccess(c, fiber.StatusOK, []struct{}{}, nil)
	}
	return respondSuccess(c, fiber.StatusOK, tracks, nil)
}

// GET /api/v1/audio-tracks/daily-mix — MUSIC-4: персональный daily mix.
// Алгоритм: top-жанры юзера за 30д → random из них (исключая recently
// played 24h). Seed deterministic по date — refresh в полночь.
func (h *AudioHandler) DailyMix(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == "" {
		return respondError(c, fiber.StatusUnauthorized, "auth required")
	}
	limit := c.QueryInt("limit", 20)
	if limit < 1 || limit > 50 {
		limit = 20
	}
	tracks, err := h.audioRepo.DailyMix(c.Context(), userID, limit)
	if err != nil {
		h.logger.Error("daily mix", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed")
	}
	if tracks == nil {
		return respondSuccess(c, fiber.StatusOK, []struct{}{}, nil)
	}
	return respondSuccess(c, fiber.StatusOK, tracks, nil)
}

// GET /api/v1/audio-tracks/me — list my uploaded tracks (any status).
func (h *AudioHandler) ListMine(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	tracks, err := h.audioRepo.ListByUser(c.Context(), userID)
	if err != nil {
		h.logger.Error("list my audio", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to list tracks")
	}
	if tracks == nil {
		tracks = []*domain.AudioTrack{}
	}
	return respondSuccess(c, fiber.StatusOK, tracks, nil)
}

// ── Admin endpoints ─────────────────────────────────────────────────────────

// GET /api/v1/admin/audio-tracks?status=pending&limit=50
func (h *AudioHandler) AdminList(c *fiber.Ctx) error {
	status := c.Query("status", "pending")
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)

	tracks, err := h.audioRepo.AdminList(c.Context(), status, limit, offset)
	if err != nil {
		h.logger.Error("admin list audio", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to list audio")
	}
	if tracks == nil {
		tracks = []*domain.AudioTrack{}
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"items": tracks}, nil)
}

// AdminApprove / AdminReject are wired through the AdminHandler so they can
// share the audit-log helper. They live here only as type-bound stubs.
func (h *AudioHandler) AdminSetStatus(c *fiber.Ctx, status, reason string) error {
	id := c.Params("id")
	reviewerID, _ := c.Locals("user_id").(string)
	if err := h.audioRepo.SetStatus(c.Context(), id, status, reason, reviewerID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return respondError(c, fiber.StatusNotFound, "track not found")
		}
		h.logger.Error("set audio status", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to update status")
	}
	return c.SendStatus(fiber.StatusNoContent)
}
