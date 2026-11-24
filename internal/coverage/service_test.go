package coverage

import (
	"context"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/analystscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/queryscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
)

func testCtxWithPermission(perms ...string) context.Context {
	res := access.Resource{WorkspaceID: "default", EnvironmentID: "local", Partition: access.PublicEvidence}
	p := access.Principal{
		ID:      "human:operator-01",
		Kind:    "human",
		HumanID: "operator-01",
		Grants:  []access.Grant{{Resource: res, Permissions: perms}},
	}
	ctx, _ := access.Bind(context.Background(), p, res, nil)
	return ctx
}

func TestProfileActionDistinguishesMonitoringPollingAndHistoricalReplay(t *testing.T) {
	now := time.Date(2026, 11, 24, 9, 0, 0, 0, time.UTC)
	monitoring, err := queryscope.Evaluate(queryscope.Temporal{Mode: "monitoring", Timezone: "Asia/Riyadh"}, now)
	if err != nil {
		t.Fatal(err)
	}
	historical, err := queryscope.Evaluate(queryscope.Temporal{
		Mode:         "historical",
		Interval:     &queryscope.Interval{From: now.Add(-48 * time.Hour), To: now.Add(-24 * time.Hour)},
		RevisionView: "latest",
		Timezone:     "UTC",
	}, now)
	if err != nil {
		t.Fatal(err)
	}

	ctx := testCtxWithPermission("sources.read", "acquisitions.request")
	pollingPolicy := publicingest.AcquisitionPolicy{
		Gate:             "ready",
		ExecutionGranted: true,
		Capability:       publicingest.Capability(publicingest.METARAdapter),
	}
	histPolicy := publicingest.AcquisitionPolicy{
		Gate:             "ready",
		ExecutionGranted: true,
		Capability:       publicingest.Capability(publicingest.USGSAdapter),
	}

	// 1. Current-polling adapter in Monitoring mode with full coverage must NOT claim historical replay failure.
	kind, reason, owner := profileAction(ctx, analystscope.Snapshot{
		Area:     areas.Revision{Kind: "drawn"},
		Temporal: monitoring,
	}, pollingPolicy, "full", false)
	if kind == "choose_compatible_dates" {
		t.Fatalf("monitoring mode emitted choose_compatible_dates for current-polling adapter: %s", reason)
	}
	if owner != "connector operator" {
		t.Fatalf("unexpected owner for current-polling adapter in monitoring mode: %s", owner)
	}

	// 2. Current-polling adapter in Historical mode MUST warn analyst to choose compatible dates.
	kind, _, owner = profileAction(ctx, analystscope.Snapshot{
		Area:     areas.Revision{Kind: "drawn"},
		Temporal: historical,
	}, pollingPolicy, "full", true)
	if kind != "choose_compatible_dates" || owner != "analyst" {
		t.Fatalf("historical mode should instruct analyst to choose compatible dates, got kind=%s owner=%s", kind, owner)
	}

	// 3. Disjoint area says outside rather than exceeds.
	kind, reason, owner = profileAction(ctx, analystscope.Snapshot{
		Area:     areas.Revision{Kind: "drawn"},
		Temporal: monitoring,
	}, histPolicy, "none", false)
	if kind != "request_coverage_review" || owner != "source administrator" || reason != "The selected area is outside this approved profile." {
		t.Fatalf("unexpected disjoint action: kind=%s owner=%s reason=%q", kind, owner, reason)
	}

	// 4. World discovery scope guides analyst to select a bounded area instead of failing.
	kind, _, owner = profileAction(ctx, analystscope.Snapshot{
		Area:     areas.Revision{Kind: "world"},
		Temporal: monitoring,
	}, histPolicy, "configured_footprint", false)
	if kind != "select_area" || owner != "analyst" {
		t.Fatalf("unexpected world action: kind=%s owner=%s", kind, owner)
	}

	// 5. Gated profile takes precedence over area/historical notices.
	gated := histPolicy
	gated.Gate = "NEEDS_CREDENTIALS"
	kind, _, owner = profileAction(ctx, analystscope.Snapshot{
		Area:     areas.Revision{Kind: "drawn"},
		Temporal: historical,
	}, gated, "none", true)
	if kind != "inspect_source" || owner != "source administrator" {
		t.Fatalf("gated profile should report prerequisite review first: kind=%s owner=%s", kind, owner)
	}

	// 6. Full overlap + ready + execution granted offers request_acquisition.
	kind, _, owner = profileAction(ctx, analystscope.Snapshot{
		Area:     areas.Revision{Kind: "drawn"},
		Temporal: historical,
	}, histPolicy, "full", true)
	if kind != "request_acquisition" || owner != "analyst" {
		t.Fatalf("full overlap ready profile should offer request_acquisition: kind=%s owner=%s", kind, owner)
	}
}

func TestEvaluateUnadmittedSourceAvoidsUnknownQueryFailure(t *testing.T) {
	ctx := testCtxWithPermission("sources.read")
	v := Source{
		ID:                      "scoped__source__local-ais",
		Name:                    "External vessel observations · AISStream",
		Family:                  "unknown",
		Support:                 "not_admitted",
		SpatialRelation:         "unknown",
		Retrieval:               "no_retrieval",
		DataResult:              "no_retrieval",
		Freshness:               "unknown",
		ObservationCompleteness: "unknown",
	}
	meta := map[string]any{
		"activation": map[string]any{
			"state":      "NEEDS_PERMISSION",
			"nextAction": "Record AIS use/retention permission and a private API key file.",
		},
	}
	evaluateUnadmittedSource(ctx, nil, v.ID, meta, &v)
	if v.Support != "not_admitted" || v.DataResult != "not_admitted" || v.Retrieval != "not_admitted" {
		t.Fatalf("unadmitted source should not degrade to unavailable_query: %+v", v)
	}
	if len(v.MissingWindows) != 0 {
		t.Fatalf("unadmitted source should not report missing polling windows: %+v", v.MissingWindows)
	}
	if len(v.Actions) != 1 || v.Actions[0].Reason != "Record AIS use/retention permission and a private API key file." {
		t.Fatalf("expected activation guidance action, got %+v", v.Actions)
	}
}

func TestExtractCoverageBoundsValidatesEnvelope(t *testing.T) {
	valid := extractCoverageBounds(map[string]any{
		"coverage": map[string]any{"west": 34.0, "south": 16.0, "east": 56.0, "north": 33.0},
	})
	if len(valid) != 1 || valid[0].West != 34.0 || valid[0].North != 33.0 {
		t.Fatalf("expected valid coverage envelope, got %+v", valid)
	}
	invalid := extractCoverageBounds(map[string]any{
		"coverage": map[string]any{"west": 56.0, "south": 16.0, "east": 34.0, "north": 33.0},
	})
	if len(invalid) != 0 {
		t.Fatalf("expected invalid inverted bbox to be rejected, got %+v", invalid)
	}
}
