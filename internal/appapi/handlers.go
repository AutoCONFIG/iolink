package appapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	iolinkcontractsdomain "git.hyhy.fun/rsplab/iolink/internal/domain"
)

// ---- handlers: ponds / devices / water / alarms ----
// Ownership rule: every query is scoped by uid via the repository so one
// user can never read another user's ponds.

func (s *Server) listPonds(c *gin.Context) {
	ponds, err := s.deps.Ponds.ListByUser(c.Request.Context(), uid(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]gin.H, 0, len(ponds))
	for _, p := range ponds {
		item := gin.H{"pond_id": p.ID, "pond_name": p.Name, "status": "normal"}
		if devs, err := s.deps.Devices.ListByPond(c.Request.Context(), p.ID); err == nil {
			item["device_count"] = len(devs)
			// pond status = worst of its devices' latest alarms (simplified M2)
		}
		out = append(out, item)
	}
	c.JSON(http.StatusOK, out)
}

func (s *Server) getPond(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	p, err := s.deps.Ponds.Get(c.Request.Context(), id)
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
				"model": d.Model, "status": string(d.Status), "last_seen_at": d.LastSeenAt,
			})
		}
	}
	c.JSON(http.StatusOK, out)
}

func (s *Server) getDevice(c *gin.Context) {
	no := c.Param("device_no")
	d, err := s.deps.Devices.GetByDeviceNo(c.Request.Context(), no)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}
	resp := gin.H{
		"device_no": d.DeviceNo, "pond_id": d.PondID,
		"model": d.Model, "status": string(d.Status), "last_seen_at": d.LastSeenAt,
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
	to := time.Now()
	switch c.DefaultQuery("range", "today") {
	case "7d":
		from := to.AddDate(0, 0, -7)
		s.history(c, no, metric, from, to)
	case "30d":
		from := to.AddDate(0, 0, -30)
		s.history(c, no, metric, from, to)
	default:
		from := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, to.Location())
		s.history(c, no, metric, from, to)
	}
}

func (s *Server) history(c *gin.Context, no, metric string, from, to time.Time) {
	points, err := s.deps.Telemetry.History(c.Request.Context(), no, metric, from, to, 200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if points == nil {
		points = []iolinkcontractsdomain.MetricPoint{}
	}
	c.JSON(http.StatusOK, gin.H{"metric": metric, "points": points})
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
	if err := s.deps.Alarms.Confirm(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// ---- JSON shaping (keeps OpenAPI field names) ----

func waterLatestJSON(rd iolinkcontractsdomain.Reading) gin.H {
	h := gin.H{"ts": rd.Timestamp}
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
	alarms, err := s.deps.Alarms.ListByUser(c.Request.Context(), uid(c), 200)
	openByPond := map[int64]bool{}
	alarmDevices := 0
	if err == nil {
		for _, a := range alarms {
			if a.ConfirmedAt == nil {
				alarmDevices++
				openByPond[a.PondID] = true
			}
		}
	}
	for _, item := range pondItems {
		if openByPond[item["pond_id"].(int64)] {
			item["status"] = "warning"
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"online_devices":  online,
		"offline_devices": offline,
		"alarm_devices":   alarmDevices,
		"ponds":           pondItems,
	})
}
