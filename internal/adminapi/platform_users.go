package adminapi

import (
	"context"
	"net/http"
	"unicode/utf8"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/gin-gonic/gin"
)

type PlatformUserStore interface {
	ListPlatformUsers(context.Context, string) ([]domain.PlatformUser, error)
}

func (s *Server) platformUsers(c *gin.Context) {
	if !c.GetBool("platform_admin") {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	store, ok := s.deps.Store.(PlatformUserStore)
	if !ok {
		s.organizationUnavailable(c)
		return
	}
	if !validBaselineQuery(c) {
		return
	}
	query := c.Query("query")
	if !utf8.ValidString(query) || utf8.RuneCountInString(query) > 128 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	users, err := store.ListPlatformUsers(c.Request.Context(), query)
	if err != nil {
		s.providerFailure(c, err)
		return
	}
	c.JSON(http.StatusOK, users)
}
