package adminapi

import (
	"encoding/json"
	"testing"
)

func TestAPIKeyResourceInput_rejectsNullAndUnknownFields(t *testing.T) {
	for _, raw := range []string{`{"resources":null}`, `{"resources":{"farm_ids":null}}`, `{"resources":{"pond_ids":null}}`, `{"resources":{"device_nos":null}}`, `{"resources":{"farmIds":[1]}}`} {
		var input apiKeyCreateRequest
		if err := json.Unmarshal([]byte(raw), &input); err == nil {
			t.Fatalf("invalid resource value accepted: %s", raw)
		}
	}
	for _, raw := range []string{`{}`, `{"resources":{}}`, `{"resources":{"farm_ids":[],"pond_ids":[],"device_nos":[]}}`} {
		var input apiKeyCreateRequest
		if err := json.Unmarshal([]byte(raw), &input); err != nil {
			t.Fatal(err)
		}
	}
}
