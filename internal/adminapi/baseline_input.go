package adminapi

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"unicode/utf8"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

func validBaselineQuery(c *gin.Context) bool {
	if _, err := url.ParseQuery(c.Request.URL.RawQuery); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return false
	}
	return true
}

func bindBaselineJSON(c *gin.Context, target any, allowed ...string) error {
	media, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || media != "application/json" {
		return domain.ErrInvalidRange
	}
	raw, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
	raw = bytes.TrimSpace(raw)
	if err != nil || len(raw) == 0 || raw[0] != '{' || !utf8.Valid(raw) {
		return domain.ErrInvalidRange
	}
	fields := json.NewDecoder(bytes.NewReader(raw))
	if _, err := fields.Token(); err != nil {
		return domain.ErrInvalidRange
	}
	seen := make(map[string]bool)
	for fields.More() {
		key, err := fields.Token()
		if err != nil {
			return domain.ErrInvalidRange
		}
		name, ok := key.(string)
		if !ok || seen[name] || !slices.Contains(allowed, name) {
			return domain.ErrInvalidRange
		}
		seen[name] = true
		var value json.RawMessage
		if err := fields.Decode(&value); err != nil {
			return domain.ErrInvalidRange
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) && name != "owner_id" && name != "min_value" && name != "max_value" {
			return domain.ErrInvalidRange
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return domain.ErrInvalidRange
	}
	if decoder.Decode(new(any)) != io.EOF {
		return domain.ErrInvalidRange
	}
	if err := binding.Validator.ValidateStruct(target); err != nil {
		return domain.ErrInvalidRange
	}
	return nil
}
