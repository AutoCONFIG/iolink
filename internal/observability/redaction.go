package observability

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
)

func NewIngestID() string { return rand.Text() }

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
	case slog.TimeKey, slog.LevelKey, slog.MessageKey, "request_id", "ingest_id", "span_id", "route", "method", "stage", "outcome", "error_code", "operation":
		return a
	case "status", "duration_ms", "tenant_id", "actor_id", "count", "fields", "field_count", "accepted_count", "rejected_count", "qos":
		if a.Value.Kind() == slog.KindInt64 || a.Value.Kind() == slog.KindUint64 {
			return a
		}
	case "retain":
		if a.Value.Kind() == slog.KindBool {
			return a
		}
	case "kind":
		switch a.Value.String() {
		case "properties", "status_change", "ack":
			return a
		}
	case "field":
		switch a.Value.String() {
		case "temperature", "dissolved_oxygen", "ph", "turbidity", "salinity", "battery", "signal":
			return a
		}
	case "protocol":
		if a.Value.String() == "mqtt" {
			return a
		}
	default:
		return slog.String(a.Key, "redacted")
	}
	return slog.String(a.Key, "redacted")
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
