package realtime

import (
	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
)

// MapEnvelope converts an internal event into the versioned browser contract.
// Each case is intentionally explicit: event struct changes are a compile
// error here rather than a silent wire-format change.
func MapEnvelope(ev events.Event) Envelope {
	env := Envelope{
		ID:         ids.New("evt"),
		Type:       ev.Topic(),
		Version:    EnvelopeVersion,
		OccurredAt: ev.OccurredAt(),
		Data:       map[string]any{},
	}

	switch e := ev.(type) {
	case events.SourceCreated:
		env.Data = map[string]any{"sourceId": e.SourceID, "name": e.Name, "type": e.Type}
	case events.SourceUpdated:
		env.Data = map[string]any{"sourceId": e.SourceID, "status": e.Status}

	case events.ObservationReceived:
		env.Data = map[string]any{
			"observationId": e.ObservationID,
			"sourceId":      e.SourceID,
			"type":          e.Type,
			"observedAt":    e.ObservedAt,
			"receivedAt":    e.ReceivedAt,
			"trackHint":     e.TrackHint,
			"duplicate":     e.Duplicate,
			"retry":         e.Retry,
		}
		addPoint(env.Data, e.Position)
	case events.ObservationRejected:
		env.Data = map[string]any{
			"observationId": e.ObservationID,
			"sourceId":      e.SourceID,
			"trackHint":     e.TrackHint,
			"reason":        e.Reason,
		}

	case events.AssetCreated:
		env.Data = map[string]any{"assetId": e.AssetID, "name": e.Name, "type": e.Type}
	case events.AssetUpdated:
		env.Data = map[string]any{"assetId": e.AssetID, "status": e.Status}
	case events.AssetPositionUpdated:
		env.Data = map[string]any{
			"assetId":    e.AssetID,
			"observedAt": e.ObservedAt,
		}
		addPoint(env.Data, &e.Position)
		addFloat(env.Data, "speed", e.Speed)
		addFloat(env.Data, "heading", e.Heading)
	case events.AssetConnectionChanged:
		env.Data = map[string]any{
			"assetId":         e.AssetID,
			"connectionState": e.ConnectionState,
			"health":          e.Health,
		}
	case events.TelemetryReceived:
		env.Data = map[string]any{
			"telemetryId": e.TelemetryID,
			"assetId":     e.AssetID,
			"sourceId":    e.SourceID,
			"observedAt":  e.ObservedAt,
			"stale":       e.Stale,
			"duplicate":   e.Duplicate,
		}

	case events.TrackCreated:
		env.Data = map[string]any{
			"trackId":     e.TrackID,
			"externalRef": e.ExternalRef,
			"observedAt":  e.ObservedAt,
		}
		addPoint(env.Data, e.Position)
	case events.TrackUpdated:
		env.Data = map[string]any{
			"trackId":    e.TrackID,
			"observedAt": e.ObservedAt,
			"lastSeenAt": e.LastSeenAt,
		}
		addPoint(env.Data, e.Position)
		addFloat(env.Data, "speed", e.Speed)
		addFloat(env.Data, "heading", e.Heading)
	case events.TrackClosed:
		env.Data = map[string]any{"trackId": e.TrackID}

	case events.ClassificationCreated:
		env.Data = map[string]any{
			"classificationId": e.ClassificationID,
			"trackId":          e.TrackID,
			"label":            e.Label,
			"method":           e.Method,
		}
		addFloat(env.Data, "confidence", e.Confidence)

	case events.GeofenceCreated:
		env.Data = map[string]any{"geofenceId": e.GeofenceID, "name": e.Name, "type": e.Type}
	case events.GeofenceBreached:
		env.Data = map[string]any{
			"geofenceId":   e.GeofenceID,
			"geofenceName": e.GeofenceName,
			"geofenceType": e.GeofenceType,
			"severity":     e.Severity,
			"trackId":      e.TrackID,
		}
		addPoint(env.Data, &e.Position)
	case events.GeofenceExited:
		env.Data = map[string]any{"geofenceId": e.GeofenceID, "trackId": e.TrackID}
		addPoint(env.Data, &e.Position)

	case events.AlertCreated:
		env.Data = map[string]any{
			"alertId":    e.AlertID,
			"type":       e.Type,
			"severity":   e.Severity,
			"title":      e.Title,
			"trackId":    e.TrackID,
			"assetId":    e.AssetID,
			"geofenceId": e.GeofenceID,
		}
	case events.AlertAcknowledged:
		env.Data = map[string]any{"alertId": e.AlertID, "operator": e.Operator}
	case events.AlertResolved:
		env.Data = map[string]any{"alertId": e.AlertID, "operator": e.Operator}

	case events.IncidentCreated:
		env.Data = map[string]any{
			"incidentId": e.IncidentID,
			"title":      e.Title,
			"priority":   e.Priority,
			"status":     e.Status,
		}
	case events.IncidentUpdated:
		env.Data = map[string]any{
			"incidentId": e.IncidentID,
			"status":     e.Status,
			"priority":   e.Priority,
		}

	case events.MissionCreated:
		env.Data = map[string]any{
			"missionId": e.MissionID,
			"name":      e.Name,
			"status":    e.Status,
		}
	case events.MissionUpdated:
		env.Data = map[string]any{"missionId": e.MissionID, "status": e.Status}

	case events.CommandIssued:
		env.Data = map[string]any{
			"commandId": e.CommandID,
			"assetId":   e.AssetID,
			"missionId": e.MissionID,
			"type":      e.Type,
			"actor":     e.Actor,
		}
	case events.CommandStatusChanged:
		env.Data = map[string]any{
			"commandId":     e.CommandID,
			"assetId":       e.AssetID,
			"state":         e.State,
			"failureReason": e.FailureReason,
			"actor":         e.Actor,
		}

	case events.AssessmentCreated:
		env.Data = map[string]any{
			"assessmentId": e.AssessmentID,
			"subjectType":  e.SubjectType,
			"subjectId":    e.SubjectID,
			"type":         e.Type,
			"method":       e.Method,
		}

	case events.ScenarioRunStarted:
		env.Data = map[string]any{
			"runId":        e.RunID,
			"scenarioName": e.ScenarioName,
			"seed":         e.Seed,
		}
	case events.ScenarioRunUpdated:
		env.Data = map[string]any{"runId": e.RunID, "status": e.Status}
	case events.ScenarioRunEnded:
		env.Data = map[string]any{"runId": e.RunID, "status": e.Status}
	}

	return env
}

func addPoint(data map[string]any, p *geo.Point) {
	if p == nil {
		return
	}
	data["position"] = map[string]any{"lat": p.Lat, "lng": p.Lng}
}

func addFloat(data map[string]any, key string, v *float64) {
	if v == nil {
		return
	}
	data[key] = *v
}
