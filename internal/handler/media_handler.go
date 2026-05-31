package handler

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/seeu/backend/internal/service"
	"github.com/seeu/backend/pkg/storage"
	"go.uber.org/zap"
)

// resolveUploadPath maps a public `/uploads/...` URL to a local filesystem
// path while rejecting anything that escapes the uploads directory. The URL
// is required to start with `/uploads/`; the remainder is cleaned and joined
// against an absolute uploads root, then the result is verified to still be
// inside that root. Returns an error for traversal attempts (`..`), absolute
// paths inside the URL, or empty paths.
func resolveUploadPath(urlPath string) (string, error) {
	if urlPath == "" || !strings.HasPrefix(urlPath, "/uploads/") {
		return "", errors.New("path must start with /uploads/")
	}
	rel := strings.TrimPrefix(urlPath, "/uploads/")
	if rel == "" {
		return "", errors.New("empty file path")
	}
	// Reject backslashes outright — public URLs always use forward slashes,
	// and on Windows `filepath.Clean` would silently treat them as separators.
	if strings.ContainsAny(rel, "\\") {
		return "", errors.New("invalid character in path")
	}
	absRoot, err := filepath.Abs(filepath.Join(".", "uploads"))
	if err != nil {
		return "", err
	}
	absTarget, err := filepath.Abs(filepath.Join(absRoot, filepath.FromSlash(rel)))
	if err != nil {
		return "", err
	}
	rootWithSep := absRoot + string(filepath.Separator)
	if absTarget != absRoot && !strings.HasPrefix(absTarget, rootWithSep) {
		return "", errors.New("path escapes uploads directory")
	}
	return absTarget, nil
}

type MediaHandler struct {
	mediaService *service.MediaService
	r2           *storage.R2
	logger       *zap.Logger
}

func NewMediaHandler(mediaService *service.MediaService, r2 *storage.R2, logger *zap.Logger) *MediaHandler {
	return &MediaHandler{
		mediaService: mediaService,
		r2:           r2,
		logger:       logger,
	}
}

// Upload godoc
// POST /api/v1/media/upload
func (h *MediaHandler) Upload(c *fiber.Ctx) error {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, "file is required (field: file)")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, "failed to open file")
	}
	defer file.Close()

	result, err := h.mediaService.Upload(c.Context(), file, fileHeader)
	if err != nil {
		h.logger.Error("upload media", zap.Error(err))
		return respondError(c, fiber.StatusBadRequest, err.Error())
	}

	return respondSuccess(c, fiber.StatusOK, result, nil)
}

// VideoThumbnail generates a thumbnail at a specific timestamp.
// POST /api/v1/media/video-thumbnail
// Body: { "video_url": "/uploads/.../video.mp4", "timestamp": 5.0 }
// Works with both local /uploads/ paths and R2 public URLs.
func (h *MediaHandler) VideoThumbnail(c *fiber.Ctx) error {
	var req struct {
		VideoURL  string  `json:"video_url"`
		Timestamp float64 `json:"timestamp"`
	}
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if req.VideoURL == "" {
		return respondError(c, fiber.StatusBadRequest, "video_url is required")
	}

	ts := strconv.FormatFloat(req.Timestamp, 'f', 2, 64)
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(req.VideoURL+":"+ts)))
	thumbFile := hash[:16] + ".jpg"

	// Resolve video to a local path, downloading from R2 if needed.
	var localVideoPath string
	var cleanupVideo func()

	if key, ok := h.r2.KeyFromURL(req.VideoURL); ok {
		data, err := h.r2.Download(c.Context(), key)
		if err != nil {
			return respondError(c, fiber.StatusNotFound, "video file not found")
		}
		ext := filepath.Ext(key)
		tmp, err := os.CreateTemp("", "seeu-video-*"+ext)
		if err != nil {
			return respondError(c, fiber.StatusInternalServerError, "failed to create temp file")
		}
		tmp.Write(data)
		tmp.Close()
		localVideoPath = tmp.Name()
		cleanupVideo = func() { os.Remove(tmp.Name()) }
	} else {
		path, err := resolveUploadPath(req.VideoURL)
		if err != nil {
			return respondError(c, fiber.StatusBadRequest, "invalid video_url")
		}
		if _, err := os.Stat(path); err != nil {
			return respondError(c, fiber.StatusNotFound, "video file not found")
		}
		localVideoPath = path
		cleanupVideo = func() {}
	}
	defer cleanupVideo()

	// Determine where ffmpeg writes the thumbnail.
	var thumbPath string
	useR2Thumb := h.r2 != nil
	if useR2Thumb {
		tmp, err := os.CreateTemp("", "seeu-thumb-*.jpg")
		if err != nil {
			return respondError(c, fiber.StatusInternalServerError, "failed to create temp file")
		}
		tmp.Close()
		thumbPath = tmp.Name()
		defer os.Remove(thumbPath)
	} else {
		thumbDir := filepath.Join(".", "uploads", "thumbs")
		os.MkdirAll(thumbDir, 0755)
		thumbPath = filepath.Join(thumbDir, thumbFile)
	}

	// BACK-3: ffmpeg context-timeout 30 сек.
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-y", "-ss", ts,
		"-i", localVideoPath,
		"-vframes", "1",
		"-q:v", "3",
		"-vf", "scale=480:-1",
		thumbPath,
	)
	if err := cmd.Run(); err != nil {
		_ = os.Remove(thumbPath)
		if ctx.Err() == context.DeadlineExceeded {
			h.logger.Error("ffmpeg thumbnail timeout", zap.String("video", req.VideoURL))
			return respondError(c, fiber.StatusGatewayTimeout, "thumbnail generation timed out")
		}
		h.logger.Error("ffmpeg thumbnail at timestamp", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to generate thumbnail")
	}

	if useR2Thumb {
		data, err := os.ReadFile(thumbPath)
		if err != nil {
			return respondError(c, fiber.StatusInternalServerError, "failed to read thumbnail")
		}
		r2Key := "uploads/thumbs/" + thumbFile
		url, err := h.r2.Upload(c.Context(), r2Key, data, "image/jpeg")
		if err != nil {
			h.logger.Error("r2 upload thumbnail", zap.Error(err))
			return respondError(c, fiber.StatusInternalServerError, "failed to upload thumbnail")
		}
		return respondSuccess(c, fiber.StatusOK, fiber.Map{"thumbnail_url": url}, nil)
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"thumbnail_url": "/uploads/thumbs/" + thumbFile,
	}, nil)
}
