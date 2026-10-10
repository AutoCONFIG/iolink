package domain

import (
	"errors"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

var ErrInvalidVideoSource = errors.New("invalid video source")

type VideoSourceKind string

const (
	VideoRTSP    VideoSourceKind = "rtsp"
	VideoGB28181 VideoSourceKind = "gb28181"
)

type VideoSource interface {
	Kind() VideoSourceKind
	videoSource()
}

// RTSPVideoSource is syntax-checked only. The provider must still enforce its
// explicit allowlist, all DNS addresses and the pinned origin on each attempt.
type RTSPVideoSource struct{ uri string }

func NewRTSPVideoSource(raw string) (RTSPVideoSource, error) {
	if len(raw) < 8 || len(raw) > 2048 || strings.ContainsAny(raw, "?#\\") || hasVideoControl(raw) {
		return RTSPVideoSource{}, ErrInvalidVideoSource
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "rtsp" || u.Opaque != "" || u.User != nil || u.Hostname() == "" {
		return RTSPVideoSource{}, ErrInvalidVideoSource
	}
	if strings.ContainsAny(u.Hostname(), "% ") || strings.HasSuffix(u.Host, ":") {
		return RTSPVideoSource{}, ErrInvalidVideoSource
	}
	if !strings.HasPrefix(u.Host, "[") && strings.Contains(u.Hostname(), ":") {
		return RTSPVideoSource{}, ErrInvalidVideoSource
	}
	if strings.HasPrefix(u.Host, "[") {
		address, err := netip.ParseAddr(u.Hostname())
		if err != nil || !address.Is6() {
			return RTSPVideoSource{}, ErrInvalidVideoSource
		}
	}
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 {
			return RTSPVideoSource{}, ErrInvalidVideoSource
		}
	}
	if hasVideoControl(u.Path) || strings.ContainsAny(u.Path, "\\%") {
		return RTSPVideoSource{}, ErrInvalidVideoSource
	}
	for segment := range strings.SplitSeq(u.Path, "/") {
		if segment == ".." {
			return RTSPVideoSource{}, ErrInvalidVideoSource
		}
	}
	return RTSPVideoSource{uri: raw}, nil
}

func hasVideoControl(raw string) bool { return strings.ContainsFunc(raw, unicode.IsControl) }

func (RTSPVideoSource) Kind() VideoSourceKind        { return VideoRTSP }
func (RTSPVideoSource) videoSource()                 {}
func (s RTSPVideoSource) URI() string                { return s.uri }
func (RTSPVideoSource) String() string               { return "rtsp source [redacted]" }
func (RTSPVideoSource) GoString() string             { return "rtsp source [redacted]" }
func (RTSPVideoSource) MarshalJSON() ([]byte, error) { return nil, ErrVideoSecretSerialization }

type GBVideoSource struct {
	deviceID  int64
	channelID string
}

func NewGBVideoSource(deviceID int64, channelID string) (GBVideoSource, error) {
	if deviceID <= 0 || len(channelID) != 20 || strings.ContainsFunc(channelID, func(r rune) bool { return r < '0' || r > '9' }) {
		return GBVideoSource{}, ErrInvalidVideoSource
	}
	return GBVideoSource{deviceID, channelID}, nil
}

func (GBVideoSource) Kind() VideoSourceKind { return VideoGB28181 }
func (GBVideoSource) videoSource()          {}
func (s GBVideoSource) DeviceID() int64     { return s.deviceID }
func (s GBVideoSource) ChannelID() string   { return s.channelID }
