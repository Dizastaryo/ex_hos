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
	viewerID := middleware.GetUserID(c)
	h.fileService.EnrichWithReadingStatus(c.Context(), files, viewerID)
	h.fileService.EnrichWithLikeStatus(c.Context(), files, viewerID)
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
		AuthorName: c.Query("author"),
		ExcludeID:  c.Query("exclude_id"),
		DocFormat:  c.Query("format"),
		Language:   c.Query("language"),
		Sort:       c.Query("sort", "date"),
		Cursor:     c.Query("cursor"),
		Limit:      limit,
	}
	files, nextCursor, err := h.fileService.ListFiles(c.Context(), p)
	if err != nil {
		h.logger.Error("list files", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to list files")
	}
	viewerID := middleware.GetUserID(c)
	h.fileService.EnrichWithReadingStatus(c.Context(), files, viewerID)
	h.fileService.EnrichWithLikeStatus(c.Context(), files, viewerID)
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
		h.fileService.EnrichWithReadingStatus(c.Context(), []*domain.File{file}, viewerID)
		if userRating, _, err := h.fileService.GetUserRating(c.Context(), id, viewerID); err == nil {
			file.UserRating = userRating
		}
	}
	return respondSuccess(c, fiber.StatusOK, file, nil)
}

// POST /api/v1/files/:id/view — increment view counter (optional auth for history)
func (h *FileHandler) TrackView(c *fiber.Ctx) error {
	fileID := c.Params("id")
	userID := middleware.GetUserID(c) // empty string if not authenticated
	if err := h.fileService.TrackView(c.Context(), fileID, userID); err != nil {
		// Non-fatal: log but don't fail the request
		h.logger.Warn("track file view", zap.String("file_id", fileID), zap.Error(err))
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}

// GET /api/v1/users/me/recently-viewed — files the user recently opened
func (h *FileHandler) GetRecentlyViewed(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	limit := c.QueryInt("limit", 20)
	if limit > 50 {
		limit = 50
	}
	files, err := h.fileService.GetRecentlyViewed(c.Context(), userID, limit)
	if err != nil {
		h.logger.Error("get recently viewed", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get recently viewed")
	}
	h.fileService.EnrichWithReadingStatus(c.Context(), files, userID)
	h.fileService.EnrichWithLikeStatus(c.Context(), files, userID)
	return respondSuccess(c, fiber.StatusOK, files, nil)
}

// PUT /api/v1/files/:id/rating — set star rating 1–5 (auth required)
func (h *FileHandler) RateFile(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	fileID := c.Params("id")
	var body struct {
		Rating     int    `json:"rating"`
		ReviewText string `json:"review_text"`
	}
	if err := c.BodyParser(&body); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid body")
	}
	if body.Rating < 1 || body.Rating > 5 {
		return respondError(c, fiber.StatusBadRequest, "rating must be between 1 and 5")
	}
	if err := h.fileService.RateFile(c.Context(), fileID, userID, body.Rating, body.ReviewText); err != nil {
		h.logger.Error("rate file", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to rate file")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}

// GET /api/v1/files/:id/rating — get current user's rating + review for the file
func (h *FileHandler) GetUserRating(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	fileID := c.Params("id")
	rating, reviewText, err := h.fileService.GetUserRating(c.Context(), fileID, userID)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, "failed to get rating")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"rating":      rating,
		"review_text": reviewText,
	}, nil)
}

// GET /api/v1/files/social-picks — files popular among users you follow
func (h *FileHandler) SocialPicks(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	limit := c.QueryInt("limit", 10)
	if limit > 20 {
		limit = 20
	}
	files, err := h.fileService.GetSocialPicks(c.Context(), userID, limit)
	if err != nil {
		h.logger.Error("social picks", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get social picks")
	}
	h.fileService.EnrichWithLikeStatus(c.Context(), files, userID)
	h.fileService.EnrichWithReadingStatus(c.Context(), files, userID)
	return respondSuccess(c, fiber.StatusOK, files, nil)
}

// GET /api/v1/files/:id/related — files by the same author or category
func (h *FileHandler) GetRelatedFiles(c *fiber.Ctx) error {
	fileID := c.Params("id")
	limit := c.QueryInt("limit", 8)
	if limit > 20 {
		limit = 20
	}
	files, err := h.fileService.GetRelatedFiles(c.Context(), fileID, limit)
	if err != nil {
		h.logger.Error("get related files", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get related files")
	}
	viewerID := middleware.GetUserID(c)
	if viewerID != "" {
		h.fileService.EnrichWithLikeStatus(c.Context(), files, viewerID)
	}
	return respondSuccess(c, fiber.StatusOK, files, nil)
}

// GET /api/v1/files/:id/reviews — get community reviews for a file
func (h *FileHandler) GetFileReviews(c *fiber.Ctx) error {
	fileID := c.Params("id")
	limit := c.QueryInt("limit", 10)
	if limit > 50 {
		limit = 50
	}
	reviews, err := h.fileService.GetFileReviews(c.Context(), fileID, limit)
	if err != nil {
		h.logger.Error("get file reviews", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get reviews")
	}
	return respondSuccess(c, fiber.StatusOK, reviews, nil)
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

// PATCH /api/v1/files/:id — редактирование метаданных (только автор)
func (h *FileHandler) UpdateFile(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	fileID := c.Params("id")
	var req domain.UpdateFileMetaRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid body")
	}
	if req.Title == "" {
		return respondError(c, fiber.StatusBadRequest, "title is required")
	}
	file, err := h.fileService.UpdateFileMeta(c.Context(), fileID, userID, req)
	if err != nil {
		if errors.Is(err, domain.ErrFileNotFound) {
			return respondError(c, fiber.StatusNotFound, "file not found or access denied")
		}
		h.logger.Error("update file meta", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to update file")
	}
	return respondSuccess(c, fiber.StatusOK, file, nil)
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

// POST /api/v1/files/:id/re-extract
// Re-runs text extraction on a stored file (admin / owner use case).
// Returns the freshly extracted text (or 204 if still empty after extraction).
func (h *FileHandler) ReExtractText(c *fiber.Ctx) error {
	id := c.Params("id")
	text, err := h.fileService.ReExtractText(c.Context(), id)
	if err != nil {
		if err == domain.ErrFileNotFound {
			return respondError(c, fiber.StatusNotFound, "file not found")
		}
		return respondError(c, fiber.StatusInternalServerError, err.Error())
	}
	if text == "" {
		return c.SendStatus(fiber.StatusNoContent)
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"text": text}, nil)
}

// GET /api/v1/files/:id/pdf
//
// Возвращает URL к PDF-версии файла. PDF отдаётся как есть.
// Конвертируемые форматы (docx, rtf, odt, fb2, pptx, odp) запускают
// фоновую конвертацию через LibreOffice; возвращает 202 пока идёт конвертация.
// Клиент должен опросить GET /files/:id/pdf-status и повторить по готовности.
func (h *FileHandler) GetPDF(c *fiber.Ctx) error {
	id := c.Params("id")
	pdfURL, err := h.fileService.GetOrConvertToPDF(c.Context(), id)
	if err != nil {
		if err == domain.ErrFileNotFound {
			return respondError(c, fiber.StatusNotFound, "file not found")
		}
		if errors.Is(err, service.ErrConversionPending) {
			return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
				"data":  fiber.Map{"status": "converting"},
				"error": nil,
			})
		}
		if isUnavailableErr(err) {
			return respondError(c, fiber.StatusServiceUnavailable, err.Error())
		}
		h.logger.Error("get or convert to pdf", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "pdf conversion failed: "+err.Error())
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"pdf_url": pdfURL}, nil)
}

// GET /api/v1/files/:id/pdf-status
//
// Возвращает статус фоновой конвертации: pending | converting | done | failed | none.
// Когда status=done, также возвращает pdf_url.
func (h *FileHandler) GetPdfStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	status, pdfURL, err := h.fileService.GetPdfStatus(c.Context(), id)
	if err != nil {
		if err == domain.ErrFileNotFound {
			return respondError(c, fiber.StatusNotFound, "file not found")
		}
		return respondError(c, fiber.StatusInternalServerError, "failed to get pdf status")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"status":  status,
		"pdf_url": pdfURL,
	}, nil)
}

func isUnavailableErr(err error) bool {
	return err != nil && len(err.Error()) > 20 &&
		err.Error()[:20] == "PDF conversion unava"
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

// GET /api/v1/files/authors/popular
func (h *FileHandler) PopularAuthors(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 10)
	if limit > 50 {
		limit = 50
	}
	authors, err := h.fileService.PopularAuthors(c.Context(), limit)
	if err != nil {
		h.logger.Error("popular authors", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get popular authors")
	}
	return respondSuccess(c, fiber.StatusOK, authors, nil)
}

// GET /api/v1/files/stats/formats
func (h *FileHandler) FormatStats(c *fiber.Ctx) error {
	stats, err := h.fileService.FormatStats(c.Context())
	if err != nil {
		h.logger.Error("format stats", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get format stats")
	}
	return respondSuccess(c, fiber.StatusOK, stats, nil)
}

// GET /api/v1/users/me/recommendations
func (h *FileHandler) Recommendations(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	limit := c.QueryInt("limit", 10)
	if limit > 30 {
		limit = 30
	}
	files, err := h.fileService.RecommendedFiles(c.Context(), userID, limit)
	if err != nil {
		h.logger.Error("recommendations", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get recommendations")
	}
	h.fileService.EnrichWithReadingStatus(c.Context(), files, userID)
	h.fileService.EnrichWithLikeStatus(c.Context(), files, userID)
	return respondSuccess(c, fiber.StatusOK, files, nil)
}

// GET /api/v1/files/suggestions?q=...
func (h *FileHandler) SearchSuggestions(c *fiber.Ctx) error {
	q := c.Query("q")
	if len(q) < 2 {
		return respondSuccess(c, fiber.StatusOK, []interface{}{}, nil)
	}
	suggestions, err := h.fileService.SearchSuggestions(c.Context(), q)
	if err != nil {
		h.logger.Error("search suggestions", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get suggestions")
	}
	return respondSuccess(c, fiber.StatusOK, suggestions, nil)
}
