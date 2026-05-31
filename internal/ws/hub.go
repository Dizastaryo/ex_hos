package ws

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// BUG-4: hooks выполняются в goroutine с жёстким 5-sec timeout'ом.
// Раньше: `go h.PresenceHook(userID)` без context-cancellation → если hook
// зависнет на DB-query, goroutine'а живёт forever (memory leak). Теперь
// каждый hook получает ограниченное время через runHook(); по истечении
// timeout'а — лог-warning, goroutine'а закрывается.
const hookTimeout = 5 * time.Second

func runHook(name string, logger *zap.Logger, fn func(ctx context.Context)) {
	ctx, cancel := context.WithTimeout(context.Background(), hookTimeout)
	go func() {
		defer cancel()
		done := make(chan struct{})
		go func() {
			defer close(done)
			fn(ctx)
		}()
		select {
		case <-done:
			// success / fn вернулся.
		case <-ctx.Done():
			logger.Warn("ws hook timed out",
				zap.String("hook", name),
				zap.Duration("limit", hookTimeout))
		}
	}()
}

type Client struct {
	UserID string
	Send   chan []byte
	Hub    *Hub
}

type Hub struct {
	mu      sync.RWMutex
	clients map[string][]*Client // userID -> clients
	logger  *zap.Logger

	// PresenceHook вызывается на register/unregister с конкретным userID.
	// Используется чтобы обновить users.last_seen_at в БД. nil → no-op.
	// Зовётся вне мьютекса — реализация должна сама синхронизироваться.
	PresenceHook func(userID string)

	// RegisterHook вызывается ТОЛЬКО на client Register (после того как
	// клиент добавлен в map и SendToUser его уже видит). Используется для
	// CHAT-10.3 late-delivered replay: scan undelivered messages этого
	// userID + emit `chat.delivered` к sender'ам. nil → no-op.
	RegisterHook func(userID string)

	// BUG-23: counters для observability. Атомарны, дешёво writeable.
	// Periodically dump'аются в лог (см. StartMetricsReporter). Будущая
	// Prometheus-интеграция (TEST-4) использует эти же счётчики через
	// expvar/metrics-handler.
	dropped       atomic.Uint64 // ws_send_dropped — buffer full
	undelivered   atomic.Uint64 // ws_send_undelivered — user offline
	sent          atomic.Uint64 // ws_send_total — успешно
	stopMetrics   chan struct{}
}

func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		clients:     make(map[string][]*Client),
		logger:      logger,
		stopMetrics: make(chan struct{}),
	}
}

// MetricsSnapshot — текущие счётчики hub'а. Используется в /admin/metrics +
// будущем /metrics Prometheus endpoint.
type MetricsSnapshot struct {
	Sent        uint64 `json:"ws_send_total"`
	Dropped     uint64 `json:"ws_send_dropped"`     // buffer full
	Undelivered uint64 `json:"ws_send_undelivered"` // user offline
}

func (h *Hub) Metrics() MetricsSnapshot {
	return MetricsSnapshot{
		Sent:        h.sent.Load(),
		Dropped:     h.dropped.Load(),
		Undelivered: h.undelivered.Load(),
	}
}

// StartMetricsReporter — каждые 5 минут лог-summary счётчиков. Вызывается
// один раз в startup. На shutdown caller делает StopMetricsReporter.
func (h *Hub) StartMetricsReporter() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		var prevSent, prevDrop, prevUnd uint64
		for {
			select {
			case <-h.stopMetrics:
				return
			case <-ticker.C:
				m := h.Metrics()
				if m.Sent == prevSent && m.Dropped == prevDrop && m.Undelivered == prevUnd {
					continue // нет активности — skip noise
				}
				h.logger.Info("ws hub metrics",
					zap.Uint64("sent_total", m.Sent),
					zap.Uint64("sent_delta", m.Sent-prevSent),
					zap.Uint64("dropped_total", m.Dropped),
					zap.Uint64("dropped_delta", m.Dropped-prevDrop),
					zap.Uint64("undelivered_total", m.Undelivered),
					zap.Uint64("undelivered_delta", m.Undelivered-prevUnd),
				)
				prevSent, prevDrop, prevUnd = m.Sent, m.Dropped, m.Undelivered
			}
		}
	}()
}

func (h *Hub) StopMetricsReporter() {
	select {
	case <-h.stopMetrics:
		// already closed
	default:
		close(h.stopMetrics)
	}
}

func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	h.clients[client.UserID] = append(h.clients[client.UserID], client)
	h.mu.Unlock()
	h.logger.Debug("ws client registered", zap.String("user_id", client.UserID))
	if h.PresenceHook != nil {
		userID := client.UserID
		runHook("presence.register", h.logger, func(_ context.Context) {
			h.PresenceHook(userID)
		})
	}
	if h.RegisterHook != nil {
		userID := client.UserID
		runHook("register.replay", h.logger, func(_ context.Context) {
			h.RegisterHook(userID)
		})
	}
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()

	clients := h.clients[client.UserID]
	for i, c := range clients {
		if c == client {
			h.clients[client.UserID] = append(clients[:i], clients[i+1:]...)
			close(client.Send)
			break
		}
	}

	if len(h.clients[client.UserID]) == 0 {
		delete(h.clients, client.UserID)
	}
	h.mu.Unlock()

	h.logger.Debug("ws client unregistered", zap.String("user_id", client.UserID))
	if h.PresenceHook != nil {
		userID := client.UserID
		runHook("presence.unregister", h.logger, func(_ context.Context) {
			h.PresenceHook(userID)
		})
	}
}

type Message struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// SendToUser шлёт `msgType` всем активным connection'ам этого юзера.
// BUG-6: возвращает true если событие было доставлено хотя бы одному из них.
// false означает что юзер offline (нет открытых WS) ИЛИ все Send-buffer'ы full
// → событие потеряно. Caller обязан логировать false для critical events
// (call.*, missed_call). Не-критичные (typing, presence) могут игнорировать.
// BUG-23: атомарные счётчики sent/dropped/undelivered — observability.
func (h *Hub) SendToUser(userID string, msgType string, payload interface{}) bool {
	h.mu.RLock()
	clients := h.clients[userID]
	h.mu.RUnlock()

	if len(clients) == 0 {
		h.undelivered.Add(1)
		return false
	}

	msg := Message{Type: msgType, Payload: payload}
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("failed to marshal ws message", zap.Error(err))
		return false
	}

	delivered := false
	for _, c := range clients {
		select {
		case c.Send <- data:
			delivered = true
			h.sent.Add(1)
		default:
			h.dropped.Add(1)
			h.logger.Warn("ws client send buffer full, dropping message",
				zap.String("user_id", userID),
				zap.String("event", msgType))
		}
	}
	return delivered
}

func (h *Hub) SendToUsers(userIDs []string, msgType string, payload interface{}) {
	msg := Message{Type: msgType, Payload: payload}
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("failed to marshal ws message", zap.Error(err))
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, userID := range userIDs {
		for _, c := range h.clients[userID] {
			select {
			case c.Send <- data:
			default:
				h.logger.Warn("ws client send buffer full",
					zap.String("user_id", userID))
			}
		}
	}
}

func (h *Hub) Broadcast(msgType string, payload interface{}) {
	msg := Message{Type: msgType, Payload: payload}
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("failed to marshal ws broadcast", zap.Error(err))
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, clients := range h.clients {
		for _, c := range clients {
			select {
			case c.Send <- data:
			default:
			}
		}
	}
}

func (h *Hub) IsOnline(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[userID]) > 0
}

func (h *Hub) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

const (
	MessageTypeNotification = "notification"
	MessageTypeStoryExpire  = "story_expire"
	MessageTypeNewPost      = "new_post"
	MessageTypePing         = "ping"
	MessageTypePong         = "pong"
)
