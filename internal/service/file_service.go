package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	"github.com/seeu/backend/pkg/docextract"
	"github.com/seeu/backend/pkg/pdfconvert"
	"github.com/seeu/backend/pkg/storage"
	"go.uber.org/zap"
)

// convertibleFormats require server-side PDF conversion via LibreOffice.
var convertibleFormats = map[string]bool{
	"fb2": true, "docx": true, "rtf": true,
	"odt": true, "pptx": true, "odp": true,
}

// ErrConversionPending signals that PDF conversion is in progress; caller should poll.
var ErrConversionPending = errors.New("pdf conversion in progress")

// allowedExtensions — белый список расширений для библиотеки.
// Только читательские форматы (документы, книги, презентации).
var allowedExtensions = map[string]bool{
	"pdf": true, "epub": true, "fb2": true,
	"docx": true, "pptx": true, "txt": true,
	"rtf": true, "md": true, "odt": true, "odp": true,
}

// allowedMimeTypes — белый список MIME-типов.
// application/octet-stream разрешён если расширение в allowedExtensions.
var allowedMimeTypes = map[string]bool{
	"application/pdf":           true,
	"application/epub+zip":      true,
	"application/x-fictionbook+xml": true,
	"text/xml":                  true, // некоторые клиенты шлют FB2 как text/xml
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":    true,
	"application/vnd.openxmlformats-officedocument.presentationml.presentation":  true,
	"text/plain":                true,
	"application/rtf":           true,
	"text/rtf":                  true,
	"text/markdown":             true,
	"text/x-markdown":           true,
	"application/vnd.oasis.opendocument.text":         true,
	"application/vnd.oasis.opendocument.presentation": true,
	"application/octet-stream": true, // проверяем по расширению
}

// MaxLibraryFileSize caps the size of any single file uploaded to the
// library. Documents typically fit well under this; videos that big should
// go via the post-creation pipeline instead.
const MaxLibraryFileSize = 50 * 1024 * 1024 // 50 MB

// libraryUploadDir is where library file blobs live on disk. Mirrors the
// /uploads/library/<date>/<hash>.<ext> URL convention. Kept separate from
// the post-media tree to keep retention/policy distinct (docs ≠ feed media).
const libraryUploadDir = "./uploads/library"

type FileService struct {
	fileRepo    *postgres.FileRepository
	readingRepo *postgres.ReadingRepository
	statsRepo   *postgres.UserStatsRepository
	logger      *zap.Logger
	r2          *storage.R2
}

func NewFileService(fileRepo *postgres.FileRepository, readingRepo *postgres.ReadingRepository, statsRepo *postgres.UserStatsRepository, logger *zap.Logger, r2 *storage.R2) *FileService {
	if r2 == nil {
		os.MkdirAll(libraryUploadDir, 0o755)
	}
	return &FileService{fileRepo: fileRepo, readingRepo: readingRepo, statsRepo: statsRepo, logger: logger, r2: r2}
}

// EnrichWithLikeStatus fills IsLiked on each file for the given viewer.
func (s *FileService) EnrichWithLikeStatus(ctx context.Context, files []*domain.File, viewerID string) {
	if viewerID == "" || len(files) == 0 {
		return
	}
	ids := make([]string, len(files))
	for i, f := range files {
		ids[i] = f.ID
	}
	likedMap, err := s.fileRepo.IsFileLikedBatch(ctx, viewerID, ids)
	if err != nil {
		s.logger.Warn("enrich like status", zap.Error(err))
		return
	}
	for _, f := range files {
		f.IsLiked = likedMap[f.ID]
	}
}

// EnrichWithReadingStatus fills ReadingStatus on each file for the given viewer.
func (s *FileService) EnrichWithReadingStatus(ctx context.Context, files []*domain.File, viewerID string) {
	if viewerID == "" || len(files) == 0 {
		return
	}
	ids := make([]string, len(files))
	for i, f := range files {
		ids[i] = f.ID
	}
	statuses, err := s.readingRepo.GetReadingStatusBatch(ctx, viewerID, ids)
	if err != nil {
		s.logger.Warn("enrich reading status", zap.Error(err))
		return
	}
	for _, f := range files {
		if st, ok := statuses[f.ID]; ok {
			f.ReadingStatus = st
		}
	}
}

// RecentlyRead returns the user's recently-read files.
func (s *FileService) RecentlyRead(ctx context.Context, userID string, limit int) ([]*domain.File, error) {
	return s.readingRepo.GetRecentlyRead(ctx, userID, limit)
}

// allowedCoverMimes — MIME-типы разрешённые для обложки (изображения).
var allowedCoverMimes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// MaxCoverSize — 5 MB максимум для обложки.
const MaxCoverSize = 5 * 1024 * 1024

// uploadCover uploads cover image to R2 and returns public URL.
// Returns ("", nil) when cover is nil (optional).
func (s *FileService) uploadCover(ctx context.Context, cover multipart.File, coverHeader *multipart.FileHeader) (string, error) {
	if cover == nil || coverHeader == nil {
		return "", nil
	}
	if coverHeader.Size > MaxCoverSize {
		return "", fmt.Errorf("cover size exceeds 5 MB")
	}
	mime := coverHeader.Header.Get("Content-Type")
	if !allowedCoverMimes[mime] {
		// fallback: detect by extension
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(coverHeader.Filename), "."))
		switch ext {
		case "jpg", "jpeg":
			mime = "image/jpeg"
		case "png":
			mime = "image/png"
		case "webp":
			mime = "image/webp"
		default:
			return "", fmt.Errorf("cover must be JPEG, PNG or WebP")
		}
	}
	data, err := io.ReadAll(cover)
	if err != nil {
		return "", fmt.Errorf("read cover: %w", err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	ext := "jpg"
	if mime == "image/png" {
		ext = "png"
	} else if mime == "image/webp" {
		ext = "webp"
	}
	key := "uploads/library/covers/" + hash[:16] + "." + ext
	if s.r2 != nil {
		return s.r2.Upload(ctx, key, data, mime)
	}
	// local fallback
	dirPath := filepath.Join(libraryUploadDir, "covers")
	_ = os.MkdirAll(dirPath, 0o755)
	_ = os.WriteFile(filepath.Join(dirPath, hash[:16]+"."+ext), data, 0o644)
	return "/uploads/library/covers/" + hash[:16] + "." + ext, nil
}

// epubMeta holds metadata extracted from an EPUB's OPF file.
type epubMeta struct {
	title    string
	author   string
	language string
}

// extractEpubMeta opens an EPUB zip in memory and parses the first OPF file
// for dc:title, dc:creator and dc:language. Returns nil on any error.
func extractEpubMeta(data []byte) *epubMeta {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil
	}
	for _, f := range zr.File {
		if !strings.HasSuffix(strings.ToLower(f.Name), ".opf") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil
		}
		raw, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil
		}

		// Parse the minimal OPF metadata block.
		type dcMeta struct {
			Title    []string `xml:"metadata>title"`
			Creator  []string `xml:"metadata>creator"`
			Language []string `xml:"metadata>language"`
		}
		var m dcMeta
		if err := xml.Unmarshal(raw, &m); err != nil {
			return nil
		}
		meta := &epubMeta{}
		if len(m.Title) > 0 {
			meta.title = strings.TrimSpace(m.Title[0])
		}
		if len(m.Creator) > 0 {
			meta.author = strings.TrimSpace(m.Creator[0])
		}
		if len(m.Language) > 0 {
			meta.language = strings.TrimSpace(m.Language[0])
		}
		return meta
	}
	return nil
}

// Upload saves a multipart file blob to disk under /uploads/library/<date>/
// and inserts the metadata row in `files`. Returns the persisted File.
//
// Only allowed: pdf, epub, fb2, docx, pptx, txt, rtf, md, odt, odp.
// На upload автоматически извлекается текст (для Tier-2 форматов) и
// подсчитываются страницы (для PDF).
// cover/coverHeader — опциональная обложка (JPEG/PNG/WebP, max 5 MB).
func (s *FileService) Upload(
	ctx context.Context,
	userID string,
	file multipart.File,
	header *multipart.FileHeader,
	cover multipart.File,
	coverHeader *multipart.FileHeader,
	categoryID, description, title, authorName, language string,
) (*domain.File, error) {
	if header.Size <= 0 {
		return nil, fmt.Errorf("empty file")
	}
	if header.Size > MaxLibraryFileSize {
		return nil, fmt.Errorf("file size exceeds %d bytes", MaxLibraryFileSize)
	}

	// ── Проверка формата ──────────────────────────────────────────────────────
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(header.Filename), "."))
	if !allowedExtensions[ext] {
		return nil, fmt.Errorf("format not allowed: %s. Supported: pdf, epub, fb2, docx, pptx, txt, rtf, md, odt, odp", ext)
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	// Если MIME не в whitelist и не octet-stream → отказываем
	if !allowedMimeTypes[contentType] {
		return nil, fmt.Errorf("mime type not allowed: %s", contentType)
	}

	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read upload: %w", err)
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(bytes))
	dotExt := "." + ext
	datePath := time.Now().Format("2006/01/02")
	storedName := hash[:16] + dotExt
	r2Key := "uploads/library/" + datePath + "/" + storedName

	var fileURL string
	if s.r2 != nil {
		var uploadErr error
		fileURL, uploadErr = s.r2.Upload(ctx, r2Key, bytes, contentType)
		if uploadErr != nil {
			return nil, fmt.Errorf("r2 upload: %w", uploadErr)
		}
	} else {
		dirPath := filepath.Join(libraryUploadDir, datePath)
		if err := os.MkdirAll(dirPath, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir: %w", err)
		}
		fullPath := filepath.Join(dirPath, storedName)
		if err := os.WriteFile(fullPath, bytes, 0o644); err != nil {
			return nil, fmt.Errorf("write file: %w", err)
		}
		fileURL = "/uploads/library/" + datePath + "/" + storedName
	}

	// ── Метаданные ───────────────────────────────────────────────────────────
	// Title: берём из формы, fallback — filename без расширения
	if title == "" {
		title = header.Filename
		if i := strings.LastIndex(title, "."); i > 0 {
			title = title[:i]
		}
	}

	// Извлечение текста для Tier-2/3 форматов (graceful — не ломает upload)
	extractedText := docextract.ExtractText(bytes, ext)

	// Auto-extract EPUB metadata when form fields are empty
	if ext == "epub" {
		if meta := extractEpubMeta(bytes); meta != nil {
			if title == "" && meta.title != "" {
				title = meta.title
			}
			if authorName == "" && meta.author != "" {
				authorName = meta.author
			}
			if language == "" && meta.language != "" {
				// normalise lang codes like "en-US" → "en"
				lang := strings.ToLower(strings.Split(meta.language, "-")[0])
				if lang == "ru" || lang == "en" {
					language = lang
				} else {
					language = "other"
				}
			}
		}
	}

	// Подсчёт страниц для PDF
	pagesCount := 0
	if ext == "pdf" {
		pagesCount = docextract.CountPDFPages(bytes)
	}

	// Опциональная обложка
	coverURL, err := s.uploadCover(ctx, cover, coverHeader)
	if err != nil {
		s.logger.Warn("upload cover (non-fatal)", zap.Error(err))
		coverURL = ""
	}

	entity := &domain.File{
		UserID:        userID,
		Filename:      header.Filename,
		Title:         title,
		AuthorName:    authorName,
		Language:      language,
		FileURL:       fileURL,
		MimeType:      contentType,
		FileSize:      header.Size,
		CategoryID:    categoryID,
		Description:   description,
		PagesCount:    pagesCount,
		DocFormat:     ext,
		ExtractedText: extractedText,
		CoverURL:      coverURL,
	}
	if err := s.fileRepo.Create(ctx, entity); err != nil {
		return nil, err
	}

	// Kick off background PDF conversion for convertible formats
	if convertibleFormats[ext] {
		if setErr := s.fileRepo.SetPdfConversionStatus(ctx, entity.ID, "pending"); setErr != nil {
			s.logger.Warn("set pdf_conversion_status pending", zap.Error(setErr))
		} else {
			go s.doConvert(entity.ID, entity.FileURL, entity.DocFormat)
		}
	}

	return s.fileRepo.GetByID(ctx, entity.ID)
}

func (s *FileService) CreateFile(ctx context.Context, userID string, req *domain.CreateFileRequest) (*domain.File, error) {
	file := &domain.File{
		UserID:      userID,
		Filename:    req.Filename,
		FileURL:     req.FileURL,
		MimeType:    req.MimeType,
		FileSize:    req.FileSize,
		CategoryID:  req.CategoryID,
		Description: req.Description,
	}
	if err := s.fileRepo.Create(ctx, file); err != nil {
		return nil, err
	}
	return s.fileRepo.GetByID(ctx, file.ID)
}

func (s *FileService) GetFile(ctx context.Context, id string) (*domain.File, error) {
	return s.fileRepo.GetByID(ctx, id)
}

// UpdateFileMeta обновляет редактируемые метаданные файла (только для автора).
func (s *FileService) UpdateFileMeta(ctx context.Context, fileID, userID string, req domain.UpdateFileMetaRequest) (*domain.File, error) {
	if err := s.fileRepo.UpdateMeta(ctx, fileID, userID, req); err != nil {
		return nil, err
	}
	return s.fileRepo.GetByID(ctx, fileID)
}

// Trending (LIB-6) — top-N files за указанный период.
func (s *FileService) Trending(ctx context.Context, limit int, period string) ([]*domain.File, error) {
	return s.fileRepo.Trending(ctx, limit, period)
}

func (s *FileService) ListFiles(ctx context.Context, p domain.FileListParams) ([]*domain.File, string, error) {
	return s.fileRepo.List(ctx, p)
}

func (s *FileService) GetUserFiles(ctx context.Context, ownerID, viewerID string, limit, offset int) ([]*domain.File, int, error) {
	if err := s.fileRepo.CheckUserVisibility(ctx, ownerID, viewerID); err != nil {
		return nil, 0, err
	}
	return s.fileRepo.GetUserFiles(ctx, ownerID, limit, offset)
}

func (s *FileService) DeleteFile(ctx context.Context, id, userID string) error {
	// Load before delete to get file URL for blob cleanup
	file, err := s.fileRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if file.UserID != userID {
		return domain.ErrFileNotFound
	}
	if err := s.fileRepo.Delete(ctx, id, userID); err != nil {
		return err
	}
	// Delete blob from R2 (best-effort)
	if s.r2 != nil {
		if key, ok := s.r2.KeyFromURL(file.FileURL); ok {
			if delErr := s.r2.Delete(ctx, key); delErr != nil {
				s.logger.Warn("delete r2 blob", zap.Error(delErr), zap.String("key", key))
			}
		}
	} else {
		// Local disk — strip leading "/" and delete
		localPath := "." + file.FileURL
		if rmErr := os.Remove(localPath); rmErr != nil {
			s.logger.Warn("delete local file", zap.Error(rmErr), zap.String("path", localPath))
		}
	}
	return nil
}

func (s *FileService) LikeFile(ctx context.Context, fileID, userID string) error {
	file, err := s.fileRepo.GetByID(ctx, fileID)
	if err != nil {
		return err
	}
	isNew, err := s.fileRepo.LikeFile(ctx, fileID, userID)
	if err != nil {
		return err
	}
	// Social score: не считаем лайки себе, и только новые лайки
	if isNew && file.UserID != userID {
		// audio/* → audio_likes; остальное (pdf, книги) → book_likes
		statsField := "book_likes"
		if len(file.MimeType) >= 5 && file.MimeType[:5] == "audio" {
			statsField = "audio_likes"
		}
		if err := s.statsRepo.IncrementLikes(ctx, file.UserID, statsField); err != nil {
			s.logger.Warn("increment file likes", zap.Error(err))
		}
	}
	return nil
}

func (s *FileService) UnlikeFile(ctx context.Context, fileID, userID string) error {
	_, err := s.fileRepo.UnlikeFile(ctx, fileID, userID)
	return err
}

func (s *FileService) IsFileLiked(ctx context.Context, fileID, userID string) (bool, error) {
	return s.fileRepo.IsFileLiked(ctx, fileID, userID)
}

// RateFile saves or updates the user's star rating (1–5) for a file.
func (s *FileService) RateFile(ctx context.Context, fileID, userID string, rating int, reviewText string) error {
	if rating < 1 || rating > 5 {
		return fmt.Errorf("rating must be between 1 and 5")
	}
	if len(reviewText) > 2000 {
		reviewText = reviewText[:2000]
	}
	return s.fileRepo.RateFile(ctx, fileID, userID, rating, reviewText)
}

// GetUserRating returns the viewer's current rating and review for a file (0/"" = not rated).
func (s *FileService) GetUserRating(ctx context.Context, fileID, userID string) (int, string, error) {
	return s.fileRepo.GetUserRating(ctx, fileID, userID)
}

// GetFileReviews returns recent reviews for a file.
func (s *FileService) GetFileReviews(ctx context.Context, fileID string, limit int) ([]map[string]interface{}, error) {
	return s.fileRepo.GetFileReviews(ctx, fileID, limit)
}

// EnrichWithUserRatings sets UserRating for each file in the slice (batch query).
func (s *FileService) EnrichWithUserRatings(ctx context.Context, files []*domain.File, viewerID string) {
	if viewerID == "" || len(files) == 0 {
		return
	}
	ids := make([]string, len(files))
	for i, f := range files {
		ids[i] = f.ID
	}
	rated, err := s.fileRepo.GetUserRatingBatch(ctx, viewerID, ids)
	if err != nil || rated == nil {
		return
	}
	for _, f := range files {
		f.UserRating = rated[f.ID]
	}
}

// TrackView increments the view counter for a file and records it in file_views if userID known.
func (s *FileService) TrackView(ctx context.Context, fileID, userID string) error {
	return s.fileRepo.IncrementViews(ctx, fileID, userID)
}

// GetRecentlyViewed returns files recently viewed by the user.
func (s *FileService) GetRecentlyViewed(ctx context.Context, userID string, limit int) ([]*domain.File, error) {
	return s.fileRepo.GetRecentlyViewed(ctx, userID, limit)
}

func (s *FileService) DownloadFile(ctx context.Context, fileID, userID string) (*domain.File, error) {
	file, err := s.fileRepo.GetByID(ctx, fileID)
	if err != nil {
		return nil, err
	}
	_ = s.fileRepo.RecordDownload(ctx, fileID, userID)
	return file, nil
}

func (s *FileService) GetExtractedText(ctx context.Context, fileID string) (string, error) {
	return s.fileRepo.GetExtractedText(ctx, fileID)
}

// ReExtractText re-runs text extraction on a stored file and saves the result.
// Used when extracted_text is NULL (files uploaded before the extraction feature
// was added) or when the extractor is updated to fix bugs.
func (s *FileService) ReExtractText(ctx context.Context, fileID string) (string, error) {
	file, err := s.fileRepo.GetByID(ctx, fileID)
	if err != nil {
		return "", err
	}

	data, err := s.downloadFileBytes(ctx, file)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	ext := docextract.DocFormat(file.Filename)
	text := docextract.ExtractText(data, ext)

	if err := s.fileRepo.UpdateExtractedText(ctx, fileID, text); err != nil {
		return "", fmt.Errorf("save extracted text: %w", err)
	}
	return text, nil
}

func (s *FileService) GetCategories(ctx context.Context) ([]*domain.FileCategory, error) {
	return s.fileRepo.GetCategories(ctx)
}

// GetOrConvertToPDF возвращает URL к PDF-версии файла.
//   - PDF: возвращает оригинальный fileUrl без конвертации.
//   - Конвертируемые форматы: проверяет кэш; если конвертация ещё идёт —
//     возвращает ErrConversionPending (клиент должен опросить /pdf-status).
func (s *FileService) GetOrConvertToPDF(ctx context.Context, fileID string) (string, error) {
	file, err := s.fileRepo.GetByID(ctx, fileID)
	if err != nil {
		return "", err
	}

	// PDF не требует конвертации
	if file.DocFormat == "pdf" {
		return file.FileURL, nil
	}

	// Проверяем кэш
	cachedURL, err := s.fileRepo.GetPdfCacheURL(ctx, fileID)
	if err != nil {
		return "", err
	}
	if cachedURL != "" {
		return cachedURL, nil
	}

	// Проверяем статус фоновой конвертации
	status, err := s.fileRepo.GetPdfConversionStatus(ctx, fileID)
	if err != nil {
		return "", err
	}
	switch status {
	case "pending", "converting":
		return "", ErrConversionPending
	}

	// Статус 'none', 'failed' или 'done' без кэша — запускаем фоновую конвертацию
	if !pdfconvert.IsAvailable() {
		return "", fmt.Errorf("PDF conversion unavailable: LibreOffice not installed on server")
	}
	if setErr := s.fileRepo.SetPdfConversionStatus(ctx, fileID, "pending"); setErr != nil {
		return "", setErr
	}
	go s.doConvert(fileID, file.FileURL, file.DocFormat)
	return "", ErrConversionPending
}

// GetPdfStatus returns the conversion status and PDF URL (if done) for a file.
func (s *FileService) GetPdfStatus(ctx context.Context, fileID string) (status, pdfURL string, err error) {
	file, err := s.fileRepo.GetByID(ctx, fileID)
	if err != nil {
		return "", "", err
	}
	if file.DocFormat == "pdf" {
		return "done", file.FileURL, nil
	}
	cached, err := s.fileRepo.GetPdfCacheURL(ctx, fileID)
	if err != nil {
		return "", "", err
	}
	if cached != "" {
		return "done", cached, nil
	}
	st, err := s.fileRepo.GetPdfConversionStatus(ctx, fileID)
	if err != nil {
		return "", "", err
	}
	return st, "", nil
}

// BatchConvertPending converts all pending files sequentially. Run in a goroutine on startup.
func (s *FileService) BatchConvertPending(ctx context.Context) {
	files, err := s.fileRepo.GetPendingConversions(ctx)
	if err != nil {
		s.logger.Error("batch convert: get pending", zap.Error(err))
		return
	}
	if len(files) == 0 {
		s.logger.Info("batch convert: no pending files")
		return
	}
	s.logger.Info("batch convert: starting", zap.Int("count", len(files)))
	for _, f := range files {
		if ctx.Err() != nil {
			return
		}
		s.doConvert(f.ID, f.FileURL, f.DocFormat)
	}
	s.logger.Info("batch convert: all done")
}

// doConvert claims a pending file and converts it to PDF in the background.
// Safe to call concurrently — only one caller will proceed per file.
func (s *FileService) doConvert(fileID, fileURL, docFormat string) {
	ctx := context.Background()

	ok, err := s.fileRepo.TryClaimConversion(ctx, fileID)
	if err != nil {
		s.logger.Error("claim pdf conversion", zap.String("file_id", fileID), zap.Error(err))
		return
	}
	if !ok {
		return // someone else claimed it
	}

	s.logger.Info("pdf conversion started", zap.String("file_id", fileID), zap.String("format", docFormat))
	pdfURL, err := s.execConvert(ctx, fileID, fileURL, docFormat)
	if err != nil {
		s.logger.Error("pdf conversion failed", zap.String("file_id", fileID), zap.Error(err))
		_ = s.fileRepo.SetPdfConversionStatus(ctx, fileID, "failed")
		return
	}
	if setErr := s.fileRepo.SetPdfCacheURL(ctx, fileID, pdfURL); setErr != nil {
		s.logger.Warn("set pdf cache url", zap.String("file_id", fileID), zap.Error(setErr))
	}
	_ = s.fileRepo.SetPdfConversionStatus(ctx, fileID, "done")
	s.logger.Info("pdf conversion done", zap.String("file_id", fileID))
}

// execConvert downloads the original file, converts to PDF via LibreOffice, uploads the result.
func (s *FileService) execConvert(ctx context.Context, fileID, fileURL, docFormat string) (string, error) {
	file := &domain.File{ID: fileID, FileURL: fileURL, DocFormat: docFormat}

	srcBytes, err := s.downloadFileBytes(ctx, file)
	if err != nil {
		return "", fmt.Errorf("download original: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "pdfconv_src_*")
	if err != nil {
		return "", fmt.Errorf("mktemp src: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	srcPath := filepath.Join(tmpDir, "input."+docFormat)
	if err := os.WriteFile(srcPath, srcBytes, 0o644); err != nil {
		return "", fmt.Errorf("write src: %w", err)
	}

	pdfData, err := pdfconvert.ConvertToPDF(ctx, srcPath)
	if err != nil {
		return "", fmt.Errorf("convert: %w", err)
	}

	r2Key := "uploads/library/pdf-cache/" + fileID + ".pdf"
	if s.r2 != nil {
		return s.r2.Upload(ctx, r2Key, pdfData, "application/pdf")
	}
	localDir := filepath.Join(libraryUploadDir, "pdf-cache")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir pdf-cache: %w", err)
	}
	localPath := filepath.Join(localDir, fileID+".pdf")
	if err := os.WriteFile(localPath, pdfData, 0o644); err != nil {
		return "", fmt.Errorf("write pdf locally: %w", err)
	}
	return "/uploads/library/pdf-cache/" + fileID + ".pdf", nil
}

// downloadFileBytes скачивает байты файла — с R2 через HTTP или из локального FS.
func (s *FileService) downloadFileBytes(ctx context.Context, file *domain.File) ([]byte, error) {
	if strings.HasPrefix(file.FileURL, "http") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, file.FileURL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		return io.ReadAll(resp.Body)
	}
	// Локальный путь (dev без R2)
	return os.ReadFile(filepath.Join(".", file.FileURL))
}

// PopularAuthors возвращает авторов с наибольшим суммарным количеством лайков+скачиваний.
func (s *FileService) PopularAuthors(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	return s.fileRepo.PopularAuthors(ctx, limit)
}

// FormatStats возвращает количество файлов по формату.
func (s *FileService) FormatStats(ctx context.Context) ([]map[string]interface{}, error) {
	return s.fileRepo.FormatStats(ctx)
}

// GetSocialPicks returns files popular among users the given user follows.
func (s *FileService) GetSocialPicks(ctx context.Context, userID string, limit int) ([]*domain.File, error) {
	return s.fileRepo.GetSocialPicks(ctx, userID, limit)
}

// GetRelatedFiles returns files by the same author or category (excludes fileID).
func (s *FileService) GetRelatedFiles(ctx context.Context, fileID string, limit int) ([]*domain.File, error) {
	return s.fileRepo.GetRelatedFiles(ctx, fileID, limit)
}

// SearchSuggestions возвращает подсказки по названиям и авторам.
func (s *FileService) SearchSuggestions(ctx context.Context, q string) ([]map[string]interface{}, error) {
	return s.fileRepo.SearchSuggestions(ctx, q, 8)
}

// RecommendedFiles возвращает персонализированные рекомендации.
func (s *FileService) RecommendedFiles(ctx context.Context, userID string, limit int) ([]*domain.File, error) {
	return s.fileRepo.RecommendedFiles(ctx, userID, limit)
}

// GetReadingGoal returns the user's reading goal for the current year.
func (s *FileService) GetReadingGoal(ctx context.Context, userID string, year int) (*domain.ReadingGoal, error) {
	return s.readingRepo.GetReadingGoal(ctx, userID, year)
}

// UpsertReadingGoal creates or updates the user's reading goal.
func (s *FileService) UpsertReadingGoal(ctx context.Context, userID string, year, goalBooks int) (*domain.ReadingGoal, error) {
	return s.readingRepo.UpsertReadingGoal(ctx, userID, year, goalBooks)
}

// DeleteReadingGoal removes the user's reading goal for the year.
func (s *FileService) DeleteReadingGoal(ctx context.Context, userID string, year int) error {
	return s.readingRepo.DeleteReadingGoal(ctx, userID, year)
}
