package adminapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

type tenantStatusRequest struct {
	Active *bool `json:"active"`
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
	if c.GetBool("platform_admin") || (role != "owner" && role != "admin") {
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
	if c.GetBool("platform_admin") {
		c.JSON(http.StatusForbidden, gin.H{"error": "tenant admin required"})
		return
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

// ---- farms ----

type farmReq struct {
	Name     string `json:"name" binding:"required"`
	Location string `json:"location"`
}
type farmCreateReq struct {
	Name     string `json:"name" binding:"required"`
	Location string `json:"location"`
	OwnerID  *int64 `json:"owner_id"`
}
type ownerReq struct {
	OwnerID *int64 `json:"owner_id"`
}

type farmMemberRequest struct {
	Role      string     `json:"role"`
	Active    *bool      `json:"active"`
	ExpiresAt *time.Time `json:"expires_at"`
}

func (s *Server) listFarmMembers(c *gin.Context) {
	role := domain.TenantRole(c.Request.Context())
	if role != "owner" && role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "tenant admin required"})
		return
	}
	store, ok := s.deps.Store.(FarmMembershipStore)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "farm membership unavailable"})
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad farm id"})
		return
	}
	items, err := store.ListFarmMembers(c.Request.Context(), id)
	if errors.Is(err, domain.ErrForbidden) {
		c.JSON(http.StatusForbidden, gin.H{"error": "farm members forbidden"})
		return
	}
	if errors.Is(err, domain.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "farm not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "farm member query failed"})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (s *Server) setFarmMember(c *gin.Context) {
	store, ok := s.deps.Store.(FarmMembershipStore)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "farm membership unavailable"})
		return
	}
	farmID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad farm id"})
		return
	}
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad user id"})
		return
	}
	var req farmMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Role == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	if err := store.SetFarmMember(c.Request.Context(), farmID, userID, req.Role, active, req.ExpiresAt, aid(c)); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "farm or user not found"})
			return
		}
		if errors.Is(err, domain.ErrInvalidProductModel) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "farm member update failed"})
		return
	}
	c.Status(http.StatusNoContent)
}

func page(c *gin.Context) (int, int, error) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil || limit < 1 || limit > 200 {
		return 0, 0, domain.ErrInvalidRange
	}
	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		return 0, 0, domain.ErrInvalidRange
	}
	return limit, offset, nil
}

func (s *Server) listUsers(c *gin.Context) {
	role := domain.TenantRole(c.Request.Context())
	if role != "owner" && role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "tenant admin required"})
		return
	}
	query := c.Query("query")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query required"})
		return
	}
	limit, offset, err := page(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pagination"})
		return
	}
	users, err := s.deps.Store.SearchUsers(c.Request.Context(), query, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user search failed"})
		return
	}
	out := make([]gin.H, 0, len(users))
	for _, u := range users {
		out = append(out, gin.H{"id": u.ID, "nickname": u.Nickname})
	}
	c.JSON(http.StatusOK, out)
}

func (s *Server) listFarms(c *gin.Context) {
	farms, err := s.deps.Store.ListFarms(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	limit, offset, err := page(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pagination"})
		return
	}
	if offset >= len(farms) {
		farms = []domain.Farm{}
	} else {
		end := offset + limit
		if end > len(farms) {
			end = len(farms)
		}
		farms = farms[offset:end]
	}
	if farms == nil {
		farms = []domain.Farm{}
	}
	c.JSON(http.StatusOK, farms)
}

func (s *Server) createFarm(c *gin.Context) {
	var req farmCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	f, err := s.deps.Store.CreateFarm(c.Request.Context(), req.OwnerID, req.Name, req.Location)
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "owner not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, f)
}

func (s *Server) setFarmOwner(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	var req ownerReq
	if err = c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if scoped, ok := s.deps.Store.(interface {
		SetFarmOwnerByActor(context.Context, int64, *int64, int64) error
	}); ok {
		err = scoped.SetFarmOwnerByActor(c.Request.Context(), id, req.OwnerID, aid(c))
	} else {
		err = s.deps.Store.SetFarmOwner(c.Request.Context(), id, req.OwnerID)
	}
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "farm or user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "owner update failed"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) updateFarm(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	var req farmReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.deps.Store.UpdateFarm(c.Request.Context(), id, req.Name, req.Location); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "farm not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) deleteFarm(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	if err := s.deps.Store.DeleteFarm(c.Request.Context(), id); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if errors.Is(err, domain.ErrFarmHasPonds) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "farm not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// ---- ponds ----

type pondReq struct {
	FarmID int64   `json:"farm_id" binding:"required"`
	Name   string  `json:"name" binding:"required"`
	AreaMu float64 `json:"area_mu"`
}

// listPonds enriches ponds with status (worst open alarm) and freshest reading.
func (s *Server) listPonds(c *gin.Context) {
	ponds, err := s.deps.Store.ListPonds(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if raw := c.Query("farm_id"); raw != "" {
		farmID, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || farmID < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid farm_id"})
			return
		}
		if checker, ok := s.deps.Store.(interface {
			FarmExists(context.Context, int64) (bool, error)
		}); ok {
			exists, checkErr := checker.FarmExists(c.Request.Context(), farmID)
			if checkErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "farm query failed"})
				return
			}
			if !exists {
				c.JSON(http.StatusNotFound, gin.H{"error": "farm not found"})
				return
			}
		}
		filtered := ponds[:0]
		for _, p := range ponds {
			if p.FarmID == farmID {
				filtered = append(filtered, p)
			}
		}
		ponds = filtered
	}
	alarms, err := s.deps.Store.ListAllAlarms(c.Request.Context(), 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "alarm query failed"})
		return
	}
	worst := pondWorstLevel(alarms)
	out := make([]gin.H, 0, len(ponds))
	for _, p := range ponds {
		item := gin.H{
			"id": p.ID, "farm_id": p.FarmID, "name": p.Name,
			"area_mu": p.AreaMu, "created_at": p.CreatedAt, "status": "normal", "device_count": 0,
		}
		if lvl, ok := worst[p.ID]; ok {
			item["status"] = string(lvl)
		}
		item["latest"] = s.pondLatest(c.Request.Context(), p.ID)
		devs, e := s.deps.Store.ListDevices(c.Request.Context(), false, p.ID, 200, 0)
		if e != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "device query failed"})
			return
		}
		item["device_count"] = len(devs)
		out = append(out, item)
	}
	limit, offset, err := page(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pagination"})
		return
	}
	if offset >= len(out) {
		out = []gin.H{}
	} else {
		end := offset + limit
		if end > len(out) {
			end = len(out)
		}
		out = out[offset:end]
	}
	if out == nil {
		out = []gin.H{}
	}
	c.JSON(http.StatusOK, out)
}

func pondWorstLevel(alarms []domain.Alarm) map[int64]domain.AlarmLevel {
	worst := map[int64]domain.AlarmLevel{}
	for _, a := range alarms {
		if a.ConfirmedAt != nil {
			continue
		}
		cur, ok := worst[a.PondID]
		if !ok || (a.Level == domain.AlarmCritical && cur != domain.AlarmCritical) {
			worst[a.PondID] = a.Level
		}
	}
	return worst
}

func (s *Server) pondLatest(ctx context.Context, pondID int64) gin.H {
	if s.deps.Telemetry == nil {
		return nil
	}
	devs, err := s.deps.Store.ListDevices(ctx, false, 0, 200, 0)
	if err != nil {
		return nil
	}
	var best *domain.Reading
	for _, d := range devs {
		if d.PondID != pondID {
			continue
		}
		rd, err := s.deps.Telemetry.Latest(ctx, d.DeviceNo)
		if err != nil {
			continue
		}
		if best == nil || rd.Timestamp.After(best.Timestamp) || (rd.Timestamp.Equal(best.Timestamp) && rd.DeviceNo < best.DeviceNo) {
			cp := rd
			best = &cp
		}
	}
	if best == nil {
		return nil
	}
	timestamps := map[string]any{}
	for _, key := range []string{"temperature", "dissolved_oxygen", "ph", "turbidity", "salinity", "signal"} {
		if ts, ok := best.Timestamps[key]; ok {
			timestamps[key] = ts
		} else {
			timestamps[key] = nil
		}
	}
	return gin.H{"device_no": best.DeviceNo, "ts": best.Timestamp, "pond_id": best.PondID, "signal": best.Signal, "timestamps": timestamps, "report_interval": best.ReportInterval, "temperature": best.Temperature, "dissolved_oxygen": best.DO, "ph": best.PH, "turbidity": best.Turbidity, "salinity": best.Salinity}
}

func (s *Server) createPond(c *gin.Context) {
	var req pondReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p, err := s.deps.Store.CreatePond(c.Request.Context(), req.FarmID, req.Name, req.AreaMu)
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "farm not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (s *Server) updatePond(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	var req pondReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.deps.Store.UpdatePond(c.Request.Context(), id, req.Name, req.AreaMu); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "pond not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) deletePond(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	if err := s.deps.Store.DeletePond(c.Request.Context(), id); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if errors.Is(err, domain.ErrPondHasDevices) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "pond not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// ---- devices ----

type registerDeviceReq struct {
	PondID         int64  `json:"pond_id" binding:"required"`
	Name           string `json:"name"`
	Model          string `json:"model"`
	ReportInterval int    `json:"report_interval"`
}

// registerDevice responds with the device secret exactly once.
func (s *Server) registerDevice(c *gin.Context) {
	var req registerDeviceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	dev, secret, err := s.deps.Store.RegisterDevice(c.Request.Context(), req.PondID, req.Name, req.Model, req.ReportInterval)
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if errors.Is(err, domain.ErrInvalidRange) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "report_interval must be 60 or 300"})
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "pond not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"device_no":       dev.DeviceNo,
		"secret":          secret, // shown once; only sha256 stored
		"pond_id":         dev.PondID,
		"model":           dev.Model,
		"name":            dev.Name,
		"status":          string(dev.Status),
		"report_interval": dev.ReportInterval,
	})
}

func (s *Server) listDevices(c *gin.Context) {
	include, err := strconv.ParseBool(c.DefaultQuery("include_disabled", "false"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid include_disabled"})
		return
	}
	pondID := int64(0)
	if raw := c.Query("pond_id"); raw != "" {
		var err error
		pondID, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || pondID < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pond_id"})
			return
		}
	}
	limit, offset, err := page(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pagination"})
		return
	}
	devs, err := s.deps.Store.ListDevices(c.Request.Context(), include, pondID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if devs == nil {
		devs = []domain.Device{}
	}
	out := make([]gin.H, 0, len(devs))
	for _, d := range devs {
		out = append(out, gin.H{"id": d.ID, "device_no": d.DeviceNo, "pond_id": d.PondID, "name": d.Name, "model": d.Model, "status": string(d.Status), "last_seen_at": d.LastSeenAt, "created_at": d.CreatedAt, "disabled_at": d.DisabledAt, "report_interval": d.ReportInterval})
	}
	c.JSON(http.StatusOK, out)
}

func (s *Server) getDevice(c *gin.Context) {
	d, err := s.deps.Store.GetDevice(c.Request.Context(), c.Param("device_no"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}
	item := gin.H{"id": d.ID, "device_no": d.DeviceNo, "pond_id": d.PondID, "model": d.Model, "name": d.Name, "status": string(d.Status), "last_seen_at": d.LastSeenAt, "created_at": d.CreatedAt, "disabled_at": d.DisabledAt, "report_interval": d.ReportInterval, "latest": nil}
	if s.deps.Telemetry != nil {
		if rd, e := s.deps.Telemetry.Latest(c.Request.Context(), d.DeviceNo); e == nil {
			item["latest"] = adminWaterLatestJSON(rd)
		} else {
			item["latest"] = nil
		}
	}
	c.JSON(http.StatusOK, item)
}

func adminWaterLatestJSON(rd domain.Reading) gin.H {
	ts := map[string]any{}
	for _, key := range []string{"temperature", "dissolved_oxygen", "ph", "turbidity", "salinity", "signal"} {
		if v, ok := rd.Timestamps[key]; ok {
			ts[key] = v
		} else {
			ts[key] = nil
		}
	}
	return gin.H{"device_no": rd.DeviceNo, "ts": rd.Timestamp, "pond_id": rd.PondID, "report_interval": rd.ReportInterval, "timestamps": ts, "signal": rd.Signal, "temperature": rd.Temperature, "dissolved_oxygen": rd.DO, "ph": rd.PH, "turbidity": rd.Turbidity, "salinity": rd.Salinity}
}

func (s *Server) moveDevice(c *gin.Context) {
	pondID := struct {
		PondID int64 `json:"pond_id"`
	}{}
	if c.ShouldBindJSON(&pondID) != nil || pondID.PondID < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pond_id required"})
		return
	}
	if err := s.deps.Store.MoveDevice(c.Request.Context(), c.Param("device_no"), pondID.PondID); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "device or pond not found"})
			return
		}
		if errors.Is(err, domain.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "device disabled"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "move failed"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) deleteDevice(c *gin.Context) {
	no := c.Param("device_no")
	if err := s.deps.Store.DeleteDevice(c.Request.Context(), no); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// ---- alarm rules ----

type ruleReq struct {
	PondID int64    `json:"pond_id" binding:"required"`
	Metric string   `json:"metric" binding:"required"`
	Min    *float64 `json:"min_value"`
	Max    *float64 `json:"max_value"`
	Level  string   `json:"level" binding:"required,oneof=critical warning"`
}

func (r ruleReq) toDomain() domain.AlarmRule {
	return domain.AlarmRule{PondID: r.PondID, Metric: r.Metric, Min: r.Min, Max: r.Max, Level: domain.AlarmLevel(r.Level), Enabled: true}
}

func (s *Server) listRules(c *gin.Context) {
	rules, err := s.deps.Store.ListRules(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	pondID := int64(0)
	if raw := c.Query("pond_id"); raw != "" {
		pondID, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || pondID < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pond_id"})
			return
		}
	}
	limit, offset, err := page(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pagination"})
		return
	}
	filtered := make([]domain.AlarmRule, 0, len(rules))
	for _, r := range rules {
		if pondID == 0 || r.PondID == pondID {
			filtered = append(filtered, r)
		}
	}
	rules = filtered
	if offset >= len(rules) {
		rules = []domain.AlarmRule{}
	} else {
		end := offset + limit
		if end > len(rules) {
			end = len(rules)
		}
		rules = rules[offset:end]
	}
	if rules == nil {
		rules = []domain.AlarmRule{}
	}
	c.JSON(http.StatusOK, rules)
}

func (s *Server) createRule(c *gin.Context) {
	var req ruleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !validRuleRequest(req) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid metric or thresholds"})
		return
	}
	rule, err := s.deps.Store.CreateRule(c.Request.Context(), req.toDomain())
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if errors.Is(err, domain.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "rule already exists"})
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "pond not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (s *Server) updateRule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	var req ruleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !validRuleRequest(req) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid metric or thresholds"})
		return
	}
	rule := req.toDomain()
	rule.ID = id
	if err := s.deps.Store.UpdateRule(c.Request.Context(), rule); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if errors.Is(err, domain.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "rule already exists"})
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) deleteRule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	if err := s.deps.Store.DeleteRule(c.Request.Context(), id); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			c.Status(http.StatusNoContent)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// ---- alarms ----

func (s *Server) listAlarms(c *gin.Context) {
	limit, offset, err := page(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pagination"})
		return
	}
	level := c.Query("level")
	if level != "" && level != string(domain.AlarmCritical) && level != string(domain.AlarmWarning) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid level"})
		return
	}
	only, parseErr := strconv.ParseBool(c.DefaultQuery("only_unconfirmed", "false"))
	if parseErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid only_unconfirmed"})
		return
	}
	alarms, err := s.deps.Store.ListAllAlarms(c.Request.Context(), 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	filtered := make([]domain.Alarm, 0, len(alarms))
	for _, a := range alarms {
		if level != "" && string(a.Level) != level {
			continue
		}
		if only && a.ConfirmedAt != nil {
			continue
		}
		filtered = append(filtered, a)
	}
	if offset >= len(filtered) {
		filtered = []domain.Alarm{}
	} else {
		end := offset + limit
		if end > len(filtered) {
			end = len(filtered)
		}
		filtered = filtered[offset:end]
	}
	if filtered == nil {
		filtered = []domain.Alarm{}
	}
	c.JSON(http.StatusOK, filtered)
}

func (s *Server) confirmAlarm(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	confirm := func() error { return s.deps.Store.ConfirmAlarm(c.Request.Context(), id) }
	role := domain.TenantRole(c.Request.Context())
	if role == "member" || role == "support" {
		if scoped, ok := s.deps.Store.(interface {
			ConfirmAlarmByActor(context.Context, int64, int64) error
		}); ok {
			confirm = func() error { return scoped.ConfirmAlarmByActor(c.Request.Context(), id, aid(c)) }
		} else {
			c.JSON(http.StatusForbidden, gin.H{"error": "alarm authorization unavailable"})
			return
		}
	}
	if err := confirm(); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "alarm not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

type batchConfirmReq struct {
	IDs []int64 `json:"ids" binding:"required,min=1"`
}

func (s *Server) batchConfirm(c *gin.Context) {
	var req batchConfirmReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.IDs) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "too many ids"})
		return
	}
	seen := make(map[int64]struct{}, len(req.IDs))
	for _, id := range req.IDs {
		if _, ok := seen[id]; ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "duplicate id"})
			return
		}
		seen[id] = struct{}{}
	}
	confirm := func() (int64, error) { return s.deps.Store.BatchConfirm(c.Request.Context(), req.IDs) }
	role := domain.TenantRole(c.Request.Context())
	if role == "member" || role == "support" {
		if scoped, ok := s.deps.Store.(interface {
			BatchConfirmByActor(context.Context, []int64, int64) (int64, error)
		}); ok {
			confirm = func() (int64, error) { return scoped.BatchConfirmByActor(c.Request.Context(), req.IDs, aid(c)) }
		} else {
			c.JSON(http.StatusForbidden, gin.H{"error": "alarm authorization unavailable"})
			return
		}
	}
	n, err := confirm()
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "alarm forbidden"})
			return
		}
		if errors.Is(err, domain.ErrInvalidRange) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ids required"})
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "alarm not found"})
			return
		}
		if errors.Is(err, domain.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "batch confirmation not atomic"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"confirmed": n})
}

// ---- stats ----

func (s *Server) stats(c *gin.Context) {
	st, err := s.deps.Store.Stats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, st)
}

// ---- password ----

type changePasswordReq struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

func (s *Server) changePassword(c *gin.Context) {
	var req changePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.deps.Store.ChangeAdminPassword(c.Request.Context(), aid(c), req.OldPassword, req.NewPassword); err != nil {
		if errors.Is(err, domain.ErrOldPasswordMismatch) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func validRuleRequest(r ruleReq) bool {
	if !domain.ValidMetric(r.Metric) || (r.Min == nil && r.Max == nil) {
		return false
	}
	if r.Min != nil && r.Max != nil && *r.Min >= *r.Max {
		return false
	}
	return true
}
