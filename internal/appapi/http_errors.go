package appapi

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) providerFailure(c *gin.Context, err error) {
	if s.log != nil {
		s.log.ErrorContext(c.Request.Context(), "HTTP provider operation failed", slog.String("stage", "http_provider"), slog.Any("err", err))
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
}

func (s *Server) providerUnavailable(c *gin.Context) {
	if s.log != nil {
		s.log.ErrorContext(c.Request.Context(), "HTTP provider capability unavailable", slog.String("stage", "http_provider"))
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
}
