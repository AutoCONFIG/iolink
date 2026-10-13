package observability

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

type Sink struct {
	mu        sync.Mutex
	file      io.WriteCloser
	stderr    io.Writer
	failed    atomic.Bool
	fileOnly  bool
	backups   int
	maxBytes  int64
	written   int64
	opened    bool
	closed    bool
	closeOnce sync.Once
	closeErr  error
	rootDir   string
	path      string
	retention time.Duration
	stop      chan struct{}
	done      chan struct{}
}

func Console(c Config, stderr io.Writer) (*slog.Logger, error) {
	handler, err := consoleHandler(c, stderr)
	if err != nil {
		return nil, err
	}
	return slog.New(contextHandler{handler}), nil
}

func consoleHandler(c Config, stderr io.Writer) (slog.Handler, error) {
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
	return slog.NewJSONHandler(stderr, &slog.HandlerOptions{Level: level, ReplaceAttr: safeAttr}), nil
}

func New(c Config, stderr io.Writer) (*slog.Logger, *Sink, error) {
	if c.Retention == 0 {
		c.Retention = 6 * time.Hour
	}
	if c.CleanupInterval == 0 {
		c.CleanupInterval = 6 * time.Hour
	}
	if _, err := Console(c, stderr); err != nil {
		return nil, nil, err
	}
	if c.Dir == "" || c.MaxMB < 1 || c.MaxMB > 1024 || c.Backups < 1 || c.Backups > 20 || c.Retention <= 0 || c.CleanupInterval <= 0 {
		return nil, nil, errors.New("invalid diagnostic log configuration")
	}
	if c.Version == "" {
		c.Version = "dev"
	}
	dir := c.Dir
	if c.Version != "" {
		version := filepath.Base(filepath.Clean(c.Version))
		if version == "." || version == ".." || version == "" || version != c.Version {
			return nil, nil, errors.New("invalid diagnostic log version")
		}
		dir = filepath.Join(c.Dir, version)
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, nil, errors.New("cannot create diagnostic log directory")
	}
	if info, err := os.Lstat(dir); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, nil, errors.New("invalid diagnostic log directory")
	}
	path := filepath.Join(dir, "iolinkd.jsonl")
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return nil, nil, errors.New("invalid diagnostic log file")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, errors.New("cannot inspect diagnostic log file")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, errors.New("cannot open diagnostic log file")
	}
	if err = f.Chmod(0o600); err != nil {
		if closeErr := f.Close(); closeErr != nil {
			return nil, nil, errors.New("cannot close diagnostic log file")
		}
		return nil, nil, errors.New("cannot protect diagnostic log file")
	}
	if err = f.Close(); err != nil {
		return nil, nil, errors.New("cannot close diagnostic log file")
	}
	file := &lumberjack.Logger{Filename: path, MaxSize: c.MaxMB}
	sink := &Sink{file: file, fileOnly: true, backups: c.Backups, maxBytes: int64(c.MaxMB) * 1024 * 1024, stderr: stderr, rootDir: c.Dir, path: path, retention: c.Retention, stop: make(chan struct{}), done: make(chan struct{})}
	if err := sink.cleanup(time.Now()); err != nil {
		if closeErr := file.Close(); closeErr != nil {
			return nil, nil, errors.New("cannot close diagnostic log file")
		}
		return nil, nil, errors.New("cannot clean diagnostic log files")
	}
	console, err := consoleHandler(c, consoleWriter{sink})
	if err != nil {
		return nil, nil, err
	}
	dump := slog.NewJSONHandler(sink, &slog.HandlerOptions{Level: slog.LevelDebug, ReplaceAttr: safeAttr})
	log := slog.New(contextHandler{dumpHandler{dump, console}})
	go sink.cleanupLoop(c.CleanupInterval)
	return log, sink, nil
}

func (s *Sink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, os.ErrClosed
	}
	nextSize := s.written + int64(len(p))
	rotated := s.maxBytes > 0 && (nextSize > s.maxBytes || (!s.opened && nextSize >= s.maxBytes))
	n, err := s.file.Write(p)
	if err == nil {
		s.opened = true
	}
	if rotated && err == nil {
		s.written = 0
		if cleanupErr := s.limitBackups(); cleanupErr != nil {
			s.markFailed("diagnostic log cleanup failed; readiness disabled\n")
			err = cleanupErr
		}
	}
	s.written += int64(n)
	if err != nil || n != len(p) {
		if !s.failed.Load() {
			s.markFailed("diagnostic log write failed; readiness disabled\n")
		}
		if err == nil {
			err = io.ErrShortWrite
		}
	}
	if s.fileOnly && err == nil {
		return n, nil
	}
	stderrN, stderrErr := s.writeStderr(p)
	if stderrN != len(p) && stderrErr == nil {
		stderrErr = io.ErrShortWrite
	}
	if err != nil {
		return n, err
	}
	return n, stderrErr
}
func (s *Sink) Healthy() bool { return !s.failed.Load() }
func (s *Sink) Close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		if s.stop != nil {
			close(s.stop)
		}
		s.closeErr = s.file.Close()
		if s.closeErr != nil {
			s.markFailed("diagnostic log close failed; readiness disabled\n")
		}
		s.mu.Unlock()
		if s.done != nil {
			<-s.done
		}
	})
	return s.closeErr
}

func (s *Sink) markFailed(message string) {
	s.failed.Store(true)
	// The failure message is constant and never includes filesystem paths or raw errors.
	if _, err := io.WriteString(s.stderr, message); err != nil {
		s.failed.Store(true)
	}
}
