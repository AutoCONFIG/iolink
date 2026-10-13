package appapi

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestExtendedRequestParsingEnforcesExactTypedFields(t *testing.T) {
	for _, body := range []string{`{"tenant_id":7}`, ` {"\u0074enant_id":7} `} {
		// Given: a valid integer tenant identifier, with optional JSON escaping.
		reader := strings.NewReader(body)
		// When: decoding a tenant switch at the input boundary.
		request, err := decodeTenantSwitch(reader)
		// Then: its exact positive integer value is preserved.
		if err != nil || request.TenantID != 7 {
			t.Fatal("valid tenant switch rejected or changed")
		}
	}
	for _, body := range []string{`{}`, `null`, `[]`, `{"tenant_id":null}`, `{"tenant_id":"7"}`, `{"tenant_id":1.5}`, `{"tenant_id":7,"extra":true}`, `{"tenant_id":7,"tenant_id":8}`, `{"Tenant_id":7}`, `{"tenant_id":7}null`} {
		if _, err := decodeTenantSwitch(strings.NewReader(body)); !errors.Is(err, errInvalidExtendedInput) {
			t.Fatal("invalid tenant switch accepted")
		}
	}
}

func TestTelemetryInputRejectsInvalidPropertiesBeforeTheRepository(t *testing.T) {
	for _, properties := range []string{`null`, `[]`, `{}`, `{"temperature":1,"temperature":2}`} {
		// Given: a timestamp with an invalid object of telemetry properties.
		body := fmt.Sprintf(`{"ts":"2026-10-14T00:00:00Z","properties":%s}`, properties)
		// When: parsing the telemetry boundary.
		_, err := decodeTelemetryV2Request(strings.NewReader(body))
		// Then: no ambiguous or empty properties can reach a repository call.
		if !errors.Is(err, errInvalidExtendedInput) {
			t.Fatal("invalid telemetry properties accepted")
		}
	}
}
