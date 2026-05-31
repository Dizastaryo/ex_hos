package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	redisRepo "github.com/seeu/backend/internal/repository/redis"
	"github.com/seeu/backend/internal/ws"
	"github.com/seeu/backend/pkg/pagination"
	"go.uber.org/zap"
)

const cacheTTL = 5 * time.Minute

type PostService struct {
	postRepo     *postgres.PostRepository
	userRepo     *postgres.UserRepository
	followRepo   *postgres.FollowRepository
	cache        *redisRepo.Cache
	wsHub        *ws.Hub
	mediaService *MediaService
	logger       *zap.Logger
}

func NewPostService(
	postRepo *postgres.PostRepository,
	userRepo *postgres.UserRepository,
	followRepo *postgres.FollowRepository,
	cache *redisRepo.Cache,
	wsHub *ws.Hub,
	mediaService *MediaService,
	logger *zap.Logger,
) *PostService {
	return &PostService{
		postRepo:     postRepo,
		userRepo:     userRepo,
		followRepo:   followRepo,
		cache:        cache,
		wsHub:        wsHub,
		mediaService: mediaService,
		logger:       logger,
	}
}

func (s *PostService) Create(ctx context.Context, userID string, req *domain.CreatePostRequest) (*domain.Post, error) {
	thumbnailURL := req.ThumbnailURL

	// Auto-generate thumbnail if not provided and post has video
	if thumbnailURL == "" {
		for i, mt := range req.MediaTypes {
			if mt == "video" && i < len(req.MediaURLs) {
				if thumb := generateVideoThumbnail(req.MediaURLs[i], s.logger); thumb != "" {
					thumbnailURL = thumb
				}
				break
			}
		}
	}

	post := &domain.Post{
		UserID:       userID,
		Caption:      req.Caption,
		MediaURLs:    req.MediaURLs,
		MediaTypes:   req.MediaTypes,
		Location:     req.Location,
		ThumbnailURL: thumbnailURL,
		AudioTrackID: req.AudioTrackID,
	}

	if err := s.postRepo.Create(ctx, post); err != nil {
		return nil, fmt.Errorf("create post: %w", err)
	}

	if err := s.userRepo.IncrementPostsCount(ctx, userID, 1); err != nil {
		s.logger.Warn("increment posts count", zap.Error(err))
	}

	s.invalidateFeedCache(ctx, userID)

	user, err := s.userRepo.GetByID(ctx, userID)
	if err == nil {
		post.User = &domain.UserShort{
			ID:         user.ID,
			Username:   user.Username,
			FullName:   user.FullName,
			AvatarURL:  user.AvatarURL,
			IsVerified: user.IsVerified,
		}
	}

	// FEED-3: realtime push к followers — frontend покажет banner «У вас N
	// новых постов ↑» в feed'е, не дёргая scroll position. Best-effort,
	// payload теряется для offline-followers (увидят при next refresh).
	if s.wsHub != nil {
		if followers, ferr := s.followRepo.GetFollowerIDs(ctx, userID); ferr == nil {
			payload := map[string]any{
				"post_id":         post.ID,
				"author_id":       userID,
				"author_username": post.User.Username,
			}
			for _, fid := range followers {
				s.wsHub.SendToUser(fid, "post.created", payload)
			}
		}
	}

	return post, nil
}

func (s *PostService) GetByID(ctx context.Context, postID, viewerID string) (*domain.Post, error) {
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		return nil, err
	}

	// Reuse the batch-aware reaction loader on a 1-element slice — keeps
	// hydration logic consistent with feed/explore.
	if err := s.postRepo.AttachReactions(ctx, []*domain.Post{post}, viewerID); err != nil {
		s.logger.Warn("attach post reactions (single)", zap.Error(err))
	}

	if viewerID != "" {
		liked, err := s.postRepo.IsLikedByUser(ctx, postID, viewerID)
		if err == nil {
			post.IsLiked = liked
		}

		saved, err := s.postRepo.IsSavedByUser(ctx, postID, viewerID)
		if err == nil {
			post.IsSaved = saved
		}
	}

	return post, nil
}

// SetReaction upserts caller's reaction on a post. Returns the new aggregate
// counts so handler can avoid an extra roundtrip and the WS-push can include
// the same data.
func (s *PostService) SetReaction(ctx context.Context, postID, userID, emoji string) (map[string]int, error) {
	if _, err := s.postRepo.GetByID(ctx, postID); err != nil {
		return nil, err
	}
	if err := s.postRepo.SetReaction(ctx, postID, userID, emoji); err != nil {
		return nil, err
	}
	counts, err := s.postRepo.CountReactions(ctx, postID)
	if err != nil {
		counts = map[string]int{}
	}
	pushPostReaction(s.wsHub, postID, counts)
	return counts, nil
}

// RemoveReaction deletes caller's reaction on a post.
func (s *PostService) RemoveReaction(ctx context.Context, postID, userID string) (map[string]int, error) {
	if _, err := s.postRepo.GetByID(ctx, postID); err != nil {
		return nil, err
	}
	if err := s.postRepo.RemoveReaction(ctx, postID, userID); err != nil {
		return nil, err
	}
	counts, err := s.postRepo.CountReactions(ctx, postID)
	if err != nil {
		counts = map[string]int{}
	}
	pushPostReaction(s.wsHub, postID, counts)
	return counts, nil
}

func (s *PostService) Delete(ctx context.Context, postID, userID string) error {
	// Capture media URLs BEFORE deleting the row so we can release dedup
	// references afterwards. If GetByID fails (post already gone, race) we
	// still try the Delete — keeps idempotent on retries.
	var mediaURLs []string
	if post, err := s.postRepo.GetByID(ctx, postID); err == nil && post != nil {
		mediaURLs = append(mediaURLs, post.MediaURLs...)
		if post.ThumbnailURL != "" {
			mediaURLs = append(mediaURLs, post.ThumbnailURL)
		}
	}

	if err := s.postRepo.Delete(ctx, postID, userID); err != nil {
		return err
	}

	if err := s.userRepo.IncrementPostsCount(ctx, userID, -1); err != nil {
		s.logger.Warn("decrement posts count", zap.Error(err))
	}

	s.invalidateFeedCache(ctx, userID)

	if s.mediaService != nil && len(mediaURLs) > 0 {
		s.mediaService.Release(ctx, mediaURLs)
	}
	return nil
}

func (s *PostService) GetFeed(ctx context.Context, userID string, page, limit int) ([]*domain.Post, pagination.Meta, error) {
	offset := pagination.Offset(page, limit)
	posts, err := s.postRepo.GetFeed(ctx, userID, limit+1, offset)
	if err != nil {
		return nil, pagination.Meta{}, fmt.Errorf("get feed: %w", err)
	}

	hasNext := len(posts) > limit
	if hasNext {
		posts = posts[:limit]
	}

	s.enrichPosts(ctx, posts, userID)

	meta := pagination.Meta{
		Page:        page,
		Limit:       limit,
		HasNextPage: hasNext,
	}

	return posts, meta, nil
}

// MarkViewed (FEED-5) — записать что userID посмотрел postID. Idempotent.
// Frontend вызывает после 5 сек просмотра в viewport.
func (s *PostService) MarkViewed(ctx context.Context, postID, userID string) error {
	if postID == "" || userID == "" {
		return nil
	}
	return s.postRepo.MarkPostViewed(ctx, postID, userID)
}

// GetFeedSmart (FEED-2) — score-based ranking. Offset-based pagination
// (cursor по float-score'у был бы fragile). Frontend toggle'ит между
// chronological и smart режимами.
func (s *PostService) GetFeedSmart(
	ctx context.Context, userID string, page, limit int,
) ([]*domain.Post, pagination.Meta, error) {
	offset := pagination.Offset(page, limit)
	posts, err := s.postRepo.GetFeedSmart(ctx, userID, limit+1, offset)
	if err != nil {
		return nil, pagination.Meta{}, fmt.Errorf("get feed smart: %w", err)
	}
	hasNext := len(posts) > limit
	if hasNext {
		posts = posts[:limit]
	}
	s.enrichPosts(ctx, posts, userID)
	return posts, pagination.Meta{
		Page:        page,
		Limit:       limit,
		HasNextPage: hasNext,
	}, nil
}

// GetFeedByCursor — FEED-1 cursor-based pagination. cursor пустой = первая
// страница. Returns posts + opaque next_cursor для followup'а (пустой если
// больше нет).
func (s *PostService) GetFeedByCursor(
	ctx context.Context, userID, cursor string, limit int,
) ([]*domain.Post, string, error) {
	var beforeTime time.Time
	var beforeID string
	if cursor != "" {
		t, id, ok := decodeFeedCursor(cursor)
		if !ok {
			return nil, "", fmt.Errorf("invalid cursor")
		}
		beforeTime = t
		beforeID = id
	}
	posts, err := s.postRepo.GetFeedByCursor(ctx, userID, beforeTime, beforeID, limit+1)
	if err != nil {
		return nil, "", fmt.Errorf("get feed cursor: %w", err)
	}
	hasNext := len(posts) > limit
	if hasNext {
		posts = posts[:limit]
	}
	s.enrichPosts(ctx, posts, userID)
	nextCursor := ""
	if hasNext && len(posts) > 0 {
		last := posts[len(posts)-1]
		nextCursor = encodeFeedCursor(last.CreatedAt, last.ID)
	}
	return posts, nextCursor, nil
}

// encodeFeedCursor / decodeFeedCursor — opaque cursor для FEED-1.
// Формат: base64url("RFC3339Nano|uuid"). Client не парсит, только passthrough.
func encodeFeedCursor(t time.Time, id string) string {
	raw := t.UTC().Format(time.RFC3339Nano) + "|" + id
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeFeedCursor(cursor string) (time.Time, string, bool) {
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", false
	}
	s := string(b)
	idx := strings.Index(s, "|")
	if idx < 0 {
		return time.Time{}, "", false
	}
	t, err := time.Parse(time.RFC3339Nano, s[:idx])
	if err != nil {
		return time.Time{}, "", false
	}
	return t, s[idx+1:], true
}

func (s *PostService) GetExplore(ctx context.Context, userID string, page, limit int, mediaType ...string) ([]*domain.Post, pagination.Meta, error) {
	offset := pagination.Offset(page, limit)
	mt := ""
	if len(mediaType) > 0 {
		mt = mediaType[0]
	}
	posts, err := s.postRepo.GetExplore(ctx, userID, limit+1, offset, mt)
	if err != nil {
		return nil, pagination.Meta{}, fmt.Errorf("get explore: %w", err)
	}

	hasNext := len(posts) > limit
	if hasNext {
		posts = posts[:limit]
	}

	s.enrichPosts(ctx, posts, userID)

	meta := pagination.Meta{
		Page:        page,
		Limit:       limit,
		HasNextPage: hasNext,
	}

	return posts, meta, nil
}

func (s *PostService) GetByUsername(ctx context.Context, username, viewerID string, page, limit int) ([]*domain.Post, pagination.Meta, error) {
	user, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return nil, pagination.Meta{}, err
	}

	// Приватный профиль: видят его только владелец и подтверждённые
	// подписчики. Анонимный viewerID="" → всегда блок.
	if user.IsPrivate && viewerID != user.ID {
		if viewerID == "" {
			return nil, pagination.Meta{}, domain.ErrPrivateAccount
		}
		isFollowing, err := s.followRepo.IsFollowing(ctx, viewerID, user.ID)
		if err != nil {
			return nil, pagination.Meta{}, fmt.Errorf("check follower: %w", err)
		}
		if !isFollowing {
			return nil, pagination.Meta{}, domain.ErrPrivateAccount
		}
	}

	offset := pagination.Offset(page, limit)
	posts, err := s.postRepo.GetByUserID(ctx, user.ID, limit+1, offset)
	if err != nil {
		return nil, pagination.Meta{}, fmt.Errorf("get user posts: %w", err)
	}

	hasNext := len(posts) > limit
	if hasNext {
		posts = posts[:limit]
	}

	s.enrichPosts(ctx, posts, viewerID)

	meta := pagination.Meta{
		Page:        page,
		Limit:       limit,
		HasNextPage: hasNext,
	}

	return posts, meta, nil
}

func (s *PostService) GetSaved(ctx context.Context, viewerID, username string, page, limit int) ([]*domain.Post, pagination.Meta, error) {
	user, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return nil, pagination.Meta{}, err
	}

	if viewerID != user.ID {
		return nil, pagination.Meta{}, domain.ErrForbidden
	}

	offset := pagination.Offset(page, limit)
	posts, err := s.postRepo.GetSavedByUserID(ctx, user.ID, limit+1, offset)
	if err != nil {
		return nil, pagination.Meta{}, fmt.Errorf("get saved posts: %w", err)
	}

	hasNext := len(posts) > limit
	if hasNext {
		posts = posts[:limit]
	}

	s.enrichPosts(ctx, posts, viewerID)

	meta := pagination.Meta{
		Page:        page,
		Limit:       limit,
		HasNextPage: hasNext,
	}

	return posts, meta, nil
}

func (s *PostService) enrichPosts(ctx context.Context, posts []*domain.Post, viewerID string) {
	if len(posts) == 0 {
		return
	}

	// Reactions are visible to everyone (including anon viewers); only
	// like/save flags depend on authenticated viewer.
	if err := s.postRepo.AttachReactions(ctx, posts, viewerID); err != nil {
		s.logger.Warn("attach post reactions", zap.Error(err))
	}

	if viewerID == "" {
		return
	}

	postIDs := make([]string, len(posts))
	for i, p := range posts {
		postIDs[i] = p.ID
	}

	likedMap, err := s.postRepo.GetLikedPostIDs(ctx, viewerID, postIDs)
	if err != nil {
		s.logger.Warn("batch get liked post ids", zap.Error(err))
		likedMap = make(map[string]bool)
	}

	savedMap, err := s.postRepo.GetSavedPostIDs(ctx, viewerID, postIDs)
	if err != nil {
		s.logger.Warn("batch get saved post ids", zap.Error(err))
		savedMap = make(map[string]bool)
	}

	for _, post := range posts {
		post.IsLiked = likedMap[post.ID]
		post.IsSaved = savedMap[post.ID]
	}
}

// invalidateFeedCache nukes every cached feed page across all users when a
// post is created or deleted. Targeted "only this author's followers'
// caches" approach would need a follow-list lookup per call (extra DB
// hit + race-prone if a new follower appeared between the lookup and the
// invalidation). Coarse-grained `feed:*` pattern is correct + cheap —
// feed TTL is short anyway, refill cost is one query per active user.
//
// Bug fix 2026-05-10: previously this only matched `feed:<authorID>:*`,
// so a post would not appear in followers' feeds until their own cache
// page expired (up to ~5 min). Authors saw their own posts via /me/posts
// which doesn't go through this cache.
func (s *PostService) invalidateFeedCache(ctx context.Context, userID string) {
	_ = userID // kept for future per-follower targeting; currently unused.
	s.cache.InvalidateAllFeeds(ctx)
}

// generateVideoThumbnail extracts a frame from video using ffmpeg.
// videoURL is relative like "/uploads/2026/05/05/abc.mp4".
// Returns relative URL to generated thumbnail or empty string on failure.
func generateVideoThumbnail(videoURL string, logger *zap.Logger) string {
	// Convert URL path to local file path
	localPath := strings.TrimPrefix(videoURL, "/")
	if _, err := os.Stat(localPath); err != nil {
		// Try with ./uploads prefix
		localPath = "." + videoURL
		if _, err := os.Stat(localPath); err != nil {
			if logger != nil {
				logger.Warn("video file not found for thumbnail", zap.String("url", videoURL))
			}
			return ""
		}
	}

	thumbDir := filepath.Join(".", "uploads", "thumbs")
	os.MkdirAll(thumbDir, 0755)

	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(videoURL)))
	thumbFile := hash[:16] + ".jpg"
	thumbPath := filepath.Join(thumbDir, thumbFile)

	// Skip if thumbnail already exists
	if _, err := os.Stat(thumbPath); err == nil {
		return "/uploads/thumbs/" + thumbFile
	}

	// BUG-21: ffmpeg с context-timeout 30 секунд + cleanup orphan thumb.
	// Раньше bare exec.Command — мог зависнуть на corrupt-video.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-y", "-ss", "1",
		"-i", localPath,
		"-vframes", "1",
		"-q:v", "3",
		"-vf", "scale=480:-1",
		thumbPath,
	)
	if err := cmd.Run(); err != nil {
		// Cleanup partial-write thumb если ffmpeg killed by timeout.
		_ = os.Remove(thumbPath)
		if logger != nil {
			if ctx.Err() == context.DeadlineExceeded {
				logger.Warn("ffmpeg thumbnail timeout",
					zap.String("video", videoURL))
			} else {
				logger.Warn("ffmpeg thumbnail generation failed", zap.Error(err))
			}
		}
		return ""
	}

	return "/uploads/thumbs/" + thumbFile
}
