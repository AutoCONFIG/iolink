# Camera HTTP/composition final read-only audit
UTC 2026-10-10T18:08:45.849034+00:00

COMMAND: git status --short
COMMAND_EXIT_CODE=0

COMMAND: rg -n platform ADMIN|/api/v1/cameras|cameraContextRequired|camera_route docs/design/M7a-video.md docs/api/video-openapi.yaml internal/appapi internal/adminapi internal/camerapg
docs/api/video-openapi.yaml:369:  /api/v1/cameras:
docs/api/video-openapi.yaml:391:  /api/v1/cameras/{camera_id}:
docs/api/video-openapi.yaml:415:  /api/v1/cameras/{camera_id}/playback-sessions:
internal/appapi/cameras.go:21:	group := v1.Group("/cameras", func(c *gin.Context) { c.Set("camera_route", true); s.authRequired(c) }, s.cameraContextRequired)
internal/appapi/cameras.go:27:	if !c.GetBool("camera_route") {
internal/appapi/cameras.go:41:func (s *Server) cameraContextRequired(c *gin.Context) {
internal/adminapi/cameras.go:15:	group := v1.Group("/cameras", func(c *gin.Context) { c.Set("camera_route", true); s.authRequired(c) }, s.cameraContextRequired)
internal/adminapi/cameras.go:24:	if !c.GetBool("camera_route") {
internal/adminapi/cameras.go:38:func (s *Server) cameraContextRequired(c *gin.Context) {
COMMAND_EXIT_CODE=0

COMMAND: nl -ba internal/appapi/server.go
     1	// Package appapi exposes the /api/v1 HTTP surface for the WeChat mini
     2	// program. It depends ONLY on git.hyhy.fun/rsplab/git.hyhy.fun/rsplab/iolink/internal/domain: domain models and
     3	// repository interfaces supplied by core. It never imports iolink-core,
     4	// iolink-access, and never touches SQL or MQTT.
     5	package appapi
     6	
     7	import (
     8		"context"
     9		"errors"
    10		"log/slog"
    11		"net/http"
    12		"time"
    13	
    14		"git.hyhy.fun/rsplab/iolink/internal/operations"
    15		"git.hyhy.fun/rsplab/iolink/internal/platform"
    16		"github.com/gin-gonic/gin"
    17		"github.com/golang-jwt/jwt/v5"
    18	
    19		iolinkcontractsdomain "git.hyhy.fun/rsplab/iolink/internal/domain"
    20	)
    21	
    22	// Config for appapi.
    23	type Config struct {
    24		Addr      string        // ":8080"
    25		SecretKey string        // JWT signing key
    26		JWT       time.Duration // token lifetime
    27		Wechat    WechatConfig  // empty AppID => stub exchanger (dev)
    28	}
    29	
    30	// Deps are the repositories appapi needs. cmd/iolinkd passes core's
    31	// implementations; tests pass fakes.
    32	type Deps struct {
    33		Ponds     iolinkcontractsdomain.PondRepo
    34		Devices   iolinkcontractsdomain.DeviceRepo
    35		Telemetry iolinkcontractsdomain.TelemetryRepo
    36		Alarms    iolinkcontractsdomain.AlarmRepo
    37		// Users abstracts login (openid lookup); core implements with users table.
    38		Users   UserStore
    39		Cameras CameraReader
    40	}
    41	
    42	// UserStore is the auth-facing slice of the users domain.
    43	type UserStore interface {
    44		FindByOpenID(ctx context.Context, openID string) (*iolinkcontractsdomain.User, error)
    45		EnsureUser(ctx context.Context, openID string) (*iolinkcontractsdomain.User, error)
    46		UserTokenVersion(ctx context.Context, id int64) (int, error)
    47	}
    48	
    49	type tenantTokenStore interface {
    50		DefaultTenantForUser(ctx context.Context, userID int64) (int64, error)
    51		TenantMembershipVersion(ctx context.Context, userID, tenantID int64) (int64, error)
    52	}
    53	
    54	type tenantRoleStore interface {
    55		TenantRole(ctx context.Context, userID, tenantID int64) (string, error)
    56	}
    57	
    58	type tenantMembershipStore interface {
    59		ListUserTenants(ctx context.Context, userID int64) ([]iolinkcontractsdomain.TenantMembership, error)
    60		TenantMembershipVersion(ctx context.Context, userID, tenantID int64) (int64, error)
    61	}
    62	
    63	// Server is the appapi HTTP server.
    64	type Server struct {
    65		cfg  Config
    66		deps Deps
    67		wx   func(code string) (openID string, err error)
    68		log  *slog.Logger
    69	}
    70	
    71	func New(cfg Config, deps Deps, log *slog.Logger) *Server {
    72		s := &Server{cfg: cfg, deps: deps, wx: WechatExchanger, log: log}
    73		if cfg.Wechat.AppID != "" && cfg.Wechat.Secret != "" {
    74			s.wx = RealWechatExchanger(cfg.Wechat)
    75		}
    76		return s
    77	}
    78	
    79	// Routes builds the gin engine with all /api/v1 routes.
    80	func (s *Server) Routes() http.Handler {
    81		r := gin.New()
    82		r.Use(operations.RequestLogging(s.log))
    83	
    84		v1 := r.Group("/api/v1")
    85		v1.POST("/auth/login", s.login)
    86	
    87		auth := v1.Group("", s.authRequired)
    88		{
    89			auth.GET("/auth/tenants", s.listTenants)
    90			auth.POST("/auth/tenant", s.switchTenant)
    91			auth.GET("/ponds", s.listPonds)
    92			auth.GET("/ponds/:id", s.getPond)
    93			auth.GET("/devices", s.listDevices)
    94			auth.GET("/devices/:device_no", s.getDevice)
    95			auth.GET("/water/latest", s.waterLatest)
    96			auth.GET("/water/history", s.waterHistory)
    97			auth.GET("/alarms", s.listAlarms)
    98			auth.GET("/stats/summary", s.statsSummary)
    99			auth.POST("/alarms/:id/confirm", s.confirmAlarm)
   100		}
   101		s.mountCameraRoutes(v1)
   102		v2 := r.Group("/api/v2", s.authRequired)
   103		v2.POST("/devices/:device_no/telemetry", s.submitTelemetryV2)
   104		v2.GET("/devices/:device_no/model/latest", s.modelLatestV2)
   105		v2.GET("/devices/:device_no/history", s.telemetryHistoryV2)
   106		return r
   107	}
   108	
   109	type tenantSwitchRequest struct {
   110		TenantID int64 `json:"tenant_id" binding:"required"`
   111	}
   112	
   113	func (s *Server) listTenants(c *gin.Context) {
   114		store, ok := s.deps.Users.(tenantMembershipStore)
   115		if !ok {
   116			c.JSON(http.StatusInternalServerError, gin.H{"error": "tenant membership unavailable"})
   117			return
   118		}
   119		tenants, err := store.ListUserTenants(c.Request.Context(), uid(c))
   120		if err != nil {
   121			c.JSON(http.StatusInternalServerError, gin.H{"error": "tenant query failed"})
   122			return
   123		}
   124		c.JSON(http.StatusOK, tenants)
   125	}
   126	
   127	func (s *Server) switchTenant(c *gin.Context) {
   128		var req tenantSwitchRequest
   129		if err := c.ShouldBindJSON(&req); err != nil || req.TenantID < 1 {
   130			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
   131			return
   132		}
   133		store, ok := s.deps.Users.(tenantMembershipStore)
   134		if !ok {
   135			c.JSON(http.StatusInternalServerError, gin.H{"error": "tenant membership unavailable"})
   136			return
   137		}
   138		tenants, err := store.ListUserTenants(c.Request.Context(), uid(c))
   139		if err != nil {
   140			c.JSON(http.StatusInternalServerError, gin.H{"error": "tenant query failed"})
   141			return
   142		}
   143		found := false
   144		for _, tenant := range tenants {
   145			if tenant.TenantID == req.TenantID {
   146				found = true
   147				break
   148			}
   149		}
   150		if !found {
   151			c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
   152			return
   153		}
   154		token, err := s.signTokenForTenant(c.Request.Context(), uid(c), req.TenantID)
   155		if err != nil {
   156			if errors.Is(err, iolinkcontractsdomain.ErrInactiveTenant) {
   157				c.JSON(http.StatusUnauthorized, gin.H{"error": "tenant membership inactive"})
   158				return
   159			}
   160			c.JSON(http.StatusInternalServerError, gin.H{"error": "token issue failed"})
   161			return
   162		}
   163		c.JSON(http.StatusOK, loginResp{Token: token, ExpiresIn: int(s.cfg.JWT.Seconds())})
   164	}
   165	
   166	// Run blocks serving until the http server returns.
   167	func (s *Server) Run() error {
   168		srv := &http.Server{Addr: s.cfg.Addr, Handler: s.Routes(), ReadHeaderTimeout: 5 * time.Second}
   169		s.log.Info("appapi listening", "addr", s.cfg.Addr)
   170		return srv.ListenAndServe()
   171	}
   172	
   173	// ---- auth ----
   174	
   175	type loginReq struct {
   176		Code string `json:"code" binding:"required"`
   177	}
   178	
   179	type loginResp struct {
   180		Token     string                      `json:"token"`
   181		ExpiresIn int                         `json:"expires_in"`
   182		User      *iolinkcontractsdomain.User `json:"user"`
   183	}
   184	
   185	var errWechatCodeInvalid = errors.New("wechat code invalid")
   186	
   187	// login exchanges a wx.login code for a JWT. The WeChat code2session call is
   188	// injected via WechatExchanger so it can be faked in tests / swapped later.
   189	func (s *Server) login(c *gin.Context) {
   190		var req loginReq
   191		if err := c.ShouldBindJSON(&req); err != nil {
   192			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
   193			return
   194		}
   195		openID, err := s.wx(req.Code)
   196		if err != nil {
   197			c.JSON(http.StatusUnauthorized, gin.H{"error": "wechat login failed"})
   198			return
   199		}
   200		u, err := s.deps.Users.EnsureUser(c.Request.Context(), openID)
   201		if err != nil {
   202			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
   203			return
   204		}
   205		token, err := s.signToken(c.Request.Context(), u.ID)
   206		if err != nil {
   207			if errors.Is(err, iolinkcontractsdomain.ErrInactiveTenant) {
   208				c.JSON(http.StatusUnauthorized, gin.H{"error": "tenant membership inactive"})
   209				return
   210			}
   211			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
   212			return
   213		}
   214		c.JSON(http.StatusOK, loginResp{Token: token, ExpiresIn: int(s.cfg.JWT.Seconds()), User: u})
   215	}
   216	
   217	// WechatExchanger is the dev stub (accepts nothing). Overridden by real
   218	// code2session when Config.Wechat is set, or via SetWechatExchanger in tests.
   219	var WechatExchanger = func(code string) (openID string, err error) {
   220		return "", errWechatCodeInvalid
   221	}
   222	
   223	func (s *Server) signToken(ctx context.Context, userID int64) (string, error) {
   224		tenants, ok := s.deps.Users.(tenantTokenStore)
   225		if !ok {
   226			return "", errors.New("tenant store unavailable")
   227		}
   228		tenantID, err := tenants.DefaultTenantForUser(ctx, userID)
   229		if err != nil {
   230			if errors.Is(err, iolinkcontractsdomain.ErrNotFound) {
   231				return "", iolinkcontractsdomain.ErrInactiveTenant
   232			}
   233			return "", err
   234		}
   235		if tenantID <= 0 {
   236			return "", iolinkcontractsdomain.ErrInactiveTenant
   237		}
   238		return s.signTokenForTenant(ctx, userID, tenantID)
   239	}
   240	
   241	func (s *Server) signTokenForTenant(ctx context.Context, userID, tenantID int64) (string, error) {
   242		if s.deps.Users == nil {
   243			return "", errors.New("user store unavailable")
   244		}
   245		version, err := s.deps.Users.UserTokenVersion(ctx, userID)
   246		if err != nil {
   247			return "", err
   248		}
   249		claims := jwt.MapClaims{"uid": userID, "ver": version, "exp": time.Now().Add(s.cfg.JWT).Unix()}
   250		if tenantID > 0 {
   251			tenants, ok := s.deps.Users.(tenantTokenStore)
   252			if !ok {
   253				return "", errors.New("tenant store unavailable")
   254			}
   255			membershipVersion, err := tenants.TenantMembershipVersion(ctx, userID, tenantID)
   256			if err != nil {
   257				return "", err
   258			}
   259			claims["tenant_id"] = tenantID
   260			claims["tenant_ver"] = membershipVersion
   261			roles, ok := s.deps.Users.(tenantRoleStore)
   262			if !ok {
   263				return "", errors.New("tenant role store unavailable")
   264			}
   265			role, roleErr := roles.TenantRole(ctx, userID, tenantID)
   266			if roleErr != nil {
   267				return "", roleErr
   268			}
   269			claims["tenant_role"] = role
   270		}
   271		return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(platform.DeriveAppKey(s.cfg.SecretKey))
   272	}
   273	
   274	func (s *Server) authRequired(c *gin.Context) {
   275		h := c.GetHeader("Authorization")
   276		const prefix = "Bearer "
   277		if len(h) <= len(prefix) || h[:len(prefix)] != prefix {
   278			abortCameraAuth(c, http.StatusUnauthorized, "missing bearer token")
   279			return
   280		}
   281		tok, err := jwt.Parse(h[len(prefix):], func(t *jwt.Token) (any, error) {
   282			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
   283				return nil, errors.New("bad signing method")
   284			}
   285			return platform.DeriveAppKey(s.cfg.SecretKey), nil
   286		})
   287		if err != nil || !tok.Valid {
   288			abortCameraAuth(c, http.StatusUnauthorized, "invalid token")
   289			return
   290		}
   291		claims := tok.Claims.(jwt.MapClaims)
   292		if _, ok := claims["exp"].(float64); !ok {
   293			abortCameraAuth(c, http.StatusUnauthorized, "missing exp")
   294			return
   295		}
   296		uid, ok := claims["uid"].(float64)
   297		if !ok {
   298			abortCameraAuth(c, http.StatusUnauthorized, "bad claims")
   299			return
   300		}
   301		c.Set("uid", int64(uid))
   302		version, ok := claims["ver"].(float64)
   303		if !ok || s.deps.Users == nil {
   304			abortCameraAuth(c, http.StatusUnauthorized, "bad claims")
   305			return
   306		}
   307		current, err := s.deps.Users.UserTokenVersion(c.Request.Context(), int64(uid))
   308		if err != nil || int64(version) != int64(current) {
   309			abortCameraAuth(c, http.StatusUnauthorized, "token revoked")
   310			return
   311		}
   312		tenants, tenantStoreOK := s.deps.Users.(tenantTokenStore)
   313		if !tenantStoreOK {
   314			abortCameraAuth(c, http.StatusUnauthorized, "tenant store unavailable")
   315			return
   316		}
   317		{
   318			tenantRaw, tenantOK := claims["tenant_id"].(float64)
   319			membershipRaw, membershipOK := claims["tenant_ver"].(float64)
   320			if !tenantOK && !membershipOK {
   321				abortCameraAuth(c, http.StatusUnauthorized, "tenant context required")
   322				return
   323			}
   324			if !tenantOK || !membershipOK {
   325				abortCameraAuth(c, http.StatusUnauthorized, "tenant context required")
   326				return
   327			}
   328			membershipVersion, err := tenants.TenantMembershipVersion(c.Request.Context(), int64(uid), int64(tenantRaw))
   329			if err != nil || int64(membershipRaw) != membershipVersion {
   330				abortCameraAuth(c, http.StatusUnauthorized, "tenant membership revoked")
   331				return
   332			}
   333			roles, roleStoreOK := s.deps.Users.(tenantRoleStore)
   334			if !roleStoreOK {
   335				abortCameraAuth(c, http.StatusUnauthorized, "tenant role unavailable")
   336				return
   337			}
   338			role, roleErr := roles.TenantRole(c.Request.Context(), int64(uid), int64(tenantRaw))
   339			claimRole, roleOK := claims["tenant_role"].(string)
   340			if roleErr != nil || !roleOK || claimRole != role {
   341				abortCameraAuth(c, http.StatusUnauthorized, "tenant role invalid")
   342				return
   343			}
   344			c.Set("tenant_id", int64(tenantRaw))
   345			requestContext := iolinkcontractsdomain.WithTenantID(c.Request.Context(), int64(tenantRaw))
   346			requestContext = iolinkcontractsdomain.WithTenantRole(requestContext, role)
   347			requestContext = iolinkcontractsdomain.WithTenantUserID(requestContext, int64(uid))
   348			requestContext = iolinkcontractsdomain.WithTenantPermissionVersion(requestContext, int64(membershipRaw))
   349			c.Request = c.Request.WithContext(requestContext)
   350		}
   351		c.Next()
   352	}
   353	
   354	func uid(c *gin.Context) int64 { return c.MustGet("uid").(int64) }
COMMAND_EXIT_CODE=0

COMMAND: nl -ba internal/camerapg/authorization.go
     1	package camerapg
     2	
     3	import (
     4		"context"
     5		"errors"
     6	
     7		"git.hyhy.fun/rsplab/iolink/internal/domain"
     8		"github.com/jackc/pgx/v5"
     9	)
    10	
    11	func (t *transaction) Authorize(ctx context.Context, write bool) error {
    12		var tenantOK, actorOK bool
    13		t.tenantID, tenantOK = domain.TenantID(ctx)
    14		t.actorID, actorOK = domain.TenantUserID(ctx)
    15		if !tenantOK || !actorOK || domain.TenantRole(ctx) == "" {
    16			return domain.ErrForbidden
    17		}
    18		var active bool
    19		err := t.tx.QueryRow(ctx, `SELECT active FROM tenants WHERE id=$1 FOR SHARE`, t.tenantID).Scan(&active)
    20		if errors.Is(err, pgx.ErrNoRows) || err == nil && !active {
    21			return domain.ErrForbidden
    22		}
    23		if err != nil {
    24			return err
    25		}
    26		err = t.tx.QueryRow(ctx, `SELECT authority FROM users WHERE id=$1 FOR SHARE`, t.actorID).Scan(&t.authority)
    27		if errors.Is(err, pgx.ErrNoRows) {
    28			return domain.ErrForbidden
    29		}
    30		if err != nil {
    31			return err
    32		}
    33		var version int64
    34		err = t.tx.QueryRow(ctx, `SELECT role,permission_version FROM tenant_memberships WHERE tenant_id=$1 AND user_id=$2 AND active
    35	 AND (expires_at IS NULL OR expires_at>clock_timestamp()) AND ($3='USER' OR ($3='ADMIN' AND role='support' AND expires_at IS NOT NULL)) FOR SHARE`, t.tenantID, t.actorID, t.authority).Scan(&t.role, &version)
    36		if errors.Is(err, pgx.ErrNoRows) {
    37			return domain.ErrForbidden
    38		}
    39		if err != nil {
    40			return err
    41		}
    42		if t.role != domain.TenantRole(ctx) {
    43			return domain.ErrForbidden
    44		}
    45		if claimed, present := domain.TenantPermissionVersion(ctx); present && claimed != version {
    46			return domain.ErrForbidden
    47		}
    48		switch t.role {
    49		case "owner", "admin":
    50			if t.authority != "USER" {
    51				return domain.ErrForbidden
    52			}
    53		case "member", "viewer", "support":
    54			if write {
    55				return domain.ErrForbidden
    56			}
    57		default:
    58			return domain.ErrForbidden
    59		}
    60		return nil
    61	}
    62	
    63	// farmScope is always evaluated against live membership rows. Values are bound
    64	// parameters, never caller-provided SQL or cached permission decisions.
    65	const farmScope = `($3 IN ('owner','admin') AND $4='USER' OR f.owner_id=$2 AND $4='USER' OR EXISTS (
    66	 SELECT 1 FROM farm_memberships fm WHERE fm.farm_id=f.id AND fm.tenant_id=f.tenant_id AND fm.user_id=$2
    67	 AND fm.active AND (fm.expires_at IS NULL OR fm.expires_at>clock_timestamp())
    68	 AND ($4='USER' OR (fm.role='support' AND fm.expires_at IS NOT NULL))))`
    69	
    70	func (t *transaction) lockFarm(ctx context.Context, id int64) error {
    71		var farmID int64
    72		err := t.tx.QueryRow(ctx, `SELECT f.id FROM farms f WHERE f.tenant_id=$1 AND f.id=$5 AND `+farmScope+` FOR SHARE OF f`, t.tenantID, t.actorID, t.role, t.authority, id).Scan(&farmID)
    73		if err != nil {
    74			return err
    75		}
    76		// Hold a present farm grant against concurrent revocation. Managers and owners
    77		// have their own live row locks and do not require an explicit grant.
    78		rows, err := t.tx.Query(ctx, `SELECT user_id FROM farm_memberships WHERE tenant_id=$1 AND farm_id=$2 AND user_id=$3 FOR SHARE`, t.tenantID, id, t.actorID)
    79		if err != nil {
    80			return err
    81		}
    82		defer rows.Close()
    83		for rows.Next() {
    84			var userID int64
    85			if err = rows.Scan(&userID); err != nil {
    86				return err
    87			}
    88		}
    89		if err := rows.Err(); err != nil {
    90			return err
    91		}
    92		return nil
    93	}
COMMAND_EXIT_CODE=0

COMMAND: nl -ba internal/adminapi/cameras.go
     1	package adminapi
     2	
     3	import (
     4		"errors"
     5		"net/http"
     6		"net/url"
     7		"strconv"
     8	
     9		"git.hyhy.fun/rsplab/iolink/internal/camera"
    10		"git.hyhy.fun/rsplab/iolink/internal/domain"
    11		"github.com/gin-gonic/gin"
    12	)
    13	
    14	func (s *Server) mountCameraRoutes(v1 *gin.RouterGroup) {
    15		group := v1.Group("/cameras", func(c *gin.Context) { c.Set("camera_route", true); s.authRequired(c) }, s.cameraContextRequired)
    16		group.GET("", s.listCameras)
    17		group.POST("", s.createCamera)
    18		group.GET("/:camera_id", s.getCamera)
    19		group.PUT("/:camera_id", s.replaceCamera)
    20		group.DELETE("/:camera_id", s.disableCamera)
    21	}
    22	
    23	func abortCameraAuth(c *gin.Context, status int, legacy string) {
    24		if !c.GetBool("camera_route") {
    25			c.AbortWithStatusJSON(status, gin.H{"error": legacy})
    26			return
    27		}
    28		if legacy == "tenant context required" {
    29			status = http.StatusForbidden
    30		}
    31		code := "unauthorized"
    32		if status == http.StatusForbidden {
    33			code = "forbidden"
    34		}
    35		c.AbortWithStatusJSON(status, gin.H{"code": code, "message": code})
    36	}
    37	
    38	func (s *Server) cameraContextRequired(c *gin.Context) {
    39		_, scoped := domain.TenantID(c.Request.Context())
    40		if c.GetBool("platform_admin") || !scoped {
    41			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": "forbidden"})
    42			return
    43		}
    44		c.Next()
    45	}
    46	
    47	func cameraError(c *gin.Context, err error) {
    48		status, code := http.StatusInternalServerError, "internal_error"
    49		switch {
    50		case errors.Is(err, camera.ErrInvalid):
    51			status, code = http.StatusBadRequest, "invalid_request"
    52		case errors.Is(err, domain.ErrForbidden):
    53			status, code = http.StatusForbidden, "forbidden"
    54		case errors.Is(err, domain.ErrNotFound):
    55			status, code = http.StatusNotFound, "not_found"
    56		case errors.Is(err, camera.ErrUnavailable):
    57			status, code = http.StatusServiceUnavailable, "unavailable"
    58		case errors.Is(err, domain.ErrConflict):
    59			status, code = http.StatusConflict, "conflict"
    60		}
    61		c.JSON(status, gin.H{"code": code, "message": code})
    62	}
    63	
    64	func cameraID(c *gin.Context) (int64, error) {
    65		id, err := strconv.ParseInt(c.Param("camera_id"), 10, 64)
    66		if err != nil || id < 1 {
    67			return 0, camera.ErrInvalid
    68		}
    69		return id, nil
    70	}
    71	
    72	func cameraPage(c *gin.Context, list bool) (camera.Page, error) {
    73		values, err := url.ParseQuery(c.Request.URL.RawQuery)
    74		if err != nil {
    75			return camera.Page{}, camera.ErrInvalid
    76		}
    77		page := camera.Page{Limit: 50}
    78		for key, entries := range values {
    79			if !list || len(entries) != 1 {
    80				return camera.Page{}, camera.ErrInvalid
    81			}
    82			switch key {
    83			case "limit":
    84				value, err := strconv.Atoi(entries[0])
    85				if err != nil || value < 1 || value > 100 {
    86					return camera.Page{}, camera.ErrInvalid
    87				}
    88				page.Limit = value
    89			case "after_id":
    90				value, err := strconv.ParseInt(entries[0], 10, 64)
    91				if err != nil || value < 1 {
    92					return camera.Page{}, camera.ErrInvalid
    93				}
    94				page.AfterID = value
    95			default:
    96				return camera.Page{}, camera.ErrInvalid
    97			}
    98		}
    99		return page, nil
   100	}
   101	
   102	func (s *Server) listCameras(c *gin.Context) {
   103		page, err := cameraPage(c, true)
   104		if err != nil {
   105			cameraError(c, err)
   106			return
   107		}
   108		if s.deps.Cameras == nil {
   109			cameraError(c, camera.ErrUnavailable)
   110			return
   111		}
   112		result, err := s.deps.Cameras.List(c.Request.Context(), page)
   113		if err != nil {
   114			cameraError(c, err)
   115			return
   116		}
   117		if result.Items == nil {
   118			result.Items = []camera.Camera{}
   119		}
   120		c.JSON(http.StatusOK, result)
   121	}
   122	func (s *Server) getCamera(c *gin.Context) {
   123		id, err := cameraID(c)
   124		if err != nil {
   125			cameraError(c, err)
   126			return
   127		}
   128		if _, err = cameraPage(c, false); err != nil {
   129			cameraError(c, err)
   130			return
   131		}
   132		if s.deps.Cameras == nil {
   133			cameraError(c, camera.ErrUnavailable)
   134			return
   135		}
   136		result, err := s.deps.Cameras.Get(c.Request.Context(), id)
   137		if err != nil {
   138			cameraError(c, err)
   139			return
   140		}
   141		c.JSON(http.StatusOK, result)
   142	}
   143	func (s *Server) createCamera(c *gin.Context)  { s.writeCamera(c, false) }
   144	func (s *Server) replaceCamera(c *gin.Context) { s.writeCamera(c, true) }
   145	func (s *Server) writeCamera(c *gin.Context, replace bool) {
   146		var id int64
   147		var err error
   148		if replace {
   149			id, err = cameraID(c)
   150			if err != nil {
   151				cameraError(c, err)
   152				return
   153			}
   154		}
   155		if _, err = cameraPage(c, false); err != nil {
   156			cameraError(c, err)
   157			return
   158		}
   159		cfg, err := parseCameraBody(c)
   160		if err != nil {
   161			cameraError(c, err)
   162			return
   163		}
   164		if s.deps.Cameras == nil {
   165			cameraError(c, camera.ErrUnavailable)
   166			return
   167		}
   168		var result camera.Camera
   169		status := http.StatusCreated
   170		if replace {
   171			result, err = s.deps.Cameras.Replace(c.Request.Context(), id, cfg)
   172			status = http.StatusOK
   173		} else {
   174			result, err = s.deps.Cameras.Create(c.Request.Context(), cfg)
   175		}
   176		if err != nil {
   177			cameraError(c, err)
   178			return
   179		}
   180		c.JSON(status, result)
   181	}
   182	func (s *Server) disableCamera(c *gin.Context) {
   183		id, err := cameraID(c)
   184		if err != nil {
   185			cameraError(c, err)
   186			return
   187		}
   188		if _, err = cameraPage(c, false); err != nil {
   189			cameraError(c, err)
   190			return
   191		}
   192		if s.deps.Cameras == nil {
   193			cameraError(c, camera.ErrUnavailable)
   194			return
   195		}
   196		if err = s.deps.Cameras.Disable(c.Request.Context(), id); err != nil {
   197			cameraError(c, err)
   198			return
   199		}
   200		c.Status(http.StatusNoContent)
   201	}
COMMAND_EXIT_CODE=0

COMMAND: rg -n --- PASS: TestCameraHTTP|--- FAIL:|--- SKIP: /media/yun/706bc403-c76c-4fdd-8a3f-d954b6189048/iolink/.omo/evidence/camera-http-final-20261011.log
rg: unrecognized flag --- PASS: TestCameraHTTP|--- FAIL:|--- SKIP:
COMMAND_EXIT_CODE=2

## Findings

**BLOCKER F1 — mini camera surface does not reject platform ADMIN explicitly.** The frozen design requires “平台 ADMIN 无摄像机详情/播放权限” and “平台 ADMIN 到 user surface 为 403”. `internal/appapi/auth.go` authenticates the mini JWT and injects tenant/user/role context but never loads or rejects the actor authority. `internal/appapi/cameras.go` only checks tenant scope. `internal/camerapg/authorization.go` intentionally permits an `authority='ADMIN'` actor when its live tenant role is `support` and the support grant is unexpired. Therefore an existing platform ADMIN with a support membership can obtain a mini JWT and read `/api/v1/cameras`, violating the explicit ADMIN prohibition. The current real-PG tests cover ADMIN on `/user/v1`, but do not exercise an ADMIN identity on `/api/v1`; the test fixture's ADMIN has no membership, so it cannot expose this case. Fix before freeze by adding an authority-aware mini camera guard (or carrying platform-admin state into context and rejecting it) before CameraReader/CameraService is called, then add a real-PG regression with an ADMIN + active support membership expecting 403.

**No other blocker found in this audit.** User routes are mounted only on `UserRoutes`, admin route is absent, camera-specific auth errors are fixed `{code,message}` while legacy routes retain `{error}`, strict JSON/query/path checks are boundary-owned, CameraStore derives farm/tenant from pond and performs live authorization, and the License/runtime ordering yields 403 for denied License and 503 for unavailable provider after target authorization. Existing camera evidence log has 15 required camera scenarios and no FAIL/SKIP lines.

Audit judgment: **BLOCKED pending F1**; do not freeze as complete until the mini ADMIN regression is fixed and rerun.
