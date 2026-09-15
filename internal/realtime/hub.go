package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
)

const (
	clientBufferSize = 256
	writeTimeout     = 10 * time.Second
	pingInterval     = 30 * time.Second
)

// Hub fans domain events out to connected operator clients. It never owns
// domain state and never blocks event processing: a client that cannot keep
// up is disconnected.
type Hub struct {
	logger  *slog.Logger
	mu      sync.RWMutex
	clients map[*client]struct{}
}

// NewHub creates a hub and subscribes it to every domain topic. Register the
// hub after other subscribers so operator notifications observe final state.
func NewHub(logger *slog.Logger, bus *events.Dispatcher) *Hub {
	h := &Hub{
		logger:  logger,
		clients: make(map[*client]struct{}),
	}
	for _, topic := range events.AllTopics() {
		bus.Subscribe(topic, h.handleEvent)
	}
	return h
}

// Mount registers the realtime endpoint on the router.
func (h *Hub) Mount(r chi.Router) {
	r.Get("/realtime", h.serveWS)
}

func (h *Hub) handleEvent(_ context.Context, ev events.Event) {
	envelope := MapEnvelope(ev)
	payload, err := json.Marshal(envelope)
	if err != nil {
		h.logger.Error("marshal realtime envelope", "topic", ev.Topic(), "error", err)
		return
	}

	h.mu.RLock()
	targets := make([]*client, 0, len(h.clients))
	for c := range h.clients {
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		select {
		case c.send <- payload:
		default:
			h.logger.Warn("disconnecting slow realtime client", "client_id", c.id)
			c.close()
		}
	}
}

func (h *Hub) serveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		h.logger.Warn("websocket accept failed", "error", err)
		return
	}

	c := &client{
		id:   r.Header.Get("X-Request-ID"),
		conn: conn,
		send: make(chan []byte, clientBufferSize),
		hub:  h,
	}
	h.add(c)
	defer h.remove(c)

	ctx := conn.CloseRead(r.Context())
	go h.writeLoop(ctx, c)

	<-ctx.Done()
}

func (h *Hub) add(c *client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	h.logger.Info("realtime client connected", "client_id", c.id, "clients", h.count())
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
	}
	h.mu.Unlock()
	c.close()
	h.logger.Info("realtime client disconnected", "client_id", c.id, "clients", h.count())
}

func (h *Hub) count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

type client struct {
	id     string
	conn   *websocket.Conn
	send   chan []byte
	hub    *Hub
	closed sync.Once
}

func (c *client) close() {
	c.closed.Do(func() {
		_ = c.conn.Close(websocket.StatusNormalClosure, "closing")
	})
}

func (h *Hub) writeLoop(ctx context.Context, c *client) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case payload := <-c.send:
			writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := c.conn.Write(writeCtx, websocket.MessageText, payload)
			cancel()
			if err != nil {
				h.logger.Debug("realtime write failed", "client_id", c.id, "error", err)
				return
			}
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := c.conn.Ping(pingCtx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}
