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
	Transition(ctx context.Context, id string, state State, failureReason string) (Command, error)
}
