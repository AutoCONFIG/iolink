package adminapi

import (
	"errors"
	"net/http"
	"strconv"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/gin-gonic/gin"
)

type productCreateRequest struct {
	Name string `json:"name"`
}
type productModelRequest struct {
	Fields []domain.ModelField `json:"fields"`
}
type assignmentRequest struct {
	ProductID    int64 `json:"product_id"`
	ModelVersion int   `json:"model_version"`
}

func (s *Server) productCatalog(c *gin.Context) (ProductCatalog, int64, bool) {
	if s.deps.Catalog == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "product catalog unavailable"})
		return nil, 0, false
	}
	if tenant, scoped := domain.TenantID(c.Request.Context()); scoped {
		return s.deps.Catalog, tenant, true
	}
	tenant, err := s.deps.Catalog.DefaultTenantID(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "tenant unavailable"})
		return nil, 0, false
	}
	return s.deps.Catalog, tenant, true
}

func (s *Server) listProducts(c *gin.Context) {
	if role := domain.TenantRole(c.Request.Context()); role != "owner" && role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "tenant admin required"})
		return
	}
	catalog, tenant, ok := s.productCatalog(c)
	if !ok {
		return
	}
	items, err := catalog.ListProducts(c.Request.Context(), tenant)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "product query failed"})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (s *Server) createProduct(c *gin.Context) {
	catalog, tenant, ok := s.productCatalog(c)
	if !ok {
		return
	}
	var req productCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	item, err := catalog.CreateProduct(c.Request.Context(), tenant, req.Name)
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "conflict"})
			return
		}
		if errors.Is(err, domain.ErrInvalidProductModel) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "product create failed"})
		return
	}
	c.JSON(http.StatusCreated, item)
}

func parseProductIDs(c *gin.Context) (int64, int, bool) {
	productID, err := strconv.ParseInt(c.Param("product_id"), 10, 64)
	if err != nil || productID < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return 0, 0, false
	}
	version, err := strconv.Atoi(c.Param("version"))
	if err != nil || version < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return 0, 0, false
	}
	return productID, version, true
}

func (s *Server) listProductModels(c *gin.Context) {
	if role := domain.TenantRole(c.Request.Context()); role != "owner" && role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "tenant admin required"})
		return
	}
	catalog, tenant, ok := s.productCatalog(c)
	if !ok {
		return
	}
	productID, err := strconv.ParseInt(c.Param("product_id"), 10, 64)
	if err != nil || productID < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	items, err := catalog.ListProductModels(c.Request.Context(), tenant, productID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "model query failed"})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (s *Server) createProductModel(c *gin.Context) {
	catalog, tenant, ok := s.productCatalog(c)
	if !ok {
		return
	}
	productID, err := strconv.ParseInt(c.Param("product_id"), 10, 64)
	if err != nil || productID < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	var req productModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	models, err := catalog.ListProductModels(c.Request.Context(), tenant, productID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "model query failed"})
		return
	}
	version := 1
	if len(models) > 0 {
		version = models[len(models)-1].Version + 1
	}
	item, err := catalog.CreateProductModel(c.Request.Context(), tenant, productID, version, req.Fields)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		if errors.Is(err, domain.ErrInvalidProductModel) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
			return
		}
		if errors.Is(err, domain.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "conflict"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "model create failed"})
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (s *Server) publishProductModel(c *gin.Context) {
	catalog, tenant, ok := s.productCatalog(c)
	if !ok {
		return
	}
	productID, version, ok := parseProductIDs(c)
	if !ok {
		return
	}
	if err := catalog.PublishProductModel(c.Request.Context(), tenant, productID, version); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "publish failed"})
		return
	}
	item, err := catalog.GetProductModel(c.Request.Context(), tenant, productID, version)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "model query failed"})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (s *Server) getDeviceProduct(c *gin.Context) {
	catalog, tenant, ok := s.productCatalog(c)
	if !ok {
		return
	}
	model, err := catalog.DeviceAssignment(c.Request.Context(), tenant, c.Param("device_no"))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "assignment query failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"device_no": c.Param("device_no"), "product_id": model.ProductID, "model_version": model.Version, "assigned_at": model.AssignedAt})
}

func (s *Server) assignDeviceProduct(c *gin.Context) {
	catalog, tenant, ok := s.productCatalog(c)
	if !ok {
		return
	}
	var req assignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.ProductID < 1 || req.ModelVersion < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	if err := catalog.AssignDeviceProductByActor(c.Request.Context(), tenant, c.Param("device_no"), req.ProductID, req.ModelVersion, aid(c)); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		if errors.Is(err, domain.ErrInvalidProductModel) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "assignment failed"})
		return
	}
	model, err := catalog.DeviceAssignment(c.Request.Context(), tenant, c.Param("device_no"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "assignment query failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"device_no": c.Param("device_no"), "product_id": model.ProductID, "model_version": model.Version, "assigned_at": model.AssignedAt})
}
