package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/seeu/backend/internal/domain"
)

// ChatConversation represents a conversation list item returned to the API.
//
// Direct (1-1) — поля Title/CoverURL пустые, OtherUser != nil.
// Group       — Title/CoverURL заполнены, OtherUser == nil, ParticipantsCount > 2.
type ChatConversation struct {
	ID                string    `json:"id"`
	Kind              string    `json:"kind"` // "direct" | "group"
	Title             string    `json:"title,omitempty"`
	CoverURL          string    `json:"cover_url,omitempty"`
	OtherUser         *ChatUser `json:"other_user"`
	ParticipantsCount int       `json:"participants_count,omitempty"`
	LastMessage       string    `json:"last_message"`
	LastSenderUsername string   `json:"last_sender_username,omitempty"`
	LastMessageAt     time.Time `json:"last_message_at"`
	UnreadCount       int       `json:"unread_count"`

	// Pinned: nil если ничего не закреплено.
	PinnedMessage *ReplyPreview `json:"pinned_message,omitempty"`

	// SborID — если чат является группой сбора, здесь хранится ID сбора.
	// nil для обычных group-чатов и direct-чатов.
	SborID *string `json:"sbor_id,omitempty"`

	// IsOrganizer — true если текущий пользователь является организатором сбора
	// (role = 'organizer' в sbor_members). false для участников и обычных чатов.
	IsOrganizer bool `json:"is_organizer"`

	// IsPinned — закреплён ли чат у текущего пользователя (хранится в
	// conversation_participants.pinned_at).
	IsPinned bool `json:"is_pinned"`

	// IsArchived — помещён ли чат в архив текущим пользователем.
	IsArchived bool `json:"archived"`

	// IsMuted — отключены ли уведомления для этого чата.
	IsMuted bool `json:"muted"`
}

// GroupParticipant — участник group-чата с ролью.
type GroupParticipant struct {
	User      *ChatUser `json:"user"`
	Role      string    `json:"role"` // "admin" | "member"
	JoinedAt  time.Time `json:"joined_at"`
}

// ChatUser is a full user object embedded in the chat list response.
type ChatUser struct {
	ID             string    `json:"id"`
	Username       string    `json:"username"`
	FullName       string    `json:"full_name"`
	Bio            string    `json:"bio"`
	AvatarURL      string    `json:"avatar_url"`
	Website        string    `json:"website"`
	Gender         string    `json:"gender"`
	DevicePublicID string    `json:"device_public_id"`
	IsPrivate      bool      `json:"is_private"`
	IsVerified     bool      `json:"is_verified"`
	PostsCount     int       `json:"posts_count"`
	FollowersCount int       `json:"followers_count"`
	FollowingCount int       `json:"following_count"`
	CreatedAt      time.Time `json:"created_at"`
}

// ChatMessage represents a single message.
type ChatMessage struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"chat_id"`
	SenderID       string    `json:"sender_id"`
	Text           string    `json:"text"`
	IsRead         bool      `json:"is_read"`
	// IsDelivered — «хотя бы один peer получил». Computed как
	// `delivered_count > 0`. Используется для 3-state UI direct-чата.
	// См. CHAT-10.1.
	IsDelivered    bool      `json:"is_delivered"`
	// Per-recipient counts (CHAT-10.2). Для direct-чата RecipientsCount=1.
	// Для group: фронт рисует «X из N прочитали» в bubble.
	DeliveredCount  int `json:"delivered_count"`
	ReadCount       int `json:"read_count"`
	RecipientsCount int `json:"recipients_count"`
	IsMe           bool      `json:"is_me"`
	CreatedAt      time.Time `json:"created_at"`
	// ExpiresAt — момент когда сообщение само удалится (CHAT-11). nil =
	// без TTL (обычное вечное сообщение). Frontend рисует ⏱ countdown +
	// auto-remove из локального state по timer'у. Janitor (cmd/api/main.go)
	// раз в 60s DELETE'ит истёкшие из DB.
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`

	// Kind = "text" (default), "shared_post", "image" или "voice".
	//   - shared_post → клиент рендерит превью поста по AttachedPostID
	//   - image       → клиент рендерит AttachedMediaURL как изображение
	//   - voice       → AttachedMediaURL — путь к audio, MediaDurationSeconds
	//                   + Waveform нужны для bubble'я без декодирования
	// Text может быть пустым или содержать комментарий к вложению.
	Kind                  string             `json:"kind"`
	AttachedPostID        *string            `json:"attached_post_id,omitempty"`
	AttachedPost          *AttachedPostShort `json:"attached_post,omitempty"`
	AttachedMediaURL      string             `json:"attached_media_url,omitempty"`
	AttachedMediaType     string             `json:"attached_media_type,omitempty"`
	MediaDurationSeconds  int                `json:"media_duration_seconds,omitempty"`
	Waveform              []float64          `json:"waveform,omitempty"`

	// Reply-to: если это reply на другое сообщение — ReplyTo содержит сжатое
	// preview оригинала (text/sender/kind). nil = обычное сообщение.
	ReplyToMessageID *string         `json:"reply_to_message_id,omitempty"`
	ReplyTo          *ReplyPreview   `json:"reply_to,omitempty"`

	// Reactions: count per emoji + the emoji *current user* placed on this
	// message (empty when none). Aggregated by GetMessages so client doesn't
	// roundtrip per message.
	Reactions  map[string]int `json:"reactions,omitempty"`
	MyReaction string         `json:"my_reaction,omitempty"`

	// Sender info — имя и аватар отправителя (для group-чатов).
	SenderName      string `json:"sender_name,omitempty"`
	SenderUsername  string `json:"sender_username,omitempty"`
	SenderAvatarURL string `json:"sender_avatar_url,omitempty"`

	// IsDeletedForAll — сообщение мягко удалено для всех (WhatsApp-стиль).
	// Если true, клиент показывает «Сообщение удалено» вместо содержимого.
	IsDeletedForAll bool `json:"is_deleted_for_all,omitempty"`
}

// ReplyPreview — сжатое превью оригинального message'а для отрисовки
// quoted-block'а в reply-bubble. Минимум: text, sender_username, kind.
type ReplyPreview struct {
	ID             string `json:"id"`
	SenderID       string `json:"sender_id"`
	SenderUsername string `json:"sender_username"`
	Text           string `json:"text"`
	Kind           string `json:"kind"`
}

// AttachedPostShort — сжатое превью поста, отдается вместе с messages
// чтобы фронт не делал отдельный запрос за каждым постом из переписки.
type AttachedPostShort struct {
	ID         string   `json:"id"`
	Caption    string   `json:"caption"`
	MediaURL   string   `json:"media_url"`
	MediaType  string   `json:"media_type"`
	Thumbnail  string   `json:"thumbnail_url,omitempty"`
	Author     string   `json:"author_username"`
	AuthorAvatar string `json:"author_avatar"`
}

type ChatRepository struct {
	db *DB
}

func NewChatRepository(db *DB) *ChatRepository {
	return &ChatRepository{db: db}
}

// GetOrCreateConversation finds an existing 1:1 conversation between two users
// or creates a new one. Returns the conversation ID. Только для kind='direct';
// group-чаты создаются через CreateGroupConversation.
func (r *ChatRepository) GetOrCreateConversation(ctx context.Context, userID1, userID2 string) (string, error) {
	// Look for an existing direct conversation where both users are participants.
	var convID string
	err := r.db.Pool.QueryRow(ctx, `
		SELECT c.id
		FROM conversations c
		JOIN conversation_participants cp1 ON cp1.conversation_id = c.id AND cp1.user_id = $1
		JOIN conversation_participants cp2 ON cp2.conversation_id = c.id AND cp2.user_id = $2
		WHERE c.kind = 'direct'
		LIMIT 1`, userID1, userID2).Scan(&convID)

	if err == nil {
		return convID, nil
	}
	if err != pgx.ErrNoRows {
		return "", fmt.Errorf("find conversation: %w", err)
	}

	// Create new conversation
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `INSERT INTO conversations DEFAULT VALUES RETURNING id`).Scan(&convID)
	if err != nil {
		return "", fmt.Errorf("create conversation: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO conversation_participants (conversation_id, user_id) VALUES ($1, $2), ($1, $3)`,
		convID, userID1, userID2)
	if err != nil {
		return "", fmt.Errorf("add participants: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}

	return convID, nil
}

// GetConversations returns all conversations for a user (direct + group),
// with last message + sender username, kind/title/cover, и unread count.
//
// Для direct-чатов: подгружается «другой» юзер через LATERAL (LIMIT 1 — для
// безопасности, если accidentally окажется > 2 participants в direct'е).
// Для group-чатов: title/cover из conversations, OtherUser остаётся NULL,
// ParticipantsCount считается отдельной subquery'ёй.
func (r *ChatRepository) GetConversations(ctx context.Context, userID string) ([]ChatConversation, error) {
	query := `
		SELECT
			c.id,
			c.kind,
			c.title,
			c.cover_url,
			u.id, u.username, u.full_name, u.bio, u.avatar_url, u.website, u.gender,
			u.device_public_id, u.is_private, u.is_verified,
			u.posts_count, u.followers_count, u.following_count, u.created_at,
			(SELECT COUNT(*) FROM conversation_participants WHERE conversation_id = c.id) AS participants_count,
			CASE WHEN m.is_deleted_for_all THEN 'Сообщение удалено' ELSE COALESCE(m.text, '') END,
			COALESCE(sender.username, ''),
			COALESCE(m.created_at, c.created_at),
			(SELECT COUNT(*) FROM messages WHERE conversation_id = c.id AND sender_id != $1 AND is_read = false),
			c.pinned_message_id, pm.id, pm.sender_id, COALESCE(pmu.username, ''), COALESCE(pm.text, ''), pm.kind,
			sb.id,
			COALESCE(
			  (SELECT sm.role = 'organizer' FROM sbor_members sm WHERE sm.sbor_id = sb.id AND sm.user_id = $1 LIMIT 1),
			  false
			) AS is_organizer,
			cp1.pinned_at IS NOT NULL AS is_pinned,
			cp1.archived_at IS NOT NULL AS is_archived,
			COALESCE(cp1.muted, false) AS is_muted
		FROM conversations c
		JOIN conversation_participants cp1 ON cp1.conversation_id = c.id AND cp1.user_id = $1 AND cp1.hidden_at IS NULL
		LEFT JOIN LATERAL (
			SELECT u.id, u.username, u.full_name, u.bio, u.avatar_url, u.website, u.gender,
			       u.device_public_id, u.is_private, u.is_verified,
			       u.posts_count, u.followers_count, u.following_count, u.created_at
			FROM conversation_participants cp2
			JOIN users u ON u.id = cp2.user_id
			WHERE cp2.conversation_id = c.id AND cp2.user_id != $1
			ORDER BY cp2.joined_at ASC
			LIMIT 1
		) u ON c.kind = 'direct'
		LEFT JOIN LATERAL (
			SELECT text, created_at, sender_id, is_deleted_for_all FROM messages
			WHERE conversation_id = c.id
			ORDER BY created_at DESC LIMIT 1
		) m ON true
		LEFT JOIN users sender ON sender.id = m.sender_id
		LEFT JOIN messages pm ON pm.id = c.pinned_message_id
		LEFT JOIN users pmu ON pmu.id = pm.sender_id
		LEFT JOIN sbory sb ON sb.chat_id = c.id AND NOT sb.is_cancelled
		ORDER BY (cp1.pinned_at IS NOT NULL) DESC, cp1.pinned_at DESC NULLS LAST, COALESCE(m.created_at, c.created_at) DESC`

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get conversations: %w", err)
	}
	defer rows.Close()

	var convs []ChatConversation
	for rows.Next() {
		var conv ChatConversation
		var otherID, otherUsername, otherFullName, otherBio, otherAvatar, otherWebsite, otherGender, otherDevice *string
		var otherIsPrivate, otherIsVerified *bool
		var otherPostsCount, otherFollowersCount, otherFollowingCount *int
		var otherCreatedAt *time.Time
		// Pinned fields — все nullable.
		var pinnedID, pmID, pmSenderID *string
		var pmUsername, pmText string
		var pmKind *string
		if err := rows.Scan(
			&conv.ID,
			&conv.Kind,
			&conv.Title,
			&conv.CoverURL,
			&otherID, &otherUsername, &otherFullName, &otherBio, &otherAvatar, &otherWebsite, &otherGender,
			&otherDevice, &otherIsPrivate, &otherIsVerified,
			&otherPostsCount, &otherFollowersCount, &otherFollowingCount, &otherCreatedAt,
			&conv.ParticipantsCount,
			&conv.LastMessage,
			&conv.LastSenderUsername,
			&conv.LastMessageAt,
			&conv.UnreadCount,
			&pinnedID, &pmID, &pmSenderID, &pmUsername, &pmText, &pmKind,
			&conv.SborID,
			&conv.IsOrganizer,
			&conv.IsPinned,
			&conv.IsArchived,
			&conv.IsMuted,
		); err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		if pinnedID != nil && pmID != nil {
			k := ""
			if pmKind != nil {
				k = *pmKind
			}
			senderID := ""
			if pmSenderID != nil {
				senderID = *pmSenderID
			}
			conv.PinnedMessage = &ReplyPreview{
				ID:             *pmID,
				SenderID:       senderID,
				SenderUsername: pmUsername,
				Text:           pmText,
				Kind:           k,
			}
		}
		if otherID != nil {
			conv.OtherUser = &ChatUser{
				ID:             *otherID,
				Username:       *otherUsername,
				FullName:       *otherFullName,
				Bio:            *otherBio,
				AvatarURL:      *otherAvatar,
				Website:        *otherWebsite,
				Gender:         *otherGender,
				DevicePublicID: *otherDevice,
				IsPrivate:      *otherIsPrivate,
				IsVerified:     *otherIsVerified,
				PostsCount:     *otherPostsCount,
				FollowersCount: *otherFollowersCount,
				FollowingCount: *otherFollowingCount,
				CreatedAt:      *otherCreatedAt,
			}
		}
		convs = append(convs, conv)
	}

	return convs, rows.Err()
}

// GetMessages returns messages for a conversation, ordered by created_at ASC.
// Joins post + author for messages of kind "shared_post" so the client doesn't
// need a roundtrip per attached post.
//
// q — optional substring filter, ILIKE %q% по полю text (CHAT-3 search).
// Пустая строка = без фильтра. Pagination limit/offset работает поверх
// отфильтрованного set'а.
func (r *ChatRepository) GetMessages(ctx context.Context, conversationID, currentUserID string, limit, offset int, q string) ([]ChatMessage, error) {
	// counts через LEFT JOIN LATERAL — один pass на message-recipients table
	// per row. Для 50 messages × 4 group-participants = 200 reads index-only.
	// is_read / is_delivered computed из counts (drop reliance on legacy
	// columns; CHAT-10.2). expires_at — CHAT-11 disappearing; фильтруем
	// `(expires_at IS NULL OR expires_at > NOW())` чтобы клиент не получил
	// сообщения которые janitor ещё не удалил.
	query := `
		SELECT m.id, m.conversation_id, m.sender_id, m.text,
		       (mc.read_count > 0)      AS is_read,
		       (mc.delivered_count > 0) AS is_delivered,
		       mc.delivered_count, mc.read_count, mc.recipients_count,
		       m.created_at, m.expires_at,
		       m.kind, m.attached_post_id, m.attached_media_url, m.attached_media_type,
		       m.media_duration_seconds, m.waveform, m.reply_to_message_id,
		       p.caption, p.media_urls, p.media_types, p.thumbnail_url,
		       pu.username, pu.avatar_url,
		       rm.id, rm.sender_id, COALESCE(ru.username, ''), COALESCE(rm.text, ''), rm.kind,
		       COALESCE(su.full_name, ''), COALESCE(su.username, ''), COALESCE(su.avatar_url, ''),
		       m.is_deleted_for_all
		FROM messages m
		LEFT JOIN LATERAL (
			SELECT COUNT(*)                AS recipients_count,
			       COUNT(delivered_at)     AS delivered_count,
			       COUNT(read_at)          AS read_count
			FROM message_recipients
			WHERE message_id = m.id
		) mc ON TRUE
		LEFT JOIN posts p ON p.id = m.attached_post_id
		LEFT JOIN users pu ON pu.id = p.user_id
		LEFT JOIN messages rm ON rm.id = m.reply_to_message_id
		LEFT JOIN users ru ON ru.id = rm.sender_id
		LEFT JOIN users su ON su.id = m.sender_id
		WHERE m.conversation_id = $1
		  AND (m.expires_at IS NULL OR m.expires_at > NOW())
		  AND NOT EXISTS (
		      SELECT 1 FROM message_hides mh
		      WHERE mh.message_id = m.id AND mh.user_id = $2
		  )`
	// $1 = conversationID, $2 = currentUserID (used in message_hides subquery).
	// Q-filter and pagination are appended dynamically after.
	args := []any{conversationID, currentUserID}
	if q != "" {
		query += fmt.Sprintf(` AND m.text ILIKE $%d`, len(args)+1)
		args = append(args, "%"+q+"%")
	}
	query += ` ORDER BY m.created_at ASC LIMIT $` +
		fmt.Sprintf("%d", len(args)+1) + ` OFFSET $` +
		fmt.Sprintf("%d", len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}
	defer rows.Close()

	var msgs []ChatMessage
	for rows.Next() {
		var msg ChatMessage
		var (
			postID       *string
			duration     *int
			waveform     []float64
			replyToID    *string
			caption      *string
			mediaURLs    []string
			mediaTypes   []string
			thumb        *string
			authorName   *string
			authorAvatar *string
			// reply preview cols — все nullable если оригинал удалён.
			rID       *string
			rSenderID *string
			rUsername string
			rText     string
			rKind     *string
		)
		if err := rows.Scan(
			&msg.ID, &msg.ConversationID, &msg.SenderID,
			&msg.Text, &msg.IsRead, &msg.IsDelivered,
			&msg.DeliveredCount, &msg.ReadCount, &msg.RecipientsCount,
			&msg.CreatedAt, &msg.ExpiresAt,
			&msg.Kind, &postID, &msg.AttachedMediaURL, &msg.AttachedMediaType,
			&duration, &waveform, &replyToID,
			&caption, &mediaURLs, &mediaTypes, &thumb,
			&authorName, &authorAvatar,
			&rID, &rSenderID, &rUsername, &rText, &rKind,
			&msg.SenderName, &msg.SenderUsername, &msg.SenderAvatarURL,
			&msg.IsDeletedForAll,
		); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		msg.IsMe = msg.SenderID == currentUserID
		// Mask content of deleted-for-all messages so clients can't read them.
		if msg.IsDeletedForAll {
			msg.Text = ""
			msg.Kind = "deleted"
			postID = nil
			msg.AttachedMediaURL = ""
			msg.AttachedMediaType = ""
			msg.ReplyTo = nil
			replyToID = nil
		}
		msg.AttachedPostID = postID
		if duration != nil {
			msg.MediaDurationSeconds = *duration
		}
		msg.Waveform = waveform
		msg.ReplyToMessageID = replyToID
		if rID != nil && replyToID != nil {
			rk := ""
			if rKind != nil {
				rk = *rKind
			}
			rs := ""
			if rSenderID != nil {
				rs = *rSenderID
			}
			msg.ReplyTo = &ReplyPreview{
				ID:             *rID,
				SenderID:       rs,
				SenderUsername: rUsername,
				Text:           rText,
				Kind:           rk,
			}
		}
		if postID != nil {
			ap := &AttachedPostShort{ID: *postID}
			if caption != nil {
				ap.Caption = *caption
			}
			if len(mediaURLs) > 0 {
				ap.MediaURL = mediaURLs[0]
			}
			if len(mediaTypes) > 0 {
				ap.MediaType = mediaTypes[0]
			}
			if thumb != nil {
				ap.Thumbnail = *thumb
			}
			if authorName != nil {
				ap.Author = *authorName
			}
			if authorAvatar != nil {
				ap.AuthorAvatar = *authorAvatar
			}
			msg.AttachedPost = ap
		}
		msgs = append(msgs, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Hydrate reactions in one extra query — cheaper than per-message join.
	if err := r.attachReactions(ctx, msgs, currentUserID); err != nil {
		// Reactions are non-critical UX — log via panic-free path: caller
		// won't notice missing reactions but message text still arrives.
		return msgs, nil
	}
	return msgs, nil
}

// SendMessageInput collects optional attachments. Pass empty for plain text.
type SendMessageInput struct {
	Text                 string
	AttachedPostID       *string
	AttachedMediaURL     string
	AttachedMediaType    string // "image", "audio" (voice), etc.
	MediaDurationSeconds int       // for voice — recorder's elapsed
	Waveform             []float64 // for voice — precomputed normalized samples 0..1
	ReplyToMessageID     *string   // если это reply на другое сообщение
	// RecipientIDs — все participants кроме sender'а. Чат-сервис вычисляет
	// до вызова (он же делает block-check). Repo INSERT'ит для каждого
	// строку в `message_recipients` (CHAT-10.2). Если пусто — никаких
	// recipients (sender — единственный participant; не должно случаться
	// в normal flow).
	RecipientIDs []string
	// ExpiresInSeconds — TTL для disappearing-сообщения (CHAT-11). 0 = вечно.
	// Repo при INSERT'е считает absolute expires_at = NOW() + interval.
	ExpiresInSeconds int
}

// SendMessage inserts a new message and returns it.
// Kind is derived from input:
//   - non-empty AttachedPostID                     → "shared_post"
//   - AttachedMediaType == "audio"                 → "voice"
//   - non-empty AttachedMediaURL (anything else)   → "image"
//   - otherwise                                    → "text"
//
// Attached post preview is loaded after insert so the response is self-contained.
func (r *ChatRepository) SendMessage(ctx context.Context, conversationID, senderID string, input SendMessageInput) (ChatMessage, error) {
	kind := "text"
	if input.AttachedPostID != nil && *input.AttachedPostID != "" {
		kind = "shared_post"
	} else if input.AttachedMediaURL != "" {
		// Audio → voice-message kind. Иные media (image, video) → image kind
		// для backwards-совместимости с существующими bubble-render'ами.
		if input.AttachedMediaType == "audio" {
			kind = "voice"
		} else {
			kind = "image"
			if input.AttachedMediaType == "" {
				input.AttachedMediaType = "image"
			}
		}
	}
	// Waveform отправляем в БД как JSONB. nil-array → NULL в БД (через
	// nullable интерфейс на pgx — пустой slice трактуем как «нет данных»).
	var waveformParam any
	if len(input.Waveform) > 0 {
		waveformParam = input.Waveform
	}
	var durParam any
	if input.MediaDurationSeconds > 0 {
		durParam = input.MediaDurationSeconds
	}
	// Reply-to: только валидное message-id в той же conversation проходит
	// FK-constraint. Если кривое — INSERT упадёт → 500.
	var replyParam any
	if input.ReplyToMessageID != nil && *input.ReplyToMessageID != "" {
		replyParam = *input.ReplyToMessageID
	}
	// Tx: INSERT message + INSERT message_recipients атомарно. Иначе при
	// сбое recipients-INSERT'а получили бы message без recipient-строк и
	// счётчики в group остались бы кривыми навсегда (CHAT-10.2).
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return ChatMessage{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var msg ChatMessage
	var waveformOut []float64
	var durOut *int
	var replyOut *string
	// CHAT-11: TTL → expires_at. NULL = вечно.
	var expiresParam any
	if input.ExpiresInSeconds > 0 {
		expiresParam = fmt.Sprintf("%d seconds", input.ExpiresInSeconds)
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO messages (conversation_id, sender_id, text, kind, attached_post_id,
		                      attached_media_url, attached_media_type,
		                      media_duration_seconds, waveform, reply_to_message_id,
		                      expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
		        CASE WHEN $11::text IS NULL THEN NULL ELSE NOW() + $11::interval END)
		RETURNING id, conversation_id, sender_id, text, is_read,
		          (delivered_at IS NOT NULL) AS is_delivered,
		          created_at, expires_at, kind,
		          attached_post_id, attached_media_url, attached_media_type,
		          media_duration_seconds, waveform, reply_to_message_id`,
		conversationID, senderID, input.Text, kind, input.AttachedPostID,
		input.AttachedMediaURL, input.AttachedMediaType,
		durParam, waveformParam, replyParam, expiresParam,
	).Scan(&msg.ID, &msg.ConversationID, &msg.SenderID, &msg.Text,
		&msg.IsRead, &msg.IsDelivered, &msg.CreatedAt, &msg.ExpiresAt,
		&msg.Kind, &msg.AttachedPostID, &msg.AttachedMediaURL, &msg.AttachedMediaType,
		&durOut, &waveformOut, &replyOut)
	if err != nil {
		return ChatMessage{}, fmt.Errorf("send message: %w", err)
	}

	// INSERT recipients (один на не-sender'а). CopyFrom батчем — быстро
	// даже для group на 100 участников.
	if len(input.RecipientIDs) > 0 {
		rcpRows := make([][]any, len(input.RecipientIDs))
		for i, uid := range input.RecipientIDs {
			rcpRows[i] = []any{msg.ID, uid}
		}
		if _, err := tx.CopyFrom(ctx,
			pgx.Identifier{"message_recipients"},
			[]string{"message_id", "user_id"},
			pgx.CopyFromRows(rcpRows),
		); err != nil {
			return ChatMessage{}, fmt.Errorf("insert recipients: %w", err)
		}
	}

	if _, err := tx.Exec(ctx,
		`UPDATE conversations SET updated_at = NOW() WHERE id = $1`,
		conversationID,
	); err != nil {
		return ChatMessage{}, fmt.Errorf("touch conversation: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return ChatMessage{}, fmt.Errorf("commit: %w", err)
	}

	if durOut != nil {
		msg.MediaDurationSeconds = *durOut
	}
	msg.Waveform = waveformOut
	msg.ReplyToMessageID = replyOut
	msg.IsMe = true
	msg.RecipientsCount = len(input.RecipientIDs)
	// DeliveredCount / ReadCount уже 0 (zero-value int).

	// Eager-load reply preview если есть — чтобы response совпадал с GetMessages.
	if replyOut != nil && *replyOut != "" {
		preview, e := r.fetchReplyPreview(ctx, *replyOut)
		if e == nil && preview != nil {
			msg.ReplyTo = preview
		}
	}

	// Eager-load the attached post preview so the response matches GetMessages shape.
	if msg.AttachedPostID != nil {
		ap := &AttachedPostShort{ID: *msg.AttachedPostID}
		var (
			caption      *string
			mediaURLs    []string
			mediaTypes   []string
			thumb        *string
			authorName   *string
			authorAvatar *string
		)
		err := r.db.Pool.QueryRow(ctx, `
			SELECT p.caption, p.media_urls, p.media_types, p.thumbnail_url,
			       u.username, u.avatar_url
			FROM posts p JOIN users u ON u.id = p.user_id
			WHERE p.id = $1`, *msg.AttachedPostID,
		).Scan(&caption, &mediaURLs, &mediaTypes, &thumb, &authorName, &authorAvatar)
		if err == nil {
			if caption != nil {
				ap.Caption = *caption
			}
			if len(mediaURLs) > 0 {
				ap.MediaURL = mediaURLs[0]
			}
			if len(mediaTypes) > 0 {
				ap.MediaType = mediaTypes[0]
			}
			if thumb != nil {
				ap.Thumbnail = *thumb
			}
			if authorName != nil {
				ap.Author = *authorName
			}
			if authorAvatar != nil {
				ap.AuthorAvatar = *authorAvatar
			}
			msg.AttachedPost = ap
		}
	}

	return msg, nil
}

// PurgeExpired удаляет все expired-сообщения (CHAT-11). Janitor в
// cmd/api/main.go вызывает раз в 60 сек. Идёт через partial-index
// `idx_messages_expires_at`. Возвращает count удалённых для logging.
//
// Note: message_recipients и reactions удаляются автоматически через
// ON DELETE CASCADE. Pinned-message reference в conversations.pinned_message_id
// (если когда-то закреплён) обнулится через ON DELETE SET NULL.
func (r *ChatRepository) PurgeExpired(ctx context.Context) (int64, error) {
	tag, err := r.db.Pool.Exec(ctx,
		`DELETE FROM messages WHERE expires_at IS NOT NULL AND expires_at <= NOW()`)
	if err != nil {
		return 0, fmt.Errorf("purge expired: %w", err)
	}
	return tag.RowsAffected(), nil
}

// MessageCounts — per-recipient счётчики для одного сообщения (CHAT-10.2).
// Frontend рисует «X из N прочитали» в group-bubble используя их.
type MessageCounts struct {
	DeliveredCount  int
	ReadCount       int
	RecipientsCount int
}

// MarkRecipientDelivered помечает delivered_at для одного recipient'а
// конкретного сообщения. Idempotent: NULL-guard → повторный вызов no-op.
// Возвращает (changed, counts, err):
//   - changed=true только при первой реальной проставке для этого
//     (message_id, user_id). Caller использует флаг чтобы решать «слать ли
//     `chat.delivered` к sender'у» (no-op event не нужен).
//   - counts — post-update свежие. Frontend получает их в payload event'а.
//
// При первой проставке также UPDATE'ит messages.delivered_at (legacy
// single-state для backward-compat).
func (r *ChatRepository) MarkRecipientDelivered(
	ctx context.Context, messageID, userID string,
) (bool, MessageCounts, error) {
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE message_recipients SET delivered_at = NOW()
		WHERE message_id = $1 AND user_id = $2 AND delivered_at IS NULL`,
		messageID, userID)
	if err != nil {
		return false, MessageCounts{}, fmt.Errorf("mark recipient delivered: %w", err)
	}
	changed := tag.RowsAffected() == 1
	if changed {
		_, _ = r.db.Pool.Exec(ctx, `
			UPDATE messages SET delivered_at = NOW()
			WHERE id = $1 AND delivered_at IS NULL`,
			messageID)
	}
	counts, err := r.getMessageCounts(ctx, messageID)
	if err != nil {
		return changed, MessageCounts{}, err
	}
	return changed, counts, nil
}

// getMessageCounts — aggregate counts из message_recipients для одного
// сообщения. Возвращает нули если recipient-строк нет.
func (r *ChatRepository) getMessageCounts(
	ctx context.Context, messageID string,
) (MessageCounts, error) {
	var c MessageCounts
	err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(delivered_at), COUNT(read_at)
		FROM message_recipients
		WHERE message_id = $1`,
		messageID,
	).Scan(&c.RecipientsCount, &c.DeliveredCount, &c.ReadCount)
	if err != nil {
		return MessageCounts{}, fmt.Errorf("get counts: %w", err)
	}
	return c, nil
}

// UndeliveredMessage — для CHAT-10.3 (late-delivered replay). На WS Register
// hook сканируем все undelivered recipient-строки этого user'а и помечаем
// их + эмиттим `chat.delivered` к sender'ам.
type UndeliveredMessage struct {
	MessageID      string
	SenderID       string
	ConversationID string
}

// GetUndeliveredForUser — сообщения с recipient = userID, ещё не помеченные
// delivered_at. Идёт через partial-index `idx_message_recipients_undelivered_user`.
// `limit` важен: на edge-case (юзер offline неделю) могут быть тысячи rows;
// принимаем до limit за один replay-цикл, последующие при следующих
// reconnect'ах (или REST refresh открытого чата).
func (r *ChatRepository) GetUndeliveredForUser(
	ctx context.Context, userID string, limit int,
) ([]UndeliveredMessage, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT m.id, m.sender_id, m.conversation_id
		FROM message_recipients mr
		JOIN messages m ON m.id = mr.message_id
		WHERE mr.user_id = $1 AND mr.delivered_at IS NULL
		ORDER BY m.created_at ASC
		LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get undelivered: %w", err)
	}
	defer rows.Close()
	var out []UndeliveredMessage
	for rows.Next() {
		var u UndeliveredMessage
		if err := rows.Scan(&u.MessageID, &u.SenderID, &u.ConversationID); err != nil {
			return nil, fmt.Errorf("scan undelivered: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// ReadFlip — один (message_id, sender_id) → counts переход в read-state.
// Сервис группирует by sender_id и эмиттит `chat.read` per-sender с
// message_ids+counts (CHAT-10.2).
type ReadFlip struct {
	MessageID string
	SenderID  string
	Counts    MessageCounts
}

// MarkRead помечает read_at для всех unread сообщений conversation'а
// которые НЕ отправлены этим userID. Также UPDATE'ит legacy messages.is_read
// для backward compat. Возвращает list of (message_id, sender_id, counts)
// которые реально перешли в read-state — caller эмиттит per-sender events.
func (r *ChatRepository) MarkRead(
	ctx context.Context, conversationID, userID string,
) ([]ReadFlip, error) {
	// Шаг 1: помечаем recipients (RETURNING чтобы знать какие message_id
	// флипнулись). Только те которые ещё не read.
	rows, err := r.db.Pool.Query(ctx, `
		UPDATE message_recipients mr
		SET read_at = NOW()
		FROM messages m
		WHERE mr.message_id = m.id
		  AND mr.user_id = $1
		  AND mr.read_at IS NULL
		  AND m.conversation_id = $2
		  AND m.sender_id != $1
		RETURNING m.id, m.sender_id`,
		userID, conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("mark recipients read: %w", err)
	}
	type pair struct{ msgID, senderID string }
	var pairs []pair
	for rows.Next() {
		var p pair
		if err := rows.Scan(&p.msgID, &p.senderID); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan read flip: %w", err)
		}
		pairs = append(pairs, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Шаг 2: legacy messages.is_read для backward compat.
	if _, err := r.db.Pool.Exec(ctx, `
		UPDATE messages SET is_read = true
		WHERE conversation_id = $1 AND sender_id != $2 AND is_read = false`,
		conversationID, userID,
	); err != nil {
		return nil, fmt.Errorf("mark messages is_read: %w", err)
	}

	// Шаг 3: для каждого affected message — counts. Это N запросов, но
	// pairs обычно ≤ unread-count (1-10 в нормальном кейсе, не тысячи).
	// Если станет узким местом — заменить на single aggregate-query с
	// массивом message_id'шников.
	flips := make([]ReadFlip, 0, len(pairs))
	for _, p := range pairs {
		counts, err := r.getMessageCounts(ctx, p.msgID)
		if err != nil {
			// одна ошибка не должна потопить весь батч — caller получит
			// частичный список и эмиттит что есть.
			continue
		}
		flips = append(flips, ReadFlip{
			MessageID: p.msgID,
			SenderID:  p.senderID,
			Counts:    counts,
		})
	}
	return flips, nil
}

// IsParticipant checks whether a user belongs to a conversation.
func (r *ChatRepository) IsParticipant(ctx context.Context, conversationID, userID string) (bool, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM conversation_participants WHERE conversation_id = $1 AND user_id = $2)`,
		conversationID, userID).Scan(&exists)
	return exists, err
}

// GetOtherParticipants returns IDs of every participant in the conversation
// except the caller. Used to enforce block-checks before sending messages.
func (r *ChatRepository) GetOtherParticipants(ctx context.Context, conversationID, exceptUserID string) ([]string, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT user_id FROM conversation_participants
		 WHERE conversation_id = $1 AND user_id <> $2`,
		conversationID, exceptUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

// MessageMeta is the minimal shape we need before reacting/unreacting:
// who owns the conversation so we can authorize + fan out WS events.
type MessageMeta struct {
	MessageID      string
	ConversationID string
}

func (r *ChatRepository) GetMessageMeta(ctx context.Context, messageID string) (*MessageMeta, error) {
	m := &MessageMeta{MessageID: messageID}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT conversation_id FROM messages WHERE id = $1`,
		messageID,
	).Scan(&m.ConversationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return m, nil
}

// GetMessageSender returns sender_id and conversation_id of a message — нужно
// для DELETE authorization (только автор может удалять собственное сообщение).
func (r *ChatRepository) GetMessageSender(ctx context.Context, messageID string) (senderID, conversationID string, err error) {
	err = r.db.Pool.QueryRow(ctx,
		`SELECT sender_id, conversation_id FROM messages WHERE id = $1`,
		messageID,
	).Scan(&senderID, &conversationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", domain.ErrNotFound
		}
		return "", "", err
	}
	return senderID, conversationID, nil
}

// DeleteMessage помечает сообщение как удалённое для всех (soft delete).
// Контент скрывается от всех participants, kind становится "deleted".
// Hard DELETE больше не используется для ручного удаления — только janitor
// удаляет истёкшие (expires_at) сообщения физически.
func (r *ChatRepository) DeleteMessage(ctx context.Context, messageID string) error {
	tag, err := r.db.Pool.Exec(ctx,
		`UPDATE messages
		 SET is_deleted_for_all = TRUE,
		     kind = 'deleted',
		     text = '',
		     attached_post_id = NULL,
		     attached_media_url = '',
		     attached_media_type = '',
		     reply_to_message_id = NULL
		 WHERE id = $1`, messageID)
	if err != nil {
		return fmt.Errorf("delete message: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// EditMessage updates text for an existing message and returns the updated row.
// Authorization is enforced by sender_id + conversation_id in the WHERE clause.
func (r *ChatRepository) EditMessage(ctx context.Context, conversationID, messageID, senderID, text string) (ChatMessage, error) {
	var msg ChatMessage
	var duration *int
	err := r.db.Pool.QueryRow(ctx, `
		UPDATE messages
		SET text = $4
		WHERE id = $1
		  AND conversation_id = $2
		  AND sender_id = $3
		  AND kind = 'text'
		  AND is_deleted_for_all = FALSE
		RETURNING id, conversation_id, sender_id, text, is_read,
		          (delivered_at IS NOT NULL) AS is_delivered,
		          created_at, expires_at, kind,
		          attached_post_id, attached_media_url, attached_media_type,
		          media_duration_seconds, waveform, reply_to_message_id`,
		messageID, conversationID, senderID, text,
	).Scan(
		&msg.ID, &msg.ConversationID, &msg.SenderID, &msg.Text,
		&msg.IsRead, &msg.IsDelivered, &msg.CreatedAt, &msg.ExpiresAt,
		&msg.Kind, &msg.AttachedPostID, &msg.AttachedMediaURL,
		&msg.AttachedMediaType, &duration, &msg.Waveform,
		&msg.ReplyToMessageID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ChatMessage{}, domain.ErrNotFound
		}
		return ChatMessage{}, fmt.Errorf("edit message: %w", err)
	}
	if duration != nil {
		msg.MediaDurationSeconds = *duration
	}
	msg.IsMe = true
	counts, err := r.getMessageCounts(ctx, msg.ID)
	if err == nil {
		msg.DeliveredCount = counts.DeliveredCount
		msg.ReadCount = counts.ReadCount
		msg.RecipientsCount = counts.RecipientsCount
		msg.IsDelivered = counts.DeliveredCount > 0
		msg.IsRead = counts.ReadCount > 0
	}
	if msg.ReplyToMessageID != nil {
		msg.ReplyTo, _ = r.fetchReplyPreview(ctx, *msg.ReplyToMessageID)
	}
	return msg, nil
}

// HideMessageForUser скрывает сообщение только для конкретного пользователя
// (DELETE FOR SELF). Остальные участники продолжают видеть сообщение.
func (r *ChatRepository) HideMessageForUser(ctx context.Context, messageID, userID string) error {
	// Verify message exists first.
	var exists bool
	err := r.db.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM messages WHERE id = $1)`, messageID,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check message: %w", err)
	}
	if !exists {
		return domain.ErrNotFound
	}
	_, err = r.db.Pool.Exec(ctx,
		`INSERT INTO message_hides (message_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		messageID, userID)
	if err != nil {
		return fmt.Errorf("hide message: %w", err)
	}
	return nil
}

// GetMessageCreatedAt returns the created_at of a message — нужно для
// проверки 1-часового ограничения на «удалить для всех».
func (r *ChatRepository) GetMessageCreatedAt(ctx context.Context, messageID string) (time.Time, error) {
	var t time.Time
	err := r.db.Pool.QueryRow(ctx,
		`SELECT created_at FROM messages WHERE id = $1`, messageID,
	).Scan(&t)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, domain.ErrNotFound
		}
		return time.Time{}, err
	}
	return t, nil
}

// SetReaction upserts the user's reaction on a message — one emoji per user
// per message, replaced on conflict.
func (r *ChatRepository) SetReaction(ctx context.Context, messageID, userID, emoji string) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO message_reactions (message_id, user_id, emoji)
		VALUES ($1, $2, $3)
		ON CONFLICT (message_id, user_id)
		DO UPDATE SET emoji = EXCLUDED.emoji, created_at = NOW()`,
		messageID, userID, emoji)
	return err
}

// RemoveReaction deletes the user's reaction. No-op if it didn't exist.
func (r *ChatRepository) RemoveReaction(ctx context.Context, messageID, userID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM message_reactions WHERE message_id = $1 AND user_id = $2`,
		messageID, userID)
	return err
}

// CountReactions returns aggregated counts for one message — used by the
// realtime push so peers can update without an extra GET.
func (r *ChatRepository) CountReactions(ctx context.Context, messageID string) (map[string]int, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT emoji, COUNT(*) FROM message_reactions
		 WHERE message_id = $1 GROUP BY emoji`,
		messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var emoji string
		var n int
		if err := rows.Scan(&emoji, &n); err != nil {
			return nil, err
		}
		out[emoji] = n
	}
	return out, nil
}

// attachReactions hydrates Reactions/MyReaction on each message in `msgs`.
// Single roundtrip even for batches.
func (r *ChatRepository) attachReactions(ctx context.Context, msgs []ChatMessage, currentUserID string) error {
	if len(msgs) == 0 {
		return nil
	}
	ids := make([]string, len(msgs))
	for i, m := range msgs {
		ids[i] = m.ID
	}
	rows, err := r.db.Pool.Query(ctx,
		`SELECT message_id, emoji, user_id
		 FROM message_reactions
		 WHERE message_id = ANY($1::uuid[])`,
		ids)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Build per-message aggregates. Two passes over result set is wasteful;
	// instead bucket as we scan.
	type bucket struct {
		counts map[string]int
		mine   string
	}
	buckets := make(map[string]*bucket, len(msgs))
	for rows.Next() {
		var msgID, emoji, userID string
		if err := rows.Scan(&msgID, &emoji, &userID); err != nil {
			return err
		}
		b, ok := buckets[msgID]
		if !ok {
			b = &bucket{counts: map[string]int{}}
			buckets[msgID] = b
		}
		b.counts[emoji]++
		if userID == currentUserID {
			b.mine = emoji
		}
	}
	for i := range msgs {
		if b, ok := buckets[msgs[i].ID]; ok {
			msgs[i].Reactions = b.counts
			msgs[i].MyReaction = b.mine
		}
	}
	return nil
}

// fetchReplyPreview — мини-SELECT для reply-bubble preview'я. Возвращает
// nil, nil если оригинал удалён (ON DELETE SET NULL → FK ⊆ ничто).
func (r *ChatRepository) fetchReplyPreview(ctx context.Context, messageID string) (*ReplyPreview, error) {
	var p ReplyPreview
	err := r.db.Pool.QueryRow(ctx, `
		SELECT m.id, m.sender_id, COALESCE(u.username, ''), COALESCE(m.text, ''), m.kind
		FROM messages m
		LEFT JOIN users u ON u.id = m.sender_id
		WHERE m.id = $1`, messageID,
	).Scan(&p.ID, &p.SenderID, &p.SenderUsername, &p.Text, &p.Kind)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("fetch reply preview: %w", err)
	}
	return &p, nil
}

// ───────────────────────── Group chats ─────────────────────────

// CreateGroupConversation создаёт group-чат с creator'ом как admin'ом и
// дополнительными memberIDs как обычными участниками. Returns conv ID.
// Дубликаты в memberIDs / совпадение с creatorID — игнорируются.
func (r *ChatRepository) CreateGroupConversation(
	ctx context.Context,
	creatorID, title, coverURL string,
	memberIDs []string,
) (string, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var convID string
	err = tx.QueryRow(ctx, `
		INSERT INTO conversations (kind, title, cover_url, created_by)
		VALUES ('group', $1, $2, $3)
		RETURNING id`,
		title, coverURL, creatorID,
	).Scan(&convID)
	if err != nil {
		return "", fmt.Errorf("create group conversation: %w", err)
	}

	// Creator → admin
	if _, err := tx.Exec(ctx, `
		INSERT INTO conversation_participants (conversation_id, user_id, role)
		VALUES ($1, $2, 'admin')`,
		convID, creatorID,
	); err != nil {
		return "", fmt.Errorf("add creator: %w", err)
	}

	// Members → member; skip duplicates and the creator itself.
	seen := map[string]struct{}{creatorID: {}}
	for _, m := range memberIDs {
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		if _, err := tx.Exec(ctx, `
			INSERT INTO conversation_participants (conversation_id, user_id, role)
			VALUES ($1, $2, 'member')
			ON CONFLICT DO NOTHING`,
			convID, m,
		); err != nil {
			return "", fmt.Errorf("add member %s: %w", m, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}
	return convID, nil
}

// GetParticipants returns the full participant list with role and join time.
func (r *ChatRepository) GetParticipants(ctx context.Context, conversationID string) ([]GroupParticipant, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT u.id, u.username, u.full_name, u.bio, u.avatar_url, u.website, u.gender,
		       u.device_public_id, u.is_private, u.is_verified,
		       u.posts_count, u.followers_count, u.following_count, u.created_at,
		       cp.role, cp.joined_at
		FROM conversation_participants cp
		JOIN users u ON u.id = cp.user_id
		WHERE cp.conversation_id = $1
		ORDER BY (cp.role = 'admin') DESC, cp.joined_at ASC`,
		conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("get participants: %w", err)
	}
	defer rows.Close()

	var out []GroupParticipant
	for rows.Next() {
		u := &ChatUser{}
		var p GroupParticipant
		if err := rows.Scan(
			&u.ID, &u.Username, &u.FullName, &u.Bio, &u.AvatarURL, &u.Website, &u.Gender,
			&u.DevicePublicID, &u.IsPrivate, &u.IsVerified,
			&u.PostsCount, &u.FollowersCount, &u.FollowingCount, &u.CreatedAt,
			&p.Role, &p.JoinedAt,
		); err != nil {
			return nil, fmt.Errorf("scan participant: %w", err)
		}
		p.User = u
		out = append(out, p)
	}
	return out, rows.Err()
}

// IsAdmin возвращает true если user — admin данного group-чата.
// Direct'ы не имеют админов — для них всегда false.
func (r *ChatRepository) IsAdmin(ctx context.Context, conversationID, userID string) (bool, error) {
	var role string
	err := r.db.Pool.QueryRow(ctx, `
		SELECT cp.role
		FROM conversation_participants cp
		JOIN conversations c ON c.id = cp.conversation_id
		WHERE cp.conversation_id = $1 AND cp.user_id = $2 AND c.kind = 'group'`,
		conversationID, userID,
	).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("is admin: %w", err)
	}
	return role == "admin", nil
}

// GetConversationKind — quick check для service'а.
func (r *ChatRepository) GetConversationKind(ctx context.Context, conversationID string) (string, error) {
	var kind string
	err := r.db.Pool.QueryRow(ctx,
		`SELECT kind FROM conversations WHERE id = $1`, conversationID,
	).Scan(&kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get kind: %w", err)
	}
	return kind, nil
}

// AddParticipant добавляет user'а к group-чату как member. Если уже есть —
// no-op (ON CONFLICT DO NOTHING).
func (r *ChatRepository) AddParticipant(ctx context.Context, conversationID, userID string) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO conversation_participants (conversation_id, user_id, role)
		VALUES ($1, $2, 'member')
		ON CONFLICT DO NOTHING`,
		conversationID, userID,
	)
	if err != nil {
		return fmt.Errorf("add participant: %w", err)
	}
	return nil
}

// RemoveParticipant удаляет user'а из чата. Не запрещает удалять последнего
// admin'а — это правило enforce'ит service-уровень.
func (r *ChatRepository) RemoveParticipant(ctx context.Context, conversationID, userID string) error {
	_, err := r.db.Pool.Exec(ctx, `
		DELETE FROM conversation_participants
		WHERE conversation_id = $1 AND user_id = $2`,
		conversationID, userID,
	)
	if err != nil {
		return fmt.Errorf("remove participant: %w", err)
	}
	return nil
}

// CountAdmins — сколько admin'ов в group-чате. Используется last-admin-
// защитой в Remove/ChangeRole, чтобы не оставлять чат без управления.
func (r *ChatRepository) CountAdmins(ctx context.Context, conversationID string) (int, error) {
	var n int
	err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM conversation_participants
		WHERE conversation_id = $1 AND role = 'admin'`,
		conversationID,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count admins: %w", err)
	}
	return n, nil
}

// UpdateParticipantRole — установить роль (admin/member) участника группы.
// Никаких CHECK здесь — service валидирует input до вызова.
func (r *ChatRepository) UpdateParticipantRole(ctx context.Context, conversationID, userID, role string) error {
	res, err := r.db.Pool.Exec(ctx, `
		UPDATE conversation_participants SET role = $1
		WHERE conversation_id = $2 AND user_id = $3`,
		role, conversationID, userID,
	)
	if err != nil {
		return fmt.Errorf("update role: %w", err)
	}
	if res.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// SetPinnedMessage — установить или снять закреплённое сообщение.
// Если messageID == nil или "" → unpin (NULL). Иначе требуется чтобы
// message принадлежал той же conversation (service-уровень validation).
func (r *ChatRepository) SetPinnedMessage(ctx context.Context, conversationID string, messageID *string) error {
	var param any
	if messageID != nil && *messageID != "" {
		param = *messageID
	}
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE conversations SET pinned_message_id = $1, updated_at = NOW()
		WHERE id = $2`,
		param, conversationID,
	)
	if err != nil {
		return fmt.Errorf("set pinned: %w", err)
	}
	return nil
}

// MessageBelongsToConversation проверяет, что message-id реально живёт в
// conversation — anti-cross-conv-pin.
func (r *ChatRepository) MessageBelongsToConversation(ctx context.Context, messageID, conversationID string) (bool, error) {
	var n int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM messages WHERE id = $1 AND conversation_id = $2`,
		messageID, conversationID,
	).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("check message-conv: %w", err)
	}
	return n > 0, nil
}

// UpdateGroupMeta — обновление title/coverURL group-чата.
func (r *ChatRepository) UpdateGroupMeta(ctx context.Context, conversationID, title, coverURL string) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE conversations SET title = $1, cover_url = $2, updated_at = NOW()
		WHERE id = $3 AND kind = 'group'`,
		title, coverURL, conversationID,
	)
	if err != nil {
		return fmt.Errorf("update group: %w", err)
	}
	return nil
}

// DeleteConversation полностью удаляет conversation и все связанные данные
// (participants, messages). Используется при отмене/удалении сбора.
func (r *ChatRepository) DeleteConversation(ctx context.Context, conversationID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM conversations WHERE id = $1`,
		conversationID,
	)
	return err
}

// TogglePinConversation переключает закреплённость чата для конкретного юзера.
// Возвращает новое состояние is_pinned (true = теперь закреплён).
func (r *ChatRepository) TogglePinConversation(ctx context.Context, conversationID, userID string) (bool, error) {
	var newPinned bool
	err := r.db.Pool.QueryRow(ctx, `
		UPDATE conversation_participants
		SET pinned_at = CASE WHEN pinned_at IS NULL THEN NOW() ELSE NULL END
		WHERE conversation_id = $1 AND user_id = $2
		RETURNING pinned_at IS NOT NULL`,
		conversationID, userID,
	).Scan(&newPinned)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, domain.ErrNotFound
	}
	return newPinned, err
}

// SetConversationArchived архивирует или разархивирует чат для пользователя.
func (r *ChatRepository) SetConversationArchived(ctx context.Context, conversationID, userID string, archived bool) error {
	var err error
	if archived {
		_, err = r.db.Pool.Exec(ctx, `
			UPDATE conversation_participants SET archived_at = NOW()
			WHERE conversation_id = $1 AND user_id = $2`,
			conversationID, userID,
		)
	} else {
		_, err = r.db.Pool.Exec(ctx, `
			UPDATE conversation_participants SET archived_at = NULL
			WHERE conversation_id = $1 AND user_id = $2`,
			conversationID, userID,
		)
	}
	return err
}

// SetConversationMuted включает или отключает уведомления для чата у пользователя.
func (r *ChatRepository) SetConversationMuted(ctx context.Context, conversationID, userID string, muted bool) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE conversation_participants SET muted = $3
		WHERE conversation_id = $1 AND user_id = $2`,
		conversationID, userID, muted,
	)
	return err
}

// HideConversationForUser скрывает чат из списка текущего пользователя
// (delete for self). Устанавливает hidden_at = NOW() в его participant-строке.
// Чат и сообщения не удаляются — другой участник видит их без изменений.
func (r *ChatRepository) HideConversationForUser(ctx context.Context, conversationID, userID string) error {
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE conversation_participants
		SET hidden_at = NOW()
		WHERE conversation_id = $1 AND user_id = $2`,
		conversationID, userID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
