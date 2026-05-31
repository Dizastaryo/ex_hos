package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/seeu/backend/pkg/probe"
	"github.com/seeu/backend/pkg/storage"
	"go.uber.org/zap"
)

const (
	MaxImageSize = 10 * 1024 * 1024  // 10MB
	MaxVideoSize = 100 * 1024 * 1024 // 100MB
	UploadDir    = "./uploads"
)

var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
}

var allowedVideoTypes = map[string]bool{
	"video/mp4":       true,
	"video/webm":      true,
	"video/mov":       true,
	"video/quicktime": true,
}

var allowedAudioTypes = map[string]bool{
	"audio/mpeg":  true, // mp3
	"audio/mp4":   true, // m4a / aac in mp4 container
	"audio/wav":   true,
	"audio/x-wav": true,
	"audio/aac":   true,
	"audio/ogg":   true,
}

// MaxAudioSize caps a single track upload. Music tracks above ~30 MB are
// rare and usually a sign of uncompressed input — reject defensively.
const MaxAudioSize = 30 * 1024 * 1024

type MediaUploadResult struct {
	URL       string `json:"url"`
	MediaType string `json:"type"`
	MimeType  string `json:"mime_type"`
	Size      int64  `json:"size"`
	// DurationSeconds is populated for audio (and video, eventually) by
	// running `ffprobe` on the saved file. 0 means probe failed or the
	// file isn't a duration-bearing type — caller falls back to its own
	// value (frontend `AudioPlayer.duration` etc.).
	DurationSeconds int `json:"duration_seconds,omitempty"`
}

type MediaService struct {
	db     *pgxpool.Pool
	logger *zap.Logger
	r2     *storage.R2
}

func NewMediaService(db *pgxpool.Pool, logger *zap.Logger, r2 *storage.R2) *MediaService {
	if r2 == nil {
		os.MkdirAll(UploadDir, 0755)
	}
	return &MediaService{
		db:     db,
		logger: logger,
		r2:     r2,
	}
}

func (s *MediaService) Upload(ctx context.Context, file multipart.File, header *multipart.FileHeader) (*MediaUploadResult, error) {
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	isImage := allowedImageTypes[contentType]
	isVideo := allowedVideoTypes[contentType]
	isAudio := allowedAudioTypes[contentType]

	if !isImage && !isVideo && !isAudio {
		ext := strings.ToLower(filepath.Ext(header.Filename))
		switch ext {
		case ".jpg", ".jpeg":
			contentType = "image/jpeg"
			isImage = true
		case ".png":
			contentType = "image/png"
			isImage = true
		case ".gif":
			contentType = "image/gif"
			isImage = true
		case ".webp":
			contentType = "image/webp"
			isImage = true
		case ".mp4":
			contentType = "video/mp4"
			isVideo = true
		case ".webm":
			contentType = "video/webm"
			isVideo = true
		case ".mov":
			contentType = "video/quicktime"
			isVideo = true
		case ".mp3":
			contentType = "audio/mpeg"
			isAudio = true
		case ".m4a":
			contentType = "audio/mp4"
			isAudio = true
		case ".wav":
			contentType = "audio/wav"
			isAudio = true
		case ".aac":
			contentType = "audio/aac"
			isAudio = true
		case ".ogg":
			contentType = "audio/ogg"
			isAudio = true
		}
	}

	if !isImage && !isVideo && !isAudio {
		return nil, fmt.Errorf("unsupported media type: %s", contentType)
	}

	if isImage && header.Size > MaxImageSize {
		return nil, fmt.Errorf("image size exceeds maximum allowed size of 10MB")
	}
	if isVideo && header.Size > MaxVideoSize {
		return nil, fmt.Errorf("video size exceeds maximum allowed size of 100MB")
	}
	if isAudio && header.Size > MaxAudioSize {
		return nil, fmt.Errorf("audio size exceeds maximum allowed size of 30MB")
	}

	mediaType := "image"
	if isVideo {
		mediaType = "video"
	}
	if isAudio {
		mediaType = "audio"
	}

	// Read file content and compute SHA-256 hash
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	// BACK-2: magic-bytes валидация. Атака: загрузка exe с Content-Type:image/jpeg.
	// http.DetectContentType сниффит первые 512 байт и определяет реальный MIME.
	// Сравниваем prefix (image/, video/, audio/) с claimed mediaType. Если не
	// совпадает — отказ. Это не fool-proof (MIME-magic не всегда покрывает все
	// форматы), но защищает от наиболее очевидной подмены.
	detected := http.DetectContentType(fileBytes)
	detectedPrefix := strings.SplitN(detected, "/", 2)[0]
	if detectedPrefix != mediaType {
		// Edge: некоторые WebP/HEIC/etc определяются как application/octet-stream
		// — пропускаем если detected = octet-stream И extension валидный (мы уже
		// validated content-type выше через allowedXxxTypes).
		if detected != "application/octet-stream" {
			return nil, fmt.Errorf(
				"file content does not match declared type: detected=%s, claimed=%s",
				detected, contentType)
		}
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(fileBytes))

	// Check if file with same hash already exists (deduplication)
	var existingPath string
	err = s.db.QueryRow(ctx,
		`SELECT file_path FROM media_files WHERE hash = $1`, hash,
	).Scan(&existingPath)

	if err == nil {
		// File already exists — increment ref_count and return existing path
		s.db.Exec(ctx, `UPDATE media_files SET ref_count = ref_count + 1 WHERE hash = $1`, hash)
		s.logger.Info("media deduplicated", zap.String("hash", hash[:12]))
		// existingPath is relative like "uploads/2026/05/04/abc.jpg"
		// Normalise Windows backslashes that filepath.Join may have stored.
		relPath := strings.ReplaceAll(existingPath, "\\", "/")
		if !strings.HasPrefix(relPath, "/") {
			relPath = "/" + relPath
		}
		duration := 0
		if isAudio || isVideo {
			duration = probe.DurationSeconds(existingPath)
		}
		return &MediaUploadResult{
			URL:             relPath,
			MediaType:       mediaType,
			MimeType:        contentType,
			Size:            int64(len(fileBytes)),
			DurationSeconds: duration,
		}, nil
	}

	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = extFromMime(contentType)
	}
	datePath := time.Now().Format("2006/01/02")
	fileName := hash[:16] + ext
	relPath := datePath + "/" + fileName
	r2Key := "uploads/" + relPath

	var publicURL string
	duration := 0

	if s.r2 != nil {
		// Upload to Cloudflare R2
		var uploadErr error
		publicURL, uploadErr = s.r2.Upload(ctx, r2Key, fileBytes, contentType)
		if uploadErr != nil {
			return nil, fmt.Errorf("r2 upload: %w", uploadErr)
		}
		// Probe duration via temp file (ffprobe needs a local path)
		if isAudio || isVideo {
			if tmp, err := os.CreateTemp("", "seeu-probe-*"+ext); err == nil {
				tmp.Write(fileBytes)
				tmp.Close()
				duration = probe.DurationSeconds(tmp.Name())
				os.Remove(tmp.Name())
			}
		}
	} else {
		// Fall back to local disk
		dirPath := filepath.Join(UploadDir, datePath)
		os.MkdirAll(dirPath, 0755)
		fullPath := filepath.Join(dirPath, fileName)
		if err := os.WriteFile(fullPath, fileBytes, 0644); err != nil {
			return nil, fmt.Errorf("write file: %w", err)
		}
		publicURL = "/uploads/" + relPath
		if isAudio || isVideo {
			duration = probe.DurationSeconds(fullPath)
		}
	}

	// Insert into media_files table — store as relative path with forward
	// slashes so dedup lookups work cross-platform.
	_, err = s.db.Exec(ctx, `
		INSERT INTO media_files (hash, file_path, mime_type, media_type, size)
		VALUES ($1, $2, $3, $4, $5)`,
		hash, r2Key, contentType, mediaType, int64(len(fileBytes)),
	)
	if err != nil {
		s.logger.Warn("insert media_files record", zap.Error(err))
	}
	return &MediaUploadResult{
		URL:             publicURL,
		MediaType:       mediaType,
		MimeType:        contentType,
		Size:            int64(len(fileBytes)),
		DurationSeconds: duration,
	}, nil
}

// Release decrements the dedup ref_count for each URL that belongs to a
// previously uploaded local file. When a row's ref_count drops to ≤0 the
// row is removed AND the underlying disk blob is deleted — completing the
// dedup contract that was half-implemented before (TODO P1: «ref_count
// increment'ится но никогда не читается»).
//
// External URLs (https://...) and missing rows are silently skipped so
// callers can pass arbitrary post.media_urls without filtering.
//
// Best-effort: any failure is logged but doesn't propagate. The caller
// (post.Delete / story.Delete) shouldn't roll back over orphan disk
// blobs — they'll just take up a bit of space until a periodic sweep.
func (s *MediaService) Release(ctx context.Context, urls []string) {
	for _, url := range urls {
		s.releaseOne(ctx, url)
	}
}

// releaseOne handles per-URL bookkeeping: rows that would drop to 0 are
// deleted (and the disk blob removed); rows with ref_count>1 just decrement.
//
// We split into two queries instead of a CTE because PostgreSQL doesn't
// guarantee execution order between data-modifying CTEs touching the same
// table — the wrapping CTE I tried first left orphan rows at ref_count=0.
// Two sequential statements (delete-first, then decrement-the-rest) avoid
// that race entirely.
func (s *MediaService) releaseOne(ctx context.Context, url string) {
	if url == "" {
		return
	}
	// Determine the DB key suffix to match against media_files.file_path.
	// Supported URL forms:
	//   /uploads/<date>/<file>          — legacy local disk
	//   https://pub-xxx.r2.dev/uploads/<date>/<file> — R2 public URL
	var suffix string
	isR2 := false
	if key, ok := s.r2.KeyFromURL(url); ok {
		suffix = key
		isR2 = true
	} else if strings.HasPrefix(url, "/uploads/") {
		suffix = strings.TrimPrefix(url, "/")
	} else {
		return // external URL — skip
	}
	likePattern := "%" + suffix

	// Step 1: delete rows whose last reference is this one. RETURNING file_path
	// gives us the list of blobs to remove from disk.
	rows, err := s.db.Query(ctx, `
		DELETE FROM media_files
		WHERE file_path LIKE $1 AND ref_count <= 1
		RETURNING file_path`,
		likePattern,
	)
	if err != nil {
		s.logger.Warn("release media_files (delete)",
			zap.String("url", url), zap.Error(err))
		return
	}
	var pathsToDelete []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err == nil {
			pathsToDelete = append(pathsToDelete, path)
		}
	}
	rows.Close()

	// Step 2: decrement rows that still have other references. Only fires
	// for the SAME row when step 1 didn't match — `WHERE ref_count > 1`
	// guarantees no overlap with the rows just deleted.
	if _, err := s.db.Exec(ctx, `
		UPDATE media_files
		SET ref_count = ref_count - 1
		WHERE file_path LIKE $1 AND ref_count > 1`,
		likePattern,
	); err != nil {
		s.logger.Warn("release media_files (decrement)",
			zap.String("url", url), zap.Error(err))
	}

	// Step 3: remove blobs for fully-released files. Best-effort.
	for _, path := range pathsToDelete {
		if isR2 {
			if err := s.r2.Delete(ctx, path); err != nil {
				s.logger.Warn("r2 delete orphan blob",
					zap.String("key", path), zap.Error(err))
			} else {
				s.logger.Info("r2 released orphan blob", zap.String("key", path))
			}
		} else {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				s.logger.Warn("remove orphan blob",
					zap.String("path", path), zap.Error(err))
			} else {
				s.logger.Info("released orphan blob", zap.String("path", path))
			}
		}
	}
}

func extFromMime(mime string) string {
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "video/quicktime", "video/mov":
		return ".mov"
	default:
		return ".bin"
	}
}
