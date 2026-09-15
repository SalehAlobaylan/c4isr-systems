package commands

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

type fakeRepo struct {
	commands map[string]Command
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{commands: map[string]Command{}}
}

func (f *fakeRepo) Create(_ context.Context, command Command) (Command, error) {
	now := time.Now().UTC()
	command.CreatedAt = now
	command.UpdatedAt = now
	f.commands[command.ID] = command
	return command, nil
}

func (f *fakeRepo) Get(_ context.Context, id string) (Command, error) {
	command, ok := f.commands[id]
	if !ok {
		return Command{}, apperr.NotFound("command", id)
	}
	return command, nil
}

func (f *fakeRepo) List(_ context.Context, filter ListFilter, limit, offset int) ([]Command, int, error) {
	out := make([]Command, 0, len(f.commands))
	for _, command := range f.commands {
		if filter.AssetID != "" && command.AssetID != filter.AssetID {
			continue
		}
		if filter.State != "" && string(command.State) != filter.State {
			continue
		}
		if filter.MissionID != "" && command.MissionID != filter.MissionID {
			continue
		}
		out = append(out, command)
	}
	return out, len(out), nil
}

func (f *fakeRepo) Transition(_ context.Context, id string, state State, failureReason string) (Command, error) {
	command, ok := f.commands[id]
	if !ok {
		return Command{}, apperr.NotFound("command", id)
	}
	command.State = state
	if failureReason != "" {
		command.FailureReason = failureReason
	}
	f.commands[id] = command
	return command, nil
}

type fakeRegistry struct{ known map[string]bool }

func (f fakeRegistry) Exists(_ context.Context, id string) (bool, error) {
	return f.known[id], nil
}

func testBus() *events.Dispatcher {
	return events.NewDispatcher(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func issueCommand(t *testing.T, svc *Service) Command {
	t.Helper()
	command, err := svc.Issue(context.Background(), IssueInput{
		AssetID: "ast_1",
		Type:    "hold",
		Actor:   "op_1",
	})
	if err != nil {
		t.Fatal(err)
	}
	return command
}

func TestIssuePublishesCommandIssued(t *testing.T) {
	bus := testBus()
	var issued []events.CommandIssued
	bus.Subscribe(events.TopicCommandIssued, func(_ context.Context, ev events.Event) {
		issued = append(issued, ev.(events.CommandIssued))
	})
	svc := NewService(newFakeRepo(), fakeRegistry{known: map[string]bool{"ast_1": true}}, bus)

	command := issueCommand(t, svc)
	if command.State != StateSent {
		t.Fatalf("state = %s, want %s", command.State, StateSent)
	}
	if len(issued) != 1 {
		t.Fatalf("got %d events, want 1", len(issued))
	}
	if issued[0].CommandID != command.ID || issued[0].AssetID != "ast_1" || issued[0].Type != "hold" || issued[0].Actor != "op_1" {
		t.Fatalf("unexpected event: %+v", issued[0])
	}
}

func TestIssueRejectsUnknownAsset(t *testing.T) {
	svc := NewService(newFakeRepo(), fakeRegistry{}, testBus())
	_, err := svc.Issue(context.Background(), IssueInput{AssetID: "ast_missing", Type: "hold"})
	if !apperr.Is(err, apperr.CodeValidation) {
		t.Fatalf("err = %v, want validation error", err)
	}
}

func TestTransitionPublishesCommandStatusChanged(t *testing.T) {
	bus := testBus()
	var changed []events.CommandStatusChanged
	bus.Subscribe(events.TopicCommandStatusChanged, func(_ context.Context, ev events.Event) {
		changed = append(changed, ev.(events.CommandStatusChanged))
	})
	svc := NewService(newFakeRepo(), fakeRegistry{known: map[string]bool{"ast_1": true}}, bus)
	command := issueCommand(t, svc)

	updated, err := svc.Acknowledge(context.Background(), command.ID, "op_2")
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != StateAcknowledged {
		t.Fatalf("state = %s, want %s", updated.State, StateAcknowledged)
	}
	if len(changed) != 1 {
		t.Fatalf("got %d events, want 1", len(changed))
	}
	if changed[0].CommandID != command.ID || changed[0].State != string(StateAcknowledged) || changed[0].Actor != "op_2" {
		t.Fatalf("unexpected event: %+v", changed[0])
	}
}

func TestTransitionRejectsInvalidTransition(t *testing.T) {
	bus := testBus()
	var changed int
	bus.Subscribe(events.TopicCommandStatusChanged, func(_ context.Context, ev events.Event) {
		changed++
	})
	svc := NewService(newFakeRepo(), fakeRegistry{known: map[string]bool{"ast_1": true}}, bus)
	command := issueCommand(t, svc)

	if _, err := svc.Complete(context.Background(), command.ID, "op_2"); !apperr.Is(err, apperr.CodeConflict) {
		t.Fatalf("err = %v, want conflict error", err)
	}
	if changed != 0 {
		t.Fatalf("got %d events, want 0", changed)
	}
}

func TestTransitionRejectsTerminalState(t *testing.T) {
	svc := NewService(newFakeRepo(), fakeRegistry{known: map[string]bool{"ast_1": true}}, testBus())
	command := issueCommand(t, svc)

	if _, err := svc.Acknowledge(context.Background(), command.ID, "op_2"); err != nil {
		t.Fatal(err)
	}
	completed, err := svc.Complete(context.Background(), command.ID, "op_2")
	if err != nil {
		t.Fatal(err)
	}
	if !IsTerminal(completed.State) {
		t.Fatalf("state = %s, want terminal", completed.State)
	}
	if _, err := svc.Fail(context.Background(), command.ID, "late failure", "op_2"); !apperr.Is(err, apperr.CodeConflict) {
		t.Fatalf("err = %v, want conflict error", err)
	}
}

func TestFailUsesDefaultReason(t *testing.T) {
	svc := NewService(newFakeRepo(), fakeRegistry{known: map[string]bool{"ast_1": true}}, testBus())
	command := issueCommand(t, svc)

	failed, err := svc.Fail(context.Background(), command.ID, "", "op_2")
	if err != nil {
		t.Fatal(err)
	}
	if failed.State != StateFailed || failed.FailureReason == "" {
		t.Fatalf("unexpected command: %+v", failed)
	}
}
