package freshness

import (
	"context"
	"time"
)

type clockKey struct{}
type Clock func(context.Context, time.Time) time.Time

func WithClock(ctx context.Context, clock Clock) context.Context {
	return context.WithValue(ctx, clockKey{}, clock)
}

// Now separates the exercise information clock from wall receipt/audit time.
func Now(ctx context.Context, wall time.Time) time.Time {
	if c, ok := ctx.Value(clockKey{}).(Clock); ok && c != nil {
		return c(ctx, wall)
	}
	return wall
}
