package observability

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var domainTracer = otel.Tracer("c4isr.domain")

// StartSpan starts a domain span using the process-wide OpenTelemetry
// provider. With no provider configured this is a cheap no-op, while staging
// and production can install an exporter without changing domain services.
func StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if ctx == nil {
		ctx = context.Background()
	}
	return domainTracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

// EndSpan records a failure before closing a domain span.
func EndSpan(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
