// Package adminapi exposes /admin/v1 for the management web console.
// Same rules as appapi: depends only on interfaces defined here (implemented
// by core), never imports core/access, never touches SQL directly.
package adminapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/license"
	"git.hyhy.fun/rsplab/iolink/internal/operations"
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
	RestoreDevice(ctx context.Context, deviceNo string) error

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

type TenantCreateStore interface {
	CreateTenant(context.Context, string, int64) (domain.Tenant, error)
}

type PlatformStatsStore interface {
	PlatformStats(context.Context) (domain.PlatformStats, error)
}

type LicenseStore interface {
	LicenseStatus(context.Context) (license.Status, error)
	ImportLicenseRaw(context.Context, []byte, license.Envelope, int64) error
	RecordLicenseRejection(context.Context, []byte, int64, string) error
	RecordLicenseRejectionDigest(context.Context, string, int64, string) error
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
	APIKeys   APIKeyAdminStore
}

type APIKeyAdminStore interface {
	IssueAPIKey(context.Context, int64, string, []string, domain.APIKeyResourceScope, int64) (domain.APIKey, string, error)
	ListAPIKeys(context.Context, int64, int64) ([]domain.APIKey, error)
	RotateAPIKey(context.Context, int64, int64, string) (domain.APIKey, string, error)
	RevokeAPIKey(context.Context, int64, int64, string) error
	ListAPIKeyAuditEvents(context.Context, int64, int64, int) ([]domain.APIKeyAuditEvent, error)
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

func (s *Server) Routes() http.Handler {
	r := gin.New()
	r.Use(operations.RequestLogging(s.deps.Logger))

	v1 := r.Group("/admin/v1")
	s.mountAuthRoutes(v1, true)
	s.mountTenantRoutes(v1, false)
	return r
}

func (s *Server) UserRoutes() http.Handler {
	r := gin.New()
	r.Use(operations.RequestLogging(s.deps.Logger))

	v1 := r.Group("/user/v1")
	v1.POST("/login", s.userLogin)
	s.mountAuthRoutes(v1, false)
	s.mountTenantRoutes(v1, true)
	return r
}

func (s *Server) mountAuthRoutes(v1 *gin.RouterGroup, platform bool) {
	if platform {
		v1.POST("/login", s.login)
	}
	v1.POST("/register", s.register)

	auth := v1.Group("", s.authRequired)
	{
		organizations := auth
		if !platform {
			organizations = auth.Group("", s.userAccountRequired)
		}
		organizations.GET("/session", s.session)
		organizations.POST("/password", s.changePassword)
		organizations.GET("/tenants", s.listTenants)
		organizations.GET("/tenants/:id/members", s.listTenantMembers)
		organizations.PUT("/tenants/:id/members/:user_id", s.setTenantMember)
		if platform {
			auth.GET("/license", s.getLicense)
			auth.POST("/license", s.importLicense)
			auth.POST("/tenants", s.createTenant)
			auth.GET("/platform/stats", s.platformStats)
			auth.GET("/platform/users", s.platformUsers)
			auth.PUT("/tenants/:id/status", s.setTenantStatus)
		}
	}
}

func (s *Server) mountTenantRoutes(v1 *gin.RouterGroup, userOnly bool) {
	middleware := []gin.HandlerFunc{s.authRequired}
	if userOnly {
		middleware = append(middleware, s.userAccountRequired)
	}
	middleware = append(middleware, s.tenantRequired)
	tenantAuth := v1.Group("", middleware...)
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
		tenantAuth.POST("/devices/:device_no/restore", s.restoreDevice)
		tenantAuth.PUT("/devices/:device_no/pond", s.moveDevice)

		tenantAuth.GET("/alarm-rules", s.listRules)
		tenantAuth.POST("/alarm-rules", s.createRule)
		tenantAuth.PUT("/alarm-rules/:id", s.updateRule)
		tenantAuth.DELETE("/alarm-rules/:id", s.deleteRule)

		tenantAuth.GET("/alarms", s.listAlarms)
		tenantAuth.POST("/alarms/:id/confirm", s.confirmAlarm)
		tenantAuth.POST("/alarms/batch-confirm", s.batchConfirm)

		tenantAuth.GET("/stats", s.stats)
		tenantAuth.GET("/api-keys", s.listAPIKeys)
		tenantAuth.GET("/api-keys/audit", s.listAPIKeyAudit)
		tenantAuth.POST("/api-keys", s.createAPIKey)
		tenantAuth.POST("/api-keys/:key_id/rotate", s.rotateAPIKey)
		tenantAuth.POST("/api-keys/:key_id/revoke", s.revokeAPIKey)
		tenantAuth.GET("/products", s.listProducts)
		tenantAuth.POST("/products", s.createProduct)
		tenantAuth.GET("/products/:product_id/models", s.listProductModels)
		tenantAuth.POST("/products/:product_id/models", s.createProductModel)
		tenantAuth.POST("/products/:product_id/models/:version/publish", s.publishProductModel)
		tenantAuth.GET("/devices/:device_no/product", s.getDeviceProduct)
		tenantAuth.PUT("/devices/:device_no/product", s.assignDeviceProduct)
	}
}
