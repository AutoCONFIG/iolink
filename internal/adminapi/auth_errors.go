package adminapi

import (
	"errors"
	"net/http"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/gin-gonic/gin"
)

func (s *Server) authProviderFailure(c *gin.Context, err error) {
	if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrInactiveTenant) {
		abortCameraAuth(c, http.StatusUnauthorized, "identity inactive")
		return
	}
	if c.GetBool("camera_route") {
		if s.deps.Logger != nil {
			s.deps.Logger.ErrorContext(c.Request.Context(), "HTTP identity operation failed", "stage", "http_auth", "err", err)
		}
		cameraError(c, err)
	} else {
		s.providerFailure(c, err)
	}
	c.Abort()
}

func (s *Server) authProviderUnavailable(c *gin.Context) {
	if c.GetBool("camera_route") {
		if s.deps.Logger != nil {
			s.deps.Logger.ErrorContext(c.Request.Context(), "HTTP identity capability unavailable", "stage", "http_auth")
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "internal_error"})
	} else {
		s.providerUnavailable(c)
	}
	c.Abort()
}
