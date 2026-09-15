package geospatial

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
)

func validPolygon() []geo.Point {
	return []geo.Point{
		{Lat: 0, Lng: 0},
		{Lat: 0, Lng: 0.01},
		{Lat: 0.01, Lng: 0.01},
		{Lat: 0.01, Lng: 0},
	}
}

func TestCreateInputValidate(t *testing.T) {
	cases := []struct {
		name    string
		in      CreateInput
		wantErr bool
	}{
		{
			name: "valid",
			in:   CreateInput{Name: "Zone", Type: TypeRestricted, Polygon: validPolygon()},
		},
		{
			name:    "missing name",
			in:      CreateInput{Type: TypeRestricted, Polygon: validPolygon()},
			wantErr: true,
		},
		{
			name:    "invalid type",
			in:      CreateInput{Name: "Zone", Type: "unknown", Polygon: validPolygon()},
			wantErr: true,
		},
		{
			name:    "invalid severity",
			in:      CreateInput{Name: "Zone", Type: TypePatrol, Severity: "extreme", Polygon: validPolygon()},
			wantErr: true,
		},
		{
			name:    "too few points",
			in:      CreateInput{Name: "Zone", Type: TypePatrol, Polygon: validPolygon()[:2]},
			wantErr: true,
		},
		{
			name:    "invalid coordinate",
			in:      CreateInput{Name: "Zone", Type: TypePatrol, Polygon: []geo.Point{{Lat: 91, Lng: 0}, {Lat: 0, Lng: 0}, {Lat: 0, Lng: 1}}},
			wantErr: true,
		},
		{
			name:    "degenerate polygon",
			in:      CreateInput{Name: "Zone", Type: TypePatrol, Polygon: []geo.Point{{Lat: 0, Lng: 0}, {Lat: 1, Lng: 1}, {Lat: 2, Lng: 2}}},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := tc.in
			in.Normalize()
			err := in.Validate()
			if tc.wantErr && err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestCreateInputNormalizeDefaults(t *testing.T) {
	var in CreateInput
	in.Normalize()

	if !strings.HasPrefix(in.ID, "geo_") {
		t.Fatalf("expected generated geo id, got %q", in.ID)
	}
	if in.Severity != SeverityMedium {
		t.Fatalf("expected medium severity, got %q", in.Severity)
	}
	if in.Active == nil || !*in.Active {
		t.Fatal("expected active to default to true")
	}
	if in.Metadata == nil {
		t.Fatal("expected metadata to default to an empty map")
	}
}

func TestCreateInputGeoJSONClosesRing(t *testing.T) {
	in := CreateInput{
		Name: "Zone",
		Type: TypeExclusion,
		Polygon: []geo.Point{
			{Lat: 10, Lng: 20},
			{Lat: 20, Lng: 30},
			{Lat: 10, Lng: 30},
		},
	}
	in.Normalize()

	var payload struct {
		Type        string        `json:"type"`
		Coordinates [][][]float64 `json:"coordinates"`
	}
	if err := json.Unmarshal([]byte(in.GeoJSON()), &payload); err != nil {
		t.Fatalf("invalid GeoJSON: %v", err)
	}
	if payload.Type != "Polygon" {
		t.Fatalf("expected Polygon, got %q", payload.Type)
	}
	if len(payload.Coordinates) != 1 {
		t.Fatalf("expected a single ring, got %d", len(payload.Coordinates))
	}
	ring := payload.Coordinates[0]
	if len(ring) != 4 {
		t.Fatalf("expected a closed ring of 4 positions, got %d", len(ring))
	}
	want := [][]float64{
		{20, 10},
		{30, 20},
		{30, 10},
		{20, 10},
	}
	for i, position := range ring {
		if position[0] != want[i][0] || position[1] != want[i][1] {
			t.Fatalf("position %d: got %v, want %v", i, position, want[i])
		}
	}
}
