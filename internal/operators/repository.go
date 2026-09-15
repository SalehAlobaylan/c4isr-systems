package operators

import "context"

// Repository persists operators. Implementations live in infrastructure layers
// and must not contain workflow logic.
type Repository interface {
	Create(ctx context.Context, operator Operator) (Operator, error)
	Get(ctx context.Context, id string) (Operator, error)
	List(ctx context.Context, limit, offset int) ([]Operator, int, error)
	Exists(ctx context.Context, id string) (bool, error)
}
