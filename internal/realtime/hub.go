package realtime

import (
	"context"
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/presentation"
	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	clientBufferSize      = 256
	maxFrameBytes         = 64 * 1024
	writeTimeout          = 5 * time.Second
	pingInterval          = 30 * time.Second
	authorizationInterval = time.Second
)

// Hub is a bounded best-effort COP stream. Each immutable queued frame carries
// its resource, and authorization is repeated immediately before delivery.
type Hub struct {
	logger   *slog.Logger
	mu       sync.RWMutex
	clients  map[*client]struct{}
	observer PublishObserver
}
type PublishObserver interface{ ObserveWebSocketPublish(time.Duration) }

func NewHub(logger *slog.Logger, bus *events.Dispatcher, observers ...PublishObserver) *Hub {
	if logger == nil {
		logger = slog.Default()
	}
	h := &Hub{logger: logger, clients: map[*client]struct{}{}}
	if len(observers) > 0 {
		h.observer = observers[0]
	}
	for topic := range topicPermissions {
		bus.Subscribe(topic, h.handleEvent)
	}
	return h
}
func (h *Hub) Mount(r chi.Router) { r.Get("/realtime", h.serveWS) }

type frame struct {
	resource access.Resource
	topic    string
	policy   eventPolicy
	payload  []byte
}

func ordinary(r access.Resource) bool {
	return r.Partition == access.PublicEvidence || r.Partition == access.ExerciseObservation
}
func (h *Hub) handleEvent(ctx context.Context, ev events.Event) {
	r, err := access.ResourceFrom(ctx)
	if err != nil || !ordinary(r) {
		return
	}
	env := MapEnvelope(ev)
	p, ok := policy(env)
	if !ok {
		return
	}
	env.Resource = r
	// Apply the same exercise disclosure policy as authorized HTTP snapshots.
	kind := map[string]string{"observation": "observations", "source": "sources", "geofence": "geofences", "alert": "alerts", "scenario": "scenarios"}[strings.Split(ev.Topic(), ".")[0]]
	raw, err := presentation.Encode(ctx, "/api/v1/"+kind, env.Data)
	if err != nil {
		return
	}
	if err = json.Unmarshal(raw, &env.Data); err != nil {
		return
	}
	payload, err := json.Marshal(env)
	if err != nil {
		return
	}
	f := frame{r, ev.Topic(), p, payload}
	h.mu.RLock()
	targets := make([]*client, 0, len(h.clients))
	for c := range h.clients {
		targets = append(targets, c)
	}
	h.mu.RUnlock()
	for _, c := range targets {
		if !c.topics[f.topic] {
			continue
		}
		fresh, err := c.refresh(ctx)
		if err != nil {
			c.close(websocket.StatusPolicyViolation)
			continue
		}
		selected, _ := access.ResourceFrom(fresh)
		if selected != r || !p.allowed(fresh) {
			continue
		}
		if len(payload) > maxFrameBytes {
			c.close(websocket.StatusTryAgainLater)
			continue
		}
		select {
		case <-c.done:
		case c.send <- f:
		default:
			c.close(websocket.StatusTryAgainLater)
		}
	}
}
func (h *Hub) serveWS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	resource, err := access.ResourceFrom(ctx)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	if !ordinary(resource) {
		access.Deny(ctx, "cop.subscribe", "ordinary partition required")
		httpx.Error(w, apperr.Forbidden("ordinary subscription requires public evidence or exercise observations"))
		return
	}
	if err = access.Require(ctx, "operational.read"); err != nil {
		httpx.Error(w, err)
		return
	}
	topics, ok := selectTopics(ctx, r.URL.Query().Get("topics"))
	if !ok {
		access.Deny(ctx, "cop.subscribe", "topic permission required")
		httpx.Error(w, apperr.Forbidden("subscription is not authorized"))
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled, Subprotocols: []string{"emad.realtime.v2"}})
	if err != nil {
		return
	}
	stream, cancel := context.WithCancel(ctx)
	stop := make(chan struct{})
	c := &client{auth: ctx, conn: conn, send: make(chan frame, clientBufferSize), topics: topics, resource: resource, cancel: cancel, done: stop, stop: stop}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
		c.close(websocket.StatusNormalClosure)
		// Complete the close handshake before cancelling the read context so
		// browser clients can distinguish policy denial from a network failure.
		_ = conn.Close(websocket.StatusCode(c.closeCode.Load()), "resynchronize using authorized snapshots")
		cancel()
	}()
	h.writeLoop(conn.CloseRead(stream), c)
}

type client struct {
	auth        context.Context
	conn        *websocket.Conn
	send        chan frame
	topics      map[string]bool
	resource    access.Resource
	cancel      context.CancelFunc
	done        <-chan struct{}
	closed      sync.Once
	fingerprint string
	stop        chan struct{}
	closeCode   atomic.Int32
}

func (c *client) close(code websocket.StatusCode) {
	c.closed.Do(func() {
		c.closeCode.Store(int32(code))
		if c.stop != nil {
			close(c.stop)
		} else if c.cancel != nil {
			c.cancel()
		}
	})
}
func (c *client) refresh(ctx context.Context) (context.Context, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	bounded, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return access.RefreshWithContext(c.auth, bounded)
}
func (c *client) authorize(ctx context.Context, f frame) (bool, error) {
	fresh, err := c.refresh(ctx)
	if err != nil {
		return false, err
	}
	r, _ := access.ResourceFrom(fresh)
	if !ordinary(r) || !access.Allows(fresh, "operational.read") {
		return false, apperr.Forbidden("subscription expired")
	}
	return r == f.resource && c.topics[f.topic] && f.policy.allowed(fresh), nil
}
func (h *Hub) writeLoop(ctx context.Context, c *client) {
	if !h.control(ctx, c, "realtime.ready") {
		return
	}
	authTick := time.NewTicker(authorizationInterval)
	defer authTick.Stop()
	ping := time.NewTicker(pingInterval)
	defer ping.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-ctx.Done():
			return
		case f := <-c.send:
			allowed, err := c.authorize(ctx, f)
			if err != nil {
				c.close(websocket.StatusPolicyViolation)
				return
			}
			if !allowed {
				continue
			}
			if c.resource != f.resource {
				c.resource = f.resource
				if !h.control(ctx, c, "realtime.resync") {
					return
				}
			}
			// A blocked control write is another opportunity for a grant to change.
			allowed, err = c.authorize(ctx, f)
			if err != nil {
				c.close(websocket.StatusPolicyViolation)
				return
			}
			if allowed && !h.write(ctx, c, f.payload) {
				return
			}
		case <-authTick.C:
			fresh, err := c.refresh(ctx)
			if err != nil {
				c.close(websocket.StatusPolicyViolation)
				return
			}
			r, _ := access.ResourceFrom(fresh)
			if !ordinary(r) || !access.Allows(fresh, "operational.read") {
				c.close(websocket.StatusPolicyViolation)
				return
			}
			if c.resource != r || c.fingerprint != c.subscriptionFingerprint(fresh) {
				c.resource = r
				if !h.control(ctx, c, "realtime.resync") {
					return
				}
			}
		case <-ping.C:
			bounded, cancel := context.WithTimeout(ctx, writeTimeout)
			err := c.conn.Ping(bounded)
			cancel()
			if err != nil {
				return
			}
		}
	}
}
func (h *Hub) control(ctx context.Context, c *client, kind string) bool {
	fresh, err := c.refresh(ctx)
	if err != nil {
		c.close(websocket.StatusPolicyViolation)
		return false
	}
	r, _ := access.ResourceFrom(fresh)
	if !ordinary(r) || !access.Allows(fresh, "operational.read") {
		c.close(websocket.StatusPolicyViolation)
		return false
	}
	c.resource = r
	c.fingerprint = c.subscriptionFingerprint(fresh)
	// Report only topics still allowed; event checks also apply source restrictions.
	visible := map[string]bool{}
	for topic := range c.topics {
		if topicAllowed(fresh, topic) {
			visible[topic] = true
		}
	}
	raw, err := json.Marshal(Envelope{ID: ids.New("evt"), Type: kind, Version: EnvelopeVersion, OccurredAt: time.Now().UTC(), Resource: r, Data: map[string]any{"resyncRequired": true, "topics": sortedTopics(visible)}})
	return err == nil && h.write(ctx, c, raw)
}
func (h *Hub) write(ctx context.Context, c *client, payload []byte) bool {
	select {
	case <-c.done:
		return false
	default:
	}
	bounded, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	started := time.Now()
	err := c.conn.Write(bounded, websocket.MessageText, payload)
	if h.observer != nil {
		h.observer.ObserveWebSocketPublish(time.Since(started))
	}
	if err != nil {
		c.close(websocket.StatusTryAgainLater)
	}
	return err == nil
}

func (c *client) subscriptionFingerprint(ctx context.Context) string {
	topics := map[string]bool{}
	for topic := range c.topics {
		if topicAllowed(ctx, topic) {
			topics[topic] = true
		}
	}
	sources := access.SourceIDs(ctx)
	sort.Strings(sources)
	r, _ := access.ResourceFrom(ctx)
	raw, _ := json.Marshal([]any{r, sortedTopics(topics), sources})
	return string(raw)
}
