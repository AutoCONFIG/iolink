package appapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	iolinkcontractsdomain "git.hyhy.fun/rsplab/iolink/internal/domain"
)

// ---- handlers: ponds / devices / water / alarms ----
// Ownership rule: every query is scoped by uid via the repository so one
// user can never read another user's ponds.

// listPonds returns ponds enriched with status (worst open alarm) and the
// most recent reading across the pond's devices — feeds the status wall.
func (s *Server) listPonds(c *gin.Context) {
	ponds, err := s.deps.Ponds.ListByUser(c.Request.Context(), uid(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	alarms, _ := s.deps.Alarms.ListByUser(c.Request.Context(), uid(c), 200)
	worst := pondWorstLevel(alarms)

	out := make([]gin.H, 0, len(ponds))
	for _, p := range ponds {
		item := gin.H{"pond_id": p.ID, "pond_name": p.Name, "status": "normal", "device_count": 0}
		if lvl, ok := worst[p.ID]; ok {
			item["status"] = string(lvl)
		}
		devs, err := s.deps.Devices.ListByPond(c.Request.Context(), p.ID)
		if err == nil {
			item["device_count"] = len(devs)
		}
		item["latest"] = s.pondLatest(c.Request.Context(), p.ID, devs)
		out = append(out, item)
	}
	c.JSON(http.StatusOK, out)
}

// pondWorstLevel maps open alarms to the worst level per pond.
func pondWorstLevel(alarms []iolinkcontractsdomain.Alarm) map[int64]iolinkcontractsdomain.AlarmLevel {
	worst := map[int64]iolinkcontractsdomain.AlarmLevel{}
	for _, a := range alarms {
		if a.ConfirmedAt != nil {
			continue
		}
		cur, ok := worst[a.PondID]
		if !ok || (a.Level == iolinkcontractsdomain.AlarmCritical && cur != iolinkcontractsdomain.AlarmCritical) {
			worst[a.PondID] = a.Level
		}
	}
	return worst
}

// pondLatest picks the freshest shadow reading among the pond's devices.
func (s *Server) pondLatest(ctx context.Context, pondID int64, devs []iolinkcontractsdomain.Device) gin.H {
	var best *iolinkcontractsdomain.Reading
	for _, d := range devs {
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
	return waterLatestJSON(*best)
}

func (s *Server) getPond(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	p, err := s.deps.Ponds.GetByUser(c.Request.Context(), id, uid(c))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "pond not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"pond_id": p.ID, "pond_name": p.Name})
}

func (s *Server) listDevices(c *gin.Context) {
	pondID, _ := strconv.ParseInt(c.Query("pond_id"), 10, 64)
	ponds, err := s.deps.Ponds.ListByUser(c.Request.Context(), uid(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]gin.H, 0)
	for _, p := range ponds {
		if pondID != 0 && p.ID != pondID {
			continue
		}
		devs, err := s.deps.Devices.ListByPond(c.Request.Context(), p.ID)
		if err != nil {
			continue
		}
		for _, d := range devs {
			out = append(out, gin.H{
				"device_no": d.DeviceNo, "pond_id": d.PondID,
				"name": d.Name, "model": d.Model, "status": string(d.Status), "last_seen_at": d.LastSeenAt, "report_interval": d.ReportInterval,
			})
		}
	}
	c.JSON(http.StatusOK, out)
}

func (s *Server) getDevice(c *gin.Context) {
	no := c.Param("device_no")
	d, err := s.deps.Devices.GetByDeviceNoForUser(c.Request.Context(), no, uid(c))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}
	resp := gin.H{
		"device_no": d.DeviceNo, "pond_id": d.PondID,
		"name": d.Name, "model": d.Model, "status": string(d.Status), "last_seen_at": d.LastSeenAt, "report_interval": d.ReportInterval,
	}
	if rd, err := s.deps.Telemetry.Latest(c.Request.Context(), no); err == nil {
		resp["latest"] = waterLatestJSON(rd)
	}
	c.JSON(http.StatusOK, resp)
}

func (s *Server) waterLatest(c *gin.Context) {
	no := c.Query("device_no")
	if no == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_no required"})
		return
	}
	if _, err := s.deps.Devices.GetByDeviceNoForUser(c.Request.Context(), no, uid(c)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no data"})
		return
	}
	rd, err := s.deps.Telemetry.Latest(c.Request.Context(), no)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no data"})
		return
	}
	c.JSON(http.StatusOK, waterLatestJSON(rd))
}

func (s *Server) waterHistory(c *gin.Context) {
	no := c.Query("device_no")
	metric := c.Query("metric")
	if no == "" || metric == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_no and metric required"})
		return
	}
	if _, err := s.deps.Devices.GetByDeviceNoForUser(c.Request.Context(), no, uid(c)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}
	if !iolinkcontractsdomain.ValidMetric(metric) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid metric"})
		return
	}
	maxPoints, err := strconv.Atoi(c.DefaultQuery("max_points", "200"))
	if err != nil || maxPoints < 1 || maxPoints > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid max_points"})
		return
	}
	from, to, err := historyWindow(c.DefaultQuery("range", "today"), time.Now())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid range"})
		return
	}
	points, err := s.deps.Telemetry.History(c.Request.Context(), no, metric, from, to, maxPoints)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "history query failed"})
		return
	}
	if points == nil {
		points = []iolinkcontractsdomain.MetricPoint{}
	}
	c.JSON(http.StatusOK, gin.H{"metric": metric, "unit": iolinkcontractsdomain.MetricUnits[metric], "points": points})
}
func historyWindow(name string, now time.Time) (time.Time, time.Time, error) {
	// Asia/Shanghai has used UTC+8 since 1991; current business-day bounds.
	local := now.In(time.FixedZone("Asia/Shanghai", 8*60*60))
	to := now.UTC()
	switch name {
	case "today":
		return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location()).UTC(), to, nil
	case "7d":
		return to.AddDate(0, 0, -7), to, nil
	case "30d":
		return to.AddDate(0, 0, -30), to, nil
	default:
		return time.Time{}, time.Time{}, iolinkcontractsdomain.ErrInvalidRange
	}
}

func (s *Server) listAlarms(c *gin.Context) {
	limit := 50
	if l, err := strconv.Atoi(c.DefaultQuery("limit", "50")); err == nil && l > 0 && l <= 200 {
		limit = l
	}
	alarms, err := s.deps.Alarms.ListByUser(c.Request.Context(), uid(c), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, alarmsJSON(alarms))
}

func (s *Server) confirmAlarm(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	if err := s.deps.Alarms.ConfirmByUser(c.Request.Context(), id, uid(c)); err != nil {
		if errors.Is(err, iolinkcontractsdomain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "alarm not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// ---- JSON shaping (keeps OpenAPI field names) ----

func waterLatestJSON(rd iolinkcontractsdomain.Reading) gin.H {
	h := gin.H{"ts": rd.Timestamp, "pond_id": rd.PondID, "report_interval": rd.ReportInterval, "timestamps": rd.Timestamps}
	h["battery"] = rd.Battery
	h["signal"] = rd.Signal
	h["temperature"] = rd.Temperature
	h["dissolved_oxygen"] = rd.DO
	h["ph"] = rd.PH
	h["turbidity"] = rd.Turbidity
	h["salinity"] = rd.Salinity
	return h
}

func alarmsJSON(in []iolinkcontractsdomain.Alarm) []gin.H {
	out := make([]gin.H, 0, len(in))
	for _, a := range in {
		out = append(out, gin.H{
			"id": a.ID, "device_no": a.DeviceNo, "pond_id": a.PondID,
			"metric": a.Metric, "current_value": a.CurrentValue, "threshold": a.Threshold,
			"level": string(a.Level), "message": a.Message,
			"confirmed_at": a.ConfirmedAt, "created_at": a.CreatedAt,
		})
	}
	return out
}

// statsSummary powers the mini-program home page:
// online/offline/alarm device counts + per-pond status list.
func (s *Server) statsSummary(c *gin.Context) {
	ponds, err := s.deps.Ponds.ListByUser(c.Request.Context(), uid(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	online, offline := 0, 0
	pondItems := make([]gin.H, 0, len(ponds))
	for _, p := range ponds {
		devs, err := s.deps.Devices.ListByPond(c.Request.Context(), p.ID)
		if err == nil {
			for _, d := range devs {
				switch d.Status {
				case iolinkcontractsdomain.DeviceOnline:
					online++
				case iolinkcontractsdomain.DeviceOffline:
					offline++
				}
			}
		}
		pondItems = append(pondItems, gin.H{
			"pond_id": p.ID, "pond_name": p.Name, "status": "normal",
		})
	}
	alarms, err := s.deps.Alarms.ListByUser(c.Request.Context(), uid(c), 5000)
	openByPond := map[int64]iolinkcontractsdomain.AlarmLevel{}
	alarmDevices := map[string]struct{}{}
	if err == nil {
		for _, a := range alarms {
			if a.ConfirmedAt == nil {
				alarmDevices[a.DeviceNo] = struct{}{}
				if openByPond[a.PondID] != iolinkcontractsdomain.AlarmCritical {
					openByPond[a.PondID] = a.Level
				}
			}
		}
	}
	for _, item := range pondItems {
		if level := openByPond[item["pond_id"].(int64)]; level != "" {
			item["status"] = string(level)
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"online_devices":  online,
		"offline_devices": offline,
		"alarm_devices":   len(alarmDevices),
		"ponds":           pondItems,
	})
}
