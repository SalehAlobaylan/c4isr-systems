package access

import "context"

type retrievalKey struct{}

// Only authenticated server routes bind this value; headers never select a policy.
func WithRetrievalProfile(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, retrievalKey{}, key)
}
func RetrievalProfile(ctx context.Context) string {
	v, _ := ctx.Value(retrievalKey{}).(string)
	if v == "" {
		return "default"
	}
	return v
}
