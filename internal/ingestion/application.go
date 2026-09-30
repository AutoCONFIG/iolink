package ingestion

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"unicode/utf8"

	"git.hyhy.fun/rsplab/iolink/internal/event"
)

var (
	ErrInvalidEvent  = errors.New("invalid normalized ingestion event")
	ErrUnknownMetric = errors.New("unknown ingestion metric")
)

var metricRanges = map[string][2]float64{
	"temperature":      {0, 50},
	"dissolved_oxygen": {0, 20},
	"ph":               {0, 14},
	"turbidity":        {0, 1000},
	"salinity":         {0, 50},
	"battery":          {0, 100},
	"signal":           {-120, 0},
}

func MetricRange(name string) (float64, float64, bool) {
	range_, ok := metricRanges[name]
	if !ok {
		return 0, 0, false
	}
	return range_[0], range_[1], true
}

func Validate(e event.Event) error {
	if e.DeviceNo == "" || !utf8.ValidString(e.DeviceNo) || strings.ContainsAny(e.DeviceNo, "/+#") || e.Ts.IsZero() {
		return ErrInvalidEvent
	}
	switch e.Kind {
	case event.KindProperties:
		if e.GenericProperties != nil {
			if len(e.Properties) != 0 || len(e.GenericProperties) == 0 {
				return ErrInvalidEvent
			}
			for name, raw := range e.GenericProperties {
				if name == "message_id" || !utf8.ValidString(name) || len(name) > 64 {
					return ErrInvalidEvent
				}
				if !json.Valid(raw) {
					return ErrInvalidEvent
				}
			}
			return nil
		}
		if !utf8.ValidString(e.MessageID) || utf8.RuneCountInString(e.MessageID) > 128 {
			return ErrInvalidEvent
		}
		for name, value := range e.Properties {
			range_, ok := metricRanges[name]
			if !ok {
				return ErrUnknownMetric
			}
			if math.IsNaN(value) || math.IsInf(value, 0) || value < range_[0] || value > range_[1] {
				return ErrInvalidEvent
			}
			if name == "signal" && value != math.Trunc(value) {
				return ErrInvalidEvent
			}
		}
	case event.KindStatusChange:
		if e.MessageID != "" || e.Properties != nil {
			return ErrInvalidEvent
		}
	default:
		return ErrInvalidEvent
	}
	return nil
}
