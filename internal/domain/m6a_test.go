package domain

import (
	"encoding/json"
	"testing"
)

func TestValidateTelemetryPropertiesRejectsUnknownAndInvalidValues(t *testing.T) {
	min, max := 0.0, 10.0
	fields := []ModelField{
		{Identifier: "temperature", Type: "number", Min: &min, Max: &max},
		{Identifier: "mode", Type: "string", Enum: []string{"auto", "manual"}},
		{Identifier: "enabled", Type: "boolean"},
	}
	accepted, rejected := ValidateTelemetryProperties(fields, map[string]json.RawMessage{
		"temperature": json.RawMessage(`11`),
		"mode":        json.RawMessage(`"bad"`),
		"enabled":     json.RawMessage(`true`),
		"unknown":     json.RawMessage(`1`),
	})
	if len(accepted) != 1 || len(rejected) != 3 {
		t.Fatalf("accepted=%v rejected=%v", accepted, rejected)
	}
	if _, ok := accepted["enabled"]; !ok {
		t.Fatal("valid field was not accepted")
	}
}

func TestValidateTelemetryPropertiesHonorsNullableAndInteger(t *testing.T) {
	fields := []ModelField{{Identifier: "count", Type: "integer"}, {Identifier: "note", Type: "string", Nullable: true}}
	accepted, rejected := ValidateTelemetryProperties(fields, map[string]json.RawMessage{
		"count": json.RawMessage(`1.2`),
		"note":  json.RawMessage(`null`),
	})
	if len(accepted) != 1 || len(rejected) != 1 || rejected[0].Reason != "invalid_type" {
		t.Fatalf("accepted=%v rejected=%v", accepted, rejected)
	}
}
