package simgateway

import (
	"context"
	"encoding/json"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
)

var operatorVisibleSalvoEvents = map[string]bool{
	"interceptor_launch": true,
	"airspace_hold_fire": true,
	"operator_command":   true,
}

// CanReadDebriefTruth enforces EMAD's 4-partition security boundary on the Go
// wire layer. Principals explicitly granted sim.truth.read always pass.
// Principals operating in an EXERCISE_OBSERVATION partition without
// sim.truth.read are ALWAYS denied (even in local development).
// Only in development/test (allowLocalMapDebrief=true) may a PUBLIC_EVIDENCE
// map analyst toggle into debrief mode for local scenario design.
func CanReadDebriefTruth(ctx context.Context, allowLocalMapDebrief bool) bool {
	if err := access.Require(ctx, "sim.truth.read"); err == nil {
		return true
	}
	principal, resource, ok := access.From(ctx)
	if !ok {
		return allowLocalMapDebrief
	}
	for _, g := range principal.Grants {
		for _, perm := range g.Permissions {
			if perm == "sim.truth.read" {
				return true
			}
		}
	}
	if resource.Partition == access.ExerciseObservation {
		return false
	}
	if allowLocalMapDebrief && resource.Partition == access.PublicEvidence {
		return true
	}
	return false
}

// FilterReplayJSON enforces wire-level EXERCISE_OBSERVATION vs EXERCISE_TRUTH
// projection in Go so sealed truth fields cannot leave c4isr-server under
// view="operator", regardless of whether bytes came from S3 or the Python hub.
func FilterReplayJSON(engine string, raw []byte, view string) ([]byte, error) {
	if engine != "sim" && engine != "salvo" {
		return raw, nil
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	if view != "operator" {
		doc["viewScope"] = "EXERCISE_TRUTH"
		doc["truthRedacted"] = false
		return json.Marshal(doc)
	}

	doc["viewScope"] = "EXERCISE_OBSERVATION"
	doc["truthRedacted"] = true
	doc["comparison"] = nil

	if engine == "sim" {
		if draft, ok := doc["sourceDraft"].(map[string]any); ok {
			delete(draft, "threat")
		}
		if runs, ok := doc["runs"].(map[string]any); ok {
			for pol, item := range runs {
				run, ok := item.(map[string]any)
				if !ok {
					continue
				}
				var ownInterceptor any = []any{}
				if truth, ok := run["truth"].(map[string]any); ok {
					if traj, exists := truth["interceptorTrajectory"]; exists && traj != nil {
						ownInterceptor = traj
					}
				}
				delete(run, "truth")
				run["ownForceTelemetry"] = map[string]any{
					"scope":                 "EXERCISE_OBSERVATION",
					"interceptorTrajectory": ownInterceptor,
				}
				run["threatKind"] = "unknown_contact"
				run["evaluation"] = nil
				if tl, ok := run["timelineEvents"].(map[string]any); ok {
					delete(tl, "firstClearLosS")
					delete(tl, "interceptS")
					delete(tl, "lastImpactS")
				}
				runs[pol] = run
			}
		}
		return json.Marshal(doc)
	}

	// engine == "salvo"
	if scen, ok := doc["scenario"].(map[string]any); ok {
		delete(scen, "threats")
	}
	if draft, ok := doc["sourceDraft"].(map[string]any); ok {
		delete(draft, "raid")
	}
	if runs, ok := doc["runs"].(map[string]any); ok {
		for pol, item := range runs {
			run, ok := item.(map[string]any)
			if !ok {
				continue
			}
			ownInts := []map[string]any{}
			if truth, ok := run["truth"].(map[string]any); ok {
				if rawInts, ok := truth["interceptors"].([]any); ok {
					for _, ri := range rawInts {
						im, ok := ri.(map[string]any)
						if !ok {
							continue
						}
						ownInts = append(ownInts, map[string]any{
							"id":          im["id"],
							"batteryId":   im["batteryId"],
							"batteryTier": im["batteryTier"],
							"trackId":     im["trackId"],
							"launchS":     im["launchS"],
							"endS":        im["endS"],
							"costKUsd":    im["costKUsd"],
							"samples":     im["samples"],
						})
					}
				}
			}
			ownBatts := []map[string]any{}
			if evObj, ok := run["evaluation"].(map[string]any); ok {
				if rawBatts, ok := evObj["batteries"].([]any); ok {
					for _, rb := range rawBatts {
						bm, ok := rb.(map[string]any)
						if !ok {
							continue
						}
						ownBatts = append(ownBatts, map[string]any{
							"id":                bm["id"],
							"tier":              bm["tier"],
							"label":             bm["label"],
							"color":             bm["color"],
							"enu":               bm["enu"],
							"maxRangeM":         bm["maxRangeM"],
							"magazineInitial":   bm["magazineInitial"],
							"magazineRemaining": bm["magazineRemaining"],
							"shotsFired":        bm["shotsFired"],
							"costKUsd":          bm["costKUsd"],
						})
					}
				}
			}
			delete(run, "truth")
			run["ownForceTelemetry"] = map[string]any{
				"scope":        "EXERCISE_OBSERVATION",
				"interceptors": ownInts,
				"batteries":    ownBatts,
			}
			run["evaluation"] = nil
			if rawEvents, ok := run["events"].([]any); ok {
				kept := make([]any, 0, len(rawEvents))
				for _, re := range rawEvents {
					em, ok := re.(map[string]any)
					if !ok {
						continue
					}
					if kind, _ := em["kind"].(string); operatorVisibleSalvoEvents[kind] {
						kept = append(kept, em)
					}
				}
				run["events"] = kept
			}
			runs[pol] = run
		}
	}
	return json.Marshal(doc)
}
