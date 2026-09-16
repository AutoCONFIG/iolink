// Package domain defines shared domain models and repository interfaces.
//
// This is the ONLY place where modules meet. access/core/appapi must not
// import each other — they import this module instead.
// Changes here are protocol changes: review + bump tag required.
package domain

import (
	"context"
	"errors"
	"time"
)

// ErrUnknownMetric is returned when a metric key is not a recognized column.
var ErrUnknownMetric = errors.New("unknown metric")

// ---- Domain entities (User -> Farm -> Pond -> Device -> Sensor) ----

type User struct {
	ID          int64
	OpenID      string // WeChat openid, unique
	Nickname    string
	Phone       string
	CreatedAt   time.Time
}

type Farm struct {
	ID        int64
	OwnerID   int64 // User.ID
	Name      string
	Location  string
	CreatedAt time.Time
}

type Pond struct {
	ID      int64
	FarmID  int64
	Name    string
	AreaMu  float64 // area in 亩
	CreatedAt time.Time
}

type Device struct {
	ID          int64
	PondID      int64
	DeviceNo    string // hardware identity, unique, used as IoT device name
	Model       string
	Status      DeviceStatus
	LastSeenAt  *time.Time
	CreatedAt   time.Time
}

type DeviceStatus string

const (
	DeviceOnline  DeviceStatus = "online"
	DeviceOffline DeviceStatus = "offline"
)

type Sensor struct {
	ID       int64
	DeviceID int64
	Key      string // "temperature", "dissolved_oxygen", "ph", "turbidity", "salinity"
	Name     string
	Unit     string
}

// ---- Telemetry ----

// Reading is one normalized sensor sample from a device.
type Reading struct {
	DeviceNo    string
	Timestamp   time.Time
	Temperature *float64 // pointers: absent fields stay nil
	DO          *float64
	PH          *float64
	Turbidity   *float64
	Salinity    *float64
}

// ---- Alarms ----

type Alarm struct {
	ID          int64
	DeviceNo    string
	PondID      int64
	Metric      string  // "dissolved_oxygen", ...
	CurrentValue float64
	Threshold   float64
	Level       AlarmLevel
	Message     string
	ConfirmedAt *time.Time
	CreatedAt   time.Time
}

type AlarmLevel string

const (
	AlarmCritical AlarmLevel = "critical" // red
	AlarmWarning  AlarmLevel = "warning"  // yellow
)

// ---- Repository interfaces ----
// appapi depends on these; core implements them. Consumers write fakes.

type PondRepo interface {
	ListByUser(ctx context.Context, userID int64) ([]Pond, error)
	Get(ctx context.Context, id int64) (Pond, error)
}

type DeviceRepo interface {
	ListByPond(ctx context.Context, pondID int64) ([]Device, error)
	GetByDeviceNo(ctx context.Context, deviceNo string) (Device, error)
	UpdateStatus(ctx context.Context, deviceNo string, s DeviceStatus) error
}

type TelemetryRepo interface {
	// Latest returns the most recent reading per metric for a device.
	Latest(ctx context.Context, deviceNo string) (Reading, error)
	// History returns readings in [from, to], bucketed for charting.
	History(ctx context.Context, deviceNo string, metric string, from, to time.Time, maxPoints int) ([]MetricPoint, error)
}

type MetricPoint struct {
	Ts    time.Time `json:"ts"`
	Value float64   `json:"value"`
}

type AlarmRepo interface {
	ListByUser(ctx context.Context, userID int64, limit int) ([]Alarm, error)
	Confirm(ctx context.Context, alarmID int64) error
}

type AlarmRuleRepo interface {
	// RulesForDevice returns active threshold rules for a device's pond.
	RulesForDevice(ctx context.Context, deviceNo string) ([]AlarmRule, error)
}

type AlarmRule struct {
	ID        int64
	PondID    int64
	Metric    string
	Min       *float64 // nil = no lower threshold
	Max       *float64 // nil = no upper threshold
	Level     AlarmLevel
	Enabled   bool
}
