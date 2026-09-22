package commands

import "context"

// ListFilter narrows command reads.
type ListFilter struct {
	AssetID   string
	State     string
	MissionID string
}

// Repository persists commands.
type Repository interface {
	Create(ctx context.Context, command Command) (Command, error)
	Get(ctx context.Context, id string) (Command, error)
	List(ctx context.Context, filter ListFilter, limit, offset int) ([]Command, int, error)
	// Transition applies the state change only when the command still has the
	// expected current state. This makes the lifecycle check and write one
	// atomic compare-and-set operation.
	Transition(ctx context.Context, id string, from, state State, failureReason string) (Command, error)
}
