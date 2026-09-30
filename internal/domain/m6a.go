package domain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"regexp"
	"sort"
	"time"
)

var (
	ErrInvalidProductModel = errors.New("invalid product model")
	ErrPublishedModel      = errors.New("product model is already published")
	ErrInactiveTenant      = errors.New("tenant is inactive")
)

type Product struct {
	ID             int64     `json:"id"`
	TenantID       int64     `json:"tenant_id"`
	Name           string    `json:"name"`
	Builtin        bool      `json:"builtin"`
	CurrentVersion *int      `json:"current_version"`
	CreatedAt      time.Time `json:"created_at"`
}
type ModelField struct {
	Identifier string   `json:"identifier"`
	Type       string   `json:"type"`
	Unit       string   `json:"unit"`
	Min        *float64 `json:"minimum"`
	Max        *float64 `json:"maximum"`
	Enum       []string `json:"enum_values"`
	Readable   bool     `json:"readable"`
	Writable   bool     `json:"writable"`
	Nullable   bool     `json:"nullable"`
}
type ProductModel struct {
	ID          int64           `json:"-"`
	ProductID   int64           `json:"product_id"`
	Version     int             `json:"version"`
	Schema      json.RawMessage `json:"-"`
	Fields      []ModelField    `json:"fields"`
	PublishedAt *time.Time      `json:"published_at"`
	CreatedAt   time.Time       `json:"created_at"`
	AssignedAt  *time.Time      `json:"-"`
}

type TelemetryRejected struct {
	Identifier string `json:"identifier"`
	Reason     string `json:"reason"`
}

type TelemetryV2Result struct {
	DeviceNo     string              `json:"device_no"`
	ModelVersion int                 `json:"model_version"`
	Accepted     []string            `json:"accepted"`
	Rejected     []TelemetryRejected `json:"rejected"`
}

type TelemetryHistoryPoint struct {
	Timestamp    time.Time `json:"ts"`
	Value        any       `json:"value"`
	ModelVersion int       `json:"model_version"`
	Unit         string    `json:"unit,omitempty"`
}

type DeviceModelLatest struct {
	DeviceNo     string                     `json:"device_no"`
	ProductID    int64                      `json:"product_id"`
	ModelVersion int                        `json:"model_version"`
	Fields       []ModelField               `json:"fields"`
	Timestamp    *time.Time                 `json:"ts"`
	Properties   map[string]json.RawMessage `json:"properties"`
}

type DeviceModelLatestRepo interface {
	ModelLatestForUser(ctx context.Context, deviceNo string, userID int64) (DeviceModelLatest, error)
}

type GenericTelemetryRepo interface {
	SubmitTelemetry(ctx context.Context, deviceNo string, userID int64, ts time.Time, properties map[string]json.RawMessage) (TelemetryV2Result, error)
	HistoryV2ForUser(ctx context.Context, deviceNo string, userID int64, metric string, from, to time.Time, limit int) ([]TelemetryHistoryPoint, string, error)
}

var telemetryIdentifierRE = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func ValidateTelemetryProperties(fields []ModelField, properties map[string]json.RawMessage) (map[string]json.RawMessage, []TelemetryRejected) {
	byID := make(map[string]ModelField, len(fields))
	for _, field := range fields {
		byID[field.Identifier] = field
	}
	accepted := make(map[string]json.RawMessage)
	rejected := make([]TelemetryRejected, 0)
	for identifier, raw := range properties {
		reason := ""
		field, ok := byID[identifier]
		if !ok || !telemetryIdentifierRE.MatchString(identifier) {
			reason = "unknown_identifier"
		} else {
			reason = validateTelemetryValue(field, raw)
		}
		if reason != "" {
			rejected = append(rejected, TelemetryRejected{Identifier: identifier, Reason: reason})
			continue
		}
		accepted[identifier] = raw
	}
	sort.Slice(rejected, func(i, j int) bool { return rejected[i].Identifier < rejected[j].Identifier })
	return accepted, rejected
}

func validateTelemetryValue(field ModelField, raw json.RawMessage) string {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		if field.Nullable {
			return ""
		}
		return "not_nullable"
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return "invalid_type"
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return "invalid_type"
	}
	switch field.Type {
	case "number", "integer":
		n, ok := value.(json.Number)
		if !ok {
			return "invalid_type"
		}
		f, err := n.Float64()
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || (field.Type == "integer" && f != math.Trunc(f)) {
			return "invalid_type"
		}
		if field.Min != nil && f < *field.Min || field.Max != nil && f > *field.Max {
			return "out_of_range"
		}
		if len(field.Enum) > 0 {
			return "invalid_enum"
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return "invalid_type"
		}
	case "string":
		s, ok := value.(string)
		if !ok {
			return "invalid_type"
		}
		if len(field.Enum) > 0 {
			found := false
			for _, allowed := range field.Enum {
				if s == allowed {
					found = true
					break
				}
			}
			if !found {
				return "invalid_enum"
			}
		}
	default:
		return "invalid_type"
	}
	return ""
}
