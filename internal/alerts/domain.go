// Package alerts manages operational attention items. Alerts are raised from
// domain events such as geofence breaches and move through an explicit
// lifecycle: ACTIVE -> ACKNOWLEDGED -> RESOLVED.
package alerts

import "time"

// State is the lifecycle state of an alert.
type State string

const (
	StateActive       State = "ACTIVE"
	StateAcknowledged State = "ACKNOWLEDGED"
	StateResolved     State = "RESOLVED"
)

// Severity ranks how urgently an alert demands attention.
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Alert is an operational attention item.
type Alert struct {
	ID              string
	Type            string
	Severity        Severity
	State           State
	Title           string
	Message         string
	SourceReference map[string]any
	TrackID         string
	AssetID         string
	GeofenceID      string
	IncidentID      string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	AcknowledgedAt  *time.Time
	AcknowledgedBy  string
	ResolvedAt      *time.Time
	ResolvedBy      string
}

// ListFilter narrows alert reads; empty fields mean no filter.
type ListFilter struct {
	State      string
	Severity   string
	TrackID    string
	IncidentID string
}

// CreateInput is the application input for raising an alert.
type CreateInput struct {
	Type            string
	Severity        Severity
	Title           string
	Message         string
	SourceReference map[string]any
	TrackID         string
	AssetID         string
	GeofenceID      string
}
