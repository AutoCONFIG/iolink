package adminapi

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) organizationUnavailable(c *gin.Context) {
	if s.deps.Logger != nil {
		s.deps.Logger.ErrorContext(c.Request.Context(), "HTTP provider capability unavailable", slog.String("stage", "http_provider"))
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{"error": "unavailable"})
}
