package adminapi

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"time"
	"unicode/utf8"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

type membershipRequest struct {
	Role      string     `json:"role" binding:"required,oneof=owner admin member viewer support"`
	Active    *bool      `json:"active" binding:"required"`
	ExpiresAt *time.Time `json:"expires_at"`
}

func parseMembershipBody(c *gin.Context) (membershipRequest, error) {
	media, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || media != "application/json" {
		return membershipRequest{}, domain.ErrInvalidRange
	}
	raw, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 4096))
	raw = bytes.TrimSpace(raw)
	if err != nil || len(raw) == 0 || raw[0] != '{' || !utf8.Valid(raw) {
		return membershipRequest{}, domain.ErrInvalidRange
	}
	fields := json.NewDecoder(bytes.NewReader(raw))
	if _, err := fields.Token(); err != nil {
		return membershipRequest{}, domain.ErrInvalidRange
	}
	seen := make(map[string]bool)
	for fields.More() {
		token, err := fields.Token()
		if err != nil {
			return membershipRequest{}, domain.ErrInvalidRange
		}
		name, ok := token.(string)
		if !ok || seen[name] {
			return membershipRequest{}, domain.ErrInvalidRange
		}
		switch name {
		case "role", "active", "expires_at":
		default:
			return membershipRequest{}, domain.ErrInvalidRange
		}
		seen[name] = true
		var value json.RawMessage
		if err := fields.Decode(&value); err != nil {
			return membershipRequest{}, domain.ErrInvalidRange
		}
	}
	var request membershipRequest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return membershipRequest{}, domain.ErrInvalidRange
	}
	if decoder.Decode(new(any)) != io.EOF {
		return membershipRequest{}, domain.ErrInvalidRange
	}
	if err := binding.Validator.ValidateStruct(&request); err != nil {
		return membershipRequest{}, domain.ErrInvalidRange
	}
	return request, nil
}
