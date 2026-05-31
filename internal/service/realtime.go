package service

import (
	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	"github.com/seeu/backend/internal/ws"
)

// flattenCounts помогает уложить MessageCounts в WS payload без лишних
// imports на стороне клиента. Не выделяем отдельный struct json'у —
// frontend читает плоско.
func flattenCounts(c postgres.MessageCounts) map[string]any {
	return map[string]any{
		"delivered_count":  c.DeliveredCount,
		"read_count":       c.ReadCount,
		"recipients_count": c.RecipientsCount,
	}
}

// pushNotif sends a realtime "notification" event to the recipient. Failures
// are silently ignored — the notification is already persisted to DB and will
// show up on next manual refresh. Hub may be nil during unit tests.
func pushNotif(hub *ws.Hub, n *domain.Notification) {
	if hub == nil || n == nil {
		return
	}
	hub.SendToUser(n.UserID, ws.MessageTypeNotification, n)
}

// pushChatMessage sends a realtime "chat.message" event to one peer.
// Frontend uses chat_id from the message to route into the right conversation.
func pushChatMessage(hub *ws.Hub, peerID string, msg postgres.ChatMessage) {
	if hub == nil || peerID == "" {
		return
	}
	// IsMe is "from the *original sender's* perspective" in the response — for
	// the recipient (peerID), it's always false. Server stamps it correctly per
	// participant in REST GET, but for WS push we rewrite for the recipient.
	msg.IsMe = false
	hub.SendToUser(peerID, "chat.message", msg)
}

// pushChatDelivered tells the *sender* that their message was just delivered
// to one more peer's WS. Payload включает per-recipient counts (CHAT-10.2):
// delivered_count / read_count / recipients_count — фронт рисует «X из N
// доставлено» в group-bubble.
func pushChatDelivered(
	hub *ws.Hub, senderID, chatID, messageID string,
	counts postgres.MessageCounts,
) {
	if hub == nil || senderID == "" {
		return
	}
	payload := map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
	}
	for k, v := range flattenCounts(counts) {
		payload[k] = v
	}
	hub.SendToUser(senderID, "chat.delivered", payload)
}

// pushChatRead notifies the *sender* that participant readerID just read
// specific messages (CHAT-10.2). msgIDs + countsByMsgID позволяют фронту
// обновить per-message read_count.
//
// Для backward compat сохраняем top-level `reader_id` — старые клиенты
// которые не умеют per-message counts всё равно флипают isRead=true на
// все собственные сообщения этого conversation'а.
func pushChatRead(
	hub *ws.Hub, peerID, chatID, readerID string,
	msgIDs []string, countsByMsgID map[string]postgres.MessageCounts,
) {
	if hub == nil || peerID == "" {
		return
	}
	// Преобразуем countsByMsgID в plain map<msg_id, flat-counts-map> для JSON.
	countsOut := make(map[string]any, len(countsByMsgID))
	for id, c := range countsByMsgID {
		countsOut[id] = flattenCounts(c)
	}
	hub.SendToUser(peerID, "chat.read", map[string]any{
		"chat_id":         chatID,
		"reader_id":       readerID,
		"message_ids":     msgIDs,
		"counts_by_msg":   countsOut,
	})
}

// pushChatGroupJoined уведомляет конкретного юзера что его добавили в group-
// чат (либо группа создана с ним внутри). Frontend получает event и тянет
// чат-лист через REST. Минимально: chat_id + actor (кто добавил/создал).
func pushChatGroupJoined(hub *ws.Hub, userID, chatID, actorID string) {
	if hub == nil || userID == "" {
		return
	}
	hub.SendToUser(userID, "chat.group.joined", map[string]any{
		"chat_id":  chatID,
		"actor_id": actorID,
	})
}

// pushChatGroupMemberAdded — для existing-participants: «в наш чат добавили X».
func pushChatGroupMemberAdded(hub *ws.Hub, userID, chatID, addedID string) {
	if hub == nil || userID == "" {
		return
	}
	hub.SendToUser(userID, "chat.group.member.added", map[string]any{
		"chat_id":    chatID,
		"member_id": addedID,
	})
}

// pushChatGroupRoleChanged — promote/demote target'а. Frontend перерисовывает
// admin-pill в members screen.
func pushChatGroupRoleChanged(hub *ws.Hub, userID, chatID, memberID, role string) {
	if hub == nil || userID == "" {
		return
	}
	hub.SendToUser(userID, "chat.group.member.role.changed", map[string]any{
		"chat_id":   chatID,
		"member_id": memberID,
		"role":      role,
	})
}

// pushChatPinned — у конкретного юзера обновляется sticky-banner закреплённого
// сообщения. messageID == "" → unpin (NULL).
func pushChatPinned(hub *ws.Hub, userID, chatID string, messageID *string) {
	if hub == nil || userID == "" {
		return
	}
	var mid any
	if messageID != nil {
		mid = *messageID
	}
	hub.SendToUser(userID, "chat.pinned", map[string]any{
		"chat_id":    chatID,
		"message_id": mid,
	})
}

// pushChatMessageDeleted — сообщение удалили автором; participant'ы должны
// убрать его из local state.
func pushChatMessageDeleted(hub *ws.Hub, userID, chatID, messageID string) {
	if hub == nil || userID == "" {
		return
	}
	hub.SendToUser(userID, "chat.message.deleted", map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
	})
}

// pushChatGroupUpdated — название или обложка группы изменились.
// Фронт обновляет title/coverUrl в чат-листе и заголовке чата без рефетча.
func pushChatGroupUpdated(hub *ws.Hub, userID, chatID, title, coverURL string) {
	if hub == nil || userID == "" {
		return
	}
	hub.SendToUser(userID, "chat.group.updated", map[string]any{
		"chat_id":   chatID,
		"title":     title,
		"cover_url": coverURL,
	})
}

// pushChatGroupMemberRemoved — для existing-participants: «из чата удалён X»
// (или X сам вышел).
func pushChatGroupMemberRemoved(hub *ws.Hub, userID, chatID, removedID string) {
	if hub == nil || userID == "" {
		return
	}
	hub.SendToUser(userID, "chat.group.member.removed", map[string]any{
		"chat_id":   chatID,
		"member_id": removedID,
	})
}

// pushChatReaction tells one peer that reactions on a message changed. We
// send the *full aggregated count map* (not delta) — simpler client logic
// and idempotent under reordering.
func pushChatReaction(hub *ws.Hub, peerID, chatID, messageID string,
	counts map[string]int, _ string) {
	if hub == nil || peerID == "" {
		return
	}
	hub.SendToUser(peerID, "chat.reaction", map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
		"reactions":  counts,
	})
}

// pushStoryViewAdded notifies the story author that someone viewed their
// story. Unicast to author only — viewers list and view-count are
// owner-private analytics. Frontend updates the open viewer badge without
// a refetch.
func pushStoryViewAdded(hub *ws.Hub, authorID, storyID string, viewsCount int) {
	if hub == nil || authorID == "" || storyID == "" {
		return
	}
	hub.SendToUser(authorID, "story.view.added", map[string]any{
		"story_id":    storyID,
		"views_count": viewsCount,
	})
}

// pushStoryReaction tells the story author that reactions on their story
// changed. Stories are author-private from an analytics perspective (only
// owner sees the viewer list and aggregated reactions), so we *unicast* to
// authorID rather than broadcasting like posts.
func pushStoryReaction(hub *ws.Hub, authorID, storyID string, counts map[string]int) {
	if hub == nil || authorID == "" || storyID == "" {
		return
	}
	hub.SendToUser(authorID, "story.reaction", map[string]any{
		"story_id":  storyID,
		"reactions": counts,
	})
}

// pushPostReaction broadcasts a post reactions update to ALL connected users.
// Posts are public — anyone viewing the post (Feed/Explore/Profile) needs to
// see the new count. Same payload shape as chat (full count map, idempotent).
//
// Broadcast cost is acceptable for now: hub is in-memory single-instance,
// fan-out is just a map lookup. When we move to Redis pubsub for multi-node
// we'll switch to per-room subscriptions (`post:<id>` channels).
func pushPostReaction(hub *ws.Hub, postID string, counts map[string]int) {
	if hub == nil || postID == "" {
		return
	}
	hub.Broadcast("post.reaction", map[string]any{
		"post_id":   postID,
		"reactions": counts,
	})
}
