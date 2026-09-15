// Package operators models the humans acting on the system. It stays minimal
// until Phase 17 introduces full authentication and RBAC, but every operator
// action is attributable today.
package operators

import (
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

// Role is the level of authority an operator holds.
type Role string

const (
	RoleOperator      Role = "operator"
	RoleSupervisor    Role = "supervisor"
	RoleAdministrator Role = "administrator"
	RoleAnalyst       Role = "analyst"
)

// Operator is a human identity used for attribution.
type Operator struct {
	ID        string
	Name      string
	Role      Role
	CreatedAt time.Time
}

// CreateInput is the application input for registering an operator.
type CreateInput struct {
	ID   string
	Name string
	Role Role
}

// Normalize trims fields and applies defaults.
func (in *CreateInput) Normalize() {
	in.ID = strings.TrimSpace(in.ID)
	in.Name = strings.TrimSpace(in.Name)
	if in.Role == "" {
		in.Role = RoleOperator
	}
}

// Validate checks create input against domain rules.
func (in CreateInput) Validate() error {
	if in.Name == "" {
		return apperr.Validation("operator name is required")
	}
	switch in.Role {
	case RoleOperator, RoleSupervisor, RoleAdministrator, RoleAnalyst:
	default:
		return apperr.Validation("operator role must be one of operator, supervisor, administrator, analyst")
	}
	return nil
}
