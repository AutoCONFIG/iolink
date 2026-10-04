package adminapi

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/license"
)

func (s *Server) licenseStore(c *gin.Context) (LicenseStore, bool) {
	store, ok := s.deps.Store.(LicenseStore)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "license_unavailable"})
	}
	return store, ok
}

func (s *Server) getLicense(c *gin.Context) {
	if !c.GetBool("platform_admin") {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	store, ok := s.licenseStore(c)
	if !ok {
		return
	}
	status, err := store.LicenseStatus(c.Request.Context())
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	c.JSON(http.StatusOK, status)
}

func (s *Server) importLicense(c *gin.Context) {
	if !c.GetBool("platform_admin") {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	store, ok := s.licenseStore(c)
	if !ok {
		return
	}
	actor, ok := c.Get("aid")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	actorID, ok := actor.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, license.MaxEnvelopeBytes+1))
	if err != nil || len(raw) > license.MaxEnvelopeBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	envelope, err := license.ParseEnvelope(raw)
	if err != nil {
		if auditErr := store.RecordLicenseRejection(c.Request.Context(), raw, actorID, "license_invalid"); auditErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	if err := store.ImportLicense(c.Request.Context(), envelope, actorID); err != nil {
		switch {
		case errors.Is(err, license.ErrUnavailable):
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "license_unavailable"})
		case errors.Is(err, license.ErrInstanceMismatch):
			c.JSON(http.StatusBadRequest, gin.H{"error": "license_instance_mismatch"})
		case errors.Is(err, license.ErrInvalidSignature):
			c.JSON(http.StatusBadRequest, gin.H{"error": "license_signature_invalid"})
		case errors.Is(err, license.ErrClockError):
			c.JSON(http.StatusConflict, gin.H{"error": "license_clock_error"})
		case errors.Is(err, license.ErrNotBefore):
			c.JSON(http.StatusBadRequest, gin.H{"error": "license_not_before"})
		case errors.Is(err, license.ErrExpired):
			c.JSON(http.StatusBadRequest, gin.H{"error": "license_expired"})
		case errors.Is(err, domain.ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "license_invalid"})
		}
		return
	}
	c.Status(http.StatusNoContent)
}
