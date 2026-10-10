package appapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"git.hyhy.fun/rsplab/iolink/internal/camera"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/gin-gonic/gin"
)

type CameraReader interface {
	List(context.Context, camera.Page) (camera.List, error)
	Get(context.Context, int64) (camera.Camera, error)
}

func (s *Server) mountCameraRoutes(v1 *gin.RouterGroup) {
	group := v1.Group("/cameras", func(c *gin.Context) { c.Set("camera_route", true); s.authRequired(c) }, s.cameraContextRequired)
	group.GET("", s.listCameras)
	group.GET("/:camera_id", s.getCamera)
}

func abortCameraAuth(c *gin.Context, status int, legacy string) {
	if !c.GetBool("camera_route") {
		c.AbortWithStatusJSON(status, gin.H{"error": legacy})
		return
	}
	if legacy == "tenant context required" {
		status = http.StatusForbidden
	}
	code := "unauthorized"
	if status == http.StatusForbidden {
		code = "forbidden"
	}
	c.AbortWithStatusJSON(status, gin.H{"code": code, "message": code})
}

func (s *Server) cameraContextRequired(c *gin.Context) {
	_, scoped := domain.TenantID(c.Request.Context())
	if !scoped {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": "forbidden"})
		return
	}
	c.Next()
}

func cameraError(c *gin.Context, err error) {
	status, code := http.StatusInternalServerError, "internal_error"
	switch {
	case errors.Is(err, camera.ErrInvalid):
		status, code = http.StatusBadRequest, "invalid_request"
	case errors.Is(err, domain.ErrForbidden):
		status, code = http.StatusForbidden, "forbidden"
	case errors.Is(err, domain.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, camera.ErrUnavailable):
		status, code = http.StatusServiceUnavailable, "unavailable"
	case errors.Is(err, domain.ErrConflict):
		status, code = http.StatusConflict, "conflict"
	}
	c.JSON(status, gin.H{"code": code, "message": code})
}

func cameraID(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("camera_id"), 10, 64)
	if err != nil || id < 1 {
		return 0, camera.ErrInvalid
	}
	return id, nil
}

func cameraPage(c *gin.Context, list bool) (camera.Page, error) {
	values, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		return camera.Page{}, camera.ErrInvalid
	}
	page := camera.Page{Limit: 50}
	for key, entries := range values {
		if !list || len(entries) != 1 {
			return camera.Page{}, camera.ErrInvalid
		}
		switch key {
		case "limit":
			value, err := strconv.Atoi(entries[0])
			if err != nil || value < 1 || value > 100 {
				return camera.Page{}, camera.ErrInvalid
			}
			page.Limit = value
		case "after_id":
			value, err := strconv.ParseInt(entries[0], 10, 64)
			if err != nil || value < 1 {
				return camera.Page{}, camera.ErrInvalid
			}
			page.AfterID = value
		default:
			return camera.Page{}, camera.ErrInvalid
		}
	}
	return page, nil
}

func (s *Server) listCameras(c *gin.Context) {
	page, err := cameraPage(c, true)
	if err != nil {
		cameraError(c, err)
		return
	}
	if s.deps.Cameras == nil {
		cameraError(c, camera.ErrUnavailable)
		return
	}
	result, err := s.deps.Cameras.List(c.Request.Context(), page)
	if err != nil {
		cameraError(c, err)
		return
	}
	if result.Items == nil {
		result.Items = []camera.Camera{}
	}
	c.JSON(http.StatusOK, result)
}
func (s *Server) getCamera(c *gin.Context) {
	id, err := cameraID(c)
	if err != nil {
		cameraError(c, err)
		return
	}
	if _, err = cameraPage(c, false); err != nil {
		cameraError(c, err)
		return
	}
	if s.deps.Cameras == nil {
		cameraError(c, camera.ErrUnavailable)
		return
	}
	result, err := s.deps.Cameras.Get(c.Request.Context(), id)
	if err != nil {
		cameraError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
