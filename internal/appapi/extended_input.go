package appapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"time"
)

var errInvalidExtendedInput = errors.New("invalid extended request")

func decodeObjectFields(body io.Reader, allowed []string) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(body)
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, errInvalidExtendedInput
	}
	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err := decoder.Token()
		key, valid := token.(string)
		_, duplicate := fields[key]
		if err != nil || !valid || duplicate || (allowed != nil && !slices.Contains(allowed, key)) {
			return nil, errInvalidExtendedInput
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, errInvalidExtendedInput
		}
		fields[key] = value
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		return nil, errInvalidExtendedInput
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, errInvalidExtendedInput
	}
	return fields, nil
}

func decodeTenantSwitch(body io.Reader) (tenantSwitchRequest, error) {
	fields, err := decodeObjectFields(body, []string{"tenant_id"})
	if err != nil {
		return tenantSwitchRequest{}, err
	}
	var tenantID *int64
	if err := json.Unmarshal(fields["tenant_id"], &tenantID); err != nil || tenantID == nil || *tenantID < 1 {
		return tenantSwitchRequest{}, errInvalidExtendedInput
	}
	return tenantSwitchRequest{TenantID: *tenantID}, nil
}

func decodeTelemetryV2Request(body io.Reader) (telemetryV2Request, error) {
	fields, err := decodeObjectFields(body, []string{"ts", "properties"})
	if err != nil {
		return telemetryV2Request{}, err
	}
	var timestamp *time.Time
	if err := json.Unmarshal(fields["ts"], &timestamp); err != nil || timestamp == nil || timestamp.IsZero() {
		return telemetryV2Request{}, errInvalidExtendedInput
	}
	properties, err := decodeObjectFields(bytes.NewReader(fields["properties"]), nil)
	if err != nil || len(properties) == 0 || len(properties) > 128 {
		return telemetryV2Request{}, errInvalidExtendedInput
	}
	return telemetryV2Request{Timestamp: timestamp.UTC(), Properties: properties}, nil
}
