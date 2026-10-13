package appapi

import (
	"net/http"
	"strconv"
	"unicode/utf8"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/gin-gonic/gin"
)

type deviceListInput struct {
	query    domain.DeviceListQuery
	page     int
	pageSize int
}

func parseDeviceListInput(c *gin.Context) (deviceListInput, error) {
	values, err := parseQueryInput(c)
	if err != nil {
		return deviceListInput{}, err
	}
	pageValue := "1"
	if values.Has("page") {
		pageValue = values.Get("page")
	}
	page, err := strconv.Atoi(pageValue)
	if err != nil || page < 1 {
		return deviceListInput{}, domain.ErrInvalidRange
	}
	sizeValue := "20"
	if values.Has("page_size") {
		sizeValue = values.Get("page_size")
	}
	size, err := strconv.Atoi(sizeValue)
	if err != nil || size < 1 || size > 200 || page-1 > int(^uint(0)>>1)/size {
		return deviceListInput{}, domain.ErrInvalidRange
	}
	pondID := int64(0)
	if values.Has("pond_id") {
		pondID, err = strconv.ParseInt(values.Get("pond_id"), 10, 64)
		if err != nil || pondID < 1 {
			return deviceListInput{}, domain.ErrInvalidRange
		}
	}
	status := "active"
	if values.Has("status") {
		status = values.Get("status")
	}
	switch status {
	case "active", "all", "online", "offline", "disabled":
	default:
		return deviceListInput{}, domain.ErrInvalidRange
	}
	search := values.Get("name")
	if utf8.RuneCountInString(search) > 128 {
		return deviceListInput{}, domain.ErrInvalidRange
	}
	return deviceListInput{query: domain.DeviceListQuery{PondID: pondID, Status: status, Search: search, IncludeDisabled: status == "all" || status == "disabled", Limit: size, Offset: (page - 1) * size}, page: page, pageSize: size}, nil
}

func (s *Server) listDevices(c *gin.Context) {
	input, err := parseDeviceListInput(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	if input.query.PondID > 0 {
		if _, err := s.deps.Ponds.GetByUser(c.Request.Context(), input.query.PondID, uid(c)); err != nil {
			s.resourceFailure(c, err)
			return
		}
	}
	repo, ok := s.deps.Devices.(domain.UserDevicePageRepo)
	if !ok {
		s.providerUnavailable(c)
		return
	}
	page, err := repo.ListPageByUser(c.Request.Context(), uid(c), input.query)
	if err != nil {
		s.providerFailure(c, err)
		return
	}
	rows := make([]gin.H, 0, len(page.Items))
	for _, device := range page.Items {
		rows = append(rows, gin.H{"id": device.ID, "device_no": device.DeviceNo, "pond_id": device.PondID, "name": device.Name, "model": device.Model, "status": string(device.Status), "last_seen_at": device.LastSeenAt, "created_at": device.CreatedAt, "disabled_at": device.DisabledAt, "report_interval": device.ReportInterval})
	}
	c.JSON(http.StatusOK, gin.H{"list": rows, "total": page.Total, "page": input.page, "page_size": input.pageSize})
}
