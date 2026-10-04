// Package appapi exposes the /api/v1 HTTP surface for the WeChat mini
// program. It depends ONLY on git.hyhy.fun/rsplab/git.hyhy.fun/rsplab/iolink/internal/domain: domain models and
// repository interfaces supplied by core. It never imports iolink-core,
// iolink-access, and never touches SQL or MQTT.
package appapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/platform"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	iolinkcontractsdomain "git.hyhy.fun/rsplab/iolink/internal/domain"
)

// Config for appapi.
type Config struct {
	Addr      string        // ":8080"
	SecretKey string        // JWT signing key
	JWT       time.Duration // token lifetime
	Wechat    WechatConfig  // empty AppID => stub exchanger (dev)
}

// Deps are the repositories appapi needs. cmd/iolinkd passes core's
// implementations; tests pass fakes.
type Deps struct {
	Ponds     iolinkcontractsdomain.PondRepo
	Devices   iolinkcontractsdomain.DeviceRepo
	Telemetry iolinkcontractsdomain.TelemetryRepo
	Alarms    iolinkcontractsdomain.AlarmRepo
	// Users abstracts login (openid lookup); core implements with users table.
	Users UserStore
}

// UserStore is the auth-facing slice of the users domain.
type UserStore interface {
	FindByOpenID(ctx context.Context, openID string) (*iolinkcontractsdomain.User, error)
	EnsureUser(ctx context.Context, openID string) (*iolinkcontractsdomain.User, error)
	UserTokenVersion(ctx context.Context, id int64) (int, error)
}

type tenantTokenStore interface {
	DefaultTenantForUser(ctx context.Context, userID int64) (int64, error)
	TenantMembershipVersion(ctx context.Context, userID, tenantID int64) (int64, error)
}

type tenantRoleStore interface {
	TenantRole(ctx context.Context, userID, tenantID int64) (string, error)
}

type tenantMembershipStore interface {
	ListUserTenants(ctx context.Context, userID int64) ([]iolinkcontractsdomain.TenantMembership, error)
	TenantMembershipVersion(ctx context.Context, userID, tenantID int64) (int64, error)
}

// Server is the appapi HTTP server.
type Server struct {
	cfg  Config
	deps Deps
	wx   func(code string) (openID string, err error)
	log  *slog.Logger
}

func New(cfg Config, deps Deps, log *slog.Logger) *Server {
	s := &Server{cfg: cfg, deps: deps, wx: WechatExchanger, log: log}
	if cfg.Wechat.AppID != "" && cfg.Wechat.Secret != "" {
		s.wx = RealWechatExchanger(cfg.Wechat)
	}
	return s
}

// Routes builds the gin engine with all /api/v1 routes.
func (s *Server) Routes() http.Handler {
	r := gin.New()
	r.Use(gin.Recovery(), gin.LoggerWithConfig(gin.LoggerConfig{SkipPaths: []string{"/healthz"}}))

	v1 := r.Group("/api/v1")
	v1.POST("/auth/login", s.login)

	auth := v1.Group("", s.authRequired)
	{
		auth.GET("/auth/tenants", s.listTenants)
		auth.POST("/auth/tenant", s.switchTenant)
		auth.GET("/ponds", s.listPonds)
		auth.GET("/ponds/:id", s.getPond)
		auth.GET("/devices", s.listDevices)
		auth.GET("/devices/:device_no", s.getDevice)
		auth.GET("/water/latest", s.waterLatest)
		auth.GET("/water/history", s.waterHistory)
		auth.GET("/alarms", s.listAlarms)
		auth.GET("/stats/summary", s.statsSummary)
		auth.POST("/alarms/:id/confirm", s.confirmAlarm)
	}
	v2 := r.Group("/api/v2", s.authRequired)
	v2.POST("/devices/:device_no/telemetry", s.submitTelemetryV2)
	v2.GET("/devices/:device_no/model/latest", s.modelLatestV2)
	v2.GET("/devices/:device_no/history", s.telemetryHistoryV2)
	return r
}

type tenantSwitchRequest struct {
	TenantID int64 `json:"tenant_id" binding:"required"`
}

func (s *Server) listTenants(c *gin.Context) {
	store, ok := s.deps.Users.(tenantMembershipStore)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "tenant membership unavailable"})
		return
	}
	tenants, err := store.ListUserTenants(c.Request.Context(), uid(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "tenant query failed"})
		return
	}
	c.JSON(http.StatusOK, tenants)
}

func (s *Server) switchTenant(c *gin.Context) {
	var req tenantSwitchRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.TenantID < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	store, ok := s.deps.Users.(tenantMembershipStore)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "tenant membership unavailable"})
		return
	}
	tenants, err := store.ListUserTenants(c.Request.Context(), uid(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "tenant query failed"})
		return
	}
	found := false
	for _, tenant := range tenants {
		if tenant.TenantID == req.TenantID {
			found = true
			break
		}
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		return
	}
	token, err := s.signTokenForTenant(c.Request.Context(), uid(c), req.TenantID)
	if err != nil {
		if errors.Is(err, iolinkcontractsdomain.ErrInactiveTenant) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "tenant membership inactive"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token issue failed"})
		return
	}
	c.JSON(http.StatusOK, loginResp{Token: token, ExpiresIn: int(s.cfg.JWT.Seconds())})
}

// Run blocks serving until the http server returns.
func (s *Server) Run() error {
	srv := &http.Server{Addr: s.cfg.Addr, Handler: s.Routes(), ReadHeaderTimeout: 5 * time.Second}
	s.log.Info("appapi listening", "addr", s.cfg.Addr)
	return srv.ListenAndServe()
}

// ---- auth ----

type loginReq struct {
	Code string `json:"code" binding:"required"`
}

type loginResp struct {
	Token     string                      `json:"token"`
	ExpiresIn int                         `json:"expires_in"`
	User      *iolinkcontractsdomain.User `json:"user"`
}

var errWechatCodeInvalid = errors.New("wechat code invalid")

// login exchanges a wx.login code for a JWT. The WeChat code2session call is
// injected via WechatExchanger so it can be faked in tests / swapped later.
func (s *Server) login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	openID, err := s.wx(req.Code)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "wechat login failed"})
		return
	}
	u, err := s.deps.Users.EnsureUser(c.Request.Context(), openID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	token, err := s.signToken(c.Request.Context(), u.ID)
	if err != nil {
		if errors.Is(err, iolinkcontractsdomain.ErrInactiveTenant) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "tenant membership inactive"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, loginResp{Token: token, ExpiresIn: int(s.cfg.JWT.Seconds()), User: u})
}

// WechatExchanger is the dev stub (accepts nothing). Overridden by real
// code2session when Config.Wechat is set, or via SetWechatExchanger in tests.
var WechatExchanger = func(code string) (openID string, err error) {
	return "", errWechatCodeInvalid
}

func (s *Server) signToken(ctx context.Context, userID int64) (string, error) {
	tenants, ok := s.deps.Users.(tenantTokenStore)
	if !ok {
		return "", errors.New("tenant store unavailable")
	}
	tenantID, err := tenants.DefaultTenantForUser(ctx, userID)
	if err != nil {
		if errors.Is(err, iolinkcontractsdomain.ErrNotFound) {
			return "", iolinkcontractsdomain.ErrInactiveTenant
		}
		return "", err
	}
	if tenantID <= 0 {
		return "", iolinkcontractsdomain.ErrInactiveTenant
	}
	return s.signTokenForTenant(ctx, userID, tenantID)
}

func (s *Server) signTokenForTenant(ctx context.Context, userID, tenantID int64) (string, error) {
	if s.deps.Users == nil {
		return "", errors.New("user store unavailable")
	}
	version, err := s.deps.Users.UserTokenVersion(ctx, userID)
	if err != nil {
		return "", err
	}
	claims := jwt.MapClaims{"uid": userID, "ver": version, "exp": time.Now().Add(s.cfg.JWT).Unix()}
	if tenantID > 0 {
		tenants, ok := s.deps.Users.(tenantTokenStore)
		if !ok {
			return "", errors.New("tenant store unavailable")
		}
		membershipVersion, err := tenants.TenantMembershipVersion(ctx, userID, tenantID)
		if err != nil {
			return "", err
		}
		claims["tenant_id"] = tenantID
		claims["tenant_ver"] = membershipVersion
		roles, ok := s.deps.Users.(tenantRoleStore)
		if !ok {
			return "", errors.New("tenant role store unavailable")
		}
		role, roleErr := roles.TenantRole(ctx, userID, tenantID)
		if roleErr != nil {
			return "", roleErr
		}
		claims["tenant_role"] = role
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(platform.DeriveAppKey(s.cfg.SecretKey))
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
		return platform.DeriveAppKey(s.cfg.SecretKey), nil
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
	uid, ok := claims["uid"].(float64)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "bad claims"})
		return
	}
	c.Set("uid", int64(uid))
	version, ok := claims["ver"].(float64)
	if !ok || s.deps.Users == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "bad claims"})
		return
	}
	current, err := s.deps.Users.UserTokenVersion(c.Request.Context(), int64(uid))
	if err != nil || int64(version) != int64(current) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token revoked"})
		return
	}
	tenants, tenantStoreOK := s.deps.Users.(tenantTokenStore)
	if !tenantStoreOK {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "tenant store unavailable"})
		return
	}
	{
		tenantRaw, tenantOK := claims["tenant_id"].(float64)
		membershipRaw, membershipOK := claims["tenant_ver"].(float64)
		if !tenantOK && !membershipOK {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "tenant context required"})
			return
		}
		if !tenantOK || !membershipOK {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "tenant context required"})
			return
		}
		membershipVersion, err := tenants.TenantMembershipVersion(c.Request.Context(), int64(uid), int64(tenantRaw))
		if err != nil || int64(membershipRaw) != membershipVersion {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "tenant membership revoked"})
			return
		}
		roles, roleStoreOK := s.deps.Users.(tenantRoleStore)
		if !roleStoreOK {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "tenant role unavailable"})
			return
		}
		role, roleErr := roles.TenantRole(c.Request.Context(), int64(uid), int64(tenantRaw))
		claimRole, roleOK := claims["tenant_role"].(string)
		if roleErr != nil || !roleOK || claimRole != role {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "tenant role invalid"})
			return
		}
		c.Set("tenant_id", int64(tenantRaw))
		requestContext := iolinkcontractsdomain.WithTenantID(c.Request.Context(), int64(tenantRaw))
		requestContext = iolinkcontractsdomain.WithTenantRole(requestContext, role)
		requestContext = iolinkcontractsdomain.WithTenantUserID(requestContext, int64(uid))
		requestContext = iolinkcontractsdomain.WithTenantPermissionVersion(requestContext, int64(membershipRaw))
		c.Request = c.Request.WithContext(requestContext)
	}
	c.Next()
}

func uid(c *gin.Context) int64 { return c.MustGet("uid").(int64) }
