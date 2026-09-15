package assessments

import (
	"context"
	"io"
	"log/slog"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
)

type fakeRepository struct {
	created         []Assessment
	listSubjectType string
	listSubjectID   string
	listLimit       int
	listOffset      int
}

func (f *fakeRepository) Create(_ context.Context, assessment Assessment) (Assessment, error) {
	f.created = append(f.created, assessment)
	return assessment, nil
}

func (f *fakeRepository) Get(context.Context, string) (Assessment, error) {
	return Assessment{}, nil
}

func (f *fakeRepository) List(_ context.Context, subjectType, subjectID string, limit, offset int) ([]Assessment, int, error) {
	f.listSubjectType = subjectType
	f.listSubjectID = subjectID
	f.listLimit = limit
	f.listOffset = offset
	return []Assessment{}, 0, nil
}

func testBus() (*events.Dispatcher, *[]events.AssessmentCreated) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bus := events.NewDispatcher(logger)
	published := &[]events.AssessmentCreated{}
	bus.Subscribe(events.TopicAssessmentCreated, func(_ context.Context, ev events.Event) {
		if e, ok := ev.(events.AssessmentCreated); ok {
			*published = append(*published, e)
		}
	})
	return bus, published
}

var _ Repository = (*fakeRepository)(nil)
