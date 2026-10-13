package adminapi

import (
	"bytes"
	"encoding/json"
	"slices"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/gin-gonic/gin"
)

func bindProductModelJSON(c *gin.Context, target *productModelRequest) error {
	var input struct {
		Fields []json.RawMessage `json:"fields"`
	}
	if err := bindBaselineJSON(c, &input, "fields"); err != nil {
		return err
	}
	if len(input.Fields) == 0 || len(input.Fields) > 256 {
		return domain.ErrInvalidProductModel
	}
	target.Fields = make([]domain.ModelField, 0, len(input.Fields))
	for _, raw := range input.Fields {
		field, err := parseProductField(raw)
		if err != nil {
			return err
		}
		target.Fields = append(target.Fields, field)
	}
	return nil
}

func parseProductField(raw json.RawMessage) (domain.ModelField, error) {
	var field domain.ModelField
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return field, domain.ErrInvalidProductModel
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if _, err := decoder.Token(); err != nil {
		return field, domain.ErrInvalidProductModel
	}
	required := []string{"identifier", "type", "unit", "readable", "writable", "nullable"}
	optional := []string{"minimum", "maximum", "enum_values"}
	seen := make(map[string]bool)
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return field, domain.ErrInvalidProductModel
		}
		name, ok := key.(string)
		if !ok || seen[name] || (!slices.Contains(required, name) && !slices.Contains(optional, name)) {
			return field, domain.ErrInvalidProductModel
		}
		seen[name] = true
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return field, domain.ErrInvalidProductModel
		}
		if slices.Contains(required, name) && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return field, domain.ErrInvalidProductModel
		}
	}
	for _, key := range required {
		if !seen[key] {
			return field, domain.ErrInvalidProductModel
		}
	}
	if err := json.Unmarshal(raw, &field); err != nil {
		return field, domain.ErrInvalidProductModel
	}
	return field, nil
}
