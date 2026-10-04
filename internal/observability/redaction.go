package observability

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
)

func safeAttr(_ []string, a slog.Attr) slog.Attr {
	if a.Key == "err" || a.Key == "error" {
		if err, ok := a.Value.Any().(error); ok {
			return slog.String(a.Key, errorClass(err))
		}
		return slog.String(a.Key, "redacted")
	}
	if a.Key == "device" {
		sum := sha256.Sum256([]byte(a.Value.String()))
		return slog.String("device_hash", fmt.Sprintf("%x", sum[:12]))
	}
	switch a.Key {
	case slog.TimeKey, slog.LevelKey, slog.MessageKey, "request_id", "route", "method", "status", "duration_ms", "tenant_id", "actor_id", "stage", "outcome", "count", "fields", "kind", "field", "http", "mqtt", "addr":
		return a
	default:
		return slog.String(a.Key, "redacted")
	}
}

func errorClass(err error) string {
	if errors.Is(err, context.Canceled) {
		return "context_canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline_exceeded"
	}
	var state interface{ SQLState() string }
	if errors.As(err, &state) {
		code := state.SQLState()
		if len(code) == 5 {
			valid := true
			for _, char := range code {
				if !(char >= '0' && char <= '9' || char >= 'A' && char <= 'Z') {
					valid = false
				}
			}
			if valid {
				return "postgres_" + code
			}
		}
		return "database_error"
	}
	var path *os.PathError
	if errors.As(err, &path) {
		return "filesystem_error"
	}
	var network net.Error
	if errors.As(err, &network) {
		if network.Timeout() {
			return "network_timeout"
		}
		return "network_error"
	}
	return fmt.Sprintf("%T", err)
}
