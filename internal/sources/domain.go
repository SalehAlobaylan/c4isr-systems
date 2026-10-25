// Package sources models where operational information comes from. A source is
// an identity, not a transport: simulators, sensors, operators, external
// systems, and future AI subsystems register here before producing evidence.
package sources

import (
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
)

// Type categorizes the origin of information.
type Type string

const (
	TypeSynthetic Type = "synthetic"
	TypeOperator  Type = "operator"
	TypeExternal  Type = "external"
	TypeSensor    Type = "sensor"
)

// Status describes source availability.
type Status string

const (
	StatusActive   Status = "active"
	StatusDegraded Status = "degraded"
	StatusOffline  Status = "offline"
)

// Source is the origin of observations.
type Source struct {
	Ingestion *IngestionSummary
	ID        string
	Name      string
	Type      Type
	Status    Status
	Metadata  map[string]any
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateInput is the application input for registering a source.
type CreateInput struct {
	ID       string
	Name     string
	Type     Type
	Status   Status
	Metadata map[string]any
}

// Normalize trims fields and applies defaults, generating an id when absent.
func (in *CreateInput) Normalize() {
	in.ID = strings.TrimSpace(in.ID)
	if in.ID == "" {
		in.ID = ids.New("src")
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Status == "" {
		in.Status = StatusActive
	}
	if in.Metadata == nil {
		in.Metadata = map[string]any{}
	}
}

// Validate checks create input against domain rules.
func (in CreateInput) Validate() error {
	if in.Name == "" {
		return apperr.Validation("source name is required")
	}
	switch in.Type {
	case TypeSynthetic, TypeOperator, TypeExternal, TypeSensor:
	default:
		return apperr.Validation("source type must be one of synthetic, operator, external, sensor")
	}
	switch in.Status {
	case StatusActive, StatusDegraded, StatusOffline:
	default:
		return apperr.Validation("source status must be one of active, degraded, offline")
	}
	return nil
}

// IngestionSummary describes provider coverage independently of registration
// status and observed entity counts. It carries no connector credentials/cursors.
type IngestionSummary struct {
	State               string     `json:"state"`
	AdmissionID         string     `json:"admissionId,omitempty"`
	OriginalProducer    string     `json:"originalProducer,omitempty"`
	UpstreamProvider    string     `json:"upstreamProvider,omitempty"`
	Dataset             string     `json:"dataset,omitempty"`
	ProductVersion      string     `json:"productVersion,omitempty"`
	LastSuccessAt       *time.Time `json:"lastSuccessAt,omitempty"`
	LastAttemptAt       *time.Time `json:"lastAttemptAt,omitempty"`
	NextAttemptAt       *time.Time `json:"nextAttemptAt,omitempty"`
	FailureCode         string     `json:"failureCode,omitempty"`
	CoverageFrom        *time.Time `json:"coverageFrom,omitempty"`
	CoverageTo          *time.Time `json:"coverageTo,omitempty"`
	PendingPublications int        `json:"pendingPublications"`
	Meaning             string     `json:"meaning"`
}
