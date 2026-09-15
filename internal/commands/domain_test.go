package commands

import (
	"testing"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

func TestCanTransition(t *testing.T) {
	valid := map[State]map[State]bool{
		StateCreated: {
			StateQueued:    true,
			StateSent:      true,
			StateCancelled: true,
		},
		StateQueued: {
			StateSent:      true,
			StateCancelled: true,
		},
		StateSent: {
			StateAcknowledged: true,
			StateRejected:     true,
			StateFailed:       true,
			StateTimedOut:     true,
			StateCancelled:    true,
		},
		StateAcknowledged: {
			StateCompleted: true,
			StateFailed:    true,
			StateCancelled: true,
		},
		StateCompleted: {},
		StateRejected:  {},
		StateFailed:    {},
		StateTimedOut:  {},
		StateCancelled: {},
	}
	all := []State{
		StateCreated, StateQueued, StateSent, StateAcknowledged, StateCompleted,
		StateRejected, StateFailed, StateTimedOut, StateCancelled,
	}
	for _, from := range all {
		for _, to := range all {
			want := valid[from][to]
			if got := CanTransition(from, to); got != want {
				t.Errorf("CanTransition(%s, %s) = %v, want %v", from, to, got, want)
			}
		}
		if CanTransition(from, State("BOGUS")) {
			t.Errorf("CanTransition(%s, BOGUS) = true, want false", from)
		}
	}
}

func TestIsTerminal(t *testing.T) {
	terminal := []State{StateCompleted, StateRejected, StateFailed, StateTimedOut, StateCancelled}
	for _, state := range terminal {
		if !IsTerminal(state) {
			t.Errorf("IsTerminal(%s) = false, want true", state)
		}
	}
	active := []State{StateCreated, StateQueued, StateSent, StateAcknowledged}
	for _, state := range active {
		if IsTerminal(state) {
			t.Errorf("IsTerminal(%s) = true, want false", state)
		}
	}
	if IsTerminal("BOGUS") {
		t.Error("IsTerminal(BOGUS) = true, want false")
	}
}

func TestIssueInputNormalizeAndValidate(t *testing.T) {
	in := IssueInput{AssetID: "  ast_1  ", Type: "  hold  ", Actor: "  op_1  "}
	in.Normalize()
	if in.AssetID != "ast_1" || in.Type != "hold" || in.Actor != "op_1" {
		t.Fatalf("unexpected normalized input: %+v", in)
	}
	if in.Payload == nil {
		t.Fatal("payload = nil, want empty map")
	}
	if err := in.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	missingAsset := IssueInput{Type: "hold"}
	missingAsset.Normalize()
	if err := missingAsset.Validate(); !apperr.Is(err, apperr.CodeValidation) {
		t.Fatalf("err = %v, want validation error", err)
	}
	missingType := IssueInput{AssetID: "ast_1"}
	missingType.Normalize()
	if err := missingType.Validate(); !apperr.Is(err, apperr.CodeValidation) {
		t.Fatalf("err = %v, want validation error", err)
	}
}
