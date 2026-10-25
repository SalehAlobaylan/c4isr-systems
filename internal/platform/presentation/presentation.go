// Package presentation defines the ordinary exercise transport view. Stored
// evidence is retained; HTTP and realtime share this explicit disclosure policy.
package presentation

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

const ObservationPrefix = "view_obs_"

func IsObservationAlias(id string) bool {
	if len(id) != len(ObservationPrefix)+32 || !strings.HasPrefix(id, ObservationPrefix) {
		return false
	}
	_, err := hex.DecodeString(id[len(ObservationPrefix):])
	return err == nil
}

// ViewResolver resolves a batch inside the selected resource. Missing entries
// represent rejected inputs without stored evidence; their IDs are not published.
type ViewResolver func(context.Context, []string) (map[string]string, error)
type viewKey struct{}

func WithViewResolver(ctx context.Context, resolve ViewResolver) context.Context {
	return context.WithValue(ctx, viewKey{}, resolve)
}
func observationID(id string, aliases map[string]string) string {
	// Even an identifier shaped like an alias must be confirmed by the
	// scoped registry. Legacy primary IDs can themselves use that prefix.
	return aliases[id]
}

type resolverKey struct{}
type trackResolverKey struct{}
type Resolver func(context.Context, string) (string, error)

func WithTrackResolver(ctx context.Context, resolve Resolver) context.Context {
	return context.WithValue(ctx, trackResolverKey{}, resolve)
}
func resolveTrackReferences(ctx context.Context, v any) error {
	switch x := v.(type) {
	case []any:
		for _, child := range x {
			if err := resolveTrackReferences(ctx, child); err != nil {
				return err
			}
		}
	case map[string]any:
		field := ""
		if x["subjectType"] == "track" {
			field = "subjectId"
		}
		if x["type"] == "track" && len(x) <= 3 {
			field = "id"
		}
		if id, ok := x[field].(string); ok && id != "" && !strings.HasPrefix(id, "trk_") {
			resolve, ok := ctx.Value(trackResolverKey{}).(Resolver)
			if !ok {
				return apperr.Forbidden("canonical track reference required")
			}
			canonical, err := resolve(ctx, id)
			if err != nil {
				return err
			}
			x[field] = canonical
		}
		for key, child := range x {
			if key != "metadata" && key != "payload" && key != "data" && key != "quality" {
				if err := resolveTrackReferences(ctx, child); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func WithResolver(ctx context.Context, resolve Resolver) context.Context {
	return context.WithValue(ctx, resolverKey{}, resolve)
}
func ResolveObservation(ctx context.Context, id string) (string, error) {
	if !IsObservationAlias(id) {
		return id, nil
	}
	if resolve, ok := ctx.Value(resolverKey{}).(Resolver); ok {
		return resolve(ctx, id)
	}
	return id, nil
}

// ResolveInput recognizes typed observation references, never arbitrary payload
// strings. Alias resolution runs through the selected resource's restricted DB.
func ResolveInput(ctx context.Context, dst any, observationIngress ...bool) error {
	if _, ok := ctx.Value(resolverKey{}).(Resolver); !ok {
		return nil
	}
	raw, err := json.Marshal(dst)
	if err != nil {
		return err
	}
	var v any
	if err = json.Unmarshal(raw, &v); err != nil {
		return err
	}
	if len(observationIngress) > 0 && observationIngress[0] {
		if root, ok := v.(map[string]any); ok {
			if id, ok := root["id"].(string); ok {
				actual, err := ResolveObservation(ctx, id)
				if err != nil {
					return err
				}
				root["id"] = actual
			}
		}
	}
	var walk func(any) error
	walk = func(v any) error {
		switch x := v.(type) {
		case []any:
			for _, child := range x {
				if err := walk(child); err != nil {
					return err
				}
			}
		case map[string]any:
			for key, value := range x {
				resolve := key == "observationId" || (key == "subjectId" && x["subjectType"] == "observation") || (key == "id" && (x["kind"] == "observation" || x["type"] == "observation"))
				if id, ok := value.(string); ok && resolve {
					actual, err := ResolveObservation(ctx, id)
					if err != nil {
						return err
					}
					x[key] = actual
				}
				if key == "observationIds" {
					if ids, ok := value.([]any); ok {
						for i, id := range ids {
							if id, ok := id.(string); ok {
								actual, err := ResolveObservation(ctx, id)
								if err != nil {
									return err
								}
								ids[i] = actual
							}
						}
					}
				}
				// These maps are retained input, not typed domain references.
				if key != "payload" && key != "metadata" && key != "quality" {
					if err := walk(value); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	if err = walk(v); err != nil {
		return err
	}
	raw, err = json.Marshal(v)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dst)
}

// Encode preserves raw public evidence. Exercise output admits only declared
// operational fields in extensible maps and never exposes correlation hints.
func Encode(ctx context.Context, path string, value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	r, err := access.ResourceFrom(ctx)
	if err != nil || r.Partition != access.ExerciseObservation {
		return raw, nil
	}
	var v any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err = decoder.Decode(&v); err != nil {
		return nil, err
	}
	if err = resolveTrackReferences(ctx, v); err != nil {
		return nil, err
	}
	author := access.Allows(ctx, "sim.scenarios.author")
	if author && strings.HasPrefix(path, "/api/v1/scenarios/runs/") && strings.HasSuffix(path, "/version") {
		return raw, nil
	}
	kind := ""
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) > 2 {
		kind = parts[2]
	}
	ids := map[string]bool{}
	collectObservationIDs(v, kind, ids)
	aliases := map[string]string{}
	if len(ids) > 0 {
		if resolve, ok := ctx.Value(viewKey{}).(ViewResolver); ok {
			keys := make([]string, 0, len(ids))
			for id := range ids {
				keys = append(keys, id)
			}
			aliases, err = resolve(ctx, keys)
			if err != nil {
				return nil, err
			}
		}
	}
	project(v, r, kind, author, aliases)
	return json.Marshal(v)
}

func project(v any, r access.Resource, kind string, author bool, aliases map[string]string) {
	switch x := v.(type) {
	case []any:
		for _, child := range x {
			project(child, r, kind, author, aliases)
		}
	case map[string]any:
		title := exerciseAlertTitle(x)
		if id, ok := x["id"].(string); ok && (kind == "observations" || x["type"] == "observation" || (x["sourceId"] != nil && x["observedAt"] != nil && x["receivedAt"] != nil && x["assetId"] == nil)) {
			x["id"] = observationID(id, aliases)
		}
		if x["subjectType"] == "observation" {
			if id, ok := x["subjectId"].(string); ok {
				x["subjectId"] = observationID(id, aliases)
			}
		}
		for key, value := range x {
			switch key {
			case "observationId", "initialObservationId":
				if id, ok := value.(string); ok {
					x[key] = observationID(id, aliases)
				}
			case "trackHint", "externalRef":
				x[key] = ""
			case "payload":
				if kind == "commands" {
					x[key] = safeMap(value, r, aliases)
				} else {
					x[key] = map[string]any{}
				}
			case "metadata", "quality":
				// Registration metadata/quality comes from callers. Even a key
				// named sourceId is not a validated reference in these maps.
				x[key] = safeAttributes(value, r)
				if key == "metadata" && kind == "tracks" {
					x[key] = safeMap(value, r, aliases)
				}
			case "data", "sourceReference", "reference":
				x[key] = safeMap(value, r, aliases)
			case "geofenceName":
				x[key] = "Exercise area"
			case "name":
				if kind == "sources" {
					x[key] = "Exercise source"
				}
				if kind == "geofences" {
					x[key] = "Exercise area"
				}
			case "title":
				if kind == "alerts" {
					x[key] = title
				}
			case "message":
				if kind == "alerts" {
					x[key] = "Reported evidence matched Exercise area"
				}
			case "type":
				if kind == "observations" {
					x[key] = "synthetic.observation"
				}
			case "error", "actionError", "failureReason", "reason":
				if text, ok := value.(string); ok && text != "" {
					x[key] = "Operation did not complete"
				}
			case "seed", "scenarioName", "resourceNamespace", "lastAction", "lastActionAtMs":
				if kind == "scenarios" && !author {
					delete(x, key)
				}
			default:
				childKind := kind
				if key == "observations" || key == "geofences" || key == "alerts" {
					childKind = key
				}
				project(value, r, childKind, author, aliases)
			}
		}
	}
}

func safeAttributes(value any, r access.Resource) map[string]any {
	x := safeMap(value, r, nil)
	for key, value := range x {
		switch value.(type) {
		case json.Number, bool:
		default:
			delete(x, key)
		}
	}
	return x
}

// An allowlist prevents new/nested metadata from silently becoming public.
// Numeric quality fields cannot carry a hidden actor's string identity.
func safeMap(value any, r access.Resource, aliases map[string]string) map[string]any {
	out := map[string]any{}
	x, ok := value.(map[string]any)
	if !ok {
		return out
	}
	for key, value := range x {
		switch key {
		case "confidence", "accuracyM", "horizontalAccuracyM", "verticalAccuracyM", "observationCount", "overlapRatio", "areaMeters2", "overlapM2", "subjectConfidence":
			if _, ok := value.(json.Number); ok {
				out[key] = value
			}
		case "synthetic", "duplicate", "retry", "stale":
			if _, ok := value.(bool); ok {
				out[key] = value
			}
		case "scenarioRunId":
			if value == r.RunID {
				out[key] = r.RunID
			}
		case "target":
			if point, ok := value.(map[string]any); ok {
				safe := map[string]any{}
				for _, coordinate := range []string{"lat", "lng", "altitudeM"} {
					if n, ok := point[coordinate].(json.Number); ok {
						safe[coordinate] = n
					}
				}
				out[key] = safe
			}
		case "sourceId", "trackId", "assetId", "geofenceId", "incidentId", "missionId", "commandId", "assessmentId":
			if id, ok := value.(string); ok {
				out[key] = id
			}
		case "observationId":
			if id, ok := value.(string); ok {
				out[key] = observationID(id, aliases)
			}
		}
	}
	return out
}

func exerciseAlertTitle(x map[string]any) string {
	if x["type"] == "geofence.area.overlap" {
		return "Reported area overlaps Exercise area"
	}
	ref, _ := x["sourceReference"].(map[string]any)
	label, _ := ref["subjectLabel"].(string)
	switch label {
	case "vehicle", "aircraft", "vessel", "person", "helicopter", "uav":
		if confirmed, _ := ref["subjectConfirmed"].(bool); !confirmed {
			return "Suspected " + label + " entered Exercise area"
		}
		return strings.ToUpper(label[:1]) + label[1:] + " entered Exercise area"
	}
	return "Observed contact entered Exercise area"
}

func collectObservationIDs(v any, kind string, ids map[string]bool) {
	switch x := v.(type) {
	case []any:
		for _, child := range x {
			collectObservationIDs(child, kind, ids)
		}
	case map[string]any:
		if id, ok := x["id"].(string); ok && (kind == "observations" || x["type"] == "observation" || (x["sourceId"] != nil && x["observedAt"] != nil && x["receivedAt"] != nil && x["assetId"] == nil)) {
			ids[id] = true
		}
		if x["subjectType"] == "observation" {
			if id, ok := x["subjectId"].(string); ok {
				ids[id] = true
			}
		}
		for key, value := range x {
			if key == "observationId" || key == "initialObservationId" {
				if id, ok := value.(string); ok {
					ids[id] = true
				}
			}
			if key == "payload" || key == "quality" || key == "metadata" {
				continue
			}
			childKind := kind
			if key == "observations" || key == "geofences" || key == "alerts" {
				childKind = key
			}
			collectObservationIDs(value, childKind, ids)
		}
	}
}
