package adminapi

import (
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/gin-gonic/gin"
	"net/http"
)

func (s *Server) session(c *gin.Context) {
	tenantID, _ := domain.TenantID(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"platform_admin": c.GetBool("platform_admin"), "tenant_id": tenantID, "tenant_role": domain.TenantRole(c.Request.Context())})
}
