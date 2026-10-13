package observability

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const rotationTimeFormat = "2006-01-02T15-04-05.000"

func (s *Sink) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	defer close(s.done)
	for {
		select {
		case <-ticker.C:
			if err := s.cleanup(time.Now()); err != nil {
				continue
			}
		case <-s.stop:
			return
		}
	}
}

func (s *Sink) cleanup(now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	// Reuse the writer: lumberjack reopens on its next Write after Close.
	if err := s.file.Close(); err != nil {
		s.markFailed("diagnostic log cleanup failed; readiness disabled\n")
		return err
	}
	s.opened = false
	err := s.pruneVersions(now.Add(-s.retention))
	if err == nil {
		err = s.limitBackups()
	}
	if err == nil {
		info, statErr := os.Stat(s.path)
		err = statErr
		if err == nil {
			s.written = info.Size()
		}
	}
	if err != nil {
		s.markFailed("diagnostic log cleanup failed; readiness disabled\n")
	}
	return err
}

func (s *Sink) pruneVersions(cutoff time.Time) error {
	versions, err := os.ReadDir(s.rootDir)
	if err != nil {
		return err
	}
	for _, version := range versions {
		if !version.IsDir() || version.Type()&os.ModeSymlink != 0 {
			continue
		}
		dir := filepath.Join(s.rootDir, version.Name())
		files, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, entry := range files {
			if !diagnosticFile(entry.Name()) || !entry.Type().IsRegular() {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			if err := pruneLogFile(path, cutoff); err != nil {
				return err
			}
			// Empty previous/rotated dumps have no retained records. Keep the active file.
			if path != s.path {
				info, err := os.Lstat(path)
				if err != nil {
					return err
				}
				if info.Size() == 0 {
					if err := os.Remove(path); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func diagnosticFile(name string) bool {
	if name == "iolinkd.jsonl" {
		return true
	}
	if !strings.HasPrefix(name, "iolinkd-") || !strings.HasSuffix(name, ".jsonl") {
		return false
	}
	timestamp := strings.TrimSuffix(strings.TrimPrefix(name, "iolinkd-"), ".jsonl")
	_, err := time.Parse(rotationTimeFormat, timestamp)
	return err == nil
}

func pruneLogFile(path string, cutoff time.Time) (err error) {
	input, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, input.Close()) }()
	if err := input.Chmod(0o600); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".iolinkd-cleanup-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		if closeErr := temporary.Close(); closeErr != nil && !errors.Is(closeErr, os.ErrClosed) {
			err = errors.Join(err, closeErr)
		}
		if removeErr := os.Remove(temporaryPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, removeErr)
		}
	}()
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	output := bufio.NewWriter(temporary)
	removed := false
	for scanner.Scan() {
		line := scanner.Bytes()
		var record struct {
			Time time.Time `json:"time"`
		}
		if json.Unmarshal(line, &record) == nil && !record.Time.IsZero() && record.Time.Before(cutoff) {
			removed = true
			continue
		}
		if _, err := output.Write(line); err != nil {
			return err
		}
		if err := output.WriteByte('\n'); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if !removed {
		return nil
	}
	if err := output.Flush(); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
