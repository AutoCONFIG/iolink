package observability

import (
	"fmt"
	"strconv"
	"time"
)

type Config struct {
	Dir             string
	Version         string
	Level           string
	MaxMB           int
	Backups         int
	Retention       time.Duration
	CleanupInterval time.Duration
}

func FromEnv(getenv func(string) string) (Config, error) {
	c := Config{Dir: getenv("IOLINK_LOG_DIR"), Version: getenv("IOLINK_VERSION"), Level: getenv("IOLINK_LOG_LEVEL"), MaxMB: 20, Backups: 5, Retention: 6 * time.Hour, CleanupInterval: 6 * time.Hour}
	if c.Dir == "" {
		c.Dir = "./logs"
	}
	if c.Level == "" {
		c.Level = "info"
	}
	if c.Version == "" {
		c.Version = "dev"
	}
	for _, field := range []struct {
		key string
		dst *time.Duration
	}{{"IOLINK_LOG_RETENTION", &c.Retention}, {"IOLINK_LOG_CLEANUP_INTERVAL", &c.CleanupInterval}} {
		if raw := getenv(field.key); raw != "" {
			d, err := time.ParseDuration(raw)
			if err != nil || d <= 0 {
				return c, fmt.Errorf("%s must be a positive duration", field.key)
			}
			*field.dst = d
		}
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
