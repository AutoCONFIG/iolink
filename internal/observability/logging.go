package observability

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
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
	mu     sync.Mutex
	file   io.WriteCloser
	stderr io.Writer
	failed atomic.Bool
	closed bool
}

func Console(c Config, stderr io.Writer) (*slog.Logger, error) {
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
		return nil, errors.New("IOLINK_LOG_LEVEL must be debug, info, warn or error")
	}
	return slog.New(slog.NewJSONHandler(stderr, &slog.HandlerOptions{Level: level, ReplaceAttr: safeAttr})), nil
}

func New(c Config, stderr io.Writer) (*slog.Logger, *Sink, error) {
	if _, err := Console(c, stderr); err != nil {
		return nil, nil, err
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
	log, err := Console(c, sink)
	return log, sink, err
}

func (s *Sink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, os.ErrClosed
	}
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
func (s *Sink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return s.file.Close()
}
