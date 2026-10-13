package openapi

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) providerFailure(c *gin.Context, err error) {
	if s.deps.Logger != nil {
		s.deps.Logger.ErrorContext(c.Request.Context(), "Open API provider operation failed", slog.String("stage", "open_provider"), slog.Any("err", err))
	}
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
}

func (s *Server) unavailable(c *gin.Context) {
	if s.deps.Logger != nil {
		s.deps.Logger.ErrorContext(c.Request.Context(), "Open API capability unavailable", slog.String("stage", "open_provider"))
	}
	c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "unavailable"})
}
