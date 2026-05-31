package handler

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/repository/postgres"
	"github.com/seeu/backend/internal/service"
	"go.uber.org/zap"
)

type ChatHandler struct {
	chatService *service.ChatService
	logger      *zap.Logger
}

func NewChatHandler(chatService *service.ChatService, logger *zap.Logger) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		logger:      logger,
	}
}

// ListChats godoc
// GET /api/v1/chats
func (h *ChatHandler) ListChats(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	convs, err := h.chatService.GetConversations(c.Context(), userID)
	if err != nil {
		h.logger.Error("list chats", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get conversations")
	}

	if convs == nil {
		return respondSuccess(c, fiber.StatusOK, []struct{}{}, nil)
	}

	return respondSuccess(c, fiber.StatusOK, convs, nil)
}

// CreateChat godoc
// POST /api/v1/chats
//
// Direct: body = {"user_id": "uuid"}                         → kind="direct"
// Group:  body = {"kind":"group", "title":"X", "cover_url":"...", "member_ids":["..."]}
func (h *ChatHandler) CreateChat(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req struct {
		Kind        string   `json:"kind"`
		OtherUserID string   `json:"user_id"`
		Title       string   `json:"title"`
		CoverURL    string   `json:"cover_url"`
		MemberIDs   []string `json:"member_ids"`
	}
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}

	if req.Kind == "group" {
		if req.Title == "" {
			return respondError(c, fiber.StatusBadRequest, "title is required for group")
		}
		if len(req.MemberIDs) == 0 {
			return respondError(c, fiber.StatusBadRequest, "member_ids must contain at least 1 user besides creator")
		}
		convID, err := h.chatService.CreateGroup(c.Context(), userID, req.Title, req.CoverURL, req.MemberIDs)
		if err != nil {
			if errors.Is(err, domain.ErrForbidden) {
				return respondError(c, fiber.StatusForbidden, "cannot create group with blocked user")
			}
			if errors.Is(err, domain.ErrInvalidInput) {
				return respondError(c, fiber.StatusBadRequest, "invalid group input")
			}
			h.logger.Error("create group", zap.Error(err))
			return respondError(c, fiber.StatusInternalServerError, "failed to create group")
		}
		return respondSuccess(c, fiber.StatusOK, fiber.Map{"id": convID, "kind": "group"}, nil)
	}

	// Default: direct chat
	if req.OtherUserID == "" {
		return respondError(c, fiber.StatusBadRequest, "user_id is required")
	}
	if req.OtherUserID == userID {
		return respondError(c, fiber.StatusBadRequest, "cannot create chat with yourself")
	}

	convID, err := h.chatService.GetOrCreateConversation(c.Context(), userID, req.OtherUserID)
	if err != nil {
		h.logger.Error("create chat", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to create conversation")
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{"id": convID, "kind": "direct"}, nil)
}

// GetGroupMembers godoc
// GET /api/v1/chats/:id/members
func (h *ChatHandler) GetGroupMembers(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	chatID := c.Params("id")
	parts, err := h.chatService.GetGroupParticipants(c.Context(), chatID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			return respondError(c, fiber.StatusForbidden, "not a participant")
		}
		h.logger.Error("get group members", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get members")
	}
	return respondSuccess(c, fiber.StatusOK, parts, nil)
}

// AddGroupMember godoc
// POST /api/v1/chats/:id/members  body: {"user_id":"uuid"}
func (h *ChatHandler) AddGroupMember(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	chatID := c.Params("id")
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := c.BodyParser(&req); err != nil || req.UserID == "" {
		return respondError(c, fiber.StatusBadRequest, "user_id is required")
	}
	if err := h.chatService.AddGroupMember(c.Context(), chatID, userID, req.UserID); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			return respondError(c, fiber.StatusForbidden, "admin only or blocked")
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			return respondError(c, fiber.StatusBadRequest, "not a group chat")
		}
		h.logger.Error("add group member", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to add member")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}

// RemoveGroupMember godoc
// DELETE /api/v1/chats/:id/members/:user_id
func (h *ChatHandler) RemoveGroupMember(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	chatID := c.Params("id")
	target := c.Params("user_id")
	if target == "" {
		return respondError(c, fiber.StatusBadRequest, "user_id is required")
	}
	if err := h.chatService.RemoveGroupMember(c.Context(), chatID, userID, target); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			return respondError(c, fiber.StatusForbidden, "admin only")
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			return respondError(c, fiber.StatusBadRequest, "not a group chat")
		}
		if errors.Is(err, domain.ErrLastAdmin) {
			return respondError(c, fiber.StatusConflict,
				"нельзя удалить единственного админа группы — сначала назначьте другого")
		}
		h.logger.Error("remove group member", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to remove member")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}

// LeaveGroup — текущий юзер покидает group-чат.
// DELETE /api/v1/chats/:id/leave
func (h *ChatHandler) LeaveGroup(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	chatID := c.Params("id")
	if err := h.chatService.RemoveGroupMember(c.Context(), chatID, userID, userID); err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			return respondError(c, fiber.StatusBadRequest, "not a group chat")
		}
		// Игнорируем ErrLastAdmin — пользователь сам себя удаляет, разрешаем
		h.logger.Warn("leave group", zap.String("chat_id", chatID), zap.String("user_id", userID), zap.Error(err))
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}

// PinMessage godoc
// PUT /api/v1/chats/:id/pin  body: {"message_id":"uuid"} | {"message_id":null}
func (h *ChatHandler) PinMessage(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	chatID := c.Params("id")
	var req struct {
		MessageID *string `json:"message_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid body")
	}
	if err := h.chatService.PinMessage(c.Context(), chatID, userID, req.MessageID); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			return respondError(c, fiber.StatusForbidden, "не разрешено закреплять")
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			return respondError(c, fiber.StatusBadRequest,
				"сообщение не принадлежит этому чату")
		}
		h.logger.Error("pin message", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}

// DeleteMessage godoc
// DELETE /api/v1/chat-messages/:id?scope=all|self
//
// scope=all (default): мягкое удаление для всех — только автор + в первый час.
//   Все участники видят «Сообщение удалено» вместо содержимого.
// scope=self: скрыть только для себя — любой участник, любое время.
func (h *ChatHandler) DeleteMessage(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	messageID := c.Params("id")
	scope := c.Query("scope", "all")

	if scope == "self" {
		if err := h.chatService.HideMessage(c.Context(), messageID, userID); err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return respondError(c, fiber.StatusNotFound, "message not found")
			}
			if errors.Is(err, domain.ErrForbidden) {
				return respondError(c, fiber.StatusForbidden, "not a participant")
			}
			h.logger.Error("hide message", zap.Error(err))
			return respondError(c, fiber.StatusInternalServerError, "failed")
		}
		return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
	}

	// scope=all
	if err := h.chatService.DeleteMessage(c.Context(), messageID, userID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return respondError(c, fiber.StatusNotFound, "message not found")
		}
		if errors.Is(err, domain.ErrForbidden) {
			return respondError(c, fiber.StatusForbidden, "только автор может удалить сообщение в первый час")
		}
		h.logger.Error("delete message", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}

// ChangeMemberRole godoc
// PUT /api/v1/chats/:id/members/:user_id/role  body: {"role":"admin"|"member"}
func (h *ChatHandler) ChangeMemberRole(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	chatID := c.Params("id")
	target := c.Params("user_id")
	if target == "" {
		return respondError(c, fiber.StatusBadRequest, "user_id is required")
	}
	var req struct {
		Role string `json:"role"`
	}
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid body")
	}
	if err := h.chatService.ChangeMemberRole(c.Context(), chatID, userID, target, req.Role); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			return respondError(c, fiber.StatusForbidden, "admin only")
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			return respondError(c, fiber.StatusBadRequest, "invalid role")
		}
		if errors.Is(err, domain.ErrLastAdmin) {
			return respondError(c, fiber.StatusConflict,
				"нельзя демоутить единственного админа группы")
		}
		h.logger.Error("change member role", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}

// UpdateGroup godoc
// PUT /api/v1/chats/:id  body: {"title":"...", "cover_url":"..."}
func (h *ChatHandler) UpdateGroup(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	chatID := c.Params("id")
	var req struct {
		Title    string `json:"title"`
		CoverURL string `json:"cover_url"`
	}
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid body")
	}
	if err := h.chatService.UpdateGroup(c.Context(), chatID, userID, req.Title, req.CoverURL); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			return respondError(c, fiber.StatusForbidden, "admin only")
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			return respondError(c, fiber.StatusBadRequest, "not a group chat")
		}
		h.logger.Error("update group", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to update group")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}

// GetMessages godoc
// GET /api/v1/chats/:id/messages?q=<search>
// q — optional full-text search по полю text (ILIKE substring, case-insensitive).
// Возвращает messages в обычном порядке (created_at ASC) — фронт сам найдёт
// match'и и подсветит / прокрутит к ним.
func (h *ChatHandler) GetMessages(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	chatID := c.Params("id")

	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	q := strings.TrimSpace(c.Query("q", ""))

	msgs, err := h.chatService.GetMessages(c.Context(), chatID, userID, limit, offset, q)
	if err != nil {
		if strings.Contains(err.Error(), "not a participant") {
			return respondError(c, fiber.StatusForbidden, "not a participant of this conversation")
		}
		h.logger.Error("get messages", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get messages")
	}

	if msgs == nil {
		return respondSuccess(c, fiber.StatusOK, []struct{}{}, nil)
	}

	return respondSuccess(c, fiber.StatusOK, msgs, nil)
}

// SendMessage godoc
// POST /api/v1/chats/:id/messages
//   body: {
//     "text": "...",
//     "attached_post_id":   "uuid-or-omit",
//     "attached_media_url": "/uploads/...",
//     "attached_media_type": "image"
//   }
//
// Discriminator: post wins over media; media wins over plain text.
// Text может быть пустым если есть любое вложение.
func (h *ChatHandler) SendMessage(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	chatID := c.Params("id")

	var req struct {
		Text                 string    `json:"text"`
		AttachedPostID       *string   `json:"attached_post_id,omitempty"`
		AttachedMediaURL     string    `json:"attached_media_url,omitempty"`
		AttachedMediaType    string    `json:"attached_media_type,omitempty"`
		MediaDurationSeconds int       `json:"media_duration_seconds,omitempty"`
		Waveform             []float64 `json:"waveform,omitempty"`
		ReplyToMessageID     *string   `json:"reply_to_message_id,omitempty"`
		// CHAT-11: TTL в секундах. nil/0 = вечно. Допустимые значения
		// фронт-стороной — 3600/86400/604800 (1ч/24ч/7д), но бэк принимает
		// любое положительное int. Capping на 30 дней чтобы предотвратить
		// злоупотребление (DoS через миллионы long-lived rows).
		ExpiresInSeconds int `json:"expires_in_seconds,omitempty"`
	}
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	hasPost := req.AttachedPostID != nil && *req.AttachedPostID != ""
	hasMedia := req.AttachedMediaURL != ""
	if req.Text == "" && !hasPost && !hasMedia {
		return respondError(c, fiber.StatusBadRequest,
			"text, attached_post_id or attached_media_url is required")
	}

	// TTL clamp: max 30 days. <0 трактуем как «без TTL» вместо ошибки.
	const maxTTL = 30 * 24 * 3600
	if req.ExpiresInSeconds > maxTTL {
		req.ExpiresInSeconds = maxTTL
	}
	if req.ExpiresInSeconds < 0 {
		req.ExpiresInSeconds = 0
	}

	input := postgres.SendMessageInput{
		Text:                 req.Text,
		AttachedMediaURL:     req.AttachedMediaURL,
		AttachedMediaType:    req.AttachedMediaType,
		MediaDurationSeconds: req.MediaDurationSeconds,
		Waveform:             req.Waveform,
		ReplyToMessageID:     req.ReplyToMessageID,
		ExpiresInSeconds:     req.ExpiresInSeconds,
	}
	if hasPost {
		input.AttachedPostID = req.AttachedPostID
	}

	msg, err := h.chatService.SendMessage(c.Context(), chatID, userID, input)
	if err != nil {
		if strings.Contains(err.Error(), "not a participant") {
			return respondError(c, fiber.StatusForbidden, "not a participant of this conversation")
		}
		h.logger.Error("send message", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to send message")
	}

	return respondSuccess(c, fiber.StatusCreated, msg, nil)
}

// React godoc
// POST /api/v1/chat-messages/:id/react   body: {"emoji": "👍"}
func (h *ChatHandler) React(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	messageID := c.Params("id")

	var req struct {
		Emoji string `json:"emoji"`
	}
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	emoji := strings.TrimSpace(req.Emoji)
	if emoji == "" {
		return respondError(c, fiber.StatusBadRequest, "emoji is required")
	}
	// Cap at 8 bytes — emoji codepoints are typically 4 bytes (with VS16
	// modifier 7-8). Anything bigger is abuse.
	if len(emoji) > 16 {
		return respondError(c, fiber.StatusBadRequest, "emoji too long")
	}

	counts, mine, err := h.chatService.SetReaction(c.Context(), messageID, userID, emoji)
	if err != nil {
		if err == domain.ErrNotFound {
			return respondError(c, fiber.StatusNotFound, "message not found")
		}
		if err == domain.ErrForbidden {
			return respondError(c, fiber.StatusForbidden,
				"not a participant of this conversation")
		}
		h.logger.Error("react message", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to react")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"message_id":  messageID,
		"reactions":   counts,
		"my_reaction": mine,
	}, nil)
}

// Unreact godoc
// DELETE /api/v1/chat-messages/:id/react
func (h *ChatHandler) Unreact(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	messageID := c.Params("id")

	counts, err := h.chatService.RemoveReaction(c.Context(), messageID, userID)
	if err != nil {
		if err == domain.ErrNotFound {
			return respondError(c, fiber.StatusNotFound, "message not found")
		}
		if err == domain.ErrForbidden {
			return respondError(c, fiber.StatusForbidden,
				"not a participant of this conversation")
		}
		h.logger.Error("unreact message", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to unreact")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"message_id": messageID,
		"reactions":  counts,
	}, nil)
}

// TogglePinConversation закрепляет/открепляет чат у текущего пользователя.
// PUT /api/v1/chats/:id/user-pin
func (h *ChatHandler) TogglePinConversation(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	chatID := c.Params("id")

	isPinned, err := h.chatService.TogglePinConversation(c.Context(), chatID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) || errors.Is(err, domain.ErrNotFound) {
			return respondError(c, fiber.StatusForbidden, "not a participant")
		}
		h.logger.Error("toggle pin conversation", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to toggle pin")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"is_pinned": isPinned}, nil)
}

// HideConversation скрывает чат из списка текущего пользователя.
// DELETE /api/v1/chats/:id
func (h *ChatHandler) HideConversation(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	chatID := c.Params("id")

	if err := h.chatService.HideConversation(c.Context(), chatID, userID); err != nil {
		if errors.Is(err, domain.ErrForbidden) || errors.Is(err, domain.ErrNotFound) {
			return respondError(c, fiber.StatusForbidden, "not a participant")
		}
		h.logger.Error("hide conversation", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to hide chat")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "chat hidden"}, nil)
}

// MarkRead godoc
// PUT /api/v1/chats/:id/read
func (h *ChatHandler) MarkRead(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	chatID := c.Params("id")

	if err := h.chatService.MarkRead(c.Context(), chatID, userID); err != nil {
		if strings.Contains(err.Error(), "not a participant") {
			return respondError(c, fiber.StatusForbidden, "not a participant of this conversation")
		}
		h.logger.Error("mark read", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to mark as read")
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "marked as read"}, nil)
}
