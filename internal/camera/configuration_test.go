package camera_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"git.hyhy.fun/rsplab/iolink/internal/camera"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

func TestConfiguration_whenSourceSyntaxInvalid(t *testing.T) {
	for _, uri := range []string{"http://192.168.10.20/live", "rtsp://user:password@host/live", "rtsp://host/live?token=a", "rtsp://host/live#fragment", "rtsp://host/../live", "rtsp://host/%0alive", "rtsp://host:99999/live", "rtsp://host/live\n", "rtsp://[bad]/live"} {
		t.Run(uri, func(t *testing.T) {
			// Given
			input := camera.SourceInput{Kind: domain.VideoRTSP, URI: uri, Credentials: &camera.Credentials{Username: "login", Password: "secret"}}
			// When
			_, err := camera.ParseConfiguration(1, "camera", input)
			// Then
			if !errors.Is(err, camera.ErrInvalid) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestConfiguration_whenFieldsInvalid(t *testing.T) {
	for _, item := range []struct {
		name   string
		pond   int64
		source camera.SourceInput
	}{
		{"", 1, camera.SourceInput{Kind: domain.VideoRTSP, URI: "rtsp://host/live"}},
		{strings.Repeat("界", 129), 1, camera.SourceInput{Kind: domain.VideoRTSP, URI: "rtsp://host/live"}},
		{"camera", 0, camera.SourceInput{Kind: domain.VideoRTSP, URI: "rtsp://host/live"}},
		{"camera", 1, camera.SourceInput{Kind: domain.VideoRTSP, URI: "rtsp://host/live", DeviceID: 1}},
		{"camera", 1, camera.SourceInput{Kind: domain.VideoRTSP, URI: "rtsp://host/live", Credentials: &camera.Credentials{Username: "a", Password: ""}}},
		{"camera", 1, camera.SourceInput{Kind: domain.VideoGB28181, DeviceID: 1, ChannelID: "invalid"}},
		{"camera", 1, camera.SourceInput{Kind: domain.VideoGB28181, DeviceID: 1, ChannelID: "34020000001310000101", URI: "rtsp://host/live"}},
	} {
		// When
		_, err := camera.ParseConfiguration(item.pond, item.name, item.source)
		// Then
		if !errors.Is(err, camera.ErrInvalid) {
			t.Fatalf("error=%v", err)
		}
	}
}

func TestConfiguration_whenFormattedOrSerialized(t *testing.T) {
	// Given
	cfg, err := camera.ParseConfiguration(1, "camera", camera.SourceInput{Kind: domain.VideoRTSP, URI: "rtsp://host/live", Credentials: &camera.Credentials{Username: "hiddenlogin", Password: "hiddensecret"}})
	if err != nil {
		t.Fatal(err)
	}
	// When
	text := fmt.Sprintf("%+v %#v", cfg, cfg)
	_, err = json.Marshal(cfg)
	// Then
	if strings.Contains(text, "hidden") || strings.Contains(text, "rtsp://") || !errors.Is(err, domain.ErrVideoSecretSerialization) {
		t.Fatal("configuration secrets exposed")
	}
}
