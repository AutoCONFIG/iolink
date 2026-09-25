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
var ErrInvalidRule = errors.New("invalid rule")
var ErrInvalidRange = errors.New("invalid range or max_points")

var ErrUnknownMetric = errors.New("unknown metric")

// ErrPondHasDevices is returned when deleting a pond that still has devices bound.
var ErrPondHasDevices = errors.New("pond has bound devices")

// ErrFarmHasPonds is returned when deleting a farm that still has ponds.
var ErrFarmHasPonds = errors.New("farm has ponds")

// ErrOldPasswordMismatch is returned when changing a password with a wrong old one.
var ErrOldPasswordMismatch = errors.New("old password mismatch")
var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("conflict")

// MetricColumns is the fixed thing-model: metric key -> sensor_data column.
// Single source of truth for the whitelist enforced everywhere.
var MetricColumns = map[string]string{
	"temperature":      "temperature",
	"dissolved_oxygen": "dissolved_oxygen",
	"ph":               "ph",
	"turbidity":        "turbidity",
	"salinity":         "salinity",
}

// ValidMetric reports whether m is a known metric key.
func ValidMetric(m string) bool {
	_, ok := MetricColumns[m]
	return ok
}

// Stats is the overview counters for the admin dashboard.
type Stats struct {
	DevicesTotal int64 `json:"devices_total"`
	Online       int64 `json:"online"`
	Offline      int64 `json:"offline"`
	OpenAlarms   int64 `json:"open_alarms"`
}

// ---- Domain entities (User -> Farm -> Pond -> Device -> Sensor) ----

type User struct {
	ID           int64 `json:"id"`
	OpenID       string `json:"-"`
	Username     *string `json:"username,omitempty"`
	PasswordHash *string `json:"-"`
	Authority    string `json:"authority,omitempty"`
	Nickname     string `json:"nickname"`
	Phone        string `json:"phone,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	TokenVersion int `json:"-"`
}

type Farm struct {
	ID        int64     `json:"id"`
	OwnerID   *int64    `json:"owner_id"`
	Name      string    `json:"name"`
	Location  string    `json:"location"`
	CreatedAt time.Time `json:"created_at"`
}

type Pond struct {
	ID        int64 `json:"id"`
	FarmID    int64 `json:"farm_id"`
	Name      string `json:"name"`
	AreaMu    float64 `json:"area_mu"`
	CreatedAt time.Time `json:"created_at"`
}

type Device struct {
	ID             int64 `json:"id"`
	PondID         int64 `json:"pond_id"`
	DeviceNo       string `json:"device_no"`
	Model          string `json:"model"`
	Name           string `json:"name"`
	Status         DeviceStatus `json:"status"`
	LastSeenAt     *time.Time `json:"last_seen_at"`
	CreatedAt      time.Time `json:"created_at"`
	DisabledAt     *time.Time `json:"disabled_at,omitempty"`
	ReportInterval int        `json:"report_interval"`
}

type DeviceStatus string

const (
	DeviceOnline  DeviceStatus = "online"
	DeviceOffline DeviceStatus = "offline"
)

type Sensor struct {
	ID       int64  `json:"id"`
	DeviceID int64  `json:"device_id"`
	Key      string `json:"key"` // "temperature", "dissolved_oxygen", "ph", "turbidity", "salinity"
	Name     string `json:"name"`
	Unit     string `json:"unit"`
}

// ---- Telemetry ----

// Reading is one normalized sensor sample from a device.
type Reading struct {
	DeviceNo       string               `json:"device_no"`
	Timestamp      time.Time            `json:"ts"`
	PondID         int64                `json:"pond_id"`
	Timestamps     map[string]time.Time `json:"timestamps"`
	ReportInterval int                  `json:"report_interval"`
	Battery        *float64             `json:"battery,omitempty"`
	Signal         *int                 `json:"signal,omitempty"`
	Temperature    *float64             `json:"temperature,omitempty"` // pointers: absent fields stay nil
	DO             *float64             `json:"dissolved_oxygen,omitempty"`
	PH             *float64             `json:"ph,omitempty"`
	Turbidity      *float64             `json:"turbidity,omitempty"`
	Salinity       *float64             `json:"salinity,omitempty"`
}

// ---- Alarms ----

type Alarm struct {
	ID           int64 `json:"id"`
	DeviceNo     string `json:"device_no"`
	PondID       int64 `json:"pond_id"`
	Metric       string `json:"metric"`
	CurrentValue float64 `json:"current_value"`
	Threshold    float64 `json:"threshold"`
	Level        AlarmLevel `json:"level"`
	Message      string `json:"message"`
	ConfirmedAt  *time.Time `json:"confirmed_at"`
	CreatedAt    time.Time `json:"created_at"`
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
	GetByUser(ctx context.Context, id, userID int64) (Pond, error)
}

type DeviceRepo interface {
	ListByPond(ctx context.Context, pondID int64) ([]Device, error)
	GetByDeviceNo(ctx context.Context, deviceNo string) (Device, error)
	GetByDeviceNoForUser(ctx context.Context, deviceNo string, userID int64) (Device, error)
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
	ConfirmByUser(ctx context.Context, alarmID, userID int64) error
}

type AlarmRuleRepo interface {
	// RulesForDevice returns active threshold rules for a device's pond.
	RulesForDevice(ctx context.Context, deviceNo string) ([]AlarmRule, error)
}

type AlarmRule struct {
	ID      int64 `json:"id"`
	PondID  int64 `json:"pond_id"`
	Metric  string `json:"metric"`
	Min     *float64 `json:"min_value"`
	Max     *float64 `json:"max_value"`
	Level   AlarmLevel `json:"level"`
	Enabled bool `json:"enabled"`
}

var MetricUnits = map[string]string{"temperature": "℃", "dissolved_oxygen": "mg/L", "ph": "", "turbidity": "NTU", "salinity": "ppt"}
