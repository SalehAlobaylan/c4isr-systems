package observability

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type dbQueryContextKey struct{}

type dbQueryState struct {
	started time.Time
	span    trace.Span
}

// NewDatabaseTracer instruments pgx queries with a duration metric and a
// child span. SQL text and arguments are deliberately not recorded so secrets
// and high-cardinality query values cannot escape into telemetry.
func NewDatabaseTracer(metrics *Metrics) pgx.QueryTracer {
	return &databaseTracer{metrics: metrics}
}

type databaseTracer struct {
	metrics *Metrics
}

func (t *databaseTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	spanCtx, span := StartSpan(ctx, "c4isr.database.query",
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", sqlOperation(data.SQL)),
	)
	return context.WithValue(spanCtx, dbQueryContextKey{}, dbQueryState{started: time.Now(), span: span})
}

func (t *databaseTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	state, ok := ctx.Value(dbQueryContextKey{}).(dbQueryState)
	if !ok {
		return
	}
	if t.metrics != nil {
		t.metrics.ObserveDatabaseQuery(time.Since(state.started))
	}
	EndSpan(state.span, data.Err)
}

func sqlOperation(query string) string {
	parts := strings.Fields(query)
	if len(parts) == 0 {
		return "OTHER"
	}
	switch strings.ToUpper(parts[0]) {
	case "SELECT", "INSERT", "UPDATE", "DELETE", "BEGIN", "COMMIT", "ROLLBACK":
		return strings.ToUpper(parts[0])
	default:
		return "OTHER"
	}
}
