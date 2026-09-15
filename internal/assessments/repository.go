package assessments

import "context"

// Repository persists assessments and their evidence links. Implementations
// live in infrastructure layers and must not contain workflow logic.
type Repository interface {
	Create(ctx context.Context, assessment Assessment) (Assessment, error)
	Get(ctx context.Context, id string) (Assessment, error)
	List(ctx context.Context, subjectType, subjectID string, limit, offset int) ([]Assessment, int, error)
}
