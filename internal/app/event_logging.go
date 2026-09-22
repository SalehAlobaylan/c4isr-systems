package app

import (
	"context"
	"log/slog"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/runctx"
)

// logDomainEvent gives centralized logs the same provenance dimensions as the
// audit stream. IDs are attributes, never metric labels, so operators can
// correlate one operational path without expanding Prometheus cardinality.
func logDomainEvent(logger *slog.Logger, ctx context.Context, ev events.Event) {
	if ev == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		logger = slog.Default()
	}

	requestID := httpx.GetRequestID(ctx)
	traceID := httpx.GetTraceID(ctx)
	correlationID := requestID
	if correlationID == "" {
		correlationID = traceID
	}
	scope, hasScope := runctx.ScopeFrom(ctx)
	if correlationID == "" && hasScope {
		correlationID = scope.RunID
	}
	attrs := []slog.Attr{
		slog.String("event", ev.Topic()),
		slog.Time("occurred_at", ev.OccurredAt()),
		slog.String("request_id", requestID),
		slog.String("trace_id", traceID),
		slog.String("correlation_id", correlationID),
		slog.String("operator_id", httpx.GetOperatorID(ctx)),
		slog.String("operator_role", httpx.GetOperatorRole(ctx)),
	}
	if hasScope {
		attrs = append(attrs,
			slog.String("scenario_id", scope.RunID),
			slog.String("scenario_run_id", scope.RunID),
			slog.String("resource_namespace", scope.ResourceNamespace),
		)
	}
	attrs = append(attrs, eventIdentityAttrs(ev)...)
	logger.LogAttrs(ctx, slog.LevelInfo, "domain event", attrs...)
}

func eventIdentityAttrs(ev events.Event) []slog.Attr {
	attrs := make([]slog.Attr, 0, 8)
	stringAttr := func(key, value string) {
		if value != "" {
			attrs = append(attrs, slog.String(key, value))
		}
	}

	switch e := ev.(type) {
	case events.SourceCreated:
		stringAttr("source_id", e.SourceID)
	case events.SourceUpdated:
		stringAttr("source_id", e.SourceID)
	case events.ObservationReceived:
		stringAttr("observation_id", e.ObservationID)
		stringAttr("source_id", e.SourceID)
		stringAttr("track_id", e.TrackHint)
	case events.ObservationRejected:
		stringAttr("observation_id", e.ObservationID)
		stringAttr("source_id", e.SourceID)
		stringAttr("track_reference", e.TrackHint)
		stringAttr("error", e.Reason)
	case events.AssetCreated:
		stringAttr("asset_id", e.AssetID)
	case events.AssetUpdated:
		stringAttr("asset_id", e.AssetID)
	case events.AssetPositionUpdated:
		stringAttr("asset_id", e.AssetID)
	case events.AssetConnectionChanged:
		stringAttr("asset_id", e.AssetID)
	case events.TelemetryReceived:
		stringAttr("telemetry_id", e.TelemetryID)
		stringAttr("asset_id", e.AssetID)
		stringAttr("source_id", e.SourceID)
	case events.TrackCreated:
		stringAttr("track_id", e.TrackID)
		stringAttr("track_reference", e.ExternalRef)
	case events.TrackUpdated:
		stringAttr("track_id", e.TrackID)
	case events.TrackClosed:
		stringAttr("track_id", e.TrackID)
	case events.ClassificationCreated:
		stringAttr("classification_id", e.ClassificationID)
		stringAttr("track_id", e.TrackID)
	case events.GeofenceCreated:
		stringAttr("geofence_id", e.GeofenceID)
	case events.GeofenceBreached:
		stringAttr("geofence_id", e.GeofenceID)
		stringAttr("track_id", e.TrackID)
	case events.GeofenceExited:
		stringAttr("geofence_id", e.GeofenceID)
		stringAttr("track_id", e.TrackID)
	case events.AlertCreated:
		stringAttr("alert_id", e.AlertID)
		stringAttr("track_id", e.TrackID)
		stringAttr("asset_id", e.AssetID)
		stringAttr("geofence_id", e.GeofenceID)
	case events.AlertAcknowledged:
		stringAttr("alert_id", e.AlertID)
	case events.AlertResolved:
		stringAttr("alert_id", e.AlertID)
	case events.IncidentCreated:
		stringAttr("incident_id", e.IncidentID)
	case events.IncidentUpdated:
		stringAttr("incident_id", e.IncidentID)
	case events.MissionCreated:
		stringAttr("mission_id", e.MissionID)
	case events.MissionUpdated:
		stringAttr("mission_id", e.MissionID)
	case events.CommandIssued:
		stringAttr("command_id", e.CommandID)
		stringAttr("asset_id", e.AssetID)
		stringAttr("mission_id", e.MissionID)
	case events.CommandStatusChanged:
		stringAttr("command_id", e.CommandID)
		stringAttr("asset_id", e.AssetID)
	case events.AssessmentCreated:
		stringAttr("assessment_id", e.AssessmentID)
		stringAttr("subject_id", e.SubjectID)
	case events.ScenarioRunStarted:
		stringAttr("scenario_id", e.RunID)
		stringAttr("scenario_run_id", e.RunID)
	case events.ScenarioRunUpdated:
		stringAttr("scenario_id", e.RunID)
		stringAttr("scenario_run_id", e.RunID)
	case events.ScenarioRunEnded:
		stringAttr("scenario_id", e.RunID)
		stringAttr("scenario_run_id", e.RunID)
	}
	return attrs
}
