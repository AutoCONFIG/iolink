package appapi

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
		if s.log != nil {
			s.log.ErrorContext(c.Request.Context(), "HTTP identity operation failed", "stage", "http_auth", "err", err)
		}
		cameraError(c, err)
	} else {
		s.providerFailure(c, err)
	}
	c.Abort()
}
