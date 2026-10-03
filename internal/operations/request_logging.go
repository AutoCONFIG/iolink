package operations

import (
	"crypto/rand"
	"log/slog"
	"net/http"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/gin-gonic/gin"
)

func RequestLogging(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		id := rand.Text()
		c.Header("X-Request-ID", id)
		defer func() {
			if recover() != nil {
				log.Error("http handler panic", "request_id", id, "route", routeName(c))
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			}
			status := c.Writer.Status()
			level := slog.LevelInfo
			if status >= 500 {
				level = slog.LevelError
			} else if status >= 400 {
				level = slog.LevelWarn
			}
			tenant, _ := domain.TenantID(c.Request.Context())
			actor, _ := domain.TenantUserID(c.Request.Context())
			log.Log(c.Request.Context(), level, "http request", "request_id", id, "route", routeName(c), "method", safeMethod(c.Request.Method), "status", status, "duration_ms", time.Since(started).Milliseconds(), "tenant_id", tenant, "actor_id", actor)
		}()
		c.Next()
	}
}

func routeName(c *gin.Context) string {
	if path := c.FullPath(); path != "" {
		return path
	}
	return "unmatched"
}
func safeMethod(method string) string {
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return method
	default:
		return "OTHER"
	}
}
