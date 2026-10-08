package adminapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) userAccountRequired(c *gin.Context) {
	if c.GetBool("platform_admin") {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "business user required"})
		return
	}
	c.Next()
}
