package adminapi

import (
	"context"
	"encoding/json"
	"errors"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
)

type RegistrationStore interface {
	RegisterUser(context.Context, domain.Registration) error
}

func (s *Server) register(c *gin.Context) {
	store, ok := s.deps.Store.(RegistrationStore)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "registration unavailable"})
		return
	}
	var input domain.Registration
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	if err := store.RegisterUser(c.Request.Context(), input); err != nil {
		switch {
		case errors.Is(err, domain.ErrBootstrapInput):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid registration"})
		case errors.Is(err, domain.ErrConflict):
			c.JSON(http.StatusConflict, gin.H{"error": "username unavailable"})
		case errors.Is(err, domain.ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "platform not initialized"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "registration failed"})
		}
		return
	}
	c.Status(http.StatusCreated)
}
