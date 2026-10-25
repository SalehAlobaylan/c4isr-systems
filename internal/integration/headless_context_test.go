package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/app"
	"github.com/SalehAlobaylan/c4isr-systems/internal/contextpackage"
	"github.com/SalehAlobaylan/c4isr-systems/internal/exerciseingest"
	"github.com/SalehAlobaylan/c4isr-systems/internal/observations"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
)

func TestM2HeadlessFrozenContextSyntheticIngressAndTruthIsolation(t *testing.T) {
	dir := t.TempDir()
	f := newPublicFixture(t, app.WithScenariosDirectory(dir))
	store := access.NewStore(f.h.pool)
	admin := access.Bootstrap(context.Background())
	raw, err := os.ReadFile("../../testdata/reference/jeddah-makkah/places.json")
	if err != nil {
		t.Fatal(err)
	}
	var input reference.Input
	if err = json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	input.SourceID = f.source
	status, dataset := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/reference-datasets", input, nil)
	if status != 201 {
		t.Fatalf("reference %d %v", status, dataset)
	}
	var collection reference.Collection
	if err = json.Unmarshal(input.Artifact, &collection); err != nil {
		t.Fatal(err)
	}
	feature := collection.Features[0]
	var geometry struct {
		Coordinates []float64 `json:"coordinates"`
	}
	if err = json.Unmarshal(feature.Geometry, &geometry); err != nil {
		t.Fatal(err)
	}
	if err = store.CreateHuman(admin, "m2-author", "operator-01", access.Grant{Resource: f.resource, Permissions: []string{"operational.read", "sources.read", "context.export"}}); err != nil {
		t.Fatal(err)
	}
	exercise := f.resource
	exercise.Partition = access.ExerciseObservation
	exercise.RunID = "authoring"
	if err = store.EnsureResource(admin, exercise); err != nil {
		t.Fatal(err)
	}
	wildcard := exercise
	wildcard.RunID = "*"
	if err = store.AddGrant(admin, "m2-author", access.Grant{Resource: wildcard, Permissions: []string{"operational.read", "scenarios.read", "scenarios.control", "sim.scenarios.author", "observations.read", "tracks.read", "assets.read", "telemetry.read"}}); err != nil {
		t.Fatal(err)
	}
	_, token, err := store.IssueCredential(admin, "m2-author", f.resource, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	build := contextpackage.BuildInput{DatasetIDs: []string{dataset["id"].(string)}, AOI: publicingest.Bounds{West: 39, South: 21, East: 40, North: 22}, TimeWindow: contextpackage.Window{From: now.Add(-time.Hour), To: now}, AcceptedQuality: "Regional reference point only; no inferred altitude", Approval: "Approve this retained point for headless integration"}
	status, pkg := scopeRequest(t, f.h, token, "POST", "/api/v1/context-packages", build, nil)
	if status != 201 {
		t.Fatalf("package %d %v", status, pkg)
	}
	definition := fmt.Sprintf(`scenario: headless-context
seed: 51
playback_speed: 5000
clock: {epoch: 2025-01-01T00:00:00Z, elapsed_unit: milliseconds, order: virtual_time_then_sequence, source_time_mapping: preserve_source_time}
context_packages: [%s]
context_anchors:
  - {track: M2_PRIVATE_ACTOR, package_id: %s, dataset_id: %s, feature_id: %s}
sources: [{id: synthetic-sensor, name: Modeled sensor, type: synthetic}]
tracks: [{id: M2_PRIVATE_ACTOR}]
assets:
  - {id: controlled-vehicle, start: {lat: %f, lng: %f}}
events:
  - {after: 0ms, action: observe, source: synthetic-sensor, track: M2_PRIVATE_ACTOR, duplicate: true, payload: {hiddenInstruction: M2_PRIVATE_INSTRUCTION}}
  - {after: 1s, action: observe, source: synthetic-sensor, track: M2_PRIVATE_ACTOR, fault: invalid_coordinate}
`, pkg["id"], pkg["id"], dataset["id"], feature.ID, geometry.Coordinates[1], geometry.Coordinates[0])
	if err = os.WriteFile(filepath.Join(dir, "headless-context.yaml"), []byte(definition), 0600); err != nil {
		t.Fatal(err)
	}
	headers := map[string]string{"X-EMAD-Partition": string(access.ExerciseObservation), "X-EMAD-Run-ID": "authoring"}
	status, run := scopeRequest(t, f.h, token, "POST", "/api/v1/scenarios/definitions/headless-context/start", nil, headers)
	if status != 201 {
		t.Fatalf("headless start %d %v", status, run)
	}
	id := run["id"].(string)
	waitFor(t, 10*time.Second, 20*time.Millisecond, "headless completion", func() bool {
		_, state := scopeRequest(t, f.h, token, "GET", "/api/v1/scenarios/runs/"+id, nil, nil)
		return state["status"] == "COMPLETED"
	})
	selected := exercise
	selected.RunID = id
	reader := scopeToken(t, f.h, "m2-operator", selected, []string{"operational.read", "scenarios.read", "observations.read", "tracks.read", "assets.read", "telemetry.read"}, nil)
	status, receipts := scopeRequest(t, f.h, reader, "GET", "/api/v1/scenarios/runs/"+id+"/synthetic-messages", nil, nil)
	if status != 200 || receipts["total"].(float64) < 3 {
		t.Fatalf("synthetic receipts %d %v", status, receipts)
	}
	for _, path := range []string{"/api/v1/observations", "/api/v1/tracks", "/api/v1/picture", "/api/v1/scenarios/runs/" + id + "/synthetic-messages"} {
		status, body := scopeRequest(t, f.h, reader, "GET", path, nil, nil)
		encoded, _ := json.Marshal(body)
		if status != 200 || strings.Contains(string(encoded), "M2_PRIVATE") || strings.Contains(string(encoded), "actorId") {
			t.Fatalf("truth disclosure on %s %d %s", path, status, encoded)
		}
	}
	var count int
	var lng, lat float64
	if err = f.h.pool.QueryRow(context.Background(), `SELECT count(*) FROM observations WHERE scope_id=$1`, selected.ID()).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate/invalid observations persisted %d %v", count, err)
	}
	if err = f.h.pool.QueryRow(context.Background(), `SELECT ST_X(position::geometry),ST_Y(position::geometry) FROM observations WHERE scope_id=$1`, selected.ID()).Scan(&lng, &lat); err != nil || lng != geometry.Coordinates[0] || lat != geometry.Coordinates[1] {
		t.Fatalf("retained context point was not consumed: %f,%f %v", lng, lat, err)
	}
	truth := selected
	truth.Partition = access.ExerciseTruth
	ingress := exerciseingest.NewService(f.h.pool, nil, nil)
	mappings, err := ingress.Mappings(scopeContext(t, f.h, truth))
	if err != nil || len(mappings) < 3 {
		t.Fatalf("sealed mappings missing %v %v", mappings, err)
	}
	if _, err = ingress.Mappings(scopeContext(t, f.h, selected)); !apperr.Is(err, apperr.CodeForbidden) {
		t.Fatal("ordinary exercise read truth", err)
	}
	if err = platformdb.Scoped(f.h.pool).QueryRow(scopeContext(t, f.h, selected), `SELECT count(*) FROM simulation_actor_mappings`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("truth crossed RLS %d %v", count, err)
	}
	if status, _ = scopeRequest(t, f.h, reader, "GET", "/api/v1/scenarios/runs/"+id+"/version", nil, nil); status != 403 {
		t.Fatal("operator read authored inputs")
	}
	if status, _ = scopeRequest(t, f.h, reader, "GET", "/api/v1/observations", nil, map[string]string{"X-EMAD-Partition": string(access.ExerciseTruth)}); status != 403 {
		t.Fatal("ordinary transport selected truth")
	}
	// No public feed is used or written by the headless runner.
	if f.fetcher.count() != 0 {
		t.Fatal("headless run fetched a live feed")
	}
	if err = f.h.pool.QueryRow(context.Background(), `SELECT count(*) FROM observations WHERE scope_id=$1`, f.resource.ID()).Scan(&count); err != nil || count != 0 {
		t.Fatal("run changed public evidence")
	}
	// Reuse of a synthetic id with different input is rejected before domain ingestion.
	var messageRaw []byte
	if err = f.h.pool.QueryRow(context.Background(), `SELECT document FROM exercise_messages WHERE scope_id=$1 AND kind='observation' AND status='DELIVERED' LIMIT 1`, selected.ID()).Scan(&messageRaw); err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		exerciseingest.Stamp
		ID    string                   `json:"id"`
		Input observations.CreateInput `json:"input"`
	}
	if err = json.Unmarshal(messageRaw, &envelope); err != nil {
		t.Fatal(err)
	}
	internal := access.Internal(context.Background(), selected, "scenario-runner")
	if _, err = ingress.Observation(internal, envelope.Stamp, "different-hidden-actor", envelope.Input.TrackHint, envelope.Input); !apperr.Is(err, apperr.CodeConflict) {
		t.Fatalf("immutable actor mapping conflict %v", err)
	}
	envelope.Input.Position = &geo.Point{Lat: 0, Lng: 0}
	if _, err = ingress.Observation(internal, envelope.Stamp, "M2_PRIVATE_ACTOR", mappings[0]["contactRef"], envelope.Input); !apperr.Is(err, apperr.CodeConflict) {
		t.Fatalf("message identity conflict %v", err)
	}
	if _, err = ingress.Observation(scopeContext(t, f.h, f.resource), envelope.Stamp, "actor", "", envelope.Input); !apperr.Is(err, apperr.CodeForbidden) {
		t.Fatal("public source submitted synthetic input", err)
	}
}
