package incidents

import (
	"testing"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

func TestCanTransition(t *testing.T) {
	valid := map[Status]map[Status]bool{
		StatusOpen: {
			StatusAcknowledged:  true,
			StatusInvestigating: true,
			StatusClosed:        true,
		},
		StatusAcknowledged: {
			StatusInvestigating: true,
			StatusResponding:    true,
			StatusClosed:        true,
		},
		StatusInvestigating: {
			StatusResponding: true,
			StatusResolved:   true,
			StatusClosed:     true,
		},
		StatusResponding: {
			StatusInvestigating: true,
			StatusResolved:      true,
			StatusClosed:        true,
		},
		StatusResolved: {
			StatusInvestigating: true,
			StatusClosed:        true,
		},
		StatusClosed: {},
	}
	all := []Status{StatusOpen, StatusAcknowledged, StatusInvestigating, StatusResponding, StatusResolved, StatusClosed}
	for _, from := range all {
		for _, to := range all {
			want := valid[from][to]
			if got := canTransition(from, to); got != want {
				t.Errorf("canTransition(%s, %s) = %v, want %v", from, to, got, want)
			}
		}
	}
	for _, from := range all {
		if canTransition(from, Status("BOGUS")) {
			t.Errorf("canTransition(%s, BOGUS) = true, want false", from)
		}
		if canTransition(Status("BOGUS"), from) {
			t.Errorf("canTransition(BOGUS, %s) = true, want false", from)
		}
	}
}

func TestCreateInputNormalize(t *testing.T) {
	in := CreateInput{Title: "  Fire  ", Description: "  smoke  ", Actor: "  op_1  "}
	in.Normalize()
	if in.Title != "Fire" {
		t.Errorf("title = %q, want %q", in.Title, "Fire")
	}
	if in.Description != "smoke" {
		t.Errorf("description = %q, want %q", in.Description, "smoke")
	}
	if in.Actor != "op_1" {
		t.Errorf("actor = %q, want %q", in.Actor, "op_1")
	}
	if in.Priority != PriorityMedium {
		t.Errorf("priority = %q, want %q", in.Priority, PriorityMedium)
	}
}

func TestCreateInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		in      CreateInput
		wantErr bool
	}{
		{"valid", CreateInput{Title: "Fire", Priority: PriorityHigh}, false},
		{"missing title", CreateInput{Priority: PriorityMedium}, true},
		{"blank title", CreateInput{Title: "   ", Priority: PriorityMedium}, true},
		{"invalid priority", CreateInput{Title: "Fire", Priority: "urgent"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.in.Normalize()
			err := tc.in.Validate()
			if tc.wantErr {
				if !apperr.Is(err, apperr.CodeValidation) {
					t.Fatalf("err = %v, want validation error", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
