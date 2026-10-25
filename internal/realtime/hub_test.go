package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

func fixtureContext(t *testing.T, r access.Resource, permissions ...string) context.Context {
	t.Helper()
	ctx, err := access.Bind(context.Background(), access.Principal{ID: "fixture", Kind: "human", Grants: []access.Grant{{Resource: r, Permissions: permissions}}}, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	return ctx
}
func TestSubscriptionsAndQueuedFramesRecheckExactAuthority(t *testing.T) {
	r := access.LegacyResource()
	ctx := fixtureContext(t, r, "operational.read", "tracks.read")
	if _, ok := selectTopics(ctx, "observation.received"); ok {
		t.Fatal("COP permission implied evidence subscription")
	}
	if _, ok := selectTopics(ctx, "simulation.truth.snapshot"); ok {
		t.Fatal("truth topic subscribed")
	}
	if _, ok := selectTopics(ctx, "exercise.input.changed"); ok {
		t.Fatal("input topic subscribed")
	}
	var revoked atomic.Bool
	auth := access.WithRefresh(ctx, func(work context.Context) (context.Context, error) {
		if revoked.Load() {
			return nil, apperr.Unauthorized("revoked")
		}
		return ctx, nil
	})
	c := &client{auth: auth, topics: map[string]bool{"track.updated": true}}
	f := frame{resource: r, topic: "track.updated", policy: eventPolicy{permissions: []string{"tracks.read"}}}
	if ok, err := c.authorize(context.Background(), f); err != nil || !ok {
		t.Fatal("authorized frame rejected")
	}
	f.resource.RunID = "other"
	if ok, _ := c.authorize(context.Background(), f); ok {
		t.Fatal("queued resource switch escaped")
	}
	f.resource = r
	revoked.Store(true)
	if ok, err := c.authorize(context.Background(), f); err == nil || ok {
		t.Fatal("queued frame survived revocation")
	}
}

func TestExerciseSerializedFramesAndSlowClientOverflow(t *testing.T) {
	r := access.LegacyResource()
	ctx := fixtureContext(t, r, "operational.read", "observations.read")
	bus := events.NewDispatcher(nil)
	hub := NewHub(nil, bus)
	stream, cancel := context.WithCancel(ctx)
	defer cancel()
	c := &client{auth: ctx, topics: map[string]bool{"observation.received": true}, send: make(chan frame, 1), cancel: cancel, done: stream.Done()}
	hub.clients[c] = struct{}{}
	ev := events.ObservationReceived{At: time.Now(), ObservationID: "run__observation__HIDDEN_ACTOR_123", TrackHint: "HIDDEN_ACTOR_123", SourceID: "sensor", Type: "track"}
	hub.handleEvent(ctx, ev)
	f := <-c.send
	if strings.Contains(string(f.payload), "HIDDEN_ACTOR_123") {
		t.Fatalf("serialized truth identity: %s", f.payload)
	}
	var envelope Envelope
	if err := json.Unmarshal(f.payload, &envelope); err != nil || envelope.Resource != r || envelope.Version != 2 {
		t.Fatal("missing scoped envelope")
	}
	hub.handleEvent(ctx, ev)
	hub.handleEvent(ctx, ev)
	select {
	case <-stream.Done():
	case <-time.After(time.Second):
		t.Fatal("full buffer did not disconnect")
	}
}

func TestInputTruthAndUnknownEventsNeverEnterOrdinaryQueue(t *testing.T) {
	r := access.LegacyResource()
	ctx := fixtureContext(t, r, "operational.read", "tracks.read")
	hub := NewHub(nil, events.NewDispatcher(nil))
	c := &client{auth: ctx, topics: map[string]bool{"track.updated": true}, send: make(chan frame, 8)}
	hub.clients[c] = struct{}{}
	for _, partition := range []access.Partition{access.ExerciseInput, access.ExerciseTruth} {
		other := r
		other.Partition = partition
		p := access.Principal{ID: "privileged", Kind: "human", Grants: []access.Grant{{Resource: other, Permissions: []string{"sim.truth.read", "tracks.read", "operational.read"}}}}
		privileged, err := access.Bind(context.Background(), p, other, nil)
		if err != nil {
			t.Fatal(err)
		}
		hub.handleEvent(privileged, events.TrackUpdated{TrackID: "HIDDEN_ACTOR"})
		request := httptest.NewRequest("GET", "/realtime", nil).WithContext(privileged)
		response := httptest.NewRecorder()
		hub.serveWS(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("privileged partition entered ordinary socket: %d", response.Code)
		}
	}
	if len(c.send) != 0 {
		t.Fatal("privileged partition queued an ordinary frame")
	}
}

func TestSourceBoundSubscriptionCannotSeePeerSensor(t *testing.T) {
	r := access.Resource{WorkspaceID: "public", EnvironmentID: "world", Partition: access.PublicEvidence}
	p := access.Principal{ID: "sensor-reader", Kind: "service", Grants: []access.Grant{{Resource: r, Permissions: []string{"operational.read", "observations.read", "tracks.read"}, SourceIDs: []string{"approved"}}}}
	ctx, err := access.Bind(context.Background(), p, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := selectTopics(ctx, "track.updated"); ok {
		t.Fatal("source-constrained reader subscribed to a mixed-source projection")
	}
	hub := NewHub(nil, events.NewDispatcher(nil))
	c := &client{auth: ctx, topics: map[string]bool{"observation.received": true}, send: make(chan frame, 4)}
	hub.clients[c] = struct{}{}
	for _, source := range []string{"peer", "approved"} {
		hub.handleEvent(ctx, events.ObservationReceived{ObservationID: source, SourceID: source, At: time.Now()})
	}
	if len(c.send) != 1 {
		t.Fatalf("source filter delivered %d frames", len(c.send))
	}
	if f := <-c.send; f.policy.source != "approved" {
		t.Fatal("peer sensor crossed source grant")
	}
}
