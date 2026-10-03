package observability

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"

	"gopkg.in/natefinch/lumberjack.v2"
)

type Config struct {
	Dir     string
	Level   string
	MaxMB   int
	Backups int
}

func FromEnv(getenv func(string) string) (Config, error) {
	c := Config{Dir: getenv("IOLINK_LOG_DIR"), Level: getenv("IOLINK_LOG_LEVEL"), MaxMB: 20, Backups: 5}
	if c.Dir == "" {
		c.Dir = "./logs"
	}
	if c.Level == "" {
		c.Level = "info"
	}
	for _, field := range []struct {
		key string
		dst *int
		max int
	}{{"IOLINK_LOG_MAX_MB", &c.MaxMB, 1024}, {"IOLINK_LOG_BACKUPS", &c.Backups, 20}} {
		if raw := getenv(field.key); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 || n > field.max {
				return c, fmt.Errorf("%s must be between 1 and %d", field.key, field.max)
			}
			*field.dst = n
		}
	}
	return c, nil
}

type Sink struct {
	file   io.WriteCloser
	stderr io.Writer
	failed atomic.Bool
}

func New(c Config, stderr io.Writer) (*slog.Logger, *Sink, error) {
	var level slog.Level
	switch c.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return nil, nil, errors.New("IOLINK_LOG_LEVEL must be debug, info, warn or error")
	}
	if c.Dir == "" || c.MaxMB < 1 || c.MaxMB > 1024 || c.Backups < 1 || c.Backups > 20 {
		return nil, nil, errors.New("invalid diagnostic log configuration")
	}
	if err := os.MkdirAll(c.Dir, 0o750); err != nil {
		return nil, nil, errors.New("cannot create diagnostic log directory")
	}
	path := filepath.Join(c.Dir, "iolinkd.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, errors.New("cannot open diagnostic log file")
	}
	if err = f.Chmod(0o600); err != nil {
		f.Close()
		return nil, nil, errors.New("cannot protect diagnostic log file")
	}
	if err = f.Close(); err != nil {
		return nil, nil, errors.New("cannot close diagnostic log file")
	}
	file := &lumberjack.Logger{Filename: path, MaxSize: c.MaxMB, MaxBackups: c.Backups}
	sink := &Sink{file: file, stderr: stderr}
	return slog.New(slog.NewJSONHandler(sink, &slog.HandlerOptions{Level: level, ReplaceAttr: safeAttr})), sink, nil
}

func (s *Sink) Write(p []byte) (int, error) {
	n, err := s.file.Write(p)
	if err != nil || n != len(p) {
		if !s.failed.Swap(true) {
			_, _ = io.WriteString(s.stderr, "diagnostic log write failed; readiness disabled\n")
		}
		if err == nil {
			err = io.ErrShortWrite
		}
	}
	_, stderrErr := s.stderr.Write(p)
	if err != nil {
		return n, err
	}
	return n, stderrErr
}
func (s *Sink) Healthy() bool { return !s.failed.Load() }
func (s *Sink) Close() error  { return s.file.Close() }

func safeAttr(_ []string, a slog.Attr) slog.Attr {
	if a.Key == "err" || a.Key == "error" {
		if err, ok := a.Value.Any().(error); ok {
			return slog.String(a.Key, fmt.Sprintf("%T", err))
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
