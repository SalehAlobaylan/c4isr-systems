// Package events defines the in-process domain event types and a synchronous
// dispatcher used for module integration, audit, and realtime publication.
//
// Events deliberately carry primitive fields rather than domain structs: they
// are integration facts, not the domain model itself. The dispatcher is
// synchronous so that handlers observe a deterministic order, mirroring what a
// transactional outbox would guarantee in a distributed future.
package events

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
)

// Topic names. These are stable integration identifiers.
const (
	TopicSourceCreated = "source.created"
	TopicSourceUpdated = "source.updated"

	TopicObservationReceived = "observation.received"

	TopicAssetCreated           = "asset.created"
	TopicAssetUpdated           = "asset.updated"
	TopicAssetPositionUpdated   = "asset.position.updated"
	TopicAssetConnectionChanged = "asset.connection.changed"
	TopicTelemetryReceived      = "telemetry.received"

	TopicTrackCreated = "track.created"
	TopicTrackUpdated = "track.updated"
	TopicTrackClosed  = "track.closed"

	TopicClassificationCreated = "classification.created"

	TopicGeofenceCreated  = "geofence.created"
	TopicGeofenceBreached = "geofence.breached"
	TopicGeofenceExited   = "geofence.exited"

	TopicAlertCreated      = "alert.created"
	TopicAlertAcknowledged = "alert.acknowledged"
	TopicAlertResolved     = "alert.resolved"

	TopicIncidentCreated = "incident.created"
	TopicIncidentUpdated = "incident.updated"

	TopicMissionCreated = "mission.created"
	TopicMissionUpdated = "mission.updated"

	TopicCommandIssued        = "command.issued"
	TopicCommandStatusChanged = "command.status.changed"

	TopicAssessmentCreated = "assessment.created"

	TopicScenarioRunStarted = "scenario.run.started"
	TopicScenarioRunUpdated = "scenario.run.updated"
	TopicScenarioRunEnded   = "scenario.run.ended"
)

// AllTopics lists every topic this package defines. Used by subscribers that
// care about the full event stream (audit, realtime).
func AllTopics() []string {
	return []string{
		TopicSourceCreated,
		TopicSourceUpdated,
		TopicObservationReceived,
		TopicAssetCreated,
		TopicAssetUpdated,
		TopicAssetPositionUpdated,
		TopicAssetConnectionChanged,
		TopicTelemetryReceived,
		TopicTrackCreated,
		TopicTrackUpdated,
		TopicTrackClosed,
		TopicClassificationCreated,
		TopicGeofenceCreated,
		TopicGeofenceBreached,
		TopicGeofenceExited,
		TopicAlertCreated,
		TopicAlertAcknowledged,
		TopicAlertResolved,
		TopicIncidentCreated,
		TopicIncidentUpdated,
		TopicMissionCreated,
		TopicMissionUpdated,
		TopicCommandIssued,
		TopicCommandStatusChanged,
		TopicAssessmentCreated,
		TopicScenarioRunStarted,
		TopicScenarioRunUpdated,
		TopicScenarioRunEnded,
	}
}

// Event is anything published on the in-process dispatcher.
type Event interface {
	Topic() string
	OccurredAt() time.Time
}

// Handler processes one event. Errors are logged by the dispatcher; event
// publication is best-effort with respect to secondary handlers.
type Handler func(ctx context.Context, ev Event)

// Dispatcher routes events to subscribers synchronously.
type Dispatcher struct {
	logger   *slog.Logger
	handlers map[string][]Handler
}

// NewDispatcher creates an empty dispatcher.
func NewDispatcher(logger *slog.Logger) *Dispatcher {
	return &Dispatcher{
		logger:   logger,
		handlers: make(map[string][]Handler),
	}
}

// Subscribe registers a handler for topic.
func (d *Dispatcher) Subscribe(topic string, handler Handler) {
	d.handlers[topic] = append(d.handlers[topic], handler)
}

// Publish delivers ev to all subscribers of its topic in registration order.
// A panicking handler cannot take down the publisher or block other handlers.
func (d *Dispatcher) Publish(ctx context.Context, ev Event) {
	if ev == nil {
		return
	}
	for _, handler := range d.handlers[ev.Topic()] {
		func() {
			defer func() {
				if p := recover(); p != nil {
					d.logger.Error("event handler panic",
						"topic", ev.Topic(),
						"panic", p,
						"stack", string(debug.Stack()),
					)
				}
			}()
			handler(ctx, ev)
		}()
	}
}

// ---------------------------------------------------------------------------
// Sources
// ---------------------------------------------------------------------------

// SourceCreated is published when a source is registered.
type SourceCreated struct {
	At       time.Time
	SourceID string
	Name     string
	Type     string
}

func (SourceCreated) Topic() string { return TopicSourceCreated }

// SourceUpdated is published when source metadata or status changes.
type SourceUpdated struct {
	At       time.Time
	SourceID string
	Status   string
}

func (SourceUpdated) Topic() string { return TopicSourceUpdated }

// ---------------------------------------------------------------------------
// Observations
// ---------------------------------------------------------------------------

// ObservationReceived is published after an observation is durably stored.
type ObservationReceived struct {
	At            time.Time
	ObservationID string
	SourceID      string
	Type          string
	ObservedAt    time.Time
	ReceivedAt    time.Time
	Position      *geo.Point
	TrackHint     string
	Duplicate     bool
}

func (ObservationReceived) Topic() string { return TopicObservationReceived }

// ---------------------------------------------------------------------------
// Assets and telemetry
// ---------------------------------------------------------------------------

// AssetCreated is published when an asset is registered.
type AssetCreated struct {
	At      time.Time
	AssetID string
	Name    string
	Type    string
}

func (AssetCreated) Topic() string { return TopicAssetCreated }

// AssetUpdated is published when asset metadata or availability changes.
type AssetUpdated struct {
	At      time.Time
	AssetID string
	Status  string
}

func (AssetUpdated) Topic() string { return TopicAssetUpdated }

// AssetPositionUpdated is published when a newer telemetry sample moves an asset.
type AssetPositionUpdated struct {
	At         time.Time
	AssetID    string
	Position   geo.Point
	Speed      *float64
	Heading    *float64
	ObservedAt time.Time
}

func (AssetPositionUpdated) Topic() string { return TopicAssetPositionUpdated }

// AssetConnectionChanged is published on connectivity transitions.
type AssetConnectionChanged struct {
	At              time.Time
	AssetID         string
	ConnectionState string
	Health          string
}

func (AssetConnectionChanged) Topic() string { return TopicAssetConnectionChanged }

// TelemetryReceived is published for every accepted telemetry message,
// including stale and duplicate samples, so history and audit stay complete.
type TelemetryReceived struct {
	At          time.Time
	TelemetryID string
	AssetID     string
	SourceID    string
	ObservedAt  time.Time
	Stale       bool
	Duplicate   bool
}

func (TelemetryReceived) Topic() string { return TopicTelemetryReceived }

// ---------------------------------------------------------------------------
// Tracks
// ---------------------------------------------------------------------------

// TrackCreated is published when a track is first derived.
type TrackCreated struct {
	At          time.Time
	TrackID     string
	ExternalRef string
	Position    *geo.Point
	ObservedAt  time.Time
}

func (TrackCreated) Topic() string { return TopicTrackCreated }

// TrackUpdated is published when track state advances.
type TrackUpdated struct {
	At         time.Time
	TrackID    string
	Position   *geo.Point
	Speed      *float64
	Heading    *float64
	ObservedAt time.Time
	LastSeenAt time.Time
}

func (TrackUpdated) Topic() string { return TopicTrackUpdated }

// TrackClosed is published when a track reaches a terminal state.
type TrackClosed struct {
	At      time.Time
	TrackID string
}

func (TrackClosed) Topic() string { return TopicTrackClosed }

// ---------------------------------------------------------------------------
// Classifications
// ---------------------------------------------------------------------------

// ClassificationCreated is published for each explicit classification record.
type ClassificationCreated struct {
	At               time.Time
	ClassificationID string
	TrackID          string
	Label            string
	Confidence       *float64
	Method           string
}

func (ClassificationCreated) Topic() string { return TopicClassificationCreated }

// ---------------------------------------------------------------------------
// Geospatial
// ---------------------------------------------------------------------------

// GeofenceCreated is published when a geofence is registered.
type GeofenceCreated struct {
	At         time.Time
	GeofenceID string
	Name       string
	Type       string
}

func (GeofenceCreated) Topic() string { return TopicGeofenceCreated }

// GeofenceBreached is published when a track enters a geofence it was outside of.
type GeofenceBreached struct {
	At           time.Time
	GeofenceID   string
	GeofenceName string
	GeofenceType string
	Severity     string
	TrackID      string
	Position     geo.Point
}

func (GeofenceBreached) Topic() string { return TopicGeofenceBreached }

// GeofenceExited is published when a track leaves a geofence it was inside of.
type GeofenceExited struct {
	At         time.Time
	GeofenceID string
	TrackID    string
	Position   geo.Point
}

func (GeofenceExited) Topic() string { return TopicGeofenceExited }

// ---------------------------------------------------------------------------
// Alerts
// ---------------------------------------------------------------------------

// AlertCreated is published when an alert enters the operator queue.
type AlertCreated struct {
	At         time.Time
	AlertID    string
	Type       string
	Severity   string
	Title      string
	TrackID    string
	AssetID    string
	GeofenceID string
}

func (AlertCreated) Topic() string { return TopicAlertCreated }

// AlertAcknowledged is published when an operator acknowledges an alert.
type AlertAcknowledged struct {
	At       time.Time
	AlertID  string
	Operator string
}

func (AlertAcknowledged) Topic() string { return TopicAlertAcknowledged }

// AlertResolved is published when an alert is resolved.
type AlertResolved struct {
	At       time.Time
	AlertID  string
	Operator string
}

func (AlertResolved) Topic() string { return TopicAlertResolved }

// ---------------------------------------------------------------------------
// Incidents
// ---------------------------------------------------------------------------

// IncidentCreated is published when an incident workspace is opened.
type IncidentCreated struct {
	At         time.Time
	IncidentID string
	Title      string
	Priority   string
	Status     string
	Actor      string
}

func (IncidentCreated) Topic() string { return TopicIncidentCreated }

// IncidentUpdated is published on incident transitions and edits.
type IncidentUpdated struct {
	At         time.Time
	IncidentID string
	Status     string
	Priority   string
	Actor      string
}

func (IncidentUpdated) Topic() string { return TopicIncidentUpdated }

// ---------------------------------------------------------------------------
// Missions
// ---------------------------------------------------------------------------

// MissionCreated is published when a mission is planned.
type MissionCreated struct {
	At        time.Time
	MissionID string
	Name      string
	Status    string
	Actor     string
}

func (MissionCreated) Topic() string { return TopicMissionCreated }

// MissionUpdated is published on mission transitions and assignments.
type MissionUpdated struct {
	At        time.Time
	MissionID string
	Status    string
	Actor     string
}

func (MissionUpdated) Topic() string { return TopicMissionUpdated }

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

// CommandIssued is published when a command is created and handed to transport.
type CommandIssued struct {
	At        time.Time
	CommandID string
	AssetID   string
	MissionID string
	Type      string
	Actor     string
}

func (CommandIssued) Topic() string { return TopicCommandIssued }

// CommandStatusChanged is published on every command lifecycle transition.
type CommandStatusChanged struct {
	At            time.Time
	CommandID     string
	AssetID       string
	State         string
	FailureReason string
	Actor         string
}

func (CommandStatusChanged) Topic() string { return TopicCommandStatusChanged }

// ---------------------------------------------------------------------------
// Assessments
// ---------------------------------------------------------------------------

// AssessmentCreated is published when an analytical conclusion is recorded.
type AssessmentCreated struct {
	At           time.Time
	AssessmentID string
	SubjectType  string
	SubjectID    string
	Type         string
	Method       string
}

func (AssessmentCreated) Topic() string { return TopicAssessmentCreated }

// ---------------------------------------------------------------------------
// Scenario runs
// ---------------------------------------------------------------------------

// ScenarioRunStarted is published when a scenario run begins.
type ScenarioRunStarted struct {
	At           time.Time
	RunID        string
	ScenarioName string
	Seed         int64
}

func (ScenarioRunStarted) Topic() string { return TopicScenarioRunStarted }

// ScenarioRunUpdated is published on pause/resume/speed changes.
type ScenarioRunUpdated struct {
	At     time.Time
	RunID  string
	Status string
}

func (ScenarioRunUpdated) Topic() string { return TopicScenarioRunUpdated }

// ScenarioRunEnded is published when a scenario run stops or fails.
type ScenarioRunEnded struct {
	At     time.Time
	RunID  string
	Status string
}

func (ScenarioRunEnded) Topic() string { return TopicScenarioRunEnded }

func (e SourceCreated) OccurredAt() time.Time { return e.At }

func (e SourceUpdated) OccurredAt() time.Time { return e.At }

func (e ObservationReceived) OccurredAt() time.Time { return e.At }

func (e AssetCreated) OccurredAt() time.Time { return e.At }

func (e AssetUpdated) OccurredAt() time.Time { return e.At }

func (e AssetPositionUpdated) OccurredAt() time.Time { return e.At }

func (e AssetConnectionChanged) OccurredAt() time.Time { return e.At }

func (e TelemetryReceived) OccurredAt() time.Time { return e.At }

func (e TrackCreated) OccurredAt() time.Time { return e.At }

func (e TrackUpdated) OccurredAt() time.Time { return e.At }

func (e TrackClosed) OccurredAt() time.Time { return e.At }

func (e ClassificationCreated) OccurredAt() time.Time { return e.At }

func (e GeofenceCreated) OccurredAt() time.Time { return e.At }

func (e GeofenceBreached) OccurredAt() time.Time { return e.At }

func (e GeofenceExited) OccurredAt() time.Time { return e.At }

func (e AlertCreated) OccurredAt() time.Time { return e.At }

func (e AlertAcknowledged) OccurredAt() time.Time { return e.At }

func (e AlertResolved) OccurredAt() time.Time { return e.At }

func (e IncidentCreated) OccurredAt() time.Time { return e.At }

func (e IncidentUpdated) OccurredAt() time.Time { return e.At }

func (e MissionCreated) OccurredAt() time.Time { return e.At }

func (e MissionUpdated) OccurredAt() time.Time { return e.At }

func (e CommandIssued) OccurredAt() time.Time { return e.At }

func (e CommandStatusChanged) OccurredAt() time.Time { return e.At }

func (e AssessmentCreated) OccurredAt() time.Time { return e.At }

func (e ScenarioRunStarted) OccurredAt() time.Time { return e.At }

func (e ScenarioRunUpdated) OccurredAt() time.Time { return e.At }

func (e ScenarioRunEnded) OccurredAt() time.Time { return e.At }
