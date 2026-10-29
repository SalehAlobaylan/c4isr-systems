package access

import "context"

type contentViewKey struct{}

// WithContentView is set only after CatalogView checks the live management grant.
func WithContentView(ctx context.Context, view string) context.Context {
	return context.WithValue(ctx, contentViewKey{}, view)
}
func ContentView(ctx context.Context) string {
	if value, ok := ctx.Value(contentViewKey{}).(string); ok {
		return value
	}
	return "ordinary"
}
