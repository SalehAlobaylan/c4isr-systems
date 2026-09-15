// Package incidents models coordinated operational situations built on top of
// evidence produced elsewhere in the platform. An incident references alerts,
// tracks, assets, observations, and assessments; it never owns them.
package incidents

import (
	"fmt"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
)

// Status describes where an incident is in its response lifecycle.
type Status string

const (
	StatusOpen          Status = "OPEN"
	StatusAcknowledged  Status = "ACKNOWLEDGED"
	StatusInvestigating Status = "INVESTIGATING"
	StatusResponding    Status = "RESPONDING"
	StatusResolved      Status = "RESOLVED"
	StatusClosed        Status = "CLOSED"
)

// Priority describes operational urgency.
type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityMedium   Priority = "medium"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

// Incident is a coordinated operational situation.
type Incident struct {
	ID               string
	Title            string
	Description      string
	Priority         Priority
	Status           Status
	AssignedOperator string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	ResolvedAt       *time.Time
	ClosedAt         *time.Time
}

// RelatedAlert is the alert projection attached to an incident.
type RelatedAlert struct {
	ID        string
	Type      string
	Severity  string
	State     string
	Title     string
	CreatedAt time.Time
}

// RelatedTrack is the track projection attached to an incident.
type RelatedTrack struct {
	ID          string
	ExternalRef string
	Status      string
	Position    *geo.Point
	LastSeenAt  time.Time
}

// RelatedAsset is the asset projection attached to an incident.
type RelatedAsset struct {
	ID              string
	Name            string
	Type            string
	Status          string
	Position        *geo.Point
	ConnectionState string
}

// RelatedObservation is the observation projection attached to an incident.
type RelatedObservation struct {
	ID         string
	SourceID   string
	Type       string
	ObservedAt time.Time
	Position   *geo.Point
}

// RelatedAssessment is the assessment projection attached to an incident.
type RelatedAssessment struct {
	ID          string
	SubjectType string
	SubjectID   string
	Type        string
	Conclusion  string
	Method      string
	Confidence  *float64
	CreatedAt   time.Time
}

// Detail is an incident together with its attached evidence.
type Detail struct {
	Incident
	Alerts       []RelatedAlert
	Tracks       []RelatedTrack
	Assets       []RelatedAsset
	Observations []RelatedObservation
	Assessments  []RelatedAssessment
}

// CreateInput is the application input for opening an incident.
type CreateInput struct {
	Title          string
	Description    string
	Priority       Priority
	AlertIDs       []string
	TrackIDs       []string
	AssetIDs       []string
	ObservationIDs []string
	AssessmentIDs  []string
	Actor          string
}

// Normalize trims input fields and applies defaults.
func (in *CreateInput) Normalize() {
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)
	in.Actor = strings.TrimSpace(in.Actor)
	if in.Priority == "" {
		in.Priority = PriorityMedium
	}
}

// Validate checks create input against domain rules.
func (in CreateInput) Validate() error {
	if in.Title == "" {
		return apperr.Validation("incident title is required")
	}
	if !validPriority(in.Priority) {
		return apperr.Validation("incident priority must be one of low, medium, high, critical")
	}
	return nil
}

// UpdateInput is the application input for editing incident metadata.
type UpdateInput struct {
	Title            string
	Description      string
	Priority         Priority
	AssignedOperator string
}

// Normalize trims input fields.
func (in *UpdateInput) Normalize() {
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)
	in.AssignedOperator = strings.TrimSpace(in.AssignedOperator)
}

// Validate checks edit input against domain rules.
func (in UpdateInput) Validate() error {
	if in.Priority != "" && !validPriority(in.Priority) {
		return apperr.Validation("incident priority must be one of low, medium, high, critical")
	}
	return nil
}

// StatusChange is an explicit lifecycle transition request.
type StatusChange struct {
	Status Status
	Actor  string
}

func canTransition(from, to Status) bool {
	switch from {
	case StatusOpen:
		return to == StatusAcknowledged || to == StatusInvestigating || to == StatusClosed
	case StatusAcknowledged:
		return to == StatusInvestigating || to == StatusResponding || to == StatusClosed
	case StatusInvestigating:
		return to == StatusResponding || to == StatusResolved || to == StatusClosed
	case StatusResponding:
		return to == StatusInvestigating || to == StatusResolved || to == StatusClosed
	case StatusResolved:
		return to == StatusInvestigating || to == StatusClosed
	}
	return false
}

func invalidTransition(from, to Status) error {
	return apperr.Conflict(fmt.Sprintf("invalid incident transition %s -> %s", from, to))
}

func validPriority(p Priority) bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh, PriorityCritical:
		return true
	}
	return false
}

func validStatus(s Status) bool {
	switch s {
	case StatusOpen, StatusAcknowledged, StatusInvestigating, StatusResponding, StatusResolved, StatusClosed:
		return true
	}
	return false
}
