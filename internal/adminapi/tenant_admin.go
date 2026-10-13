package adminapi

import (
	"errors"
	"net/http"
	"strconv"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/gin-gonic/gin"
)

type tenantStatusRequest struct {
	Active *bool `json:"active" binding:"required"`
}

type tenantCreateRequest struct {
	Name string `json:"name" binding:"required,max=128"`
}

func (s *Server) tenantAdminStore(c *gin.Context) (TenantAdminStore, bool) {
	store, ok := s.deps.Store.(TenantAdminStore)
	if !ok {
		s.providerUnavailable(c)
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
		s.providerFailure(c, err)
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
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	store, ok := s.deps.Store.(TenantCreateStore)
	if !ok {
		s.organizationUnavailable(c)
		return
	}
	var req tenantCreateRequest
	if err := bindBaselineJSON(c, &req, "name"); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	tenant, err := store.CreateTenant(c.Request.Context(), req.Name, aid(c))
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrConflict):
			c.JSON(http.StatusConflict, gin.H{"error": "conflict"})
		case errors.Is(err, domain.ErrInvalidProductModel):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		case errors.Is(err, domain.ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		default:
			s.providerFailure(c, err)
		}
		return
	}
	c.JSON(http.StatusCreated, tenant)
}

func (s *Server) platformStats(c *gin.Context) {
	if !c.GetBool("platform_admin") {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	store, ok := s.deps.Store.(PlatformStatsStore)
	if !ok {
		s.organizationUnavailable(c)
		return
	}
	stats, err := store.PlatformStats(c.Request.Context())
	if err != nil {
		s.providerFailure(c, err)
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (s *Server) setTenantStatus(c *gin.Context) {
	if !c.GetBool("platform_admin") {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	store, ok := s.tenantAdminStore(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	var req tenantStatusRequest
	if err = bindBaselineJSON(c, &req, "active"); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	if err = store.SetTenantActive(c.Request.Context(), id, *req.Active, aid(c)); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		s.providerFailure(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) listTenantMembers(c *gin.Context) {
	role := domain.TenantRole(c.Request.Context())
	if !c.GetBool("platform_admin") && (role != "owner" && role != "admin") {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	store, ok := s.tenantAdminStore(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	if tenantID, scoped := domain.TenantID(c.Request.Context()); scoped && tenantID != id {
		if !c.GetBool("platform_admin") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
	}
	items, err := store.ListTenantMembers(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		s.providerFailure(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (s *Server) setTenantMember(c *gin.Context) {
	if !c.GetBool("platform_admin") {
		_, scoped := domain.TenantID(c.Request.Context())
		role := domain.TenantRole(c.Request.Context())
		if !scoped || (role != "owner" && role != "admin") {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}
	store, ok := s.tenantAdminStore(c)
	if !ok {
		return
	}
	tenantID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || tenantID < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	if scopedTenant, scoped := domain.TenantID(c.Request.Context()); scoped {
		platform := c.GetBool("platform_admin")
		if !platform && scopedTenant != tenantID {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		role := domain.TenantRole(c.Request.Context())
		if !platform && role != "owner" && role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil || userID < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	req, err := parseMembershipBody(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	if err = store.SetTenantMember(c.Request.Context(), tenantID, userID, req.Role, *req.Active, req.ExpiresAt, aid(c)); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		if errors.Is(err, domain.ErrInvalidProductModel) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
			return
		}
		s.providerFailure(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
