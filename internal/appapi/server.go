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
	return r
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
	token, err := s.signToken(u.ID)
	if err != nil {
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

func (s *Server) signToken(userID int64) (string, error) {
	claims := jwt.MapClaims{"uid": userID, "exp": time.Now().Add(s.cfg.JWT).Unix()}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.SecretKey))
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
		return []byte(s.cfg.SecretKey), nil
	})
	if err != nil || !tok.Valid {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	uid, ok := tok.Claims.(jwt.MapClaims)["uid"].(float64)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "bad claims"})
		return
	}
	c.Set("uid", int64(uid))
	c.Next()
}

func uid(c *gin.Context) int64 { return c.MustGet("uid").(int64) }
