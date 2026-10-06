package openapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/license"
	"git.hyhy.fun/rsplab/iolink/internal/operations"
)

type Authenticator interface {
	AuthenticateOpen(context.Context, domain.OpenRequest, time.Time) (domain.OpenPrincipal, error)
}

type ResourceStore interface {
	ListOpenPonds(context.Context, int64, domain.APIKeyResourceScope) ([]domain.Pond, error)
	ListOpenDevices(context.Context, int64, domain.APIKeyResourceScope) ([]domain.Device, error)
	ListOpenAlarms(context.Context, int64, domain.APIKeyResourceScope) ([]domain.Alarm, error)
}

type Deps struct {
	Auth      Authenticator
	Resources ResourceStore
}

type Server struct{ deps Deps }

func New(deps Deps) *Server { return &Server{deps: deps} }

func (s *Server) Routes() http.Handler {
	r := gin.New()
	r.Use(operations.RequestLogging(nil))
	v1 := r.Group("/open/v1", s.authenticate)
	v1.GET("/ponds", s.listPonds)
	v1.GET("/ponds/:id", s.getPond)
	v1.GET("/devices", s.listDevices)
	v1.GET("/devices/:device_no", s.getDevice)
	v1.GET("/alarms", s.listAlarms)
	return r
}

func (s *Server) authenticate(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 4<<20))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	principal, err := s.deps.Auth.AuthenticateOpen(c.Request.Context(), domain.OpenRequest{
		KeyID: c.GetHeader("X-Key-Id"), Timestamp: parseInt64(c.GetHeader("X-Timestamp")), Nonce: c.GetHeader("X-Nonce"), Signature: c.GetHeader("X-Signature"), Method: c.Request.Method, Path: c.Request.URL.EscapedPath(), Query: c.Request.URL.RawQuery, Body: body,
	}, time.Now().UTC())
	if err != nil {
		var rate interface{ RetryAfterValue() int }
		if errors.As(err, &rate) {
			c.Header("Retry-After", strconv.Itoa(rate.RetryAfterValue()))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate_limited"})
			return
		}
		if errors.Is(err, license.ErrUnavailable) || errors.Is(err, license.ErrRequired) || errors.Is(err, license.ErrFeatureDenied) || errors.Is(err, license.ErrClockError) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "feature_unavailable"})
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	c.Set("open_principal", principal)
	c.Next()
}

func principal(c *gin.Context) domain.OpenPrincipal {
	value, _ := c.Get("open_principal")
	return value.(domain.OpenPrincipal)
}

func hasScope(p domain.OpenPrincipal, name string) bool {
	for _, scope := range p.Scopes {
		if scope == name {
			return true
		}
	}
	return false
}

func (s *Server) listPonds(c *gin.Context) {
	p := principal(c)
	if !hasScope(p, "ponds:read") {
		c.JSON(http.StatusForbidden, gin.H{"error": "scope_required"})
		return
	}
	items, err := s.deps.Resources.ListOpenPonds(c.Request.Context(), p.TenantID, p.Resources)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query_failed"})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (s *Server) getPond(c *gin.Context) {
	p := principal(c)
	if !hasScope(p, "ponds:read") {
		c.JSON(http.StatusForbidden, gin.H{"error": "scope_required"})
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	items, err := s.deps.Resources.ListOpenPonds(c.Request.Context(), p.TenantID, p.Resources)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query_failed"})
		return
	}
	for _, item := range items {
		if item.ID == id {
			c.JSON(http.StatusOK, item)
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
}

func (s *Server) listDevices(c *gin.Context) {
	p := principal(c)
	if !hasScope(p, "devices:read") {
		c.JSON(http.StatusForbidden, gin.H{"error": "scope_required"})
		return
	}
	items, err := s.deps.Resources.ListOpenDevices(c.Request.Context(), p.TenantID, p.Resources)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query_failed"})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (s *Server) getDevice(c *gin.Context) {
	p := principal(c)
	if !hasScope(p, "devices:read") {
		c.JSON(http.StatusForbidden, gin.H{"error": "scope_required"})
		return
	}
	items, err := s.deps.Resources.ListOpenDevices(c.Request.Context(), p.TenantID, p.Resources)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query_failed"})
		return
	}
	for _, item := range items {
		if item.DeviceNo == c.Param("device_no") {
			c.JSON(http.StatusOK, item)
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
}

func (s *Server) listAlarms(c *gin.Context) {
	p := principal(c)
	if !hasScope(p, "alarms:read") {
		c.JSON(http.StatusForbidden, gin.H{"error": "scope_required"})
		return
	}
	items, err := s.deps.Resources.ListOpenAlarms(c.Request.Context(), p.TenantID, p.Resources)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query_failed"})
		return
	}
	c.JSON(http.StatusOK, items)
}

func parseInt64(value string) int64 { n, _ := strconv.ParseInt(value, 10, 64); return n }
