// r2upload uploads all files from the local uploads/ directory to Cloudflare R2
// and then updates every URL in the database to point at the R2 public URL.
//
// Run from backend/: go run ./cmd/r2upload
package main

import (
	"context"
	"fmt"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/seeu/backend/config"
	"github.com/seeu/backend/pkg/storage"
)

const uploadDir = "uploads"
const workers = 1 // one at a time — safe for slow connections

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if !cfg.R2.IsConfigured() {
		log.Fatal("R2 not configured — check R2_ENDPOINT, R2_ACCESS_KEY, R2_SECRET_KEY, R2_PUBLIC_URL in .env.local")
	}

	r2, err := storage.NewR2(cfg.R2.Endpoint, cfg.R2.AccessKey, cfg.R2.SecretKey, cfg.R2.Bucket, cfg.R2.PublicURL)
	if err != nil {
		log.Fatalf("init r2: %v", err)
	}

	ctx := context.Background()

	// ── 1. Collect files ──────────────────────────────────────────────────────
	var files []string
	err = filepath.Walk(uploadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		files = append(files, filepath.ToSlash(path))
		return nil
	})
	if err != nil {
		log.Fatalf("walk uploads: %v", err)
	}
	fmt.Printf("Found %d files to upload\n", len(files))

	// ── 2. Upload in parallel ─────────────────────────────────────────────────
	var (
		done    int64
		failed  int64
		total   = int64(len(files))
		queue   = make(chan string, len(files))
		wg      sync.WaitGroup
		mu      sync.Mutex
		errors  []string
	)
	for _, f := range files {
		queue <- f
	}
	close(queue)

	start := time.Now()
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range queue {
				data, err := os.ReadFile(path)
				if err != nil {
					atomic.AddInt64(&failed, 1)
					mu.Lock()
					errors = append(errors, fmt.Sprintf("read %s: %v", path, err))
					mu.Unlock()
					continue
				}
				ct := mimeFor(path)
				uploadCtx, uploadCancel := context.WithTimeout(ctx, 10*time.Minute)
				_, uploadErr := r2.Upload(uploadCtx, path, data, ct)
				uploadCancel()
				if uploadErr != nil {
					atomic.AddInt64(&failed, 1)
					mu.Lock()
					errors = append(errors, fmt.Sprintf("upload %s: %v", path, uploadErr))
					mu.Unlock()
					continue
				}
				n := atomic.AddInt64(&done, 1)
				fmt.Printf("  [%d/%d] %s\n", n, total, path)
			}
		}()
	}
	wg.Wait()

	fmt.Printf("\nUpload done in %s — %d ok, %d failed\n", time.Since(start).Round(time.Second), done, failed)
	for _, e := range errors {
		fmt.Println("  ERR:", e)
	}
	if failed > 0 {
		log.Fatal("some files failed to upload, aborting DB update")
	}

	// ── 3. Update database URLs ───────────────────────────────────────────────
	pool, err := pgxpool.New(ctx, cfg.Database.URL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer pool.Close()

	pubBase := strings.TrimRight(cfg.R2.PublicURL, "/")
	fmt.Println("\nUpdating database URLs...")

	// Each query replaces /uploads/ prefix with the R2 public URL.
	// Array columns (media_urls) require a different update pattern.
	queries := []struct {
		label string
		sql   string
	}{
		{"users.avatar_url", fmt.Sprintf(`
			UPDATE users SET avatar_url = '%s' || avatar_url
			WHERE avatar_url LIKE '/uploads/%%'`, pubBase)},
		{"posts.media_urls", fmt.Sprintf(`
			UPDATE posts SET media_urls = (
				SELECT array_agg(
					CASE WHEN u LIKE '/uploads/%%' THEN '%s' || u ELSE u END
				) FROM unnest(media_urls) u
			)
			WHERE EXISTS (
				SELECT 1 FROM unnest(media_urls) u WHERE u LIKE '/uploads/%%'
			)`, pubBase)},
		{"posts.thumbnail_url", fmt.Sprintf(`
			UPDATE posts SET thumbnail_url = '%s' || thumbnail_url
			WHERE thumbnail_url LIKE '/uploads/%%'`, pubBase)},
		{"stories.media_url", fmt.Sprintf(`
			UPDATE stories SET media_url = '%s' || media_url
			WHERE media_url LIKE '/uploads/%%'`, pubBase)},
		{"highlights.cover_url", fmt.Sprintf(`
			UPDATE highlights SET cover_url = '%s' || cover_url
			WHERE cover_url LIKE '/uploads/%%'`, pubBase)},
		{"audio_tracks.audio_url", fmt.Sprintf(`
			UPDATE audio_tracks SET audio_url = '%s' || audio_url
			WHERE audio_url LIKE '/uploads/%%'`, pubBase)},
		{"audio_tracks.cover_url", fmt.Sprintf(`
			UPDATE audio_tracks SET cover_url = '%s' || cover_url
			WHERE cover_url LIKE '/uploads/%%'`, pubBase)},
		{"videos.video_url", fmt.Sprintf(`
			UPDATE videos SET video_url = '%s' || video_url
			WHERE video_url LIKE '/uploads/%%'`, pubBase)},
		{"videos.thumbnail_url", fmt.Sprintf(`
			UPDATE videos SET thumbnail_url = '%s' || thumbnail_url
			WHERE thumbnail_url LIKE '/uploads/%%'`, pubBase)},
		{"files.file_url", fmt.Sprintf(`
			UPDATE files SET file_url = '%s' || file_url
			WHERE file_url LIKE '/uploads/%%'`, pubBase)},
		{"messages.media_url", fmt.Sprintf(`
			UPDATE messages SET media_url = '%s' || media_url
			WHERE media_url LIKE '/uploads/%%'`, pubBase)},
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Fatalf("begin tx: %v", err)
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback(ctx)
		}
	}()

	for _, q := range queries {
		tag, err := tx.Exec(ctx, q.sql)
		if err != nil {
			// skip missing columns gracefully
			if strings.Contains(err.Error(), "column") && strings.Contains(err.Error(), "does not exist") {
				fmt.Printf("  %-30s skipped (column missing)\n", q.label)
				continue
			}
			log.Fatalf("update %s: %v", q.label, err)
		}
		fmt.Printf("  %-30s %d rows\n", q.label, tag.RowsAffected())
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("commit: %v", err)
	}
	committed = true

	fmt.Println("\nDone! All files are on R2 and URLs updated in DB.")
}

func mimeFor(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	known := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
		".mp4":  "video/mp4",
		".webm": "video/webm",
		".mov":  "video/quicktime",
		".mp3":  "audio/mpeg",
		".ogg":  "audio/ogg",
		".oga":  "audio/ogg",
		".wav":  "audio/wav",
		".aac":  "audio/aac",
		".pdf":  "application/pdf",
		".zip":  "application/zip",
	}
	if ct, ok := known[ext]; ok {
		return ct
	}
	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct
	}
	return "application/octet-stream"
}
