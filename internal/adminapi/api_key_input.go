package adminapi

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"slices"
	"unicode/utf8"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

func apiKeyObject(raw []byte, allowed ...string) (map[string]json.RawMessage, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' || !utf8.Valid(raw) {
		return nil, domain.ErrInvalidAPIKey
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if _, err := decoder.Token(); err != nil {
		return nil, domain.ErrInvalidAPIKey
	}
	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, domain.ErrInvalidAPIKey
		}
		name, ok := token.(string)
		if !ok || !slices.Contains(allowed, name) {
			return nil, domain.ErrInvalidAPIKey
		}
		if _, exists := fields[name]; exists {
			return nil, domain.ErrInvalidAPIKey
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, domain.ErrInvalidAPIKey
		}
		fields[name] = value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, domain.ErrInvalidAPIKey
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, domain.ErrInvalidAPIKey
	}
	return fields, nil
}

func parseAPIKeyBody(c *gin.Context) (apiKeyCreateRequest, error) {
	media, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || media != "application/json" {
		return apiKeyCreateRequest{}, domain.ErrInvalidAPIKey
	}
	raw, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
	if err != nil {
		return apiKeyCreateRequest{}, domain.ErrInvalidAPIKey
	}
	fields, err := apiKeyObject(raw, "name", "scopes", "resources")
	if err != nil {
		return apiKeyCreateRequest{}, err
	}
	if resources, exists := fields["resources"]; exists {
		if _, err := apiKeyObject(resources, "farm_ids", "pond_ids", "device_nos"); err != nil {
			return apiKeyCreateRequest{}, err
		}
	}
	var request apiKeyCreateRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		return apiKeyCreateRequest{}, domain.ErrInvalidAPIKey
	}
	if err := binding.Validator.ValidateStruct(&request); err != nil {
		return apiKeyCreateRequest{}, domain.ErrInvalidAPIKey
	}
	return request, nil
}
