package missions

import "context"

// Repository persists missions, their asset assignments, and their tasks.
type Repository interface {
	Create(ctx context.Context, mission Mission) (Mission, error)
	Get(ctx context.Context, id string) (Mission, error)
	List(ctx context.Context, status string, limit, offset int) ([]Mission, int, error)
	UpdateStatus(ctx context.Context, id string, status, expected Status) (Mission, error)
	AssignAsset(ctx context.Context, missionID, assetID string) error
	CreateTask(ctx context.Context, task Task) (Task, error)
	ListTasks(ctx context.Context, missionID string) ([]Task, error)
	UpdateTaskStatus(ctx context.Context, missionID, taskID string, status TaskStatus) (Task, error)
}
