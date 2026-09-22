package observations

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
)

type rejectionRepository struct{}

func (rejectionRepository) Create(context.Context, Observation) (Observation, error) {
	return Observation{}, errors.New("duplicate key value reveals internal table details")
}
func (rejectionRepository) Get(context.Context, string) (Observation, error) {
	return Observation{}, errors.New("not used")
}
func (rejectionRepository) List(context.Context, ListFilter, int, int) ([]Observation, int, error) {
	return nil, 0, nil
}
func (rejectionRepository) MarkProcessed(context.Context, string) error { return nil }

type rejectionSourceRegistry struct{ err error }

func (r rejectionSourceRegistry) Exists(context.Context, string) (bool, error) { return false, r.err }

func TestIngestPublishesStableRejectionReason(t *testing.T) {
	bus := events.NewDispatcher(slog.Default())
	var rejected events.ObservationRejected
	bus.Subscribe(events.TopicObservationRejected, func(_ context.Context, ev events.Event) {
		rejected = ev.(events.ObservationRejected)
	})
	svc := NewService(rejectionRepository{}, rejectionSourceRegistry{err: errors.New("database password and relation leaked")}, bus)
	svc.now = func() time.Time { return time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC) }

	_, err := svc.Ingest(context.Background(), CreateInput{
		ID:         "obs-1",
		SourceID:   "source-1",
		Type:       "track.observation",
		ObservedAt: time.Date(2026, 9, 20, 11, 59, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("Ingest() error = nil, want source lookup failure")
	}
	if rejected.Reason != "internal_error" {
		t.Fatalf("rejection reason = %q, want internal_error", rejected.Reason)
	}
	if rejected.Reason == err.Error() || rejected.Reason == "database password and relation leaked" {
		t.Fatalf("rejection reason leaked infrastructure detail: %q", rejected.Reason)
	}
}
