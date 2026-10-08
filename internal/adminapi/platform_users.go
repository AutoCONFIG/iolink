package adminapi

import (
	"context"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/gin-gonic/gin"
	"net/http"
)

type PlatformUserStore interface {
	ListPlatformUsers(context.Context, string) ([]domain.PlatformUser, error)
}

func (s *Server) platformUsers(c *gin.Context) {
	if !c.GetBool("platform_admin") {
		c.JSON(http.StatusForbidden, gin.H{"error": "platform admin required"})
		return
	}
	store, ok := s.deps.Store.(PlatformUserStore)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "user directory unavailable"})
		return
	}
	query := c.Query("query")
	if len(query) > 128 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query"})
		return
	}
	users, err := store.ListPlatformUsers(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user directory unavailable"})
		return
	}
	c.JSON(http.StatusOK, users)
}
