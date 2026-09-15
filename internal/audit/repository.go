package audit

import "context"

// Repository persists audit entries. Implementations live in infrastructure
// layers and must not contain workflow logic.
type Repository interface {
	Append(ctx context.Context, entry Entry) (Entry, error)
	List(ctx context.Context, filter ListFilter, limit, offset int) ([]Entry, int, error)
}
