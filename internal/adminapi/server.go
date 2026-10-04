// Package adminapi exposes /admin/v1 for the management web console.
// Same rules as appapi: depends only on interfaces defined here (implemented
// by core), never imports core/access, never touches SQL directly.
package adminapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/operations"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
)

// Config for adminapi.
type Config struct {
	SecretKey string        // root secret; admin JWT key is derived from it
	JWT       time.Duration // token lifetime
}

// AdminStore is the management-facing slice of the domain. Consumer-side
// interface: tests provide fakes; core.Service implements it with SQL.
type AdminStore interface {
	FindAdminByLogin(ctx context.Context, login string) (*domain.User, error)
	AdminTokenVersion(ctx context.Context, id int64) (int, error)
	UpgradeAdminPassword(ctx context.Context, id int64, hash string) error

	ListFarms(ctx context.Context) ([]domain.Farm, error)
	CreateFarm(ctx context.Context, ownerID *int64, name, location string) (domain.Farm, error)
	UpdateFarm(ctx context.Context, id int64, name, location string) error
	DeleteFarm(ctx context.Context, id int64) error
	SetFarmOwner(ctx context.Context, id int64, ownerID *int64) error
	SearchUsers(ctx context.Context, query string, limit, offset int) ([]domain.User, error)

	ListPonds(ctx context.Context) ([]domain.Pond, error)
	CreatePond(ctx context.Context, farmID int64, name string, areaMu float64) (domain.Pond, error)
	UpdatePond(ctx context.Context, id int64, name string, areaMu float64) error
	DeletePond(ctx context.Context, id int64) error // ErrPondHasDevices if devices bound

	RegisterDevice(ctx context.Context, pondID int64, name, model string, reportInterval int) (dev domain.Device, secret string, err error)
	ListDevices(ctx context.Context, includeDisabled bool, pondID int64, limit, offset int) ([]domain.Device, error)
	GetDevice(ctx context.Context, deviceNo string) (domain.Device, error)
	MoveDevice(ctx context.Context, deviceNo string, pondID int64) error
	DeleteDevice(ctx context.Context, deviceNo string) error

	ListRules(ctx context.Context) ([]domain.AlarmRule, error)
	CreateRule(ctx context.Context, rule domain.AlarmRule) (domain.AlarmRule, error)
	UpdateRule(ctx context.Context, rule domain.AlarmRule) error
	DeleteRule(ctx context.Context, id int64) error

	FindAdminByID(ctx context.Context, id int64) (*domain.User, error)
	ChangeAdminPassword(ctx context.Context, id int64, oldPassword, newPassword string) error

	ListAllAlarms(ctx context.Context, limit int) ([]domain.Alarm, error)
	ConfirmAlarm(ctx context.Context, id int64) error
	ConfirmAlarmByActor(ctx context.Context, id, actorID int64) error
	BatchConfirm(ctx context.Context, ids []int64) (int64, error)

	Stats(ctx context.Context) (domain.Stats, error)
}

type TenantAdminStore interface {
	ListTenants(context.Context) ([]domain.Tenant, error)
	SetTenantActive(context.Context, int64, bool, int64) error
	ListTenantMembers(context.Context, int64) ([]domain.TenantMembership, error)
	SetTenantMember(context.Context, int64, int64, string, bool, *time.Time, int64) error
}

type FarmMembershipStore interface {
	ListFarmMembers(context.Context, int64) ([]domain.FarmMembership, error)
	SetFarmMember(context.Context, int64, int64, string, bool, *time.Time, int64) error
}

type adminTenantStore interface {
	DefaultTenantForUser(context.Context, int64) (int64, error)
	TenantMembershipVersion(context.Context, int64, int64) (int64, error)
	TenantRole(context.Context, int64, int64) (string, error)
}

// Deps wires the store plus optional repos for pond enrichment
// (both implemented by core).
type Deps struct {
	Logger    *slog.Logger
	Store     AdminStore           // required
	Telemetry domain.TelemetryRepo // optional; enables pond latest readings
	Catalog   ProductCatalog
	Policy    domain.PermissionPolicy
}

type ProductCatalog interface {
	DefaultTenantID(context.Context) (int64, error)
	CreateProduct(context.Context, int64, string) (domain.Product, error)
	ListProducts(context.Context, int64) ([]domain.Product, error)
	CreateProductModel(context.Context, int64, int64, int, []domain.ModelField) (domain.ProductModel, error)
	ListProductModels(context.Context, int64, int64) ([]domain.ProductModel, error)
	PublishProductModel(context.Context, int64, int64, int) error
	GetProductModel(context.Context, int64, int64, int) (domain.ProductModel, error)
	AssignDeviceProduct(context.Context, int64, string, int64, int) error
	AssignDeviceProductByActor(context.Context, int64, string, int64, int, int64) error
	DeviceAssignment(context.Context, int64, string) (domain.ProductModel, error)
}

// Server is the adminapi HTTP server.
type Server struct {
	cfg  Config
	deps Deps
}

func New(cfg Config, deps Deps) *Server {
	return &Server{cfg: cfg, deps: deps}
}

// Routes builds the gin engine with all /admin/v1 routes.
func (s *Server) Routes() http.Handler {
	r := gin.New()
	r.Use(operations.RequestLogging(s.deps.Logger))

	v1 := r.Group("/admin/v1")
	v1.POST("/login", s.login)

	auth := v1.Group("", s.authRequired)
	{
		auth.GET("/tenants", s.listTenants)
		auth.PUT("/tenants/:id/status", s.setTenantStatus)
		auth.GET("/tenants/:id/members", s.listTenantMembers)
		auth.PUT("/tenants/:id/members/:user_id", s.setTenantMember)
		auth.POST("/password", s.changePassword)
	}
	tenantAuth := v1.Group("", s.authRequired, s.tenantRequired)
	{
		tenantAuth.GET("/farms", s.listFarms)
		tenantAuth.POST("/farms", s.createFarm)
		tenantAuth.GET("/users", s.listUsers)
		tenantAuth.PUT("/farms/:id", s.updateFarm)
		tenantAuth.DELETE("/farms/:id", s.deleteFarm)
		tenantAuth.PUT("/farms/:id/owner", s.setFarmOwner)
		tenantAuth.GET("/farms/:id/members", s.listFarmMembers)
		tenantAuth.PUT("/farms/:id/members/:user_id", s.setFarmMember)

		tenantAuth.GET("/ponds", s.listPonds)
		tenantAuth.POST("/ponds", s.createPond)
		tenantAuth.PUT("/ponds/:id", s.updatePond)
		tenantAuth.DELETE("/ponds/:id", s.deletePond)

		tenantAuth.GET("/devices", s.listDevices)
		tenantAuth.POST("/devices", s.registerDevice)
		tenantAuth.GET("/devices/:device_no", s.getDevice)
		tenantAuth.DELETE("/devices/:device_no", s.deleteDevice)
		tenantAuth.PUT("/devices/:device_no/pond", s.moveDevice)

		tenantAuth.GET("/alarm-rules", s.listRules)
		tenantAuth.POST("/alarm-rules", s.createRule)
		tenantAuth.PUT("/alarm-rules/:id", s.updateRule)
		tenantAuth.DELETE("/alarm-rules/:id", s.deleteRule)

		tenantAuth.GET("/alarms", s.listAlarms)
		tenantAuth.POST("/alarms/:id/confirm", s.confirmAlarm)
		tenantAuth.POST("/alarms/batch-confirm", s.batchConfirm)

		tenantAuth.GET("/stats", s.stats)
		tenantAuth.GET("/products", s.listProducts)
		tenantAuth.POST("/products", s.createProduct)
		tenantAuth.GET("/products/:product_id/models", s.listProductModels)
		tenantAuth.POST("/products/:product_id/models", s.createProductModel)
		tenantAuth.POST("/products/:product_id/models/:version/publish", s.publishProductModel)
		tenantAuth.GET("/devices/:device_no/product", s.getDeviceProduct)
		tenantAuth.PUT("/devices/:device_no/product", s.assignDeviceProduct)
	}
	return r
}

// ---- auth ----

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
		} else if tenantErr != nil && u.Authority != "ADMIN" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "tenant membership required"})
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
	c.JSON(http.StatusOK, gin.H{"token": token, "expires_in": int(s.cfg.JWT.Seconds())})
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
			if !platformAdmin {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "tenant context required"})
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
