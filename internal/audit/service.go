package audit

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/runctx"
)

// DefaultLimit and MaxLimit bound list queries.
const (
	DefaultLimit = 100
	MaxLimit     = 1000
)

// Service records and queries the system's memory.
type Service struct {
	repo Repository
	bus  *events.Dispatcher
}

// NewService wires an audit service and subscribes it to every domain topic.
func NewService(repo Repository, bus *events.Dispatcher) *Service {
	s := &Service{repo: repo, bus: bus}
	s.SubscribeAll(bus)
	return s
}

// SubscribeAll subscribes Handle to every domain event topic.
func (s *Service) SubscribeAll(bus *events.Dispatcher) {
	for _, topic := range []string{
		events.TopicSourceCreated,
		events.TopicSourceUpdated,
		events.TopicObservationReceived,
		events.TopicObservationRejected,
		events.TopicAssetCreated,
		events.TopicAssetUpdated,
		events.TopicAssetPositionUpdated,
		events.TopicAssetConnectionChanged,
		events.TopicTelemetryReceived,
		events.TopicTrackCreated,
		events.TopicTrackUpdated,
		events.TopicTrackClosed,
		events.TopicClassificationCreated,
		events.TopicGeofenceCreated,
		events.TopicGeofenceBreached,
		events.TopicGeofenceExited,
		events.TopicAlertCreated,
		events.TopicAlertAcknowledged,
		events.TopicAlertResolved,
		events.TopicIncidentCreated,
		events.TopicIncidentUpdated,
		events.TopicMissionCreated,
		events.TopicMissionUpdated,
		events.TopicCommandIssued,
		events.TopicCommandStatusChanged,
		events.TopicAssessmentCreated,
		events.TopicScenarioRunStarted,
		events.TopicScenarioRunUpdated,
		events.TopicScenarioRunEnded,
	} {
		bus.Subscribe(topic, s.Handle)
	}
}

// Handle maps one domain event to an audit entry. Failures are logged and
// swallowed: auditing must never break ingestion.
func (s *Service) Handle(ctx context.Context, ev events.Event) {
	if ev == nil {
		return
	}
	entry := entryFromEvent(ev)
	entry.CorrelationID = httpx.GetRequestID(ctx)
	if entry.Data == nil {
		entry.Data = map[string]any{}
	}
	if traceID := httpx.GetTraceID(ctx); traceID != "" {
		entry.Data["traceId"] = traceID
	}
	if scope, ok := runctx.ScopeFrom(ctx); ok {
		entry.Data["scenarioRunId"] = scope.RunID
		entry.Data["resourceNamespace"] = scope.ResourceNamespace
		if entry.ActorID == "" {
			entry.ActorType = ActorScenario
			entry.ActorID = scope.ActorID()
		}
	}
	if _, err := s.Append(ctx, entry); err != nil {
		slog.Default().Error("audit event not recorded", "topic", ev.Topic(), "error", err)
	}
}

// Append records an explicit entry, filling id and occurrence time when absent.
func (s *Service) Append(ctx context.Context, entry Entry) (Entry, error) {
	if entry.ID == "" {
		entry.ID = ids.New("aud")
	}
	if entry.OccurredAt.IsZero() {
		entry.OccurredAt = time.Now().UTC()
	}
	if entry.ActorType == "" {
		entry.ActorType = ActorSystem
	}
	if entry.Data == nil {
		entry.Data = map[string]any{}
	}
	return s.repo.Append(ctx, entry)
}

// List returns audit entries matching the filter, newest first.
func (s *Service) List(ctx context.Context, filter ListFilter, limit, offset int) ([]Entry, int, error) {
	limit = clampLimit(limit)
	if offset < 0 {
		offset = 0
	}
	filter.SubjectType = strings.TrimSpace(filter.SubjectType)
	filter.SubjectID = strings.TrimSpace(filter.SubjectID)
	filter.Action = strings.TrimSpace(filter.Action)
	return s.repo.List(ctx, filter, limit, offset)
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return DefaultLimit
	}
	if limit > MaxLimit {
		return MaxLimit
	}
	return limit
}

func entryFromEvent(ev events.Event) Entry {
	entry := Entry{
		OccurredAt: ev.OccurredAt(),
		ActorType:  ActorSystem,
		Action:     ev.Topic(),
		Data:       map[string]any{},
	}

	switch e := ev.(type) {
	case events.SourceCreated:
		entry.SubjectType, entry.SubjectID = "source", e.SourceID
		entry.Data["name"] = e.Name
		entry.Data["type"] = e.Type
	case events.SourceUpdated:
		entry.SubjectType, entry.SubjectID = "source", e.SourceID
		entry.Data["status"] = e.Status
	case events.ObservationReceived:
		entry.SubjectType, entry.SubjectID = "observation", e.ObservationID
		entry.Data["sourceId"] = e.SourceID
		entry.Data["type"] = e.Type
		entry.Data["duplicate"] = e.Duplicate
		entry.Data["retry"] = e.Retry
		if e.Position != nil {
			entry.Data["position"] = pointData(*e.Position)
		}
		if e.TrackHint != "" {
			entry.Data["trackHint"] = e.TrackHint
		}
	case events.ObservationRejected:
		entry.SubjectType, entry.SubjectID = "observation", e.ObservationID
		entry.Data["sourceId"] = e.SourceID
		entry.Data["trackHint"] = e.TrackHint
		entry.Data["reason"] = e.Reason
	case events.AssetCreated:
		entry.SubjectType, entry.SubjectID = "asset", e.AssetID
		entry.Data["name"] = e.Name
		entry.Data["type"] = e.Type
	case events.AssetUpdated:
		entry.SubjectType, entry.SubjectID = "asset", e.AssetID
		entry.Data["status"] = e.Status
	case events.AssetPositionUpdated:
		entry.SubjectType, entry.SubjectID = "asset", e.AssetID
		entry.Data["position"] = pointData(e.Position)
		if e.Speed != nil {
			entry.Data["speed"] = *e.Speed
		}
		if e.Heading != nil {
			entry.Data["heading"] = *e.Heading
		}
	case events.AssetConnectionChanged:
		entry.SubjectType, entry.SubjectID = "asset", e.AssetID
		entry.Data["connectionState"] = e.ConnectionState
		entry.Data["health"] = e.Health
	case events.TelemetryReceived:
		entry.SubjectType, entry.SubjectID = "telemetry", e.TelemetryID
		entry.Data["assetId"] = e.AssetID
		entry.Data["sourceId"] = e.SourceID
		entry.Data["stale"] = e.Stale
		entry.Data["duplicate"] = e.Duplicate
	case events.TrackCreated:
		entry.SubjectType, entry.SubjectID = "track", e.TrackID
		if e.ExternalRef != "" {
			entry.Data["externalRef"] = e.ExternalRef
		}
		if e.Position != nil {
			entry.Data["position"] = pointData(*e.Position)
		}
	case events.TrackUpdated:
		entry.SubjectType, entry.SubjectID = "track", e.TrackID
		if e.Position != nil {
			entry.Data["position"] = pointData(*e.Position)
		}
		if e.Speed != nil {
			entry.Data["speed"] = *e.Speed
		}
		if e.Heading != nil {
			entry.Data["heading"] = *e.Heading
		}
	case events.TrackClosed:
		entry.SubjectType, entry.SubjectID = "track", e.TrackID
	case events.ClassificationCreated:
		entry.SubjectType, entry.SubjectID = "classification", e.ClassificationID
		entry.Data["trackId"] = e.TrackID
		entry.Data["label"] = e.Label
		entry.Data["method"] = e.Method
		if e.Confidence != nil {
			entry.Data["confidence"] = *e.Confidence
		}
	case events.GeofenceCreated:
		entry.SubjectType, entry.SubjectID = "geofence", e.GeofenceID
		entry.Data["name"] = e.Name
		entry.Data["type"] = e.Type
	case events.GeofenceBreached:
		entry.SubjectType, entry.SubjectID = "geofence", e.GeofenceID
		entry.Data["geofenceName"] = e.GeofenceName
		entry.Data["geofenceType"] = e.GeofenceType
		entry.Data["severity"] = e.Severity
		entry.Data["trackId"] = e.TrackID
		entry.Data["position"] = pointData(e.Position)
	case events.GeofenceExited:
		entry.SubjectType, entry.SubjectID = "geofence", e.GeofenceID
		entry.Data["trackId"] = e.TrackID
		entry.Data["position"] = pointData(e.Position)
	case events.AlertCreated:
		entry.SubjectType, entry.SubjectID = "alert", e.AlertID
		entry.Data["type"] = e.Type
		entry.Data["severity"] = e.Severity
		entry.Data["title"] = e.Title
		if e.TrackID != "" {
			entry.Data["trackId"] = e.TrackID
		}
		if e.AssetID != "" {
			entry.Data["assetId"] = e.AssetID
		}
		if e.GeofenceID != "" {
			entry.Data["geofenceId"] = e.GeofenceID
		}
	case events.AlertAcknowledged:
		entry.SubjectType, entry.SubjectID = "alert", e.AlertID
		setOperator(&entry, e.Operator)
		if e.Operator != "" {
			entry.Data["operator"] = e.Operator
		}
	case events.AlertResolved:
		entry.SubjectType, entry.SubjectID = "alert", e.AlertID
		setOperator(&entry, e.Operator)
		if e.Operator != "" {
			entry.Data["operator"] = e.Operator
		}
	case events.IncidentCreated:
		entry.SubjectType, entry.SubjectID = "incident", e.IncidentID
		setOperator(&entry, e.Actor)
		entry.Data["title"] = e.Title
		entry.Data["priority"] = e.Priority
		entry.Data["status"] = e.Status
	case events.IncidentUpdated:
		entry.SubjectType, entry.SubjectID = "incident", e.IncidentID
		setOperator(&entry, e.Actor)
		entry.Data["status"] = e.Status
		entry.Data["priority"] = e.Priority
	case events.MissionCreated:
		entry.SubjectType, entry.SubjectID = "mission", e.MissionID
		setOperator(&entry, e.Actor)
		entry.Data["name"] = e.Name
		entry.Data["status"] = e.Status
	case events.MissionUpdated:
		entry.SubjectType, entry.SubjectID = "mission", e.MissionID
		setOperator(&entry, e.Actor)
		entry.Data["status"] = e.Status
	case events.CommandIssued:
		entry.SubjectType, entry.SubjectID = "command", e.CommandID
		setOperator(&entry, e.Actor)
		entry.Data["assetId"] = e.AssetID
		entry.Data["type"] = e.Type
		if e.MissionID != "" {
			entry.Data["missionId"] = e.MissionID
		}
	case events.CommandStatusChanged:
		entry.SubjectType, entry.SubjectID = "command", e.CommandID
		setOperator(&entry, e.Actor)
		entry.Data["assetId"] = e.AssetID
		entry.Data["state"] = e.State
		if e.FailureReason != "" {
			entry.Data["failureReason"] = e.FailureReason
		}
	case events.AssessmentCreated:
		entry.SubjectType, entry.SubjectID = "assessment", e.AssessmentID
		entry.Data["subjectType"] = e.SubjectType
		entry.Data["subjectId"] = e.SubjectID
		entry.Data["type"] = e.Type
		entry.Data["method"] = e.Method
	case events.ScenarioRunStarted:
		entry.ActorType, entry.ActorID = ActorScenario, e.RunID
		entry.SubjectType, entry.SubjectID = "scenario_run", e.RunID
		entry.Data["scenarioName"] = e.ScenarioName
		entry.Data["seed"] = e.Seed
	case events.ScenarioRunUpdated:
		entry.ActorType, entry.ActorID = ActorScenario, e.RunID
		entry.SubjectType, entry.SubjectID = "scenario_run", e.RunID
		entry.Data["status"] = e.Status
	case events.ScenarioRunEnded:
		entry.ActorType, entry.ActorID = ActorScenario, e.RunID
		entry.SubjectType, entry.SubjectID = "scenario_run", e.RunID
		entry.Data["status"] = e.Status
	}
	return entry
}

func setOperator(entry *Entry, actor string) {
	if actor == "" {
		return
	}
	if strings.HasPrefix(actor, "scenario:") {
		entry.ActorType = ActorScenario
	} else {
		entry.ActorType = ActorOperator
	}
	entry.ActorID = actor
}

func pointData(p geo.Point) map[string]any {
	return map[string]any{"lat": p.Lat, "lng": p.Lng}
}
