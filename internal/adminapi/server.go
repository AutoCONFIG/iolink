// Package adminapi exposes /admin/v1 for the management web console.
// Same rules as appapi: depends only on interfaces defined here (implemented
// by core), never imports core/access, never touches SQL directly.
package adminapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
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

	ListFarms(ctx context.Context) ([]domain.Farm, error)
	CreateFarm(ctx context.Context, ownerID int64, name, location string) (domain.Farm, error)
	UpdateFarm(ctx context.Context, id int64, name, location string) error
	DeleteFarm(ctx context.Context, id int64) error

	ListPonds(ctx context.Context) ([]domain.Pond, error)
	CreatePond(ctx context.Context, farmID int64, name string, areaMu float64) (domain.Pond, error)
	UpdatePond(ctx context.Context, id int64, name string, areaMu float64) error
	DeletePond(ctx context.Context, id int64) error // ErrPondHasDevices if devices bound

	RegisterDevice(ctx context.Context, pondID int64, model string) (dev domain.Device, secret string, err error)
	ListDevices(ctx context.Context) ([]domain.Device, error)
	DeleteDevice(ctx context.Context, deviceNo string) error

	ListRules(ctx context.Context) ([]domain.AlarmRule, error)
	CreateRule(ctx context.Context, rule domain.AlarmRule) (domain.AlarmRule, error)
	UpdateRule(ctx context.Context, rule domain.AlarmRule) error
	DeleteRule(ctx context.Context, id int64) error

	ListAllAlarms(ctx context.Context, limit int) ([]domain.Alarm, error)
	ConfirmAlarm(ctx context.Context, id int64) error
	BatchConfirm(ctx context.Context, ids []int64) (int64, error)

	Stats(ctx context.Context) (domain.Stats, error)
}

// Deps wires the store plus optional repos for pond enrichment
// (both implemented by core).
type Deps struct {
	Store     AdminStore           // required
	Telemetry domain.TelemetryRepo // optional; enables pond latest readings
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
	r.Use(gin.Recovery())

	v1 := r.Group("/admin/v1")
	v1.POST("/login", s.login)

	auth := v1.Group("", s.authRequired)
	{
		auth.GET("/farms", s.listFarms)
		auth.POST("/farms", s.createFarm)
		auth.PUT("/farms/:id", s.updateFarm)
		auth.DELETE("/farms/:id", s.deleteFarm)

		auth.GET("/ponds", s.listPonds)
		auth.POST("/ponds", s.createPond)
		auth.PUT("/ponds/:id", s.updatePond)
		auth.DELETE("/ponds/:id", s.deletePond)

		auth.GET("/devices", s.listDevices)
		auth.POST("/devices", s.registerDevice)
		auth.DELETE("/devices/:device_no", s.deleteDevice)

		auth.GET("/alarm-rules", s.listRules)
		auth.POST("/alarm-rules", s.createRule)
		auth.PUT("/alarm-rules/:id", s.updateRule)
		auth.DELETE("/alarm-rules/:id", s.deleteRule)

		auth.GET("/alarms", s.listAlarms)
		auth.POST("/alarms/:id/confirm", s.confirmAlarm)
		auth.POST("/alarms/batch-confirm", s.batchConfirm)

		auth.GET("/stats", s.stats)
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
	key := platform.DeriveAdminKey(s.cfg.SecretKey)
	claims := jwt.MapClaims{"aid": u.ID, "exp": time.Now().Add(s.cfg.JWT).Unix()}
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
	aid, ok := tok.Claims.(jwt.MapClaims)["aid"].(float64)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "bad claims"})
		return
	}
	c.Set("aid", int64(aid))
	c.Next()
}

func aid(c *gin.Context) int64 { return c.MustGet("aid").(int64) }
