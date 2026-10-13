package appapi

import (
	"strings"
	"testing"
)

func TestLoginInputAcceptsOneExactCodeField(t *testing.T) {
	for _, body := range []string{` {"code":"valid-code"} `, `{"\u0063ode":"valid-code"}`} {
		// Given: a valid JSON object, including equivalent escaped field spelling.
		reader := strings.NewReader(body)
		// When: parsing the login request boundary.
		request, err := decodeLoginRequest(reader)
		// Then: the code survives as a typed string without treating JSON escapes as extra fields.
		if err != nil || request.Code != "valid-code" {
			t.Fatal("valid login code rejected or changed")
		}
	}
}
