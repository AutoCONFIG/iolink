package license

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func ParseEnvelope(raw []byte) (Envelope, error) {
	if len(raw) == 0 || len(raw) > MaxEnvelopeBytes || !utf8.Valid(raw) || !json.Valid(raw) || hasDuplicateJSONKeys(raw) {
		return Envelope{}, ErrInvalidEnvelope
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || len(fields) != 2 || fields["payload_b64"] == nil || fields["signature_b64"] == nil {
		return Envelope{}, ErrInvalidEnvelope
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var envelope Envelope
	if err := decoder.Decode(&envelope); err != nil {
		return Envelope{}, fmt.Errorf("%w: envelope encoding", ErrInvalidEnvelope)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Envelope{}, fmt.Errorf("%w: trailing envelope", ErrInvalidEnvelope)
	}
	if strings.TrimSpace(envelope.PayloadB64) == "" || strings.TrimSpace(envelope.SignatureB64) == "" || len(envelope.PayloadB64) > MaxPayloadB64Chars || len(envelope.SignatureB64) > MaxSignatureB64Chars {
		return Envelope{}, ErrInvalidEnvelope
	}
	return envelope, nil
}

func ParsePayload(raw []byte) (Payload, error) {
	if len(raw) == 0 || len(raw) > MaxPayloadBytes || !utf8.Valid(raw) || !json.Valid(raw) || hasDuplicateJSONKeys(raw) {
		return Payload{}, fmt.Errorf("%w: payload encoding", ErrInvalidPayload)
	}
	var payload Payload
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return Payload{}, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Payload{}, fmt.Errorf("%w: trailing payload", ErrInvalidPayload)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Payload{}, fmt.Errorf("%w: payload object", ErrInvalidPayload)
	}
	if len(fields) != 8 {
		return Payload{}, ErrInvalidPayload
	}
	for _, name := range []string{"license_id", "deployment_id", "issued_at", "not_before", "expires_at", "max_devices", "features", "key_id"} {
		if _, ok := fields[name]; !ok {
			return Payload{}, fmt.Errorf("%w: required field missing", ErrInvalidPayload)
		}
	}
	if string(fields["features"]) == "null" {
		return Payload{}, fmt.Errorf("%w: features must be an array", ErrInvalidPayload)
	}
	if _, err := strconv.ParseInt(string(fields["max_devices"]), 10, 64); err != nil {
		return Payload{}, ErrInvalidPayload
	}
	for _, name := range []string{"issued_at", "not_before"} {
		if !strictUTCTimestamp(fields[name]) {
			return Payload{}, fmt.Errorf("%w: timestamps must use UTC Z", ErrInvalidPayload)
		}
	}
	if string(fields["expires_at"]) != "null" && !strictUTCTimestamp(fields["expires_at"]) {
		return Payload{}, fmt.Errorf("%w: timestamps must use UTC Z", ErrInvalidPayload)
	}
	if !isUTC(payload.IssuedAt) || !isUTC(payload.NotBefore) || (payload.ExpiresAt != nil && !isUTC(*payload.ExpiresAt)) {
		return Payload{}, fmt.Errorf("%w: timestamps must be UTC", ErrInvalidPayload)
	}
	if err := ValidatePayload(payload); err != nil {
		return Payload{}, err
	}
	return payload, nil
}

func strictUTCTimestamp(raw json.RawMessage) bool {
	var value string
	if json.Unmarshal(raw, &value) != nil || !strings.HasSuffix(value, "Z") {
		return false
	}
	_, err := time.Parse(time.RFC3339Nano, value)
	return err == nil
}

func isUTC(value time.Time) bool { return value.Location() == time.UTC }

func ValidatePayload(payload Payload) error {
	if !validIdentifier(payload.LicenseID) || !validIdentifier(payload.DeploymentID) || !validIdentifier(payload.KeyID) {
		return fmt.Errorf("%w: required identifier missing", ErrInvalidPayload)
	}
	if payload.IssuedAt.IsZero() || payload.NotBefore.IsZero() || payload.MaxDevices < 0 || payload.IssuedAt.After(payload.NotBefore) {
		return fmt.Errorf("%w: invalid time or device limit", ErrInvalidPayload)
	}
	if payload.ExpiresAt != nil && !payload.ExpiresAt.After(payload.NotBefore) {
		return fmt.Errorf("%w: expiry must be after not_before", ErrInvalidPayload)
	}
	seen := make(map[string]struct{}, len(payload.Features))
	for _, feature := range payload.Features {
		if _, ok := knownFeatures[feature]; !ok {
			return fmt.Errorf("%w: unknown feature", ErrInvalidPayload)
		}
		if _, ok := seen[feature]; ok {
			return fmt.Errorf("%w: duplicate feature", ErrInvalidPayload)
		}
		seen[feature] = struct{}{}
	}
	return nil
}

func validIdentifier(value string) bool {
	return len(value) > 0 && len(value) <= 128 && strings.TrimSpace(value) == value
}

func hasDuplicateJSONKeys(raw []byte) bool {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	var walk func() bool
	walk = func() bool {
		token, err := decoder.Token()
		if err != nil {
			return false
		}
		switch delimiter := token.(type) {
		case json.Delim:
			switch delimiter {
			case '{':
				seen := make(map[string]struct{})
				for decoder.More() {
					keyToken, err := decoder.Token()
					if err != nil {
						return false
					}
					key := keyToken.(string)
					if _, exists := seen[key]; exists {
						return true
					}
					seen[key] = struct{}{}
					if walk() {
						return true
					}
				}
				_, _ = decoder.Token()
			case '[':
				for decoder.More() {
					if walk() {
						return true
					}
				}
				_, _ = decoder.Token()
			}
		}
		return false
	}
	return walk()
}
