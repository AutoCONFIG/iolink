package observability

import (
	"context"
	"log/slog"
)

type (
	requestIDContextKey struct{}
	ingestIDContextKey  struct{}
)

func ContextWithIngestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ingestIDContextKey{}, id)
}

func ContextWithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, id)
}

func RequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDContextKey{}).(string); ok {
		return id
	}
	return ""
}

type contextHandler struct{ slog.Handler }

func (h contextHandler) Handle(ctx context.Context, record slog.Record) error {
	record = record.Clone()
	if id := RequestID(ctx); id != "" {
		record.AddAttrs(slog.String("request_id", id))
	}
	if id, ok := ctx.Value(ingestIDContextKey{}).(string); ok && id != "" {
		record.AddAttrs(slog.String("ingest_id", id))
	}
	return h.Handler.Handle(ctx, record)
}

func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{h.Handler.WithAttrs(attrs)}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{h.Handler.WithGroup(name)}
}
