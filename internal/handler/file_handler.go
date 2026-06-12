package handler

import (
	"errors"
	"mime/multipart"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/service"
	"github.com/seeu/backend/pkg/pagination"
	"go.uber.org/zap"
)

type FileHandler struct {
	fileService *service.FileService
	validate    *validator.Validate
	logger      *zap.Logger
}

func NewFileHandler(fileService *service.FileService, validate *validator.Validate, logger *zap.Logger) *FileHandler {
	return &FileHandler{fileService: fileService, validate: validate, logger: logger}
}

// Upload godoc
// POST /api/v1/files/upload   multipart: file=<blob>, category_id=<uuid?>, description=<str?>
//
// Single-call multipart upload. Saves the blob under /uploads/library/...
// and inserts the metadata row in `files`. Returns the persisted File.
//
// The legacy POST /api/v1/files (CreateFile below) is kept for clients that
// already have an uploaded URL and just want to attach metadata, but it's
// rarely the right entry point — most callers should hit /upload.
func (h *FileHandler) Upload(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	header, err := c.FormFile("file")
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, "file is required (field: file)")
	}
	src, err := header.Open()
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, "failed to open file")
	}
	defer src.Close()

	categoryID := c.FormValue("category_id")
	description := c.FormValue("description")
	title := c.FormValue("title")
	authorName := c.FormValue("author_name")
	language := c.FormValue("language")

	// Обложка — опциональна
	var coverFile multipart.File
	var coverHeader *multipart.FileHeader
	if ch, cerr := c.FormFile("cover"); cerr == nil {
		if cf, cerr := ch.Open(); cerr == nil {
			coverFile = cf
			coverHeader = ch
			defer cf.Close()
		}
	}

	file, err := h.fileService.Upload(c.Context(), userID, src, header, coverFile, coverHeader, categoryID, description, title, authorName, language)
	if err != nil {
		h.logger.Error("upload file", zap.Error(err))
		return respondError(c, fiber.StatusBadRequest, err.Error())
	}
	return respondSuccess(c, fiber.StatusCreated, file, nil)
}

// POST /api/v1/files
func (h *FileHandler) CreateFile(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	var req domain.CreateFileRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}
	file, err := h.fileService.CreateFile(c.Context(), userID, &req)
	if err != nil {
		h.logger.Error("create file", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to create file")
	}
	return respondSuccess(c, fiber.StatusCreated, file, nil)
}

// GET /api/v1/files/trending (LIB-6)
// Top-N files по hot-score. ?period=week|month|all  ?limit=10
func (h *FileHandler) Trending(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 10)
	if limit > 50 {
		limit = 50
	}
	period := c.Query("period", "week")
	files, err := h.fileService.Trending(c.Context(), limit, period)
	if err != nil {
		h.logger.Error("trending files", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get trending")
	}
	return respondSuccess(c, fiber.StatusOK, files, nil)
}

// GET /api/v1/files
// Query: category_id, q, sort (date|likes|downloads|title), cursor, limit
func (h *FileHandler) ListFiles(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 20)
	if limit > 100 {
		limit = 100
	}
	p := domain.FileListParams{
		CategoryID: c.Query("category_id"),
		Q:          c.Query("q"),
		Sort:       c.Query("sort", "date"),
		Cursor:     c.Query("cursor"),
		Limit:      limit,
	}
	files, nextCursor, err := h.fileService.ListFiles(c.Context(), p)
	if err != nil {
		h.logger.Error("list files", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to list files")
	}
	return respondSuccess(c, fiber.StatusOK, files, fiber.Map{"next_cursor": nextCursor})
}

// GET /api/v1/files/:id
func (h *FileHandler) GetFile(c *fiber.Ctx) error {
	id := c.Params("id")
	viewerID := middleware.GetUserID(c)
	file, err := h.fileService.GetFile(c.Context(), id)
	if err != nil {
		if err == domain.ErrFileNotFound {
			return respondError(c, fiber.StatusNotFound, "file not found")
		}
		return respondError(c, fiber.StatusInternalServerError, "failed to get file")
	}
	if viewerID != "" {
		if liked, err := h.fileService.IsFileLiked(c.Context(), id, viewerID); err == nil {
			file.IsLiked = liked
		}
	}
	return respondSuccess(c, fiber.StatusOK, file, nil)
}

// POST /api/v1/files/:id/like
func (h *FileHandler) LikeFile(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	fileID := c.Params("id")
	if err := h.fileService.LikeFile(c.Context(), fileID, userID); err != nil {
		return respondError(c, fiber.StatusInternalServerError, "failed to like file")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}

// DELETE /api/v1/files/:id/like
func (h *FileHandler) UnlikeFile(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	fileID := c.Params("id")
	if err := h.fileService.UnlikeFile(c.Context(), fileID, userID); err != nil {
		return respondError(c, fiber.StatusInternalServerError, "failed to unlike file")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}

// DELETE /api/v1/files/:id
func (h *FileHandler) DeleteFile(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	if err := h.fileService.DeleteFile(c.Context(), id, userID); err != nil {
		if err == domain.ErrFileNotFound {
			return respondError(c, fiber.StatusNotFound, "file not found")
		}
		return respondError(c, fiber.StatusInternalServerError, "failed to delete file")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "file deleted"}, nil)
}

// GET /api/v1/files/:id/download
func (h *FileHandler) DownloadFile(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	file, err := h.fileService.DownloadFile(c.Context(), id, userID)
	if err != nil {
		if err == domain.ErrFileNotFound {
			return respondError(c, fiber.StatusNotFound, "file not found")
		}
		return respondError(c, fiber.StatusInternalServerError, "failed to download file")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"file_url": file.FileURL,
		"filename": file.Filename,
	}, nil)
}

// GET /api/v1/files/:id/preview
func (h *FileHandler) PreviewFile(c *fiber.Ctx) error {
	id := c.Params("id")
	file, err := h.fileService.GetFile(c.Context(), id)
	if err != nil {
		if err == domain.ErrFileNotFound {
			return respondError(c, fiber.StatusNotFound, "file not found")
		}
		return respondError(c, fiber.StatusInternalServerError, "failed to get file")
	}
	if !file.IsPreviewable {
		return respondError(c, fiber.StatusBadRequest, "file is not previewable")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"file_url":  file.FileURL,
		"mime_type": file.MimeType,
		"filename":  file.Filename,
	}, nil)
}

// GetText godoc
// GET /api/v1/files/:id/text
//
// Возвращает извлечённый plain-text документа.
// Используется Flutter-ридерами для Tier-2 форматов (FB2, DOCX, RTF, ODT)
// и Tier-3 slide-preview (PPTX, ODP).
// 204 если текст не был извлечён (PDF, EPUB, пустой файл).
func (h *FileHandler) GetText(c *fiber.Ctx) error {
	id := c.Params("id")
	text, err := h.fileService.GetExtractedText(c.Context(), id)
	if err != nil {
		if err == domain.ErrFileNotFound {
			return respondError(c, fiber.StatusNotFound, "file not found")
		}
		return respondError(c, fiber.StatusInternalServerError, "failed to get text")
	}
	if text == "" {
		return c.SendStatus(fiber.StatusNoContent)
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"text": text}, nil)
}

// GET /api/v1/files/categories
func (h *FileHandler) GetCategories(c *fiber.Ctx) error {
	cats, err := h.fileService.GetCategories(c.Context())
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, "failed to get categories")
	}
	return respondSuccess(c, fiber.StatusOK, cats, nil)
}

// GET /api/v1/users/:id/files
func (h *FileHandler) GetUserFiles(c *fiber.Ctx) error {
	ownerID := c.Params("id")
	viewerID := middleware.GetUserID(c)
	p := pagination.FromFiber(c.Query("page", "1"), c.Query("limit", "20"))
	files, total, err := h.fileService.GetUserFiles(c.Context(), ownerID, viewerID, p.Limit, p.Offset)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return respondError(c, fiber.StatusNotFound, "user not found")
		}
		if errors.Is(err, domain.ErrPrivateAccount) {
			return respondError(c, fiber.StatusForbidden, "this account is private")
		}
		return respondError(c, fiber.StatusInternalServerError, "failed to get user files")
	}
	return respondSuccess(c, fiber.StatusOK, files, pagination.MetaFromTotal(total, p))
}
