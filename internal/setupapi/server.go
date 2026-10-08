package setupapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/operations"
	"github.com/gin-gonic/gin"
)

type Store interface {
	Required(context.Context) (bool, error)
	Initialize(context.Context, string, string) error
}

type Server struct {
	store  Store
	key    [32]byte
	logger *slog.Logger
}

func New(store Store, key string, logger *slog.Logger) *Server {
	return &Server{store: store, key: sha256.Sum256([]byte(key)), logger: logger}
}

func (s *Server) Routes() http.Handler {
	r := gin.New()
	r.Use(operations.RequestLogging(s.logger))
	r.GET("/setup/v1/status", func(c *gin.Context) { s.status(c.Writer, c.Request) })
	r.POST("/setup/v1/initialize", func(c *gin.Context) { s.initialize(c.Writer, c.Request) })
	return r
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		return
	}
}

func (s *Server) required(w http.ResponseWriter, r *http.Request) (bool, bool) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	required, err := s.store.Required(ctx)
	if err != nil {
		s.logger.Error("bootstrap state unavailable", "stage", "setup", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "初始化状态暂不可用，请重试"})
		return false, false
	}
	return required, true
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	if required, ok := s.required(w, r); ok {
		writeJSON(w, http.StatusOK, map[string]bool{"required": required})
	}
}

func (s *Server) initialize(w http.ResponseWriter, r *http.Request) {
	required, ok := s.required(w, r)
	if !ok {
		return
	}
	if !required {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "系统已初始化，请登录"})
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Host != r.Host || (u.Scheme != "http" && u.Scheme != "https") {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "请从本站初始化页面提交"})
			return
		}
	}
	key := sha256.Sum256([]byte(r.Header.Get("X-IoLink-Setup-Key")))
	if r.Header.Get("X-IoLink-Setup-Key") == "" || subtle.ConstantTimeCompare(key[:], s.key[:]) != 1 {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "安装密钥不正确"})
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "请使用 JSON 提交"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	defer r.Body.Close()
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "初始化参数无效"})
		return
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "初始化参数无效"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := s.store.Initialize(ctx, input.Username, input.Password); err != nil {
		switch {
		case errors.Is(err, domain.ErrBootstrapInput):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "账号需为 1–64 字节，密码需为 12–256 字节，均不能带首尾空白"})
		case errors.Is(err, domain.ErrBootstrapInitialized):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "系统已初始化，请登录"})
		default:
			s.logger.Error("bootstrap initialization failed", "stage", "setup", "err", err)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "初始化失败，请检查数据库状态后重试"})
		}
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		required, ok := s.required(w, r)
		if !ok {
			return
		}
		if required {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "请先完成系统初始化"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
