package assets

import (
	"strings"
	"testing"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

func TestCreateInputNormalize(t *testing.T) {
	in := CreateInput{Name: "  UAV Alpha  ", Type: " uav "}
	in.Normalize()

	if !strings.HasPrefix(in.ID, "ast_") {
		t.Fatalf("id = %q, want ast_ prefix", in.ID)
	}
	if in.Name != "UAV Alpha" {
		t.Fatalf("name = %q, want trimmed", in.Name)
	}
	if in.Type != "uav" {
		t.Fatalf("type = %q, want trimmed", in.Type)
	}
	if in.Status != StatusAvailable {
		t.Fatalf("status = %q, want %q", in.Status, StatusAvailable)
	}
	if in.Capabilities == nil || len(in.Capabilities) != 0 {
		t.Fatalf("capabilities = %v, want empty slice", in.Capabilities)
	}
	if in.Metadata == nil {
		t.Fatal("metadata = nil, want empty map")
	}
}

func TestCreateInputNormalizeKeepsExplicitFields(t *testing.T) {
	in := CreateInput{
		ID:           " ast_custom ",
		Name:         "Falcon",
		Type:         "fixed_wing",
		Status:       StatusMaintenance,
		Capabilities: []string{"isr"},
		Metadata:     map[string]any{"operator": "alpha"},
	}
	in.Normalize()

	if in.ID != "ast_custom" {
		t.Fatalf("id = %q, want ast_custom", in.ID)
	}
	if in.Status != StatusMaintenance {
		t.Fatalf("status = %q, want %q", in.Status, StatusMaintenance)
	}
	if len(in.Capabilities) != 1 || in.Capabilities[0] != "isr" {
		t.Fatalf("capabilities = %v, want [isr]", in.Capabilities)
	}
	if in.Metadata["operator"] != "alpha" {
		t.Fatalf("metadata = %v, want operator alpha", in.Metadata)
	}
}

func TestCreateInputValidate(t *testing.T) {
	valid := CreateInput{Name: "UAV", Type: "uav", Status: StatusAvailable}

	tests := []struct {
		name    string
		in      CreateInput
		wantErr bool
	}{
		{name: "valid", in: valid},
		{name: "missing name", in: CreateInput{Type: "uav", Status: StatusAvailable}, wantErr: true},
		{name: "missing type", in: CreateInput{Name: "UAV", Status: StatusAvailable}, wantErr: true},
		{name: "invalid status", in: CreateInput{Name: "UAV", Type: "uav", Status: "flying"}, wantErr: true},
		{name: "empty status", in: CreateInput{Name: "UAV", Type: "uav"}, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.in.Validate()
			if tc.wantErr {
				if !apperr.Is(err, apperr.CodeValidation) {
					t.Fatalf("err = %v, want validation error", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
		})
	}
}

func TestValidStatus(t *testing.T) {
	for _, status := range []Status{
		StatusAvailable,
		StatusAssigned,
		StatusUnavailable,
		StatusOffline,
		StatusMaintenance,
	} {
		if !validStatus(status) {
			t.Fatalf("validStatus(%q) = false, want true", status)
		}
	}
	if validStatus("flying") {
		t.Fatal("validStatus(flying) = true, want false")
	}
}
