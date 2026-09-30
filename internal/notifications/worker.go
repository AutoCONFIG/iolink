package notifications

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"
)

var (
	ErrDisabled  = errors.New("notification provider disabled")
	ErrPermanent = errors.New("permanent notification failure")
	ErrUnknown   = errors.New("notification result unknown")
	ErrRetryable = errors.New("retryable notification failure")
	ErrLeaseLost = errors.New("notification lease lost")
)

type Delivery struct {
	AlarmID      int64
	RecipientID  string
	DeviceNo     string
	PondID       int64
	Metric       string
	CurrentValue float64
	Threshold    float64
	Level        string
	Message      string
	CreatedAt    time.Time
}

type Sender interface {
	Send(context.Context, Delivery) error
}

type Claim struct {
	ID         int64
	Attempts   int
	LeaseToken string
	Delivery   Delivery
}

type Result struct {
	Status    string
	NextAt    time.Time
	LastError string
}

type DeliveryStore interface {
	Claim(context.Context, time.Time, time.Duration, int) (Claim, bool, error)
	Finish(context.Context, Claim, Result, time.Time) error
}

type QueueDepthStore interface {
	QueueDepth(context.Context) (int, error)
}

type Config struct {
	Store           DeliveryStore
	Sender          Sender
	LeaseDuration   time.Duration
	DeliveryTimeout time.Duration
	PollInterval    time.Duration
	MaxAttempts     int
	Concurrency     int
	Now             func() time.Time
	Logger          *slog.Logger
	OnSent          func()
	OnFailure       func()
}

type Worker struct{ cfg Config }

func NewWorker(cfg Config) (*Worker, error) {
	if cfg.Store == nil {
		return nil, errors.New("notification worker requires a delivery store")
	}
	if cfg.DeliveryTimeout <= 0 {
		cfg.DeliveryTimeout = 20 * time.Second
	}
	if cfg.LeaseDuration <= 0 {
		cfg.LeaseDuration = 2 * time.Minute
	}
	if cfg.LeaseDuration <= cfg.DeliveryTimeout {
		return nil, errors.New("notification lease must exceed delivery timeout")
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = time.Second
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 6
	}
	if cfg.MaxAttempts > 6 {
		return nil, errors.New("notification attempts exceed initial send plus five retries")
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 2
	}
	if cfg.Concurrency > 32 {
		return nil, errors.New("notification concurrency exceeds 32")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Worker{cfg: cfg}, nil
}

func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	c, ok, err := w.cfg.Store.Claim(ctx, w.cfg.Now().UTC(), w.cfg.LeaseDuration, w.cfg.MaxAttempts)
	if queue, ok := w.cfg.Store.(QueueDepthStore); ok {
		if depth, depthErr := queue.QueueDepth(ctx); depthErr == nil {
			MetricQueueDepth.Set(float64(depth))
		}
	}
	if err != nil || !ok {
		return ok, err
	}
	var sendErr error
	started := time.Now()
	if c.Delivery.RecipientID == "" || w.cfg.Sender == nil {
		sendErr = ErrDisabled
	} else {
		sendCtx, cancel := context.WithTimeout(ctx, w.cfg.DeliveryTimeout)
		sendErr = w.cfg.Sender.Send(sendCtx, c.Delivery)
		cancel()
	}
	MetricDeliverySeconds.Observe(time.Since(started).Seconds())
	now := w.cfg.Now().UTC()
	result := Result{Status: "sent", NextAt: now}
	if sendErr != nil {
		result.LastError = sendErr.Error()
		switch {
		case errors.Is(sendErr, ErrDisabled):
			result.Status = "disabled"
		case errors.Is(sendErr, ErrPermanent):
			result.Status = "failed"
		case errors.Is(sendErr, ErrUnknown), errors.Is(sendErr, context.Canceled), errors.Is(sendErr, context.DeadlineExceeded):
			result.Status = "unknown"
		case errors.Is(sendErr, ErrRetryable):
			result.Status = "failed"
			if c.Attempts < w.cfg.MaxAttempts {
				result.Status = "retryable"
				result.NextAt = now.Add(time.Duration(1<<uint(c.Attempts-1)) * time.Minute)
			}
		default:
			result.Status = "failed"
			if c.Attempts < w.cfg.MaxAttempts {
				result.Status = "retryable"
				result.NextAt = now.Add(time.Duration(1<<uint(c.Attempts-1)) * time.Minute)
			}
		}
	}
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err = w.cfg.Store.Finish(finishCtx, c, result, now); err != nil {
		return true, err
	}
	if sendErr != nil {
		if w.cfg.OnFailure != nil {
			w.cfg.OnFailure()
		}
		return true, fmt.Errorf("notification %s: %w", result.Status, sendErr)
	}
	if w.cfg.OnSent != nil {
		w.cfg.OnSent()
	}
	return true, nil
}

func (w *Worker) Run(ctx context.Context) {
	var group errgroup.Group
	for range w.cfg.Concurrency {
		group.Go(func() error {
			for ctx.Err() == nil {
				ok, err := w.RunOnce(ctx)
				if err != nil && ctx.Err() == nil {
					w.cfg.Logger.Warn("notification delivery", "err", err)
				}
				if ok && err == nil {
					continue
				}
				timer := time.NewTimer(w.cfg.PollInterval)
				select {
				case <-ctx.Done():
					timer.Stop()
					return nil
				case <-timer.C:
				}
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		w.cfg.Logger.Error("notification worker stopped", "err", err)
	}
}
