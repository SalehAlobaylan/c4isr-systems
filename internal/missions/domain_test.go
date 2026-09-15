package missions

import (
	"testing"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

func TestCanTransition(t *testing.T) {
	valid := map[Status]map[Status]bool{
		StatusPlanned: {
			StatusActive:  true,
			StatusAborted: true,
		},
		StatusActive: {
			StatusCompleted: true,
			StatusAborted:   true,
		},
		StatusCompleted: {},
		StatusAborted:   {},
	}
	all := []Status{StatusPlanned, StatusActive, StatusCompleted, StatusAborted}
	for _, from := range all {
		for _, to := range all {
			want := valid[from][to]
			if got := canTransition(from, to); got != want {
				t.Errorf("canTransition(%s, %s) = %v, want %v", from, to, got, want)
			}
		}
		if canTransition(from, Status("BOGUS")) {
			t.Errorf("canTransition(%s, BOGUS) = true, want false", from)
		}
	}
}

func TestCreateInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		in      CreateInput
		wantErr bool
	}{
		{"valid", CreateInput{Name: "Patrol", Priority: PriorityHigh}, false},
		{"missing name", CreateInput{Priority: PriorityMedium}, true},
		{"invalid priority", CreateInput{Name: "Patrol", Priority: "urgent"}, true},
		{"task missing type", CreateInput{Name: "Patrol", Priority: PriorityMedium, Tasks: []TaskInput{{Description: "x"}}}, true},
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

func TestTaskInputValidate(t *testing.T) {
	in := TaskInput{Type: "  Recon  ", Description: "  route  "}
	in.Normalize()
	if in.Type != "Recon" || in.Description != "route" {
		t.Fatalf("unexpected normalized task: %+v", in)
	}
	if err := in.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := (TaskInput{}).Validate(); !apperr.Is(err, apperr.CodeValidation) {
		t.Fatalf("err = %v, want validation error", err)
	}
}

func TestValidTaskStatus(t *testing.T) {
	valid := []TaskStatus{TaskPending, TaskActive, TaskCompleted, TaskFailed, TaskCancelled}
	for _, status := range valid {
		if !validTaskStatus(status) {
			t.Errorf("validTaskStatus(%s) = false, want true", status)
		}
	}
	if validTaskStatus("BOGUS") {
		t.Error("validTaskStatus(BOGUS) = true, want false")
	}
}
