package adminapi

import (
	"context"
	"errors"
	"net/http"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/gin-gonic/gin"
)

type RegistrationStore interface {
	RegisterUser(context.Context, domain.Registration) error
}

func (s *Server) register(c *gin.Context) {
	store, ok := s.deps.Store.(RegistrationStore)
	if !ok {
		s.organizationUnavailable(c)
		return
	}
	var input domain.Registration
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	if err := bindBaselineJSON(c, &input, "username", "password"); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	if err := store.RegisterUser(c.Request.Context(), input); err != nil {
		switch {
		case errors.Is(err, domain.ErrBootstrapInput):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		case errors.Is(err, domain.ErrConflict):
			c.JSON(http.StatusConflict, gin.H{"error": "conflict"})
		case errors.Is(err, domain.ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		default:
			s.providerFailure(c, err)
		}
		return
	}
	c.Status(http.StatusCreated)
}
