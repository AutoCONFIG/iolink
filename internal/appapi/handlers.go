package appapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
		s.providerFailure(c, err)
		return
	}
	alarms, err := s.deps.Alarms.ListByUser(c.Request.Context(), uid(c), 0)
	if err != nil {
		s.providerFailure(c, err)
		return
	}
	worst := pondWorstLevel(alarms)

	out := make([]gin.H, 0, len(ponds))
	for _, p := range ponds {
		item := gin.H{"pond_id": p.ID, "pond_name": p.Name, "status": "normal", "device_count": 0}
		if lvl, ok := worst[p.ID]; ok {
			item["status"] = string(lvl)
		}
		devs, err := s.deps.Devices.ListByPond(c.Request.Context(), p.ID)
		if err != nil {
			s.providerFailure(c, err)
			return
		}
		item["device_count"] = len(devs)
		latest, err := s.pondLatest(c.Request.Context(), p.ID, devs)
		if err != nil {
			s.latestFailure(c, err)
			return
		}
		item["latest"] = latest
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
func (s *Server) pondLatest(ctx context.Context, pondID int64, devs []iolinkcontractsdomain.Device) (gin.H, error) {
	var best *iolinkcontractsdomain.Reading
	for _, d := range devs {
		rd, err := s.deps.Telemetry.Latest(ctx, d.DeviceNo)
		if err != nil {
			if errors.Is(err, iolinkcontractsdomain.ErrNotFound) || errors.Is(err, iolinkcontractsdomain.ErrInvalidProductModel) {
				continue
			}
			return nil, fmt.Errorf("pond latest: %w", err)
		}
		if best == nil || rd.Timestamp.After(best.Timestamp) || (rd.Timestamp.Equal(best.Timestamp) && rd.DeviceNo < best.DeviceNo) {
			cp := rd
			best = &cp
		}
	}
	if best == nil {
		return nil, nil
	}
	return waterLatestJSON(*best), nil
}

func (s *Server) getPond(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	p, err := s.deps.Ponds.GetByUser(c.Request.Context(), id, uid(c))
	if err != nil {
		s.resourceFailure(c, err)
		return
	}
	devs, err := s.deps.Devices.ListByPond(c.Request.Context(), p.ID)
	if err != nil {
		s.providerFailure(c, err)
		return
	}
	alarms, err := s.deps.Alarms.ListByUser(c.Request.Context(), uid(c), 0)
	if err != nil {
		s.providerFailure(c, err)
		return
	}
	status := "normal"
	worst := pondWorstLevel(alarms)
	if level, ok := worst[p.ID]; ok {
		status = string(level)
	}
	latest, err := s.pondLatest(c.Request.Context(), p.ID, devs)
	if err != nil {
		s.latestFailure(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"pond_id": p.ID, "pond_name": p.Name, "status": status, "device_count": len(devs), "latest": latest})
}

func (s *Server) getDevice(c *gin.Context) {
	no := c.Param("device_no")
	d, err := s.deps.Devices.GetByDeviceNoForUser(c.Request.Context(), no, uid(c))
	if err != nil {
		s.resourceFailure(c, err)
		return
	}
	resp := gin.H{
		"id": d.ID, "device_no": d.DeviceNo, "pond_id": d.PondID,
		"name": d.Name, "model": d.Model, "status": string(d.Status), "last_seen_at": d.LastSeenAt,
		"created_at": d.CreatedAt, "disabled_at": d.DisabledAt, "report_interval": d.ReportInterval, "latest": nil,
	}
	rd, err := s.deps.Telemetry.Latest(c.Request.Context(), no)
	if err == nil {
		resp["latest"] = waterLatestJSON(rd)
	} else if !errors.Is(err, iolinkcontractsdomain.ErrNotFound) && !errors.Is(err, iolinkcontractsdomain.ErrInvalidProductModel) {
		s.latestFailure(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (s *Server) waterLatest(c *gin.Context) {
	values, err := parseQueryInput(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	no := values.Get("device_no")
	if no == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	if _, err := s.deps.Devices.GetByDeviceNoForUser(c.Request.Context(), no, uid(c)); err != nil {
		s.resourceFailure(c, err)
		return
	}
	rd, err := s.deps.Telemetry.Latest(c.Request.Context(), no)
	if err != nil {
		if errors.Is(err, iolinkcontractsdomain.ErrInvalidProductModel) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
			return
		}
		if errors.Is(err, iolinkcontractsdomain.ErrNotFound) {
			c.JSON(http.StatusOK, nil)
			return
		}
		s.latestFailure(c, err)
		return
	}
	c.JSON(http.StatusOK, waterLatestJSON(rd))
}

func (s *Server) latestFailure(c *gin.Context, err error) {
	if s.log != nil {
		s.log.ErrorContext(c.Request.Context(), "telemetry latest query failed", slog.String("stage", "telemetry_latest"), slog.String("outcome", "failed"), slog.Any("err", err))
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
}

func (s *Server) waterHistory(c *gin.Context) {
	values, err := parseQueryInput(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	no := values.Get("device_no")
	metric := values.Get("metric")
	if no == "" || metric == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	_, currentOwner := s.deps.Devices.GetByDeviceNoForUser(c.Request.Context(), no, uid(c))
	if currentOwner != nil && !errors.Is(currentOwner, iolinkcontractsdomain.ErrNotFound) {
		s.providerFailure(c, currentOwner)
		return
	}
	_, scopedHistory := s.deps.Telemetry.(iolinkcontractsdomain.UserTelemetryRepo)
	if currentOwner != nil && !scopedHistory {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	if !iolinkcontractsdomain.ValidMetric(metric) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	maxPoints, err := strconv.Atoi(queryValueOrDefault(values, "max_points", "200"))
	if err != nil || maxPoints < 1 || maxPoints > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	from, to, err := historyWindow(queryValueOrDefault(values, "range", "today"), time.Now())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	scoped, ok := s.deps.Telemetry.(iolinkcontractsdomain.UserTelemetryRepo)
	if !ok {
		s.providerUnavailable(c)
		return
	}
	points, err := scoped.HistoryForUser(c.Request.Context(), no, uid(c), metric, from, to, maxPoints)
	if err != nil {
		if errors.Is(err, iolinkcontractsdomain.ErrInvalidProductModel) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
			return
		}
		s.providerFailure(c, err)
		return
	}
	if currentOwner != nil && len(points) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
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
	values, err := parseQueryInput(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	limit, err := strconv.Atoi(queryValueOrDefault(values, "limit", "50"))
	if err != nil || limit < 1 || limit > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	offset, err := strconv.Atoi(queryValueOrDefault(values, "offset", "0"))
	if err != nil || offset < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	level := values.Get("level")
	if level != "" && level != string(iolinkcontractsdomain.AlarmCritical) && level != string(iolinkcontractsdomain.AlarmWarning) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	onlyUnconfirmed, err := strconv.ParseBool(queryValueOrDefault(values, "only_unconfirmed", "false"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	var alarms []iolinkcontractsdomain.Alarm
	if filtered, ok := s.deps.Alarms.(iolinkcontractsdomain.FilteredAlarmRepo); ok {
		alarms, err = filtered.ListByUserFiltered(c.Request.Context(), uid(c), iolinkcontractsdomain.AlarmLevel(level), onlyUnconfirmed, limit, offset)
	} else {
		alarms, err = s.deps.Alarms.ListByUser(c.Request.Context(), uid(c), 0)
		if err == nil {
			filtered := make([]iolinkcontractsdomain.Alarm, 0, len(alarms))
			for _, a := range alarms {
				if level != "" && string(a.Level) != level || onlyUnconfirmed && a.ConfirmedAt != nil {
					continue
				}
				filtered = append(filtered, a)
			}
			if offset >= len(filtered) {
				alarms = []iolinkcontractsdomain.Alarm{}
			} else {
				end := offset + limit
				if end > len(filtered) {
					end = len(filtered)
				}
				alarms = filtered[offset:end]
			}
		}
	}
	if err != nil {
		s.providerFailure(c, err)
		return
	}
	c.JSON(http.StatusOK, alarmsJSON(alarms))
}

func (s *Server) confirmAlarm(c *gin.Context) {
	role := iolinkcontractsdomain.TenantRole(c.Request.Context())
	if role != "" && role != "owner" && role != "admin" && role != "member" && role != "support" {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	if err := s.deps.Alarms.ConfirmByUser(c.Request.Context(), id, uid(c)); err != nil {
		if errors.Is(err, iolinkcontractsdomain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		s.providerFailure(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ---- JSON shaping (keeps OpenAPI field names) ----

func waterLatestJSON(rd iolinkcontractsdomain.Reading) gin.H {
	timestamps := map[string]any{}
	for _, key := range []string{"temperature", "dissolved_oxygen", "ph", "turbidity", "salinity", "signal"} {
		if ts, ok := rd.Timestamps[key]; ok {
			timestamps[key] = ts
		} else {
			timestamps[key] = nil
		}
	}
	h := gin.H{"device_no": rd.DeviceNo, "ts": rd.Timestamp, "pond_id": rd.PondID, "report_interval": rd.ReportInterval, "timestamps": timestamps}
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
		s.providerFailure(c, err)
		return
	}
	online, offline := 0, 0
	pondItems := make([]gin.H, 0, len(ponds))
	for _, p := range ponds {
		devs, err := s.deps.Devices.ListByPond(c.Request.Context(), p.ID)
		if err != nil {
			s.providerFailure(c, err)
			return
		}
		for _, d := range devs {
			switch d.Status {
			case iolinkcontractsdomain.DeviceOnline:
				online++
			case iolinkcontractsdomain.DeviceOffline:
				offline++
			}
		}
		pondItems = append(pondItems, gin.H{
			"pond_id": p.ID, "pond_name": p.Name, "status": "normal",
		})
	}
	alarms, err := s.deps.Alarms.ListByUser(c.Request.Context(), uid(c), 0)
	if err != nil {
		s.providerFailure(c, err)
		return
	}
	openByPond := map[int64]iolinkcontractsdomain.AlarmLevel{}
	alarmDevices := map[string]struct{}{}
	for _, a := range alarms {
		if a.ConfirmedAt == nil {
			alarmDevices[a.DeviceNo] = struct{}{}
			if openByPond[a.PondID] != iolinkcontractsdomain.AlarmCritical {
				openByPond[a.PondID] = a.Level
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
