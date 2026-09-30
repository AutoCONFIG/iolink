package ingestion

import (
	"errors"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/event"
)

func TestIngestionValidatePropertiesWhenMetricsAreValid(t *testing.T) {
	e := event.Event{
		Kind: event.KindProperties, DeviceNo: "device-1", Ts: time.Unix(10, 0).UTC(),
		MessageID: "sample-1", Properties: map[string]float64{"temperature": 26, "signal": -65},
	}
	if err := Validate(e); err != nil {
		t.Fatalf("valid event rejected: %v", err)
	}
}

func TestIngestionValidatePropertiesWhenMetricIsUnknown(t *testing.T) {
	e := event.Event{Kind: event.KindProperties, DeviceNo: "device-1", Ts: time.Unix(10, 0).UTC(), Properties: map[string]float64{"Temperature": 26}}
	if !errors.Is(Validate(e), ErrUnknownMetric) {
		t.Fatalf("want unknown metric error, got %v", Validate(e))
	}
}

func TestIngestionValidatePropertiesWhenSignalIsFractional(t *testing.T) {
	e := event.Event{Kind: event.KindProperties, DeviceNo: "device-1", Ts: time.Unix(10, 0).UTC(), Properties: map[string]float64{"signal": -65.5}}
	if !errors.Is(Validate(e), ErrInvalidEvent) {
		t.Fatalf("want invalid event error, got %v", Validate(e))
	}
}

func TestIngestionValidatePropertiesWhenDeviceIdentityCanEscapeTopicScope(t *testing.T) {
	e := event.Event{Kind: event.KindProperties, DeviceNo: "device/other", Ts: time.Unix(10, 0).UTC(), Properties: map[string]float64{"temperature": 26}}
	if !errors.Is(Validate(e), ErrInvalidEvent) {
		t.Fatalf("want invalid identity error, got %v", Validate(e))
	}
}

func TestIngestionValidateStatusWhenPayloadIsPresent(t *testing.T) {
	e := event.Event{Kind: event.KindStatusChange, DeviceNo: "device-1", Ts: time.Unix(10, 0).UTC(), Properties: map[string]float64{"battery": 90}}
	if !errors.Is(Validate(e), ErrInvalidEvent) {
		t.Fatalf("want invalid status error, got %v", Validate(e))
	}
}
