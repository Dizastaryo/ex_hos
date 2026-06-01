package handler

import (
	"context"
	"encoding/json"
	"time"

	fiberws "github.com/gofiber/websocket/v2"
	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/repository/postgres"
	"github.com/seeu/backend/internal/ws"
	"go.uber.org/zap"
)

type WSHandler struct {
	hub        *ws.Hub
	chatRepo   *postgres.ChatRepository
	userRepo   *postgres.UserRepository
	callRepo   *postgres.CallRepository
	notifRepo  *postgres.NotificationRepository
	followRepo *postgres.FollowRepository
	audioRepo  *postgres.AudioRepository
	logger     *zap.Logger
}

func NewWSHandler(
	hub *ws.Hub,
	chatRepo *postgres.ChatRepository,
	userRepo *postgres.UserRepository,
	callRepo *postgres.CallRepository,
	notifRepo *postgres.NotificationRepository,
	followRepo *postgres.FollowRepository,
	audioRepo *postgres.AudioRepository,
	logger *zap.Logger,
) *WSHandler {
	// BUG-11: ранее `if h.callRepo == nil return` silently no-op'нул на
	// invalid configuration. В prod это могло скрыть missed dependency.
	// Здесь, на startup, явно warn'аем если кто-то передал nil — owner
	// видит проблему сразу в логах, а не через cascading effect позже.
	if callRepo == nil {
		logger.Warn("WSHandler: callRepo is nil — call persistence + history disabled")
	}
	if chatRepo == nil {
		logger.Warn("WSHandler: chatRepo is nil — typing fan-out + group calls disabled")
	}
	if notifRepo == nil {
		logger.Warn("WSHandler: notifRepo is nil — missed-call notifications disabled")
	}
	if followRepo == nil {
		logger.Warn("WSHandler: followRepo is nil — music.now_playing fan-out disabled")
	}
	if userRepo == nil {
		logger.Warn("WSHandler: userRepo is nil — call.invite payload enrichment disabled")
	}
	return &WSHandler{
		hub:        hub,
		chatRepo:   chatRepo,
		userRepo:   userRepo,
		callRepo:   callRepo,
		notifRepo:  notifRepo,
		followRepo: followRepo,
		audioRepo:  audioRepo,
		logger:     logger,
	}
}

// Handle handles WebSocket connections using the gofiber websocket adapter.
// This must be wrapped with fiberws.New() at the route level.
func (h *WSHandler) Handle(c *fiberws.Conn) {
	userID, _ := c.Locals(middleware.UserIDKey).(string)
	if userID == "" {
		c.Close()
		return
	}

	client := &ws.Client{
		UserID: userID,
		Send:   make(chan []byte, 256),
		Hub:    h.hub,
	}

	h.hub.Register(client)
	defer h.hub.Unregister(client)

	go h.writePump(client, c)
	h.readPump(client, c)
}

func (h *WSHandler) readPump(client *ws.Client, conn *fiberws.Conn) {
	defer conn.Close()

	// 128 KB — достаточно для SDP offer/answer (~2-5 KB) и ICE-кандидатов.
	// Раньше было 1024 → call.offer с SDP всегда превышало лимит → backend
	// закрывал WS с кодом 1009 "message too big" → звонки не работали.
	conn.SetReadLimit(128 * 1024)
	// BUG-24: read-deadline 95 секунд. writePump шлёт ping каждые 30 сек,
	// pong reset'ит deadline. 95s = 3 ping-cycles + buffer → выдерживаем 2
	// потерянных pong'а подряд прежде чем close (flaky-сети, mobile-roaming).
	// Раньше 60s = 1 missed = death, что обрывало звонки при кратком rebufer'е.
	const readDeadlineSec = 95
	conn.SetReadDeadline(time.Now().Add(readDeadlineSec * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(readDeadlineSec * time.Second))
		return nil
	})

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			h.logger.Debug("ws read error",
				zap.String("user_id", client.UserID),
				zap.Error(err))
			break
		}
		h.handleClientMessage(client, data)
	}
}

// handleClientMessage parses an upstream WS frame and routes it. Right now
// only `chat.typing` is recognised; everything else is ignored. Errors don't
// abort the connection — clients can keep sending.
func (h *WSHandler) handleClientMessage(client *ws.Client, data []byte) {
	var msg struct {
		Type    string         `json:"type"`
		Payload map[string]any `json:"payload"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}
	switch msg.Type {
	case "chat.typing":
		chatID, _ := msg.Payload["chat_id"].(string)
		if chatID == "" {
			return
		}
		h.fanOutTyping(client.UserID, chatID)
	// Video/voice call signaling. Backend ничего не знает про SDP/ICE —
	// просто форвардит payload target'у с подмешанным from_user_id. Все 6
	// событий обрабатываются единообразно.
	case "call.invite", "call.accept", "call.decline",
		"call.offer", "call.answer", "call.ice", "call.end":
		h.relayCallEvent(client.UserID, msg.Type, msg.Payload)
	// C-7: group call signaling. Three events: invite (start), join (accept),
	// leave (hangup). Backend смотрит chat_id, резолвит других participants
	// и fan-out'ит соответствующий «server-facing» event:
	//   call.group.invite → call.group.invite (всем members кроме sender'а)
	//   call.group.join   → call.group.member.joined (другим уже-active)
	//   call.group.leave  → call.group.member.left
	// Peer-to-peer signaling внутри mesh (offer/answer/ice) идёт через
	// существующий relayCallEvent с to_user_id.
	case "call.group.invite", "call.group.join", "call.group.leave":
		h.fanOutGroupCall(client.UserID, msg.Type, msg.Payload)
	// MUSIC-1: «сейчас звучит у друзей». Frontend audio_player_service шлёт
	// upstream при play start; мы fan-out'им к followers с обогащением
	// title/artist (фронт хочет рендерить без отдельного fetch'а).
	case "music.now_playing", "music.stopped":
		h.fanOutNowPlaying(client.UserID, msg.Type, msg.Payload)
	}
}

// fanOutNowPlaying (MUSIC-1) — отсылает now-playing event followers'ам.
// На `music.now_playing` обогащает payload track-метаданными (title/artist)
// чтобы фронту не делать отдельный fetch /audio-tracks/:id.
// `music.stopped` — пустой, frontend на receive удаляет userID из map'ы.
func (h *WSHandler) fanOutNowPlaying(senderID, eventType string, payload map[string]any) {
	if h.followRepo == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	followers, err := h.followRepo.GetFollowerIDs(ctx, senderID)
	if err != nil || len(followers) == 0 {
		return
	}
	payload["from_user_id"] = senderID

	// Для now_playing — обогащаем title/artist (best-effort).
	if eventType == "music.now_playing" && h.audioRepo != nil {
		if trackID, _ := payload["track_id"].(string); trackID != "" {
			if track, terr := h.audioRepo.GetByID(ctx, trackID); terr == nil && track != nil {
				payload["title"] = track.Title
				payload["artist"] = track.Artist
				payload["cover_url"] = track.CoverURL
			}
		}
	}
	for _, fid := range followers {
		h.hub.SendToUser(fid, eventType, payload)
	}
}

// fanOutGroupCall рассылает group-call event всем не-sender участникам
// чата. Сервер не трекает active call'а — он лишь signaling-router.
// Frontend сам решает кто реально joined и поддерживает mesh peer'ов.
func (h *WSHandler) fanOutGroupCall(senderID, eventType string, payload map[string]any) {
	if h.chatRepo == nil {
		return
	}
	chatID, _ := payload["chat_id"].(string)
	if chatID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Anti-spoof — sender должен быть участником чата.
	if ok, err := h.chatRepo.IsParticipant(ctx, chatID, senderID); err != nil || !ok {
		return
	}
	others, err := h.chatRepo.GetOtherParticipants(ctx, chatID, senderID)
	if err != nil {
		return
	}
	payload["from_user_id"] = senderID

	// Обогащаем payload username'ом sender'а — аналогично relayCallEvent для
	// call.invite. Без этого групповой звонок показывает пустое имя инициатора.
	if h.userRepo != nil {
		// L-4: defer ucancel чтобы контекст не утёк при раннем return или panic.
		uctx, ucancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer ucancel()
		if u, uerr := h.userRepo.GetByID(uctx, senderID); uerr == nil && u != nil {
			payload["from_username"] = u.Username
			payload["from_full_name"] = u.FullName
			payload["from_avatar"] = u.AvatarURL
		}
	}

	// «.join» → others получают «.member.joined». «.leave» → «.member.left».
	// «.invite» проходит как есть.
	outType := eventType
	switch eventType {
	case "call.group.join":
		outType = "call.group.member.joined"
	case "call.group.leave":
		outType = "call.group.member.left"
	}
	for _, peerID := range others {
		h.hub.SendToUser(peerID, outType, payload)
	}
}

// relayCallEvent — generic forwarding для всех call.* signaling-сообщений.
// Берёт target id из payload.to_user_id, добавляет from_user_id, шлёт target'у.
// Если target offline — событие теряется (на этой стадии нет push'а).
//
// C-1 / C-8: side-effect — пишет в call_invitations table для истории
// звонков и создаёт `missed_call` notification если caller сбросил пока
// peer ещё не accepted'ил.
func (h *WSHandler) relayCallEvent(senderID, eventType string, payload map[string]any) {
	toID, _ := payload["to_user_id"].(string)
	if toID == "" || toID == senderID {
		h.logger.Warn("call event missing/self to_user_id",
			zap.String("event", eventType),
			zap.String("sender", senderID),
			zap.String("to", toID))
		return
	}
	// Подмешиваем from_user_id чтобы target знал кто звонит/отвечает.
	payload["from_user_id"] = senderID

	// BUG-2 (a): обогащаем call.invite через userRepo. Раньше callee получал
	// только from_user_id — UI рисовал пустые имя/аватар, выглядело «никто
	// не звонит». Теперь добавляем from_username + from_full_name + from_avatar
	// чтобы CallScreen сразу был информативный.
	// Делаем ДО SendToUser чтобы receiver получил уже обогащённый payload.
	if eventType == "call.invite" && h.userRepo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		if u, uerr := h.userRepo.GetByID(ctx, senderID); uerr == nil && u != nil {
			payload["from_username"] = u.Username
			payload["from_full_name"] = u.FullName
			payload["from_avatar"] = u.AvatarURL
		} else if uerr != nil {
			h.logger.Warn("call.invite: failed to enrich caller info",
				zap.String("sender", senderID), zap.Error(uerr))
		}
		cancel()
	}

	// BUG-6: SendToUser молча drop'ает событие если callee offline.
	// Для call.* events это критично — caller думает что invite ушёл,
	// а в реальности callee никогда его не получит. Логируем delivery
	// outcome чтобы в логах сразу видеть mismatch.
	delivered := h.hub.SendToUser(toID, eventType, payload)
	if !delivered {
		h.logger.Warn("call event undelivered (target offline)",
			zap.String("event", eventType),
			zap.String("from", senderID),
			zap.String("to", toID))
	}

	// Side-effect: DB updates per event-type. callRepo может быть nil
	// в unit-тестах — guard перед каждым.
	if h.callRepo == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	switch eventType {
	case "call.invite":
		kind, _ := payload["kind"].(string)
		if _, err := h.callRepo.CreateInvite(ctx, senderID, toID, kind); err != nil {
			h.logger.Warn("call.invite: persist failed", zap.Error(err))
		}
	case "call.accept":
		// senderID = callee, toID = caller. Pending row у нас created
		// при call.invite — `from=caller, to=callee`. Поэтому matching:
		if err := h.callRepo.MarkAccepted(ctx, toID, senderID); err != nil {
			h.logger.Warn("call.accept: persist failed", zap.Error(err))
		}
	case "call.decline":
		if err := h.callRepo.MarkDeclined(ctx, toID, senderID); err != nil {
			h.logger.Warn("call.decline: persist failed", zap.Error(err))
		}
	case "call.end":
		// Любая сторона может вызвать call.end. MarkEnded ищет latest
		// active row для пары — direction не важен.
		callerID, calleeID, wasMissed, err := h.callRepo.MarkEnded(ctx, senderID, toID)
		if err != nil {
			h.logger.Warn("call.end: persist failed", zap.Error(err))
			return
		}
		// C-8: caller сбросил пока pending → callee получает missed-call
		// нотификацию. Каллер сам ничего не получает (он уже знает что
		// сбросил).
		if wasMissed && callerID != "" && calleeID != "" && h.notifRepo != nil {
			from := callerID
			n := &domain.Notification{
				UserID:     calleeID,
				FromUserID: &from,
				Type:       domain.NotificationTypeMissedCall,
				Message:    "Пропущенный звонок",
			}
			if err := h.notifRepo.Create(ctx, n); err != nil {
				h.logger.Warn("missed-call notif: create failed", zap.Error(err))
			} else {
				// realtime push если callee online (он сейчас offline для
				// звонка — но мог быть на другом девайсе с открытым app'ом).
				h.hub.SendToUser(calleeID, ws.MessageTypeNotification, n)
			}
		}
	}
}

// fanOutTyping resolves the conversation's other participants and pushes
// `chat.typing` to each. Frontend on the receiving side displays a transient
// "печатает..." indicator and auto-hides after a short timeout.
func (h *WSHandler) fanOutTyping(senderID, chatID string) {
	if h.chatRepo == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Verify sender is actually a participant — anti-spoof check.
	if ok, err := h.chatRepo.IsParticipant(ctx, chatID, senderID); err != nil || !ok {
		return
	}
	others, err := h.chatRepo.GetOtherParticipants(ctx, chatID, senderID)
	if err != nil {
		return
	}
	// CHAT-2.1: резолвим username typer'а — для group-чатов фронт рисует
	// "@username печатает…", а не generic "кто-то…". Failure при lookup'е
	// — non-fatal, фронт fallback'нётся на "кто-то…".
	username := ""
	if h.userRepo != nil {
		if u, uerr := h.userRepo.GetByID(ctx, senderID); uerr == nil && u != nil {
			username = u.Username
		}
	}
	payload := map[string]any{
		"chat_id":  chatID,
		"user_id":  senderID,
		"username": username,
	}
	for _, peerID := range others {
		h.hub.SendToUser(peerID, "chat.typing", payload)
	}
}

func (h *WSHandler) writePump(client *ws.Client, conn *fiberws.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				conn.WriteMessage(fiberws.CloseMessage, []byte{})
				return
			}

			w, err := conn.NextWriter(fiberws.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Drain buffered messages
			n := len(client.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte("\n"))
				w.Write(<-client.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(fiberws.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
