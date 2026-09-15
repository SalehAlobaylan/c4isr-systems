// Package assets owns controlled and coordinated entities: registration,
// capabilities, availability, and the current operational state projection.
package assets

import (
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
)

// Status describes asset availability.
type Status string

const (
	StatusAvailable   Status = "available"
	StatusAssigned    Status = "assigned"
	StatusUnavailable Status = "unavailable"
	StatusOffline     Status = "offline"
	StatusMaintenance Status = "maintenance"
)

// Asset is a controlled entity combined with its current state projection.
type Asset struct {
	ID              string
	Name            string
	Type            string
	Status          Status
	Capabilities    []string
	Metadata        map[string]any
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Position        *geo.Point
	Speed           *float64
	Heading         *float64
	Health          string
	ConnectionState string
	LastSeenAt      *time.Time
}

// CreateInput is the application input for registering an asset.
type CreateInput struct {
	ID           string
	Name         string
	Type         string
	Status       Status
	Capabilities []string
	Metadata     map[string]any
}

// Normalize trims fields and applies defaults, generating an id when absent.
func (in *CreateInput) Normalize() {
	in.ID = strings.TrimSpace(in.ID)
	if in.ID == "" {
		in.ID = ids.New("ast")
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Type = strings.TrimSpace(in.Type)
	if in.Status == "" {
		in.Status = StatusAvailable
	}
	if in.Capabilities == nil {
		in.Capabilities = []string{}
	}
	if in.Metadata == nil {
		in.Metadata = map[string]any{}
	}
}

// Validate checks create input against domain rules.
func (in CreateInput) Validate() error {
	if in.Name == "" {
		return apperr.Validation("asset name is required")
	}
	if in.Type == "" {
		return apperr.Validation("asset type is required")
	}
	if !validStatus(in.Status) {
		return apperr.Validation("asset status must be one of available, assigned, unavailable, offline, maintenance")
	}
	return nil
}

// StateUpdate is a change to the current state projection of an asset. It is
// applied from telemetry ingestion and command acknowledgments.
type StateUpdate struct {
	AssetID         string
	Position        *geo.Point
	Speed           *float64
	Heading         *float64
	Health          string
	ConnectionState string
	ObservedAt      time.Time
}

func validStatus(status Status) bool {
	switch status {
	case StatusAvailable, StatusAssigned, StatusUnavailable, StatusOffline, StatusMaintenance:
		return true
	}
	return false
}
