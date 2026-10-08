package adminapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/license"
)

type apiKeyCreateRequest struct {
	Name      string              `json:"name" binding:"required"`
	Scopes    []string            `json:"scopes" binding:"required"`
	Resources apiKeyResourceInput `json:"resources"`
}

func (s *Server) apiKeyStore(c *gin.Context) (APIKeyAdminStore, bool) {
	if s.deps.APIKeys == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "api key management unavailable"})
		return nil, false
	}
	return s.deps.APIKeys, true
}

func (s *Server) apiKeyTenant(c *gin.Context) (int64, bool) {
	tenantID, ok := domain.TenantID(c.Request.Context())
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "tenant context required"})
		return 0, false
	}
	return tenantID, true
}

func apiKeyError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, license.ErrUnavailable) || errors.Is(err, license.ErrRequired) || errors.Is(err, license.ErrFeatureDenied) || errors.Is(err, license.ErrClockError):
		c.JSON(http.StatusForbidden, gin.H{"error": "feature_unavailable"})
	case errors.Is(err, domain.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	case errors.Is(err, domain.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "api key not found"})
	case errors.Is(err, domain.ErrInvalidAPIKey):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid api key request"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
	}
}

func (s *Server) listAPIKeys(c *gin.Context) {
	store, ok := s.apiKeyStore(c)
	if !ok {
		return
	}
	tenantID, ok := s.apiKeyTenant(c)
	if !ok {
		return
	}
	keys, err := store.ListAPIKeys(c.Request.Context(), tenantID, aid(c))
	if err != nil {
		apiKeyError(c, err)
		return
	}
	c.JSON(http.StatusOK, keys)
}

func (s *Server) listAPIKeyAudit(c *gin.Context) {
	store, ok := s.apiKeyStore(c)
	if !ok {
		return
	}
	tenantID, ok := s.apiKeyTenant(c)
	if !ok {
		return
	}
	items, err := store.ListAPIKeyAuditEvents(c.Request.Context(), tenantID, aid(c), 100)
	if err != nil {
		apiKeyError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (s *Server) createAPIKey(c *gin.Context) {
	store, ok := s.apiKeyStore(c)
	if !ok {
		return
	}
	tenantID, ok := s.apiKeyTenant(c)
	if !ok {
		return
	}
	var req apiKeyCreateRequest
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid api key request"})
		return
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid api key request"})
		return
	}
	resources := domain.APIKeyResourceScope{FarmIDs: req.Resources.FarmIDs, PondIDs: req.Resources.PondIDs, DeviceNos: req.Resources.DeviceNos}
	key, secret, err := store.IssueAPIKey(c.Request.Context(), tenantID, strings.TrimSpace(req.Name), req.Scopes, resources, aid(c))
	if err != nil {
		apiKeyError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"key": key, "secret": secret})
}

func (s *Server) rotateAPIKey(c *gin.Context) {
	store, ok := s.apiKeyStore(c)
	if !ok {
		return
	}
	tenantID, ok := s.apiKeyTenant(c)
	if !ok {
		return
	}
	key, secret, err := store.RotateAPIKey(c.Request.Context(), tenantID, aid(c), c.Param("key_id"))
	if err != nil {
		apiKeyError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"key": key, "secret": secret})
}

func (s *Server) revokeAPIKey(c *gin.Context) {
	store, ok := s.apiKeyStore(c)
	if !ok {
		return
	}
	tenantID, ok := s.apiKeyTenant(c)
	if !ok {
		return
	}
	if err := store.RevokeAPIKey(c.Request.Context(), tenantID, aid(c), c.Param("key_id")); err != nil {
		apiKeyError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
