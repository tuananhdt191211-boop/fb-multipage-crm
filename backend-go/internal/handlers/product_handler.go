package handlers

import (
	"net/http"
	"strings"

	"fb-multipage-crm/internal/models"

	"github.com/gin-gonic/gin"
)

const maxProductsPerSite = 200

type productReq struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Price        int64  `json:"price"`
	ComparePrice int64  `json:"compare_price"`
	ImageURL     string `json:"image_url"`
	Stock        int    `json:"stock"`
	IsActive     *bool  `json:"is_active"`
	SortOrder    int    `json:"sort_order"`
}

// normalize làm sạch dữ liệu và trả về thông báo lỗi (rỗng nếu hợp lệ).
func (r *productReq) normalize() string {
	r.Name = strings.TrimSpace(r.Name)
	r.Description = strings.TrimSpace(r.Description)
	r.ImageURL = safeMediaURL(r.ImageURL)
	switch {
	case r.Name == "" || len([]rune(r.Name)) > 255:
		return "Tên sản phẩm bắt buộc và tối đa 255 ký tự."
	case len([]rune(r.Description)) > 5000:
		return "Mô tả tối đa 5000 ký tự."
	case r.Price < 0 || r.Price > 10000000000:
		return "Giá bán không hợp lệ."
	case r.ComparePrice < 0 || r.ComparePrice > 10000000000:
		return "Giá gốc không hợp lệ."
	case r.Stock < -1 || r.Stock > 1000000:
		return "Tồn kho không hợp lệ (dùng -1 nếu không giới hạn)."
	}
	return ""
}

// ListProducts GET /api/v1/storefront/sites/:id/products
func (h *StorefrontHandler) ListProducts(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	var items []models.Product
	if err := h.DB.Where("storefront_id = ?", sf.ID).Order("sort_order ASC, id ASC").Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không tải được danh sách sản phẩm."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"products": items, "max_products": maxProductsPerSite})
}

// CreateProduct POST /api/v1/storefront/sites/:id/products
func (h *StorefrontHandler) CreateProduct(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	var req productReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu sản phẩm không hợp lệ."})
		return
	}
	if msg := req.normalize(); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	var count int64
	h.DB.Model(&models.Product{}).Where("storefront_id = ?", sf.ID).Count(&count)
	if count >= maxProductsPerSite {
		c.JSON(http.StatusForbidden, gin.H{"error": "Đã đạt số sản phẩm tối đa của website."})
		return
	}

	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	p := models.Product{
		StorefrontID: sf.ID,
		UserID:       sf.UserID,
		Name:         req.Name,
		Description:  req.Description,
		Price:        req.Price,
		ComparePrice: req.ComparePrice,
		ImageURL:     req.ImageURL,
		Stock:        req.Stock,
		IsActive:     active,
		SortOrder:    req.SortOrder,
	}
	if err := h.DB.Create(&p).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tạo sản phẩm thất bại."})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"product": p})
}

// UpdateProduct PUT /api/v1/storefront/sites/:id/products/:pid
func (h *StorefrontHandler) UpdateProduct(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	var p models.Product
	if err := h.DB.Where("id = ? AND storefront_id = ?", c.Param("pid"), sf.ID).First(&p).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy sản phẩm."})
		return
	}
	var req productReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu sản phẩm không hợp lệ."})
		return
	}
	if msg := req.normalize(); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	p.Name = req.Name
	p.Description = req.Description
	p.Price = req.Price
	p.ComparePrice = req.ComparePrice
	p.ImageURL = req.ImageURL
	p.Stock = req.Stock
	p.SortOrder = req.SortOrder
	if req.IsActive != nil {
		p.IsActive = *req.IsActive
	}
	if err := h.DB.Save(&p).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lưu sản phẩm thất bại."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"product": p})
}

// DeleteProduct DELETE /api/v1/storefront/sites/:id/products/:pid
func (h *StorefrontHandler) DeleteProduct(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	res := h.DB.Where("id = ? AND storefront_id = ?", c.Param("pid"), sf.ID).Delete(&models.Product{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Xóa sản phẩm thất bại."})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy sản phẩm."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã xóa sản phẩm."})
}
