package adminapi

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

const productInputField = `{"identifier":"temperature","type":"number","unit":"C","readable":false,"writable":false,"nullable":false,"minimum":null,"maximum":null,"enum_values":null}`

func TestProductFieldInputRejectsAmbiguousOrIncompleteFields(t *testing.T) {
	// Given: malformed shapes, missing required keys and ambiguous nested field JSON.
	for name, raw := range map[string]string{
		"null": "null", "array": "[]", "empty": "{}",
		"missing_required": `{"identifier":"temperature","type":"number"}`,
		"unknown":          strings.Replace(productInputField, `"unit"`, `"unknown":1,"unit"`, 1),
		"case":             strings.Replace(productInputField, `"readable"`, `"READABLE"`, 1),
		"duplicate":        strings.Replace(productInputField, `"readable"`, `"readable":true,"readable"`, 1),
		"null_boolean":     strings.Replace(productInputField, `"readable":false`, `"readable":null`, 1),
		"null_unit":        strings.Replace(productInputField, `"unit":"C"`, `"unit":null`, 1),
		"wrong_type":       strings.Replace(productInputField, `"writable":false`, `"writable":"false"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			// When: the product field boundary parses the actual JSON object.
			_, err := parseProductField(json.RawMessage(raw))
			// Then: invalid shape is rejected without a partially usable field.
			if !errors.Is(err, domain.ErrInvalidProductModel) {
				t.Fatal("ambiguous field was accepted")
			}
		})
	}
}

func TestProductFieldInputPreservesExplicitFalseAndNullableBounds(t *testing.T) {
	// Given: all required properties are present, with false flags and nullable optional constraints.
	// When: the existing domain DTO is parsed through the strict input boundary.
	field, err := parseProductField(json.RawMessage(productInputField))
	// Then: false is not confused with omission; nullable bounds retain the domain representation.
	if err != nil || field.Readable || field.Writable || field.Nullable || field.Min != nil || field.Max != nil || field.Enum != nil {
		t.Fatal("valid model field changed meaning during parsing")
	}
}
