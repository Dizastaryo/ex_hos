package service

import (
	"context"
	"fmt"
	"time"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	"github.com/seeu/backend/internal/ws"
	"go.uber.org/zap"
)

type ChatService struct {
	chatRepo  *postgres.ChatRepository
	sborRepo  *postgres.SborRepository
	blockRepo *postgres.BlockRepository
	wsHub     *ws.Hub
	logger    *zap.Logger
}

func NewChatService(
	chatRepo *postgres.ChatRepository,
	sborRepo *postgres.SborRepository,
	blockRepo *postgres.BlockRepository,
	wsHub *ws.Hub,
	logger *zap.Logger,
) *ChatService {
	return &ChatService{
		chatRepo:  chatRepo,
		sborRepo:  sborRepo,
		blockRepo: blockRepo,
		wsHub:     wsHub,
		logger:    logger,
	}
}

// ReplayUndeliveredFor (CHAT-10.3): когда юзер появился online после
// offline-периода, сканируем все его undelivered messages, помечаем
// recipient-row delivered_at + эмиттим `chat.delivered` к sender'у с
// актуальными counts. Лимит ≤200 за один replay-цикл; больше — следующий
// reconnect / REST refresh открытого чата подхватит.
//
// Wired в `cmd/api/main.go::hub.RegisterHook`. Безопасно если userID
// неавторизован или conversation удалён — все ошибки логируются и пропускаются.
func (s *ChatService) ReplayUndeliveredFor(ctx context.Context, userID string) {
	const replayLimit = 200
	undelivered, err := s.chatRepo.GetUndeliveredForUser(ctx, userID, replayLimit)
	if err != nil {
		s.logger.Warn("late-delivered replay: scan failed",
			zap.String("user_id", userID), zap.Error(err))
		return
	}
	if len(undelivered) == 0 {
		return
	}
	s.logger.Debug("late-delivered replay starting",
		zap.String("user_id", userID), zap.Int("count", len(undelivered)))
	for _, m := range undelivered {
		changed, counts, mErr := s.chatRepo.MarkRecipientDelivered(ctx, m.MessageID, userID)
		if mErr != nil || !changed {
			continue
		}
		pushChatDelivered(s.wsHub, m.SenderID, m.ConversationID, m.MessageID, counts)
	}
}

func (s *ChatService) GetOrCreateConversation(ctx context.Context, userID1, userID2 string) (string, error) {
	if blocked, err := s.blockRepo.IsEitherBlocked(ctx, userID1, userID2); err != nil {
		return "", err
	} else if blocked {
		return "", domain.ErrForbidden
	}
	convID, err := s.chatRepo.GetOrCreateConversation(ctx, userID1, userID2)
	if err != nil {
		return "", fmt.Errorf("get or create conversation: %w", err)
	}
	return convID, nil
}

func (s *ChatService) GetConversations(ctx context.Context, userID string) ([]postgres.ChatConversation, error) {
	convs, err := s.chatRepo.GetConversations(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get conversations: %w", err)
	}
	return convs, nil
}

func (s *ChatService) GetMessages(ctx context.Context, conversationID, currentUserID string, limit, offset int, q string) ([]postgres.ChatMessage, error) {
	// Verify user is a participant
	ok, err := s.chatRepo.IsParticipant(ctx, conversationID, currentUserID)
	if err != nil {
		return nil, fmt.Errorf("check participant: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("not a participant")
	}

	msgs, err := s.chatRepo.GetMessages(ctx, conversationID, currentUserID, limit, offset, q)
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}
	return msgs, nil
}

func (s *ChatService) SendMessage(ctx context.Context, conversationID, senderID string, input postgres.SendMessageInput) (postgres.ChatMessage, error) {
	// Verify user is a participant.
	ok, err := s.chatRepo.IsParticipant(ctx, conversationID, senderID)
	if err != nil {
		return postgres.ChatMessage{}, fmt.Errorf("check participant: %w", err)
	}
	if !ok {
		return postgres.ChatMessage{}, fmt.Errorf("not a participant")
	}

	// Refuse send if any other participant is blocked by/blocking the sender.
	others, err := s.chatRepo.GetOtherParticipants(ctx, conversationID, senderID)
	if err == nil {
		for _, peerID := range others {
			if blocked, _ := s.blockRepo.IsEitherBlocked(ctx, senderID, peerID); blocked {
				return postgres.ChatMessage{}, domain.ErrForbidden
			}
		}
	}

	// Передаём recipients в repo — там в одной транзакции INSERT message +
	// INSERT message_recipients (CHAT-10.2). Без этого counts всегда = 0.
	input.RecipientIDs = others

	msg, err := s.chatRepo.SendMessage(ctx, conversationID, senderID, input)
	if err != nil {
		return postgres.ChatMessage{}, fmt.Errorf("send message: %w", err)
	}

	// Realtime fan-out + per-peer delivered (CHAT-10.2). Для каждого
	// online peer'а отдельно помечаем его recipient-строку и эмиттим
	// `chat.delivered` к sender'у с актуальными counts. Latest counts
	// в payload позволяют frontend'у показать «X из N доставлено».
	var latestCounts postgres.MessageCounts
	gotCounts := false
	for _, peerID := range others {
		pushChatMessage(s.wsHub, peerID, msg)
		if s.wsHub == nil || !s.wsHub.IsOnline(peerID) {
			continue
		}
		changed, counts, mErr := s.chatRepo.MarkRecipientDelivered(ctx, msg.ID, peerID)
		if mErr != nil {
			continue
		}
		if changed {
			gotCounts = true
			latestCounts = counts
		}
	}
	if gotCounts {
		msg.IsDelivered = true
		msg.DeliveredCount = latestCounts.DeliveredCount
		msg.ReadCount = latestCounts.ReadCount
		msg.RecipientsCount = latestCounts.RecipientsCount
		pushChatDelivered(s.wsHub, senderID, conversationID, msg.ID, latestCounts)
	}

	return msg, nil
}

// EditMessage updates a sender-owned text message and returns the updated row.
func (s *ChatService) EditMessage(
	ctx context.Context,
	conversationID, messageID, callerID, text string,
) (postgres.ChatMessage, error) {
	ok, err := s.chatRepo.IsParticipant(ctx, conversationID, callerID)
	if err != nil {
		return postgres.ChatMessage{}, err
	}
	if !ok {
		return postgres.ChatMessage{}, domain.ErrForbidden
	}

	senderID, actualConversationID, err := s.chatRepo.GetMessageSender(ctx, messageID)
	if err != nil {
		return postgres.ChatMessage{}, err
	}
	if actualConversationID != conversationID || senderID != callerID {
		return postgres.ChatMessage{}, domain.ErrForbidden
	}

	msg, err := s.chatRepo.EditMessage(ctx, conversationID, messageID, callerID, text)
	if err != nil {
		return postgres.ChatMessage{}, err
	}
	return msg, nil
}

// SetReaction upserts the caller's emoji on a message. Returns the updated
// per-emoji count map so the handler can pass it to the WS push without a
// second roundtrip.
func (s *ChatService) SetReaction(ctx context.Context, messageID, userID, emoji string) (map[string]int, string, error) {
	meta, err := s.chatRepo.GetMessageMeta(ctx, messageID)
	if err != nil {
		return nil, "", err
	}
	ok, err := s.chatRepo.IsParticipant(ctx, meta.ConversationID, userID)
	if err != nil {
		return nil, "", fmt.Errorf("check participant: %w", err)
	}
	if !ok {
		return nil, "", domain.ErrForbidden
	}
	if err := s.chatRepo.SetReaction(ctx, messageID, userID, emoji); err != nil {
		return nil, "", fmt.Errorf("set reaction: %w", err)
	}
	counts, err := s.chatRepo.CountReactions(ctx, messageID)
	if err != nil {
		s.logger.Warn("count reactions after set",
			zap.String("message_id", messageID), zap.Error(err))
		counts = map[string]int{}
	}
	// Push update to other participants. Sender is included via REST response.
	if others, err := s.chatRepo.GetOtherParticipants(ctx, meta.ConversationID, userID); err == nil {
		for _, peerID := range others {
			pushChatReaction(s.wsHub, peerID, meta.ConversationID, messageID, counts, "")
		}
	}
	return counts, emoji, nil
}

// RemoveReaction deletes the caller's reaction. Same fan-out semantics.
func (s *ChatService) RemoveReaction(ctx context.Context, messageID, userID string) (map[string]int, error) {
	meta, err := s.chatRepo.GetMessageMeta(ctx, messageID)
	if err != nil {
		return nil, err
	}
	ok, err := s.chatRepo.IsParticipant(ctx, meta.ConversationID, userID)
	if err != nil {
		return nil, fmt.Errorf("check participant: %w", err)
	}
	if !ok {
		return nil, domain.ErrForbidden
	}
	if err := s.chatRepo.RemoveReaction(ctx, messageID, userID); err != nil {
		return nil, fmt.Errorf("remove reaction: %w", err)
	}
	counts, err := s.chatRepo.CountReactions(ctx, messageID)
	if err != nil {
		s.logger.Warn("count reactions after remove",
			zap.String("message_id", messageID), zap.Error(err))
		counts = map[string]int{}
	}
	if others, err := s.chatRepo.GetOtherParticipants(ctx, meta.ConversationID, userID); err == nil {
		for _, peerID := range others {
			pushChatReaction(s.wsHub, peerID, meta.ConversationID, messageID, counts, "")
		}
	}
	return counts, nil
}

// CreateGroup создаёт group-чат с creator'ом как admin'ом и memberIDs как
// обычными участниками. Проверяет block'и: если creator <-> любой member в
// блок-листе, возвращает ErrForbidden. После создания шлёт WS-event
// chat.member.joined всем участникам кроме creator'а (creator получит chat
// в обычном refresh'е чат-листа).
func (s *ChatService) CreateGroup(
	ctx context.Context,
	creatorID, title string,
	coverURL string,
	memberIDs []string,
) (string, error) {
	if title == "" {
		return "", domain.ErrInvalidInput
	}
	// Block-проверка: creator не может пригласить заблокировавшего/заблокированного.
	for _, m := range memberIDs {
		if m == creatorID {
			continue
		}
		blocked, err := s.blockRepo.IsEitherBlocked(ctx, creatorID, m)
		if err != nil {
			return "", fmt.Errorf("block check: %w", err)
		}
		if blocked {
			return "", domain.ErrForbidden
		}
	}
	convID, err := s.chatRepo.CreateGroupConversation(ctx, creatorID, title, coverURL, memberIDs)
	if err != nil {
		return "", fmt.Errorf("create group: %w", err)
	}
	// Fan-out: все participants должны узнать о новом чате через WS, чтобы
	// chat-list refresh'ился без явного pull'а.
	for _, m := range memberIDs {
		if m == creatorID {
			continue
		}
		pushChatGroupJoined(s.wsHub, m, convID, creatorID)
	}
	return convID, nil
}

// AddGroupMember — admin-only действие.
func (s *ChatService) AddGroupMember(
	ctx context.Context,
	conversationID, callerID, newMemberID string,
) error {
	kind, err := s.chatRepo.GetConversationKind(ctx, conversationID)
	if err != nil {
		return err
	}
	if kind != "group" {
		return domain.ErrInvalidInput
	}
	isAdmin, err := s.chatRepo.IsAdmin(ctx, conversationID, callerID)
	if err != nil {
		return err
	}
	if !isAdmin {
		return domain.ErrForbidden
	}
	if blocked, _ := s.blockRepo.IsEitherBlocked(ctx, callerID, newMemberID); blocked {
		return domain.ErrForbidden
	}
	if err := s.chatRepo.AddParticipant(ctx, conversationID, newMemberID); err != nil {
		return err
	}
	pushChatGroupJoined(s.wsHub, newMemberID, conversationID, callerID)
	// Также уведомляем существующих participants о новом юзере.
	if others, err := s.chatRepo.GetOtherParticipants(ctx, conversationID, newMemberID); err == nil {
		for _, peer := range others {
			pushChatGroupMemberAdded(s.wsHub, peer, conversationID, newMemberID)
		}
	}
	return nil
}

// RemoveGroupMember: admin может убрать любого; member может убрать только
// себя (leave). Last-admin-protection: если admin пытается убрать ДРУГОГО
// единственного admin'а — запрещается (ErrLastAdmin). Self-leave всегда разрешён.
func (s *ChatService) RemoveGroupMember(
	ctx context.Context,
	conversationID, callerID, targetID string,
) error {
	kind, err := s.chatRepo.GetConversationKind(ctx, conversationID)
	if err != nil {
		return err
	}
	if kind != "group" {
		return domain.ErrInvalidInput
	}
	if callerID != targetID {
		isAdmin, err := s.chatRepo.IsAdmin(ctx, conversationID, callerID)
		if err != nil {
			return err
		}
		if !isAdmin {
			return domain.ErrForbidden
		}
	}
	// Last-admin guard: только для случая когда admin удаляет ДРУГОГО участника.
	// Self-leave (выход из своей группы) всегда разрешён — иначе пользователь
	// не сможет покинуть чат, а frontend будет получать ложный успех (200 OK)
	// без фактического удаления из conversation_participants.
	if callerID != targetID {
		targetIsAdmin, err := s.chatRepo.IsAdmin(ctx, conversationID, targetID)
		if err != nil {
			return err
		}
		if targetIsAdmin {
			count, err := s.chatRepo.CountAdmins(ctx, conversationID)
			if err != nil {
				return err
			}
			if count <= 1 {
				return domain.ErrLastAdmin
			}
		}
	}
	// Снимем target'а до отправки WS — иначе он получит уведомление о собственном leave'е.
	others, _ := s.chatRepo.GetOtherParticipants(ctx, conversationID, targetID)
	if err := s.chatRepo.RemoveParticipant(ctx, conversationID, targetID); err != nil {
		return err
	}
	for _, peer := range others {
		pushChatGroupMemberRemoved(s.wsHub, peer, conversationID, targetID)
	}

	// Bidirectional sync: если это чат сбора — удаляем targetID из сбора тоже.
	// Хост сбора (organizer) не может быть удалён из сбора через чат (у него
	// роль 'organizer', sborRepo.Leave вернёт ErrNotJoined — игнорируем).
	if s.sborRepo != nil {
		sborID, hostID, sErr := s.sborRepo.GetSborByChatID(ctx, conversationID)
		if sErr == nil && sborID != "" && targetID != hostID {
			if lErr := s.sborRepo.Leave(ctx, sborID, targetID); lErr != nil {
				s.logger.Warn("sync chat leave to sbor failed",
					zap.String("chat_id", conversationID),
					zap.String("sbor_id", sborID),
					zap.String("user_id", targetID),
					zap.Error(lErr))
			}
		}
	}
	return nil
}

// ChangeMemberRole — promote/demote. Admin-only. Запрещено демоутить
// единственного admin'а (нужно сначала кого-то promote'нуть).
func (s *ChatService) ChangeMemberRole(
	ctx context.Context,
	conversationID, callerID, targetID, newRole string,
) error {
	if newRole != "admin" && newRole != "member" {
		return domain.ErrInvalidInput
	}
	kind, err := s.chatRepo.GetConversationKind(ctx, conversationID)
	if err != nil {
		return err
	}
	if kind != "group" {
		return domain.ErrInvalidInput
	}
	isAdmin, err := s.chatRepo.IsAdmin(ctx, conversationID, callerID)
	if err != nil {
		return err
	}
	if !isAdmin {
		return domain.ErrForbidden
	}
	// Demote'ить можно только если есть другой admin.
	if newRole == "member" {
		targetWasAdmin, err := s.chatRepo.IsAdmin(ctx, conversationID, targetID)
		if err != nil {
			return err
		}
		if targetWasAdmin {
			count, err := s.chatRepo.CountAdmins(ctx, conversationID)
			if err != nil {
				return err
			}
			if count <= 1 {
				return domain.ErrLastAdmin
			}
		}
	}
	if err := s.chatRepo.UpdateParticipantRole(ctx, conversationID, targetID, newRole); err != nil {
		return err
	}
	// Fan-out на всех participants (включая target) — у них обновится role-pill.
	if peers, err := s.chatRepo.GetOtherParticipants(ctx, conversationID, ""); err == nil {
		for _, peer := range peers {
			pushChatGroupRoleChanged(s.wsHub, peer, conversationID, targetID, newRole)
		}
	}
	return nil
}

// UpdateGroup — admin-only. Синхронизирует изменения в сбор (если чат
// привязан к сбору) и рассылает WS-событие всем участникам.
func (s *ChatService) UpdateGroup(
	ctx context.Context,
	conversationID, callerID, title, coverURL string,
) error {
	kind, err := s.chatRepo.GetConversationKind(ctx, conversationID)
	if err != nil {
		return err
	}
	if kind != "group" {
		return domain.ErrInvalidInput
	}
	isAdmin, err := s.chatRepo.IsAdmin(ctx, conversationID, callerID)
	if err != nil {
		return err
	}
	if !isAdmin {
		return domain.ErrForbidden
	}
	if err := s.chatRepo.UpdateGroupMeta(ctx, conversationID, title, coverURL); err != nil {
		return err
	}
	// Bidirectional sync: если чат привязан к сбору — обновляем сбор тоже.
	if s.sborRepo != nil {
		if err := s.sborRepo.UpdateByChatID(ctx, conversationID, title, coverURL); err != nil {
			s.logger.Warn("sync chat update to sbor failed",
				zap.String("chat_id", conversationID), zap.Error(err))
		}
	}
	// WS fan-out: все участники получат обновлённый title/cover без рефетча.
	if peers, err := s.chatRepo.GetOtherParticipants(ctx, conversationID, ""); err == nil {
		for _, peer := range peers {
			pushChatGroupUpdated(s.wsHub, peer, conversationID, title, coverURL)
		}
	}
	return nil
}

// PinMessage / UnpinMessage:
//  - direct: любой participant.
//  - group: admin-only.
//  - сообщение должно быть из этой же conversation (anti-spoof).
// messageID == nil ИЛИ "" → unpin.
func (s *ChatService) PinMessage(
	ctx context.Context,
	conversationID, callerID string,
	messageID *string,
) error {
	kind, err := s.chatRepo.GetConversationKind(ctx, conversationID)
	if err != nil {
		return err
	}
	ok, err := s.chatRepo.IsParticipant(ctx, conversationID, callerID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	if kind == "group" {
		isAdmin, err := s.chatRepo.IsAdmin(ctx, conversationID, callerID)
		if err != nil {
			return err
		}
		if !isAdmin {
			return domain.ErrForbidden
		}
	}
	if messageID != nil && *messageID != "" {
		belongs, err := s.chatRepo.MessageBelongsToConversation(ctx, *messageID, conversationID)
		if err != nil {
			return err
		}
		if !belongs {
			return domain.ErrInvalidInput
		}
	}
	if err := s.chatRepo.SetPinnedMessage(ctx, conversationID, messageID); err != nil {
		return err
	}
	// Fan-out: все participants должны обновить sticky-banner.
	if peers, err := s.chatRepo.GetOtherParticipants(ctx, conversationID, ""); err == nil {
		for _, peer := range peers {
			pushChatPinned(s.wsHub, peer, conversationID, messageID)
		}
	}
	return nil
}

// DeleteMessage помечает сообщение как удалённое для всех (WhatsApp-стиль).
// Permission: только автор + сообщение не старше 24 часов.
// Fan-out WS-event chat.message.deleted — все participants обновляют
// локальный state (показывают «Сообщение удалено»).
func (s *ChatService) DeleteMessage(
	ctx context.Context,
	messageID, callerID string,
) error {
	senderID, conversationID, err := s.chatRepo.GetMessageSender(ctx, messageID)
	if err != nil {
		return err
	}
	if senderID != callerID {
		return domain.ErrForbidden
	}
	// Проверяем возраст: «удалить для всех» доступно только в первые 24 часа.
	createdAt, err := s.chatRepo.GetMessageCreatedAt(ctx, messageID)
	if err != nil {
		return err
	}
	if time.Since(createdAt) > 24*time.Hour {
		return domain.ErrForbidden
	}
	if err := s.chatRepo.DeleteMessage(ctx, messageID); err != nil {
		return err
	}
	// Fan-out всем участникам, включая автора (чтобы синхронизировалось
	// на других девайсах того же юзера).
	peers, err := s.chatRepo.GetOtherParticipants(ctx, conversationID, "")
	if err == nil {
		for _, peer := range peers {
			pushChatMessageDeleted(s.wsHub, peer, conversationID, messageID)
		}
	}
	return nil
}

func (s *ChatService) DeleteMessageInConversation(
	ctx context.Context,
	conversationID, messageID, callerID string,
) error {
	belongs, err := s.chatRepo.MessageBelongsToConversation(ctx, messageID, conversationID)
	if err != nil {
		return err
	}
	if !belongs {
		return domain.ErrNotFound
	}
	return s.DeleteMessage(ctx, messageID, callerID)
}

// TogglePinConversation закрепляет/открепляет чат у callerID.
// Возвращает новое состояние isPinned.
func (s *ChatService) TogglePinConversation(ctx context.Context, conversationID, callerID string) (bool, error) {
	ok, err := s.chatRepo.IsParticipant(ctx, conversationID, callerID)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, domain.ErrForbidden
	}
	return s.chatRepo.TogglePinConversation(ctx, conversationID, callerID)
}

// ArchiveConversation архивирует или разархивирует чат у callerID.
func (s *ChatService) ArchiveConversation(ctx context.Context, conversationID, callerID string, archived bool) error {
	ok, err := s.chatRepo.IsParticipant(ctx, conversationID, callerID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return s.chatRepo.SetConversationArchived(ctx, conversationID, callerID, archived)
}

// MuteConversation включает или отключает уведомления для чата у callerID.
func (s *ChatService) MuteConversation(ctx context.Context, conversationID, callerID string, muted bool) error {
	ok, err := s.chatRepo.IsParticipant(ctx, conversationID, callerID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return s.chatRepo.SetConversationMuted(ctx, conversationID, callerID, muted)
}

// HideConversation скрывает чат из списка callerID (delete for self).
// Для group — проверяет что caller является участником. Чат не удаляется.
func (s *ChatService) HideConversation(ctx context.Context, conversationID, callerID string) error {
	ok, err := s.chatRepo.IsParticipant(ctx, conversationID, callerID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return s.chatRepo.HideConversationForUser(ctx, conversationID, callerID)
}

// HideMessage скрывает сообщение только для callerID (delete for self).
// Доступно для любого участника чата, для любого сообщения в любое время.
func (s *ChatService) HideMessage(
	ctx context.Context,
	messageID, callerID string,
) error {
	// Verify caller is a participant of the conversation that contains this message.
	_, conversationID, err := s.chatRepo.GetMessageSender(ctx, messageID)
	if err != nil {
		return err
	}
	ok, err := s.chatRepo.IsParticipant(ctx, conversationID, callerID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return s.chatRepo.HideMessageForUser(ctx, messageID, callerID)
}

// GetGroupParticipants — любой участник чата может видеть список.
func (s *ChatService) GetGroupParticipants(
	ctx context.Context,
	conversationID, callerID string,
) ([]postgres.GroupParticipant, error) {
	ok, err := s.chatRepo.IsParticipant(ctx, conversationID, callerID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrForbidden
	}
	return s.chatRepo.GetParticipants(ctx, conversationID)
}

func (s *ChatService) MarkRead(ctx context.Context, conversationID, userID string) error {
	ok, err := s.chatRepo.IsParticipant(ctx, conversationID, userID)
	if err != nil {
		return fmt.Errorf("check participant: %w", err)
	}
	if !ok {
		return fmt.Errorf("not a participant")
	}

	// MarkRead возвращает per-message flips (msg_id, sender_id, counts).
	// Группируем по sender_id чтобы эмитить один chat.read event на каждого
	// уникального автора с его list of msg_ids + counts (CHAT-10.2).
	flips, err := s.chatRepo.MarkRead(ctx, conversationID, userID)
	if err != nil {
		return err
	}

	// Группировка by sender → его msgIDs + parallel counts list.
	type bySender struct {
		msgIDs        []string
		latestCounts  postgres.MessageCounts
		countsByMsgID map[string]postgres.MessageCounts
	}
	groups := map[string]*bySender{}
	for _, f := range flips {
		g, ok := groups[f.SenderID]
		if !ok {
			g = &bySender{countsByMsgID: map[string]postgres.MessageCounts{}}
			groups[f.SenderID] = g
		}
		g.msgIDs = append(g.msgIDs, f.MessageID)
		g.latestCounts = f.Counts
		g.countsByMsgID[f.MessageID] = f.Counts
	}
	for senderID, g := range groups {
		pushChatRead(s.wsHub, senderID, conversationID, userID, g.msgIDs, g.countsByMsgID)
	}
	return nil
}
