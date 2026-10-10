package adminapi

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"slices"
	"unicode/utf8"

	"git.hyhy.fun/rsplab/iolink/internal/camera"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/gin-gonic/gin"
)

type cameraRequest struct {
	Name   string          `json:"name"`
	PondID int64           `json:"pond_id"`
	Source json.RawMessage `json:"source"`
}

func strictCameraObject(raw []byte, target any, allowed ...string) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' || !utf8.Valid(raw) {
		return camera.ErrInvalid
	}
	fields := json.NewDecoder(bytes.NewReader(raw))
	if _, err := fields.Token(); err != nil {
		return camera.ErrInvalid
	}
	seen := make(map[string]bool)
	for fields.More() {
		token, err := fields.Token()
		if err != nil {
			return camera.ErrInvalid
		}
		key, ok := token.(string)
		if !ok || seen[key] || !slices.Contains(allowed, key) {
			return camera.ErrInvalid
		}
		seen[key] = true
		var value json.RawMessage
		if err := fields.Decode(&value); err != nil {
			return camera.ErrInvalid
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return camera.ErrInvalid
	}
	if decoder.Decode(new(any)) != io.EOF {
		return camera.ErrInvalid
	}
	return nil
}

func parseCameraBody(c *gin.Context) (camera.Configuration, error) {
	media, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || media != "application/json" {
		return camera.Configuration{}, camera.ErrInvalid
	}
	body := http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	raw, err := io.ReadAll(body)
	if err != nil {
		return camera.Configuration{}, camera.ErrInvalid
	}
	var request cameraRequest
	if err = strictCameraObject(raw, &request, "name", "pond_id", "source"); err != nil {
		return camera.Configuration{}, err
	}
	var kind struct {
		Kind domain.VideoSourceKind `json:"kind"`
	}
	if err = json.Unmarshal(request.Source, &kind); err != nil {
		return camera.Configuration{}, camera.ErrInvalid
	}
	input := camera.SourceInput{Kind: kind.Kind}
	switch kind.Kind {
	case domain.VideoRTSP:
		var source struct {
			Kind        domain.VideoSourceKind `json:"kind"`
			URI         string                 `json:"uri"`
			Credentials json.RawMessage        `json:"credentials"`
		}
		if err = strictCameraObject(request.Source, &source, "kind", "uri", "credentials"); err != nil {
			return camera.Configuration{}, err
		}
		input.URI = source.URI
		if len(source.Credentials) > 0 {
			var credentials struct {
				Username string `json:"username"`
				Password string `json:"password"`
			}
			if err = strictCameraObject(source.Credentials, &credentials, "username", "password"); err != nil {
				return camera.Configuration{}, err
			}
			input.Credentials = &camera.Credentials{Username: credentials.Username, Password: credentials.Password}
		}
	case domain.VideoGB28181:
		var source struct {
			Kind      domain.VideoSourceKind `json:"kind"`
			DeviceID  int64                  `json:"device_id"`
			ChannelID string                 `json:"channel_id"`
		}
		if err = strictCameraObject(request.Source, &source, "kind", "device_id", "channel_id"); err != nil {
			return camera.Configuration{}, err
		}
		input.DeviceID, input.ChannelID = source.DeviceID, source.ChannelID
	default:
		return camera.Configuration{}, camera.ErrInvalid
	}
	return camera.ParseConfiguration(request.PondID, request.Name, input)
}
