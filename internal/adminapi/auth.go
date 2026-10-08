package adminapi

import (
	"errors"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"strings"
	"time"
)

type loginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (s *Server) login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	u, err := s.deps.Store.FindAdminByLogin(c.Request.Context(), req.Username)
	if err != nil || u.PasswordHash == nil || !platform.CheckPassword(*u.PasswordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if platform.IsLegacyPassword(*u.PasswordHash) {
		_ = s.deps.Store.UpgradeAdminPassword(c.Request.Context(), u.ID, platform.HashPassword(req.Password))
	}
	version, err := s.deps.Store.AdminTokenVersion(c.Request.Context(), u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "login unavailable"})
		return
	}
	key := platform.DeriveAdminKey(s.cfg.SecretKey)
	claims := jwt.MapClaims{"aid": u.ID, "ver": version, "exp": time.Now().Add(s.cfg.JWT).Unix()}
	if tenants, ok := s.deps.Store.(adminTenantStore); ok {
		if tenantID, tenantErr := tenants.DefaultTenantForUser(c.Request.Context(), u.ID); tenantErr == nil && tenantID > 0 {
			membershipVersion, versionErr := tenants.TenantMembershipVersion(c.Request.Context(), u.ID, tenantID)
			if versionErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "tenant unavailable"})
				return
			}
			claims["tenant_id"] = tenantID
			claims["tenant_ver"] = membershipVersion
			role, roleErr := tenants.TenantRole(c.Request.Context(), u.ID, tenantID)
			if roleErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "tenant role unavailable"})
				return
			}
			claims["tenant_role"] = role
		} else if errors.Is(tenantErr, domain.ErrInactiveTenant) && u.Authority != "ADMIN" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "tenant membership inactive"})
			return
		} else if tenantErr != nil && !errors.Is(tenantErr, domain.ErrNotFound) && !errors.Is(tenantErr, domain.ErrInactiveTenant) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "tenant unavailable"})
			return
		}
	} else if u.Authority != "ADMIN" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "tenant store unavailable"})
		return
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "expires_in": int(s.cfg.JWT.Seconds()), "platform_admin": u.Authority == "ADMIN"})
}

func (s *Server) authRequired(c *gin.Context) {
	h := c.GetHeader("Authorization")
	const prefix = "Bearer "
	if len(h) <= len(prefix) || h[:len(prefix)] != prefix {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
		return
	}
	tok, err := jwt.Parse(h[len(prefix):], func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("bad signing method")
		}
		return platform.DeriveAdminKey(s.cfg.SecretKey), nil
	})
	if err != nil || !tok.Valid {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	claims := tok.Claims.(jwt.MapClaims)
	if _, ok := claims["exp"].(float64); !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing exp"})
		return
	}
	aid, ok := claims["aid"].(float64)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "bad claims"})
		return
	}
	c.Set("aid", int64(aid))
	version, ok := claims["ver"].(float64)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "bad claims"})
		return
	}
	current, err := s.deps.Store.AdminTokenVersion(c.Request.Context(), int64(aid))
	if err != nil || int64(version) != int64(current) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token revoked"})
		return
	}
	admin, adminErr := s.deps.Store.FindAdminByID(c.Request.Context(), int64(aid))
	if adminErr != nil || (admin.Authority != "ADMIN" && admin.Authority != "USER") {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid admin"})
		return
	}
	platformAdmin := admin.Authority == "ADMIN"
	c.Set("platform_admin", platformAdmin)
	if platformAdmin {
		c.Request = c.Request.WithContext(domain.WithPlatformActor(c.Request.Context(), domain.PlatformActor{ID: int64(aid), TokenVersion: current}))
	}
	if tenants, ok := s.deps.Store.(adminTenantStore); ok {
		tenantRaw, tenantOK := claims["tenant_id"].(float64)
		membershipRaw, membershipOK := claims["tenant_ver"].(float64)
		if !tenantOK && !membershipOK {
			_, membershipErr := tenants.DefaultTenantForUser(c.Request.Context(), int64(aid))
			if membershipErr == nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "tenant context required"})
				return
			}
			if !errors.Is(membershipErr, domain.ErrNotFound) && !errors.Is(membershipErr, domain.ErrInactiveTenant) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "tenant unavailable"})
				return
			}
			if !platformAdmin && errors.Is(membershipErr, domain.ErrInactiveTenant) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "tenant membership inactive"})
				return
			}
			c.Next()
			return
		}
		if !tenantOK || !membershipOK {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "tenant context required"})
			return
		}
		membershipVersion, err := tenants.TenantMembershipVersion(c.Request.Context(), int64(aid), int64(tenantRaw))
		if err != nil || int64(membershipRaw) != membershipVersion {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "tenant membership revoked"})
			return
		}
		role, roleErr := tenants.TenantRole(c.Request.Context(), int64(aid), int64(tenantRaw))
		claimRole, roleOK := claims["tenant_role"].(string)
		if roleErr != nil || !roleOK || claimRole != role {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "tenant role invalid"})
			return
		}
		requestContext := domain.WithTenantUserID(domain.WithTenantRole(domain.WithTenantID(c.Request.Context(), int64(tenantRaw)), role), int64(aid))
		requestContext = domain.WithTenantPermissionVersion(requestContext, int64(membershipRaw))
		c.Set("tenant_id", int64(tenantRaw))
		c.Request = c.Request.WithContext(requestContext)
	}
	c.Next()
}

func (s *Server) tenantRequired(c *gin.Context) {
	if _, scopedStore := s.deps.Store.(adminTenantStore); !scopedStore {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "tenant store unavailable"})
		return
	}
	if _, ok := domain.TenantID(c.Request.Context()); !ok {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "tenant context required"})
		return
	}
	if s.deps.Policy == nil {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "tenant policy unavailable"})
		return
	}
	resource, action := tenantPermission(c.FullPath(), c.Request.Method)
	allowed, err := s.deps.Policy.Allow(domain.TenantRole(c.Request.Context()), resource, action)
	if err != nil || !allowed {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "tenant action forbidden"})
		return
	}
	c.Next()
}

func tenantPermission(path, method string) (string, string) {
	relative := strings.TrimPrefix(path, "/admin/v1/")
	resource, _, _ := strings.Cut(relative, "/")
	if resource == "farms" && strings.Contains(relative, "/members") {
		resource = "farm_members"
	}
	if resource == "alarm-rules" {
		resource = "alarm_rules"
	}
	action := "write"
	if method == http.MethodGet {
		action = "read"
	} else if method == http.MethodPost && resource == "alarms" && (strings.HasSuffix(relative, "/confirm") || strings.HasSuffix(relative, "/batch-confirm")) {
		action = "confirm"
	}
	return resource, action
}

func aid(c *gin.Context) int64 { return c.MustGet("aid").(int64) }
