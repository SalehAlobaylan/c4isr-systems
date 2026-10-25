package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

func TestWorkerSurvivesArchiveAndQueueFailures(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	var delays []time.Duration
	var messages []string
	err := workLoop(ctx, false, func(context.Context) (bool, error) {
		calls++
		switch {
		case calls == 1:
			return true, errors.New("private backend credential must not be logged")
		case calls < 9:
			return false, errors.New("temporary database outage")
		case calls == 9:
			return true, nil
		case calls == 10:
			return false, nil
		default:
			cancel()
			return false, ctx.Err()
		}
	}, func(_ context.Context, d time.Duration) error {
		delays = append(delays, d)
		return nil
	}, func(v string) { messages = append(messages, v) })
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second, 30 * time.Second, time.Second}
	if err != nil || calls != 11 || len(delays) != len(want) {
		t.Fatal("worker did not continue through recovery", calls, delays, err)
	}
	for i := range want {
		if delays[i] != want[i] {
			t.Fatalf("delay %d = %s, want %s", i, delays[i], want[i])
		}
	}
	if strings.Contains(strings.Join(messages, " "), "private backend") {
		t.Fatal("backend error leaked in worker diagnostics")
	}
}

func TestWorkerStopsOnAuthorizationLossAndOnceFailure(t *testing.T) {
	for _, tc := range []struct {
		name string
		once bool
		err  error
	}{
		{"revoked", false, apperr.Unauthorized("revoked")},
		{"grant-removed", false, apperr.Forbidden("denied")},
		{"one-attempt", true, errors.New("archive failed")},
		{"once-empty", true, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			err := workLoop(context.Background(), tc.once, func(context.Context) (bool, error) {
				calls++
				return false, tc.err
			}, func(context.Context, time.Duration) error {
				t.Fatal("worker retried a terminal condition")
				return nil
			}, func(string) {})
			if calls != 1 || err != tc.err {
				t.Fatal(calls, err)
			}
		})
	}
}

func TestWorkerWaitIsCancellable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := wait(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatal("shutdown did not interrupt backoff", err)
	}
}
