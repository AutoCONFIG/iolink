package domain_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

func TestVideoRTSPSourceRejectsUnsafeSyntax(t *testing.T) {
	for i, raw := range []string{
		"", "https://camera/live", "rtsp:///live", "rtsp://user:secret@camera/live", "rtsp://camera/live?x=1",
		"rtsp://camera/live#x", "rtsp://camera/live?", "rtsp://camera/live#", "rtsp://camera:0/live", "rtsp://camera:65536/live",
		"rtsp://camera:/live", "rtsp://camera/../live", "rtsp://camera/%2e%2e/live", "rtsp://camera/%5clive", "rtsp://camera/%0alive",
		"rtsp://camera/%252e%252e/live", "rtsp://camera/live\n", "rtsp://camera/" + strings.Repeat("x", 2048),
		"rtsp://[camera]/live", "rtsp://[192.168.10.20]/live", "rtsp://2001:db8::1/live", "rtsp://::1/live",
		"rtsp://[fd00::1%25eth0]/live", "rtsp://[fd00::1]extra/live", "rtsp://[fd00::1]:/live",
	} {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			_, err := domain.NewRTSPVideoSource(raw)
			if !errors.Is(err, domain.ErrInvalidVideoSource) {
				t.Fatalf("unsafe syntax accepted: %v", err)
			}
		})
	}
}

func TestVideoRTSPSourcePreservesValidEncodedPath(t *testing.T) {
	for _, raw := range []string{"rtsp://camera:554/live", "rtsp://192.168.10.20/live", "rtsp://[fd00::1]:554/live", "rtsp://[fd00::1]/live", "rtsp://camera/live%20stream"} {
		source, err := domain.NewRTSPVideoSource(raw)
		if err != nil || source.URI() != raw || source.Kind() != domain.VideoRTSP {
			t.Fatalf("valid source rejected: %v", err)
		}
	}
}

func TestVideoRTSPSourceDoesNotSerializeOrFormatURI(t *testing.T) {
	source, err := domain.NewRTSPVideoSource("rtsp://private-camera/live")
	if err != nil {
		t.Fatal(err)
	}
	_, err = json.Marshal(source)
	if !errors.Is(err, domain.ErrVideoSecretSerialization) {
		t.Fatalf("source serialized: %v", err)
	}
	for _, format := range []string{"%s", "%v", "%+v", "%#v"} {
		if strings.Contains(fmt.Sprintf(format, source), "private-camera") {
			t.Fatal("source leaked through formatting")
		}
	}
}

func TestVideoGBSourceRejectsInvalidIdentity(t *testing.T) {
	for _, item := range []struct {
		device  int64
		channel string
	}{
		{0, "34020000001320000011"}, {1, "3402000000132000001"}, {1, "3402000000132000001x"}, {1, "３4020000001320000011"},
	} {
		_, err := domain.NewGBVideoSource(item.device, item.channel)
		if !errors.Is(err, domain.ErrInvalidVideoSource) {
			t.Fatalf("invalid GB identity accepted: %v", err)
		}
	}
}

func TestVideoGBSourceKeepsDeviceAndChannel(t *testing.T) {
	source, err := domain.NewGBVideoSource(42, "34020000001320000011")
	if err != nil || source.Kind() != domain.VideoGB28181 || source.DeviceID() != 42 || source.ChannelID() != "34020000001320000011" {
		t.Fatalf("valid GB source rejected: %v", err)
	}
}

func TestVideoPlaybackWindowEnforcesFiveMinuteMaximum(t *testing.T) {
	created := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	for _, duration := range []time.Duration{-time.Second, 0, 300*time.Second + time.Nanosecond} {
		_, err := domain.NewVideoPlaybackWindow(created, created.Add(duration))
		if !errors.Is(err, domain.ErrInvalidVideoPlayback) {
			t.Fatalf("invalid TTL accepted: %v", err)
		}
	}
	_, err := domain.NewVideoPlaybackWindow(time.Time{}, created)
	if !errors.Is(err, domain.ErrInvalidVideoPlayback) {
		t.Fatal("zero creation time accepted")
	}
}

func TestVideoPlaybackActiveAtChecksExactBoundariesAndTerminalStates(t *testing.T) {
	created := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	window, err := domain.NewVideoPlaybackWindow(created, created.Add(300*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		state  domain.VideoSessionState
		at     time.Time
		active bool
	}{
		{domain.VideoSessionPending, created, true},
		{domain.VideoSessionReady, created.Add(300*time.Second - time.Nanosecond), true},
		{domain.VideoSessionReady, created.Add(300 * time.Second), false},
		{domain.VideoSessionPending, created.Add(-time.Nanosecond), false},
		{domain.VideoSessionFailed, created, false},
		{domain.VideoSessionRevoked, created, false},
		{domain.VideoSessionExpired, created, false},
		{"invalid", created, false},
	} {
		if window.ActiveAt(item.state, item.at) != item.active {
			t.Fatalf("state %s boundary mismatch", item.state)
		}
	}
}

func TestVideoPlaybackZeroWindowIsInactive(t *testing.T) {
	window := domain.VideoPlaybackWindow{}
	for _, state := range []domain.VideoSessionState{domain.VideoSessionPending, domain.VideoSessionReady} {
		if window.ActiveAt(state, time.Time{}) {
			t.Fatal("zero playback window is active")
		}
	}
}

func TestVideoCodecsRejectImplicitTranscoding(t *testing.T) {
	for _, item := range []struct {
		video, audio string
		playable     bool
	}{
		{"H264", "AAC", true}, {"H264", "", true}, {"H265", "AAC", false}, {"H264", "G711", false}, {"", "AAC", false},
	} {
		if domain.CompatibleVideoTracks(item.video, item.audio) != item.playable {
			t.Fatalf("unexpected codec compatibility %s/%s", item.video, item.audio)
		}
	}
}
