package httpx

import "context"

// contextT aliases context.Context so the helpers above read cleanly.
type contextT = context.Context

func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func withOperatorID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, operatorIDKey, id)
}
