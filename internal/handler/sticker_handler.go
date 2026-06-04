package handler

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/repository/postgres"
	"github.com/seeu/backend/pkg/storage"
	"go.uber.org/zap"
)

const rembgServerURL = "http://127.0.0.1:8004/remove-bg"

type StickerHandler struct {
	repo   *postgres.StickerRepository
	r2     *storage.R2
	logger *zap.Logger
}

func NewStickerHandler(repo *postgres.StickerRepository, r2 *storage.R2, logger *zap.Logger) *StickerHandler {
	return &StickerHandler{repo: repo, r2: r2, logger: logger}
}

// RemoveBg godoc
// POST /api/v1/stickers/remove-bg
// Accepts a PNG/JPG image, runs rembg to remove background, returns URL.
// Requires rembg installed: pip install rembg[cli]
func (h *StickerHandler) RemoveBg(c *fiber.Ctx) error {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, "file is required (field: file)")
	}
	if fileHeader.Size > 20*1024*1024 {
		return respondError(c, fiber.StatusBadRequest, "file too large (max 20MB)")
	}

	// Save uploaded file to temp
	uploaded, err := fileHeader.Open()
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, "failed to open uploaded file")
	}
	defer uploaded.Close()

	fileBytes, err := io.ReadAll(uploaded)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, "failed to read uploaded file")
	}

	// Forward to persistent rembg FastAPI server (BRIA RMBG 1.4).
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", fileHeader.Filename)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, "failed to build multipart request")
	}
	if _, err = fw.Write(fileBytes); err != nil {
		return respondError(c, fiber.StatusInternalServerError, "failed to write multipart body")
	}
	mw.Close()

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(c.Context(), http.MethodPost, rembgServerURL, &buf)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, "failed to build rembg request")
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		h.logger.Error("rembg server request failed", zap.Error(err))
		return respondError(c, fiber.StatusBadGateway,
			"background removal service unavailable — start rembg_server.py on port 8004")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		h.logger.Error("rembg server error", zap.Int("status", resp.StatusCode), zap.String("body", string(body)))
		if resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusGatewayTimeout {
			return respondError(c, fiber.StatusGatewayTimeout, "background removal timed out")
		}
		return respondError(c, fiber.StatusInternalServerError, "background removal failed")
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, "failed to read rembg response")
	}

	filename := fmt.Sprintf("rembg_%d.png", time.Now().UnixNano())

	if h.r2 != nil {
		r2Key := "uploads/stickers/" + filename
		url, err := h.r2.Upload(c.Context(), r2Key, data, "image/png")
		if err != nil {
			h.logger.Error("r2 upload rembg result", zap.Error(err))
			return respondError(c, fiber.StatusInternalServerError, "failed to store result")
		}
		return respondSuccess(c, fiber.StatusOK, fiber.Map{"url": url}, nil)
	}

	// Local storage
	dir := filepath.Join(".", "uploads", "stickers")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return respondError(c, fiber.StatusInternalServerError, "failed to create stickers dir")
	}
	localPath := filepath.Join(dir, filename)
	if err := os.WriteFile(localPath, data, 0644); err != nil {
		return respondError(c, fiber.StatusInternalServerError, "failed to save file")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"url": "/uploads/stickers/" + filename}, nil)
}

// List godoc
// GET /api/v1/stickers
func (h *StickerHandler) List(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	stickers, err := h.repo.ListByUser(c.Context(), userID)
	if err != nil {
		h.logger.Error("list stickers", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to list stickers")
	}
	if stickers == nil {
		stickers = []postgres.StickerRow{}
	}
	return respondSuccess(c, fiber.StatusOK, stickers, nil)
}

// Create godoc
// POST /api/v1/stickers
// Body: {"url": "https://..."}
func (h *StickerHandler) Create(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	var req struct {
		URL string `json:"url"`
	}
	if err := c.BodyParser(&req); err != nil || req.URL == "" {
		return respondError(c, fiber.StatusBadRequest, "url is required")
	}
	sticker, err := h.repo.Create(c.Context(), userID, req.URL)
	if err != nil {
		h.logger.Error("create sticker", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to create sticker")
	}
	return respondSuccess(c, fiber.StatusCreated, sticker, nil)
}

// Delete godoc
// DELETE /api/v1/stickers/:id
func (h *StickerHandler) Delete(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id := c.Params("id")
	deleted, err := h.repo.Delete(c.Context(), id, userID)
	if err != nil {
		h.logger.Error("delete sticker", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to delete sticker")
	}
	if !deleted {
		return respondError(c, fiber.StatusNotFound, "sticker not found")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"deleted": true}, nil)
}
