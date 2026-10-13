package appapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/gin-gonic/gin"
)

type telemetryV2Request struct {
	Timestamp  time.Time                  `json:"ts"`
	Properties map[string]json.RawMessage `json:"properties"`
}

func (s *Server) modelLatestV2(c *gin.Context) {
	repo, ok := s.deps.Telemetry.(domain.DeviceModelLatestRepo)
	if !ok {
		s.providerUnavailable(c)
		return
	}
	result, err := repo.ModelLatestForUser(c.Request.Context(), c.Param("device_no"), uid(c))
	if errors.Is(err, domain.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	if err != nil {
		s.providerFailure(c, err)
		return
	}
	if result.Fields == nil {
		result.Fields = []domain.ModelField{}
	}
	c.JSON(http.StatusOK, result)
}

func (s *Server) submitTelemetryV2(c *gin.Context) {
	repo, ok := s.deps.Telemetry.(domain.GenericTelemetryRepo)
	if !ok {
		s.providerUnavailable(c)
		return
	}
	req, err := decodeTelemetryV2Request(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	result, err := repo.SubmitTelemetry(c.Request.Context(), c.Param("device_no"), uid(c), req.Timestamp, req.Properties)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		case errors.Is(err, domain.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		case errors.Is(err, domain.ErrInvalidProductModel):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		default:
			s.providerFailure(c, err)
		}
		return
	}
	if len(result.Rejected) > 0 || len(result.Accepted) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	c.JSON(http.StatusAccepted, result)
}

func (s *Server) telemetryHistoryV2(c *gin.Context) {
	repo, ok := s.deps.Telemetry.(domain.GenericTelemetryRepo)
	if !ok {
		s.providerUnavailable(c)
		return
	}
	values, err := parseQueryInput(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	metric := values.Get("metric")
	if !validTelemetryIdentifier(metric) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	if (values.Has("from") && values.Get("from") == "") || (values.Has("to") && values.Get("to") == "") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	from, to, err := parseTelemetryWindow(values.Get("from"), values.Get("to"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	limit, err := strconv.Atoi(queryValueOrDefault(values, "limit", "1000"))
	if err != nil || limit < 1 || limit > 10000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	points, unit, err := repo.HistoryV2ForUser(c.Request.Context(), c.Param("device_no"), uid(c), metric, from, to, limit)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		if errors.Is(err, domain.ErrInvalidProductModel) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
			return
		}
		s.providerFailure(c, err)
		return
	}
	if points == nil {
		points = []domain.TelemetryHistoryPoint{}
	}
	c.JSON(http.StatusOK, gin.H{"device_no": c.Param("device_no"), "metric": metric, "unit": unit, "points": points})
}

func validTelemetryIdentifier(value string) bool {
	if value == "" || len(value) > 64 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for _, ch := range value[1:] {
		if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '_') {
			return false
		}
	}
	return true
}

func parseTelemetryWindow(fromValue, toValue string) (time.Time, time.Time, error) {
	now := time.Now().UTC()
	from, to := now.Add(-24*time.Hour), now
	var err error
	if fromValue != "" {
		from, err = time.Parse(time.RFC3339, fromValue)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	if toValue != "" {
		to, err = time.Parse(time.RFC3339, toValue)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	if !from.Before(to) {
		return time.Time{}, time.Time{}, errors.New("invalid telemetry window")
	}
	return from.UTC(), to.UTC(), nil
}
