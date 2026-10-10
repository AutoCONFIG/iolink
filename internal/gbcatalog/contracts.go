package gbcatalog

import (
	"context"
	"errors"
	"time"
)

const (
	RefreshTimeout = 10 * time.Second
	MaxItems       = 1000
	MaxNameBytes   = 128
)

var (
	ErrInvalid           = errors.New("invalid catalog refresh")
	ErrDeadline          = errors.New("catalog refresh deadline exceeded")
	ErrDuplicateConflict = errors.New("conflicting duplicate catalog item")
	ErrStore             = errors.New("catalog store failed")
)

type Item struct {
	ChannelID string
	Name      string
	Online    bool
}

type Page struct {
	DeviceID string
	SN       int
	SumNum   int
	Items    []Item
}

type Refresh struct {
	DeviceID      string
	SN            int
	ExpectedCount int
	Deadline      time.Time
}

type Snapshot struct {
	DeviceID   string
	SN         int
	Items      []Item
	ObservedAt time.Time
}

type CatalogStore interface {
	StartRefresh(context.Context, Refresh) (Refresh, error)
	ReplaceCatalog(context.Context, Snapshot) error
	MarkStale(context.Context, string, time.Time) error
}

type PageResult struct {
	Refresh   Refresh
	Complete  bool
	Duplicate bool
}

type pending struct {
	refresh    Refresh
	items      map[string]Item
	staling    bool
	committing bool
}
