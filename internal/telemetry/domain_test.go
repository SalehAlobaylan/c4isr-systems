package telemetry

import (
	"strings"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
)

func TestCreateInputNormalize(t *testing.T) {
	zone := time.FixedZone("utc+3", 3*60*60)
	observedAt := time.Date(2026, 1, 2, 12, 0, 0, 0, zone)

	in := CreateInput{
		MessageID:  " msg_1 ",
		AssetID:    " ast_1 ",
		SourceID:   " src_1 ",
		ObservedAt: observedAt,
	}
	in.Normalize()

	if !strings.HasPrefix(in.ID, "tel_") {
		t.Fatalf("id = %q, want tel_ prefix", in.ID)
	}
	if in.MessageID != "msg_1" || in.AssetID != "ast_1" || in.SourceID != "src_1" {
		t.Fatalf("ids not trimmed: %+v", in)
	}
	if in.ObservedAt.Location() != time.UTC {
		t.Fatalf("observedAt location = %v, want UTC", in.ObservedAt.Location())
	}
	if in.ReceivedAt.IsZero() {
		t.Fatal("receivedAt = zero, want now")
	}
	if in.Payload == nil {
		t.Fatal("payload = nil, want empty map")
	}
}

func TestCreateInputNormalizeKeepsExplicitValues(t *testing.T) {
	receivedAt := time.Date(2026, 1, 2, 12, 0, 30, 0, time.UTC)
	in := CreateInput{
		ID:         "tel_custom",
		AssetID:    "ast_1",
		ObservedAt: time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC),
		ReceivedAt: receivedAt,
		Payload:    map[string]any{"battery": 87},
	}
	in.Normalize()

	if in.ID != "tel_custom" {
		t.Fatalf("id = %q, want tel_custom", in.ID)
	}
	if !in.ReceivedAt.Equal(receivedAt) {
		t.Fatalf("receivedAt = %v, want %v", in.ReceivedAt, receivedAt)
	}
	if in.Payload["battery"] != 87 {
		t.Fatalf("payload = %v, want battery 87", in.Payload)
	}
}

func TestCreateInputValidate(t *testing.T) {
	now := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	valid := CreateInput{
		AssetID:         "ast_1",
		ObservedAt:      now.Add(-time.Minute),
		Position:        &geo.Point{Lat: 10, Lng: 20},
		ConnectionState: "degraded",
	}

	tests := []struct {
		name    string
		in      CreateInput
		wantErr bool
	}{
		{name: "valid", in: valid},
		{name: "missing asset", in: CreateInput{ObservedAt: now}, wantErr: true},
		{name: "missing observedAt", in: CreateInput{AssetID: "ast_1"}, wantErr: true},
		{
			name:    "far future observedAt",
			in:      CreateInput{AssetID: "ast_1", ObservedAt: now.Add(6 * time.Minute)},
			wantErr: true,
		},
		{
			name: "small clock skew allowed",
			in:   CreateInput{AssetID: "ast_1", ObservedAt: now.Add(4 * time.Minute)},
		},
		{
			name:    "invalid position",
			in:      CreateInput{AssetID: "ast_1", ObservedAt: now, Position: &geo.Point{Lat: 91, Lng: 20}},
			wantErr: true,
		},
		{name: "invalid connection state", in: CreateInput{
			AssetID:         "ast_1",
			ObservedAt:      now,
			ConnectionState: "linking",
		}, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.in.Validate(now)
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

func TestValidConnectionState(t *testing.T) {
	for _, state := range []string{"connected", "degraded", "disconnected", "unknown"} {
		if !validConnectionState(state) {
			t.Fatalf("validConnectionState(%q) = false, want true", state)
		}
	}
	if validConnectionState("linking") {
		t.Fatal("validConnectionState(linking) = true, want false")
	}
}
