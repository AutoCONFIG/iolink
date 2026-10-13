package wechat

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/notifications"
)

type downstreamTrace struct {
	ctx       context.Context
	operation string
	started   time.Time
	status    int
	outcome   string
	errorCode string
}

func (s *Sender) startTrace(ctx context.Context, operation string) *downstreamTrace {
	s.logger.DebugContext(ctx, "wechat downstream request", "stage", "wechat", "operation", operation, "outcome", "started")
	return &downstreamTrace{ctx: ctx, operation: operation, started: time.Now(), outcome: "success"}
}

func (t *downstreamTrace) complete(logger *slog.Logger, err error) {
	if err != nil {
		t.outcome = "failed"
		if t.errorCode == "" {
			switch {
			case errors.Is(err, notifications.ErrRetryable):
				t.errorCode = "retryable"
			case errors.Is(err, notifications.ErrPermanent):
				t.errorCode = "permanent"
			case errors.Is(err, notifications.ErrUnknown):
				t.errorCode = "unknown"
			case errors.Is(err, errTokenExpired):
				t.errorCode = "token_expired"
			default:
				t.errorCode = "provider_error"
			}
		}
	}
	logger.DebugContext(t.ctx, "wechat downstream request", "stage", "wechat", "operation", t.operation, "outcome", t.outcome, "status", t.status, "duration_ms", time.Since(t.started).Milliseconds(), "error_code", t.errorCode)
}

func transportErrorCode(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "context_canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline_exceeded"
	default:
		var network net.Error
		if errors.As(err, &network) && network.Timeout() {
			return "network_timeout"
		}
		return "network_error"
	}
}
