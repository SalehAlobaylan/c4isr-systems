// Package missions coordinates operational objectives and the assets and tasks
// dedicated to them. Missions reference incidents and assets without owning
// them.
package missions

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
)

// Status describes where a mission is in its lifecycle.
type Status string

const (
	StatusPlanned   Status = "PLANNED"
	StatusActive    Status = "ACTIVE"
	StatusCompleted Status = "COMPLETED"
	StatusAborted   Status = "ABORTED"
)

// TaskStatus describes where a mission task is in its lifecycle.
type TaskStatus string

const (
	TaskPending   TaskStatus = "PENDING"
	TaskActive    TaskStatus = "ACTIVE"
	TaskCompleted TaskStatus = "COMPLETED"
	TaskFailed    TaskStatus = "FAILED"
	TaskCancelled TaskStatus = "CANCELLED"
)

// Priority describes operational urgency.
type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityMedium   Priority = "medium"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

// Mission is a coordinated objective with assigned assets and tasks.
type Mission struct {
	ID         string
	Name       string
	Objective  string
	Priority   Priority
	Status     Status
	IncidentID string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	StartedAt  *time.Time
	EndedAt    *time.Time
	Assets     []RelatedAsset
	Tasks      []Task
}

// Task is a unit of work within a mission.
type Task struct {
	ID          string
	MissionID   string
	Type        string
	Description string
	Status      TaskStatus
	Target      *geo.Point
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// RelatedAsset is the asset projection assigned to a mission.
type RelatedAsset struct {
	ID              string
	Name            string
	Type            string
	Status          string
	Position        *geo.Point
	ConnectionState string
}

// CreateInput is the application input for planning a mission.
type CreateInput struct {
	Name       string
	Objective  string
	Priority   Priority
	IncidentID string
	Actor      string
	Assets     []string
	Tasks      []TaskInput
}

// TaskInput is the application input for a mission task.
type TaskInput struct {
	Type        string
	Description string
	Target      *geo.Point
}

// AssetRegistry verifies that mission assets exist without importing the
// assets module.
type AssetRegistry interface {
	Exists(ctx context.Context, id string) (bool, error)
}

// Normalize trims input fields and applies defaults.
func (in *CreateInput) Normalize() {
	in.Name = strings.TrimSpace(in.Name)
	in.Objective = strings.TrimSpace(in.Objective)
	in.IncidentID = strings.TrimSpace(in.IncidentID)
	in.Actor = strings.TrimSpace(in.Actor)
	if in.Priority == "" {
		in.Priority = PriorityMedium
	}
	for i := range in.Tasks {
		in.Tasks[i].Normalize()
	}
}

// Validate checks create input against domain rules.
func (in CreateInput) Validate() error {
	if in.Name == "" {
		return apperr.Validation("mission name is required")
	}
	if !validPriority(in.Priority) {
		return apperr.Validation("mission priority must be one of low, medium, high, critical")
	}
	for _, task := range in.Tasks {
		if err := task.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Normalize trims task input fields.
func (in *TaskInput) Normalize() {
	in.Type = strings.TrimSpace(in.Type)
	in.Description = strings.TrimSpace(in.Description)
}

// Validate checks task input against domain rules.
func (in TaskInput) Validate() error {
	if in.Type == "" {
		return apperr.Validation("task type is required")
	}
	if in.Target != nil && !in.Target.Valid() {
		return apperr.Validation("task target must be a valid WGS84 coordinate")
	}
	return nil
}

func canTransition(from, to Status) bool {
	switch from {
	case StatusPlanned:
		return to == StatusActive || to == StatusAborted
	case StatusActive:
		return to == StatusCompleted || to == StatusAborted
	}
	return false
}

func invalidTransition(from, to Status) error {
	return apperr.Conflict(fmt.Sprintf("invalid mission transition %s -> %s", from, to))
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
	case StatusPlanned, StatusActive, StatusCompleted, StatusAborted:
		return true
	}
	return false
}

func validTaskStatus(s TaskStatus) bool {
	switch s {
	case TaskPending, TaskActive, TaskCompleted, TaskFailed, TaskCancelled:
		return true
	}
	return false
}
