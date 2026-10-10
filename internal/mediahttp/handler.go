package mediahttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"git.hyhy.fun/rsplab/iolink/internal/playback"
	"github.com/google/uuid"
)

type Handler struct {
	playback *playback.Service
}

func New(service *playback.Service) http.Handler {
	return &Handler{playback: service}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "invalid_request", "method not allowed")
		return
	}
	if h.playback == nil || r.URL.RawQuery != "" || r.URL.RawPath != "" || r.URL.Fragment != "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid media path")
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/media/v1/") {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid media path")
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if len(parts) < 5 || parts[0] != "media" || parts[1] != "v1" || !validToken(parts[2]) || !validUUID(parts[3]) {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid media path")
		return
	}
	token, sessionID := parts[2], parts[3]
	ctx := r.Context()
	if len(parts) == 5 && parts[4] == "index.m3u8" {
		body, err := h.playback.ReadManifest(ctx, token, sessionID)
		if err != nil {
			writeMediaError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
		return
	}
	if len(parts) == 6 && parts[4] == "segments" {
		if !validUUID(parts[5]) {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid media path")
			return
		}
		body, err := h.playback.ReadSegment(ctx, token, sessionID, parts[5])
		if err != nil {
			writeMediaError(w, err)
			return
		}
		w.Header().Set("Content-Type", "video/mp2t")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "media not found")
}

func validToken(token string) bool {
	if len(token) < 16 || len(token) > 2048 {
		return false
	}
	for _, r := range token {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '_' && r != '.' && r != '-' {
			return false
		}
	}
	return true
}

func validUUID(raw string) bool {
	_, err := uuid.Parse(raw)
	return err == nil
}

func writeMediaError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, playback.ErrInvalid):
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid media request")
	case errors.Is(err, playback.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, "unauthorized", "media authorization failed")
	case errors.Is(err, playback.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "media access forbidden")
	case errors.Is(err, playback.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "media not found")
	case errors.Is(err, playback.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "media conflict")
	case errors.Is(err, playback.ErrPending):
		w.Header().Set("Retry-After", "1")
		writeError(w, http.StatusTooEarly, "media_pending", "media pending")
	case errors.Is(err, playback.ErrBadMedia), errors.Is(err, playback.ErrUnsupported):
		writeError(w, http.StatusBadGateway, "bad_media", "media source rejected")
	case errors.Is(err, playback.ErrUnavailable):
		writeError(w, http.StatusServiceUnavailable, "unavailable", "media unavailable")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "media request failed")
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{Code: code, Message: message})
}

var _ http.Handler = (*Handler)(nil)
