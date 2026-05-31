package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	"github.com/seeu/backend/pkg/storage"
	"go.uber.org/zap"
)

// MaxLibraryFileSize caps the size of any single file uploaded to the
// library. Documents typically fit well under this; videos that big should
// go via the post-creation pipeline instead.
const MaxLibraryFileSize = 50 * 1024 * 1024 // 50 MB

// libraryUploadDir is where library file blobs live on disk. Mirrors the
// /uploads/library/<date>/<hash>.<ext> URL convention. Kept separate from
// the post-media tree to keep retention/policy distinct (docs ≠ feed media).
const libraryUploadDir = "./uploads/library"

type FileService struct {
	fileRepo *postgres.FileRepository
	logger   *zap.Logger
	r2       *storage.R2
}

func NewFileService(fileRepo *postgres.FileRepository, logger *zap.Logger, r2 *storage.R2) *FileService {
	if r2 == nil {
		os.MkdirAll(libraryUploadDir, 0o755)
	}
	return &FileService{fileRepo: fileRepo, logger: logger, r2: r2}
}

// Upload saves a multipart file blob to disk under /uploads/library/<date>/
// and inserts the metadata row in `files`. Returns the persisted File.
//
// Allowed: any MIME, capped at MaxLibraryFileSize. Unlike post-media we don't
// dedup via media_files — library is a separate domain (documents vs feed
// content), and dedup'd "ref_count" semantics don't carry over (a doc owned
// by user A can't share an inode with user B's doc and let either delete it).
func (s *FileService) Upload(
	ctx context.Context,
	userID string,
	file multipart.File,
	header *multipart.FileHeader,
	categoryID, description string,
) (*domain.File, error) {
	if header.Size <= 0 {
		return nil, fmt.Errorf("empty file")
	}
	if header.Size > MaxLibraryFileSize {
		return nil, fmt.Errorf("file size exceeds %d bytes", MaxLibraryFileSize)
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read upload: %w", err)
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(bytes))
	ext := filepath.Ext(header.Filename)
	datePath := time.Now().Format("2006/01/02")
	storedName := hash[:16] + ext
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

	entity := &domain.File{
		UserID:      userID,
		Filename:    header.Filename,
		FileURL:     fileURL,
		MimeType:    contentType,
		FileSize:    header.Size,
		CategoryID:  categoryID,
		Description: description,
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
	_, err := s.fileRepo.LikeFile(ctx, fileID, userID)
	return err
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

func (s *FileService) GetCategories(ctx context.Context) ([]*domain.FileCategory, error) {
	return s.fileRepo.GetCategories(ctx)
}
