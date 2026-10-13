package observability

import (
	"context"
	"crypto/rand"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

type QueryTracer struct{ Logger *slog.Logger }

type queryTraceKey struct{}

type queryTrace struct {
	id      string
	started time.Time
}

func (t QueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	trace := queryTrace{id: rand.Text(), started: time.Now()}
	t.Logger.DebugContext(ctx, "database query started", "stage", "database", "outcome", "started", "span_id", trace.id)
	return context.WithValue(ctx, queryTraceKey{}, trace)
}

func (t QueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	trace, ok := ctx.Value(queryTraceKey{}).(queryTrace)
	if !ok {
		return
	}
	level, outcome := slog.LevelDebug, "completed"
	if data.Err != nil {
		level = slog.LevelWarn
		outcome = "failed"
	}
	attrs := []any{"stage", "database", "outcome", outcome, "span_id", trace.id, "duration_ms", time.Since(trace.started).Milliseconds()}
	if data.Err != nil {
		attrs = append(attrs, "err", data.Err)
	}
	t.Logger.Log(ctx, level, "database query completed", attrs...)
}
