package observability

import (
	"context"
	"io"
	"log/slog"
	"os"
)

type dumpHandler struct{ dump, console slog.Handler }

func (h dumpHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.dump.Enabled(ctx, level) || h.console.Enabled(ctx, level)
}
func (h dumpHandler) Handle(ctx context.Context, r slog.Record) error {
	if h.dump.Enabled(ctx, r.Level) {
		if err := h.dump.Handle(ctx, r.Clone()); err != nil {
			return err
		}
	}
	if h.console.Enabled(ctx, r.Level) {
		return h.console.Handle(ctx, r)
	}
	return nil
}
func (h dumpHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return dumpHandler{h.dump.WithAttrs(attrs), h.console.WithAttrs(attrs)}
}
func (h dumpHandler) WithGroup(name string) slog.Handler {
	return dumpHandler{h.dump.WithGroup(name), h.console.WithGroup(name)}
}

type consoleWriter struct{ sink *Sink }

func (w consoleWriter) Write(p []byte) (int, error) {
	w.sink.mu.Lock()
	defer w.sink.mu.Unlock()
	if w.sink.closed {
		return 0, os.ErrClosed
	}
	return w.sink.writeStderr(p)
}

func (s *Sink) writeStderr(p []byte) (int, error) {
	n, err := s.stderr.Write(p)
	if err != nil || n != len(p) {
		s.failed.Store(true)
		if err == nil {
			err = io.ErrShortWrite
		}
	}
	return n, err
}
