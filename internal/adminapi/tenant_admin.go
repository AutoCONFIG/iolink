package adminapi

import (
	"errors"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"time"
)

type tenantStatusRequest struct {
	Active *bool `json:"active"`
}
type tenantCreateRequest struct {
	Name string `json:"name" binding:"required"`
}
type tenantMemberRequest struct {
	Role      string     `json:"role"`
	Active    *bool      `json:"active"`
	ExpiresAt *time.Time `json:"expires_at"`
}

func (s *Server) tenantAdminStore(c *gin.Context) (TenantAdminStore, bool) {
	store, ok := s.deps.Store.(TenantAdminStore)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "tenant management unavailable"})
		return nil, false
	}
	return store, true
}

func (s *Server) listTenants(c *gin.Context) {
	if !c.GetBool("platform_admin") {
		if _, ok := domain.TenantID(c.Request.Context()); !ok {
			c.JSON(http.StatusOK, []domain.Tenant{})
			return
		}
	}
	store, ok := s.tenantAdminStore(c)
	if !ok {
		return
	}
	items, err := store.ListTenants(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "tenant query failed"})
		return
	}
	if tenantID, scoped := domain.TenantID(c.Request.Context()); scoped {
		if c.GetBool("platform_admin") {
			c.JSON(http.StatusOK, items)
			return
		}
		filtered := items[:0]
		for _, item := range items {
			if item.ID == tenantID {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	c.JSON(http.StatusOK, items)
}

func (s *Server) createTenant(c *gin.Context) {
	if !c.GetBool("platform_admin") {
		c.JSON(http.StatusForbidden, gin.H{"error": "platform admin required"})
		return
	}
	store, ok := s.deps.Store.(TenantCreateStore)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "tenant creation unavailable"})
		return
	}
	var req tenantCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	tenant, err := store.CreateTenant(c.Request.Context(), req.Name, aid(c))
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrConflict):
			c.JSON(http.StatusConflict, gin.H{"error": "tenant already exists"})
		case errors.Is(err, domain.ErrInvalidProductModel):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant name"})
		case errors.Is(err, domain.ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "tenant creation failed"})
		}
		return
	}
	c.JSON(http.StatusCreated, tenant)
}

func (s *Server) platformStats(c *gin.Context) {
	if !c.GetBool("platform_admin") {
		c.JSON(http.StatusForbidden, gin.H{"error": "platform admin required"})
		return
	}
	store, ok := s.deps.Store.(PlatformStatsStore)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "platform status unavailable"})
		return
	}
	stats, err := store.PlatformStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "platform status unavailable"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (s *Server) setTenantStatus(c *gin.Context) {
	if !c.GetBool("platform_admin") {
		c.JSON(http.StatusForbidden, gin.H{"error": "platform admin required"})
		return
	}
	store, ok := s.tenantAdminStore(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	var req tenantStatusRequest
	if err = c.ShouldBindJSON(&req); err != nil || req.Active == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	if err = store.SetTenantActive(c.Request.Context(), id, *req.Active, aid(c)); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "tenant update failed"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) listTenantMembers(c *gin.Context) {
	role := domain.TenantRole(c.Request.Context())
	if !c.GetBool("platform_admin") && (role != "owner" && role != "admin") {
		c.JSON(http.StatusForbidden, gin.H{"error": "tenant admin required"})
		return
	}
	store, ok := s.tenantAdminStore(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	if tenantID, scoped := domain.TenantID(c.Request.Context()); scoped && tenantID != id {
		if !c.GetBool("platform_admin") {
			c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
			return
		}
	}
	items, err := store.ListTenantMembers(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "member query failed"})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (s *Server) setTenantMember(c *gin.Context) {
	if !c.GetBool("platform_admin") {
		_, scoped := domain.TenantID(c.Request.Context())
		role := domain.TenantRole(c.Request.Context())
		if !scoped || (role != "owner" && role != "admin") {
			c.JSON(http.StatusForbidden, gin.H{"error": "tenant admin required"})
			return
		}
	}
	store, ok := s.tenantAdminStore(c)
	if !ok {
		return
	}
	tenantID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad tenant id"})
		return
	}
	if scopedTenant, scoped := domain.TenantID(c.Request.Context()); scoped {
		platform := c.GetBool("platform_admin")
		if !platform && scopedTenant != tenantID {
			c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
			return
		}
		role := domain.TenantRole(c.Request.Context())
		if !platform && role != "owner" && role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "tenant admin required"})
			return
		}
	}
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad user id"})
		return
	}
	var req tenantMemberRequest
	if err = c.ShouldBindJSON(&req); err != nil || req.Role == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	if err = store.SetTenantMember(c.Request.Context(), tenantID, userID, req.Role, active, req.ExpiresAt, aid(c)); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "tenant or user not found"})
			return
		}
		if errors.Is(err, domain.ErrInvalidProductModel) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "member update failed"})
		return
	}
	c.Status(http.StatusNoContent)
}
