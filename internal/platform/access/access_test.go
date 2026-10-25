package access

import (
	"context"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

func TestResourceGrantsDoNotImplyOtherAuthority(t *testing.T) {
	r := Resource{"one", "lab", ExerciseObservation, "run-a"}
	denials := []Denial{}
	p := Principal{ID: "runner", Kind: "service", ExpiresAt: time.Now().Add(time.Hour), Grants: []Grant{{Resource: r, Permissions: []string{"observations.ingest"}, SourceIDs: []string{"sensor-a"}}}}
	ctx, err := Bind(context.Background(), p, r, func(_ context.Context, d Denial) { denials = append(denials, d) })
	if err != nil {
		t.Fatal(err)
	}
	if err := Require(ctx, "observations.ingest"); err != nil {
		t.Fatal(err)
	}
	for _, permission := range []string{"commands.issue", "admin.manage", "sim.truth.read"} {
		if !apperr.Is(Require(ctx, permission), apperr.CodeForbidden) {
			t.Fatalf("permission escaped grant: %s", permission)
		}
	}
	if err := RequireSource(ctx, "sensor-a"); err != nil {
		t.Fatal(err)
	}
	if !apperr.Is(RequireSource(ctx, "sensor-b"), apperr.CodeForbidden) {
		t.Fatal("foreign source allowed")
	}
	for _, other := range []Resource{{"two", "lab", ExerciseObservation, "run-a"}, {"one", "other", ExerciseObservation, "run-a"}, {"one", "lab", ExerciseObservation, "run-b"}, {"one", "lab", ExerciseTruth, "run-a"}} {
		if _, err := Rebind(ctx, other); !apperr.Is(err, apperr.CodeForbidden) {
			t.Fatalf("foreign resource allowed: %+v (%v)", other, err)
		}
	}
	if len(denials) != 8 {
		t.Fatalf("denials=%d", len(denials))
	}
}

func TestPublicAndTruthBoundaries(t *testing.T) {
	r := Resource{"one", "world", PublicEvidence, ""}
	p := Principal{ID: "analyst", Kind: "human", Grants: []Grant{{Resource: r, Permissions: []string{"observations.read", "commands.issue"}}}}
	ctx, err := Bind(context.Background(), p, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := Require(ctx, "observations.read"); err != nil {
		t.Fatal(err)
	}
	if !apperr.Is(Require(ctx, "commands.issue"), apperr.CodeForbidden) {
		t.Fatal("public command accepted")
	}
	truth := Resource{"one", "lab", ExerciseTruth, "run-a"}
	p.Grants = []Grant{{Resource: truth, Permissions: []string{"admin.manage", "observations.read"}}}
	if _, err := Bind(context.Background(), p, truth, nil); !apperr.Is(err, apperr.CodeForbidden) {
		t.Fatal("administration implicitly granted truth")
	}
	if _, err := ResourceFrom(Internal(context.Background(), truth, "test")); !apperr.Is(err, apperr.CodeUnauthorized) {
		t.Fatal("background authority implicitly granted truth")
	}
	if _, err := ResourceFrom(Internal(ctx, truth, "test")); !apperr.Is(err, apperr.CodeUnauthorized) {
		t.Fatal("denied truth context inherited ordinary publication authority")
	}
	background := Internal(context.Background(), LegacyResource(), "test")
	for _, permission := range []string{"admin.manage", "platform.maintenance", "sim.evaluation.read", "sim.truth.grant", "unknown.permission"} {
		if !apperr.Is(Require(background, permission), apperr.CodeForbidden) {
			t.Fatalf("projection implied privileged authority: %s", permission)
		}
	}
}

func TestExpiryAndRequiredContext(t *testing.T) {
	r := LegacyResource()
	p := Principal{ID: "expired", Kind: "service", ExpiresAt: time.Now().Add(-time.Second), Grants: []Grant{{Resource: r, Permissions: []string{"observations.read"}}}}
	if _, err := Bind(context.Background(), p, r, nil); !apperr.Is(err, apperr.CodeUnauthorized) {
		t.Fatal("expired credential accepted")
	}
	if !apperr.Is(Require(context.Background(), "observations.read"), apperr.CodeUnauthorized) {
		t.Fatal("missing authentication accepted")
	}
	for _, r := range []Resource{{"one", "lab", PublicEvidence, "run"}, {"one", "lab", ExerciseObservation, ""}, {"one/other", "lab", PublicEvidence, ""}, {"one", "lab", "QUARANTINE", ""}} {
		if r.Validate() == nil {
			t.Fatalf("invalid resource accepted: %+v", r)
		}
	}
}

func TestRefreshedContextRetainsLiveAuthorizationChecks(t *testing.T) {
	r := LegacyResource()
	p := Principal{ID: "worker", Kind: "service", Grants: []Grant{{Resource: r, Permissions: []string{"operational.read"}}}}
	bound, err := Bind(context.Background(), p, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	checks := 0
	bound = WithRefresh(bound, func(ctx context.Context) (context.Context, error) {
		checks++
		if checks > 1 {
			return nil, apperr.Unauthorized("credential revoked during work")
		}
		return Bind(ctx, p, r, nil)
	})
	fresh, err := Refresh(bound)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Refresh(fresh); !apperr.Is(err, apperr.CodeUnauthorized) || checks != 2 {
		t.Fatal("refresh reused cached grants after the first check", checks, err)
	}
}
