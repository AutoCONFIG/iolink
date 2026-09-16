package adminapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

// ---- farms ----

type farmReq struct {
	Name     string `json:"name" binding:"required"`
	Location string `json:"location"`
}

func (s *Server) listFarms(c *gin.Context) {
	farms, err := s.deps.Store.ListFarms(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, farms)
}

func (s *Server) createFarm(c *gin.Context) {
	var req farmReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	f, err := s.deps.Store.CreateFarm(c.Request.Context(), aid(c), req.Name, req.Location)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, f)
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
		if errors.Is(err, domain.ErrFarmHasPonds) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
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
	var worst map[int64]domain.AlarmLevel
	if alarms, err := s.deps.Store.ListAllAlarms(c.Request.Context(), 500); err == nil {
		worst = pondWorstLevel(alarms)
	}
	out := make([]gin.H, 0, len(ponds))
	for _, p := range ponds {
		item := gin.H{
			"id": p.ID, "farm_id": p.FarmID, "name": p.Name,
			"area_mu": p.AreaMu, "created_at": p.CreatedAt, "status": "normal",
		}
		if lvl, ok := worst[p.ID]; ok {
			item["status"] = string(lvl)
		}
		item["latest"] = s.pondLatest(c.Request.Context(), p.ID)
		out = append(out, item)
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
	devs, err := s.deps.Store.ListDevices(ctx)
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
		if best == nil || rd.Timestamp.After(best.Timestamp) {
			cp := rd
			best = &cp
		}
	}
	if best == nil {
		return nil
	}
	return gin.H{
		"ts": best.Timestamp, "temperature": best.Temperature,
		"dissolved_oxygen": best.DO, "ph": best.PH,
		"turbidity": best.Turbidity, "salinity": best.Salinity,
	}
}

func (s *Server) createPond(c *gin.Context) {
	var req pondReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p, err := s.deps.Store.CreatePond(c.Request.Context(), req.FarmID, req.Name, req.AreaMu)
	if err != nil {
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
		if errors.Is(err, domain.ErrPondHasDevices) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// ---- devices ----

type registerDeviceReq struct {
	PondID int64  `json:"pond_id" binding:"required"`
	Model  string `json:"model"`
}

// registerDevice responds with the device secret exactly once.
func (s *Server) registerDevice(c *gin.Context) {
	var req registerDeviceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	dev, secret, err := s.deps.Store.RegisterDevice(c.Request.Context(), req.PondID, req.Model)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"device_no": dev.DeviceNo,
		"secret":    secret, // shown once; only sha256 stored
		"pond_id":   dev.PondID,
		"model":     dev.Model,
		"status":    string(dev.Status),
	})
}

func (s *Server) listDevices(c *gin.Context) {
	devs, err := s.deps.Store.ListDevices(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, devs)
}

func (s *Server) deleteDevice(c *gin.Context) {
	no := c.Param("device_no")
	if err := s.deps.Store.DeleteDevice(c.Request.Context(), no); err != nil {
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
	c.JSON(http.StatusOK, rules)
}

func (s *Server) createRule(c *gin.Context) {
	var req ruleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !domain.ValidMetric(req.Metric) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown metric"})
		return
	}
	if req.Min == nil && req.Max == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "min_value or max_value required"})
		return
	}
	rule, err := s.deps.Store.CreateRule(c.Request.Context(), req.toDomain())
	if err != nil {
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
	if !domain.ValidMetric(req.Metric) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown metric"})
		return
	}
	rule := req.toDomain()
	rule.ID = id
	if err := s.deps.Store.UpdateRule(c.Request.Context(), rule); err != nil {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// ---- alarms ----

func (s *Server) listAlarms(c *gin.Context) {
	limit := 50
	if l, err := strconv.Atoi(c.DefaultQuery("limit", "50")); err == nil && l > 0 && l <= 500 {
		limit = l
	}
	alarms, err := s.deps.Store.ListAllAlarms(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, alarms)
}

func (s *Server) confirmAlarm(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	if err := s.deps.Store.ConfirmAlarm(c.Request.Context(), id); err != nil {
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
	n, err := s.deps.Store.BatchConfirm(c.Request.Context(), req.IDs)
	if err != nil {
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
