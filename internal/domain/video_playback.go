package domain

import (
	"errors"
	"time"
)

const MaxVideoPlaybackTTL = 300 * time.Second

var ErrInvalidVideoPlayback = errors.New("invalid video playback")

type VideoSessionState string

const (
	VideoSessionPending VideoSessionState = "pending"
	VideoSessionReady   VideoSessionState = "ready"
	VideoSessionFailed  VideoSessionState = "failed"
	VideoSessionRevoked VideoSessionState = "revoked"
	VideoSessionExpired VideoSessionState = "expired"
)

type VideoPlaybackWindow struct{ createdAt, expiresAt time.Time }

func NewVideoPlaybackWindow(createdAt, expiresAt time.Time) (VideoPlaybackWindow, error) {
	if createdAt.IsZero() || !expiresAt.After(createdAt) || expiresAt.Sub(createdAt) > MaxVideoPlaybackTTL {
		return VideoPlaybackWindow{}, ErrInvalidVideoPlayback
	}
	return VideoPlaybackWindow{createdAt, expiresAt}, nil
}

func (w VideoPlaybackWindow) CreatedAt() time.Time { return w.createdAt }
func (w VideoPlaybackWindow) ExpiresAt() time.Time { return w.expiresAt }

func (w VideoPlaybackWindow) ActiveAt(state VideoSessionState, now time.Time) bool {
	switch state {
	case VideoSessionPending, VideoSessionReady:
		return !now.Before(w.createdAt) && now.Before(w.expiresAt)
	case VideoSessionFailed, VideoSessionRevoked, VideoSessionExpired:
		return false
	default:
		return false
	}
}

func CompatibleVideoTracks(videoCodec, audioCodec string) bool {
	return videoCodec == "H264" && (audioCodec == "" || audioCodec == "AAC")
}
