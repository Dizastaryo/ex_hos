package service

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// ExportService produces a complete dump of a user's data for GDPR / KZ ПДн export.
// Output is a single JSON object — small enough to fit in memory for any realistic
// user, large enough to satisfy the "all my data" expectation.
type ExportService struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewExportService(pool *pgxpool.Pool, logger *zap.Logger) *ExportService {
	return &ExportService{pool: pool, logger: logger}
}

type UserExport struct {
	GeneratedAt time.Time   `json:"generated_at"`
	Profile     interface{} `json:"profile"`
	Posts       []map[string]any `json:"posts"`
	Stories     []map[string]any `json:"stories"`
	Comments    []map[string]any `json:"comments"`
	LikesGiven  []map[string]any `json:"likes_given"`
	SavedPosts  []map[string]any `json:"saved_posts"`
	Following   []map[string]any `json:"following"`
	Followers   []map[string]any `json:"followers"`
	Chats       []map[string]any `json:"chats"`
}

func (s *ExportService) BuildExport(ctx context.Context, userID string) (*UserExport, error) {
	exp := &UserExport{GeneratedAt: time.Now().UTC()}

	// Profile
	var profile map[string]any
	row := s.pool.QueryRow(ctx, `
		SELECT id, username, phone, full_name, bio, avatar_url, website, gender,
		       is_private, is_verified, posts_count, followers_count, following_count,
		       device_public_id, created_at, updated_at
		FROM users WHERE id = $1`, userID)
	{
		var (
			id, username, phone, fullName, bio, avatarURL, website, gender, devicePub string
			isPrivate, isVerified                                                     bool
			postsCount, followersCount, followingCount                                int
			createdAt, updatedAt                                                      time.Time
		)
		if err := row.Scan(&id, &username, &phone, &fullName, &bio, &avatarURL, &website, &gender,
			&isPrivate, &isVerified, &postsCount, &followersCount, &followingCount,
			&devicePub, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("export profile: %w", err)
		}
		profile = map[string]any{
			"id": id, "username": username, "phone": phone, "full_name": fullName,
			"bio": bio, "avatar_url": avatarURL, "website": website, "gender": gender,
			"is_private": isPrivate, "is_verified": isVerified,
			"posts_count": postsCount, "followers_count": followersCount, "following_count": followingCount,
			"device_public_id": devicePub,
			"created_at":       createdAt, "updated_at": updatedAt,
		}
	}
	exp.Profile = profile

	// Posts
	if err := s.fetchRows(ctx, &exp.Posts, `
		SELECT id, caption, media_urls, media_types, location, thumbnail_url,
		       likes_count, comments_count, saves_count, created_at
		FROM posts WHERE user_id = $1 ORDER BY created_at DESC`,
		[]string{"id", "caption", "media_urls", "media_types", "location", "thumbnail_url",
			"likes_count", "comments_count", "saves_count", "created_at"},
		userID); err != nil {
		return nil, fmt.Errorf("export posts: %w", err)
	}

	// Stories (active + recent expired, last 90 days)
	if err := s.fetchRows(ctx, &exp.Stories, `
		SELECT id, media_url, media_type, text_overlay, views_count, likes_count, expires_at, created_at
		FROM stories WHERE user_id = $1 AND created_at > NOW() - INTERVAL '90 days'
		ORDER BY created_at DESC`,
		[]string{"id", "media_url", "media_type", "text_overlay", "views_count", "likes_count", "expires_at", "created_at"},
		userID); err != nil {
		return nil, fmt.Errorf("export stories: %w", err)
	}

	// Comments authored by user
	if err := s.fetchRows(ctx, &exp.Comments, `
		SELECT id, post_id, parent_id, text, likes_count, created_at
		FROM comments WHERE user_id = $1 ORDER BY created_at DESC`,
		[]string{"id", "post_id", "parent_id", "text", "likes_count", "created_at"},
		userID); err != nil {
		return nil, fmt.Errorf("export comments: %w", err)
	}

	// Likes given
	if err := s.fetchRows(ctx, &exp.LikesGiven, `
		SELECT entity_id, entity_type, created_at
		FROM likes WHERE user_id = $1 ORDER BY created_at DESC`,
		[]string{"entity_id", "entity_type", "created_at"},
		userID); err != nil {
		return nil, fmt.Errorf("export likes: %w", err)
	}

	// Saved posts
	if err := s.fetchRows(ctx, &exp.SavedPosts, `
		SELECT post_id, saved_at FROM saved_posts WHERE user_id = $1 ORDER BY saved_at DESC`,
		[]string{"post_id", "saved_at"}, userID); err != nil {
		return nil, fmt.Errorf("export saved: %w", err)
	}

	// Following / Followers — usernames only (we don't dump other users' private data)
	if err := s.fetchRows(ctx, &exp.Following, `
		SELECT u.username, f.created_at FROM follows f
		JOIN users u ON u.id = f.following_id
		WHERE f.follower_id = $1 ORDER BY f.created_at DESC`,
		[]string{"username", "since"}, userID); err != nil {
		return nil, fmt.Errorf("export following: %w", err)
	}
	if err := s.fetchRows(ctx, &exp.Followers, `
		SELECT u.username, f.created_at FROM follows f
		JOIN users u ON u.id = f.follower_id
		WHERE f.following_id = $1 ORDER BY f.created_at DESC`,
		[]string{"username", "since"}, userID); err != nil {
		return nil, fmt.Errorf("export followers: %w", err)
	}

	// Chats — conversations user is in, with messages they sent.
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.created_at FROM conversations c
		JOIN conversation_participants p ON p.conversation_id = c.id
		WHERE p.user_id = $1
		ORDER BY c.created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("export chats: %w", err)
	}
	defer rows.Close()

	type convoRow struct {
		id        string
		createdAt time.Time
	}
	var convos []convoRow
	for rows.Next() {
		var cr convoRow
		if err := rows.Scan(&cr.id, &cr.createdAt); err != nil {
			return nil, fmt.Errorf("export chats scan: %w", err)
		}
		convos = append(convos, cr)
	}
	rows.Close()

	for _, cr := range convos {
		var participants []string
		pRows, err := s.pool.Query(ctx, `
			SELECT u.username FROM conversation_participants p
			JOIN users u ON u.id = p.user_id
			WHERE p.conversation_id = $1`, cr.id)
		if err != nil {
			return nil, fmt.Errorf("export chat participants: %w", err)
		}
		for pRows.Next() {
			var name string
			if err := pRows.Scan(&name); err == nil {
				participants = append(participants, name)
			}
		}
		pRows.Close()

		var msgs []map[string]any
		if err := s.fetchRows(ctx, &msgs, `
			SELECT id, text, is_read, created_at FROM messages
			WHERE conversation_id = $1 AND sender_id = $2 ORDER BY created_at`,
			[]string{"id", "text", "is_read", "created_at"}, cr.id, userID); err != nil {
			return nil, fmt.Errorf("export chat messages: %w", err)
		}
		exp.Chats = append(exp.Chats, map[string]any{
			"id":               cr.id,
			"created_at":       cr.createdAt,
			"participants":     participants,
			"my_messages":      msgs,
			"my_messages_only": "сообщения других участников не включены — это их данные",
		})
	}

	return exp, nil
}

// fetchRows runs a SQL query and returns each row as a map keyed by `cols`.
// Saves us from declaring a struct per query — the export shape is one-off.
func (s *ExportService) fetchRows(ctx context.Context, dst *[]map[string]any, sql string, cols []string, args ...any) error {
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			row[c] = vals[i]
		}
		*dst = append(*dst, row)
	}
	return rows.Err()
}
