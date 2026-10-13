package adminapi

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBaselineJSONRejectsAmbiguousObjects(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"valid", `{"name":"farm","location":"local"}`, true},
		{"owner_omitted", `{"name":"farm"}`, true},
		{"owner_null", `{"name":"farm","owner_id":null}`, true},
		{"owner_positive", `{"name":"farm","owner_id":1}`, true},
		{"unknown", `{"name":"farm","extra":true}`, false},
		{"duplicate", `{"name":"farm","name":"other"}`, false},
		{"wrong_case", `{"Name":"farm"}`, false},
		{"trailing_object", `{"name":"farm"} {}`, false},
		{"trailing_invalid", `{"name":"farm"} broken`, false},
		{"array", `[]`, false},
		{"null_object", `null`, false},
		{"null_name", `{"name":null}`, false},
		{"null_location", `{"name":"farm","location":null}`, false},
		{"missing_name", `{}`, false},
		{"owner_zero", `{"name":"farm","owner_id":0}`, false},
		{"owner_string", `{"name":"farm","owner_id":"1"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Given: the actual baseline parser and typed farm DTO.
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/farms", strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")
			var request farmCreateReq
			// When: parse the wire object at the HTTP boundary.
			err := bindBaselineJSON(c, &request, "name", "location", "owner_id")
			// Then: ambiguous or out-of-contract inputs are rejected.
			if (err == nil) != tc.valid {
				t.Errorf("parse valid=%t want=%t", err == nil, tc.valid)
			}
		})
	}
}

func TestBaselineOwnerReplacementDistinguishesOmittedAndNull(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"omitted", `{}`, false}, {"null", `{"owner_id":null}`, true}, {"positive", `{"owner_id":1}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("PUT", "/farms/1/owner", strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")
			var request ownerReq
			err := bindBaselineJSON(c, &request, "owner_id")
			if (err == nil) != tc.valid {
				t.Errorf("parse valid=%t want=%t", err == nil, tc.valid)
			}
			if err == nil {
				var id *int64
				if err := json.Unmarshal(request.OwnerID, &id); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestBaselineRuleEnabledHonorsFalseAndDefault(t *testing.T) {
	for _, tc := range []struct {
		body    string
		enabled bool
	}{
		{`{"pond_id":1,"metric":"temperature","max_value":25,"level":"warning"}`, true},
		{`{"pond_id":1,"metric":"temperature","max_value":25,"level":"warning","enabled":false}`, false},
	} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("POST", "/alarm-rules", strings.NewReader(tc.body))
		c.Request.Header.Set("Content-Type", "application/json")
		var request ruleReq
		if err := bindBaselineJSON(c, &request, "pond_id", "metric", "min_value", "max_value", "level", "enabled"); err != nil {
			t.Fatal(err)
		}
		if request.toDomain().Enabled != tc.enabled {
			t.Error("rule enabled differs from request/default")
		}
	}
}
