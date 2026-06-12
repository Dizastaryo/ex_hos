package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	"github.com/seeu/backend/pkg/docextract"
	"github.com/seeu/backend/pkg/storage"
	"go.uber.org/zap"
)

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
	fileRepo  *postgres.FileRepository
	statsRepo *postgres.UserStatsRepository
	logger    *zap.Logger
	r2        *storage.R2
}

func NewFileService(fileRepo *postgres.FileRepository, statsRepo *postgres.UserStatsRepository, logger *zap.Logger, r2 *storage.R2) *FileService {
	if r2 == nil {
		os.MkdirAll(libraryUploadDir, 0o755)
	}
	return &FileService{fileRepo: fileRepo, statsRepo: statsRepo, logger: logger, r2: r2}
}

// Upload saves a multipart file blob to disk under /uploads/library/<date>/
// and inserts the metadata row in `files`. Returns the persisted File.
//
// Only allowed: pdf, epub, fb2, docx, pptx, txt, rtf, md, odt, odp.
// На upload автоматически извлекается текст (для Tier-2 форматов) и
// подсчитываются страницы (для PDF).
func (s *FileService) Upload(
	ctx context.Context,
	userID string,
	file multipart.File,
	header *multipart.FileHeader,
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

	// Подсчёт страниц для PDF
	pagesCount := 0
	if ext == "pdf" {
		pagesCount = docextract.CountPDFPages(bytes)
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
	}
	if err := s.fileRepo.Create(ctx, entity); err != nil {
		return nil, err
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

// Trending (LIB-6) — top-N files за 7 дней.
func (s *FileService) Trending(ctx context.Context, limit int) ([]*domain.File, error) {
	return s.fileRepo.Trending(ctx, limit)
}

func (s *FileService) ListFiles(ctx context.Context, categoryID string, limit, offset int) ([]*domain.File, int, error) {
	return s.fileRepo.List(ctx, categoryID, limit, offset)
}

func (s *FileService) GetUserFiles(ctx context.Context, ownerID, viewerID string, limit, offset int) ([]*domain.File, int, error) {
	if err := s.fileRepo.CheckUserVisibility(ctx, ownerID, viewerID); err != nil {
		return nil, 0, err
	}
	return s.fileRepo.GetUserFiles(ctx, ownerID, limit, offset)
}

func (s *FileService) DeleteFile(ctx context.Context, id, userID string) error {
	return s.fileRepo.Delete(ctx, id, userID)
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

func (s *FileService) GetCategories(ctx context.Context) ([]*domain.FileCategory, error) {
	return s.fileRepo.GetCategories(ctx)
}
