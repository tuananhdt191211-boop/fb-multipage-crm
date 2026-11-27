package handlers

// Thư viện ảnh của website
// ---------------------------------------------------------------------------
// Ảnh được lưu thẳng trong PostgreSQL (bytea) thay vì ổ đĩa, vì ổ đĩa của
// Render bị xoá mỗi lần deploy lại. Ảnh công khai qua
// GET /api/v1/storefront/media/:mid (cache 1 năm, nội dung không đổi).

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	maxSiteMediaBytes   = 5 << 20 // 5MB / ảnh (giao diện đã nén trước khi tải lên)
	maxSiteMediaPerSite = 300
)

// Chỉ nhận ảnh raster; không nhận SVG (có thể chứa script)
var allowedSiteMediaTypes = map[string]bool{
	"image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true,
}

type SiteMedia struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"index;not null" json:"-"`
	StorefrontID uint      `gorm:"index;not null" json:"storefront_id"`
	FileName     string    `gorm:"size:255" json:"file_name"`
	ContentType  string    `gorm:"size:50;not null" json:"content_type"`
	Size         int64     `json:"size"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	Data         []byte    `gorm:"type:bytea;not null" json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

func migrateSiteMedia(db *gorm.DB) {
	_ = db.AutoMigrate(&SiteMedia{})
}

// publicBaseURL dựng địa chỉ gốc của backend (ảnh phải là URL tuyệt đối vì web công khai chạy ở domain khác)
func publicBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if p := strings.TrimSpace(strings.Split(c.GetHeader("X-Forwarded-Proto"), ",")[0]); p != "" {
		scheme = p
	}
	host := c.Request.Host
	if fh := strings.TrimSpace(strings.Split(c.GetHeader("X-Forwarded-Host"), ",")[0]); fh != "" {
		host = fh
	}
	return scheme + "://" + host
}

func siteMediaURL(c *gin.Context, id uint) string {
	return fmt.Sprintf("%s/api/v1/storefront/media/%d", publicBaseURL(c), id)
}

// UploadSiteMedia POST /api/v1/storefront/sites/:id/media (multipart, field "file")
func (h *StorefrontHandler) UploadSiteMedia(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}

	var count int64
	h.DB.Model(&SiteMedia{}).Where("storefront_id = ?", sf.ID).Count(&count)
	if count >= maxSiteMediaPerSite {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Mỗi website tối đa %d ảnh. Hãy xoá bớt ảnh không dùng.", maxSiteMediaPerSite)})
		return
	}

	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng chọn ảnh."})
		return
	}
	if fh.Size > maxSiteMediaBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Ảnh tối đa %dMB.", maxSiteMediaBytes>>20)})
		return
	}
	f, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Không đọc được ảnh."})
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxSiteMediaBytes+1))
	if err != nil || len(data) == 0 || int64(len(data)) > maxSiteMediaBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ảnh không hợp lệ hoặc quá lớn."})
		return
	}

	ctype := http.DetectContentType(data)
	if !allowedSiteMediaTypes[ctype] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chỉ nhận ảnh JPG, PNG, GIF hoặc WEBP."})
		return
	}

	name := strings.TrimSpace(fh.Filename)
	if r := []rune(name); len(r) > 120 {
		name = string(r[:120])
	}
	w, _ := strconv.Atoi(c.PostForm("width"))
	hgt, _ := strconv.Atoi(c.PostForm("height"))

	m := SiteMedia{
		UserID:       sf.UserID,
		StorefrontID: sf.ID,
		FileName:     name,
		ContentType:  ctype,
		Size:         int64(len(data)),
		Width:        w,
		Height:       hgt,
		Data:         data,
	}
	if err := h.DB.Create(&m).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lưu ảnh thất bại."})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"media": gin.H{
		"id": m.ID, "url": siteMediaURL(c, m.ID), "file_name": m.FileName,
		"size": m.Size, "width": m.Width, "height": m.Height, "created_at": m.CreatedAt,
	}})
}

// ListSiteMedia GET /api/v1/storefront/sites/:id/media
func (h *StorefrontHandler) ListSiteMedia(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	var list []SiteMedia
	if err := h.DB.Select("id, storefront_id, file_name, content_type, size, width, height, created_at").
		Where("storefront_id = ?", sf.ID).Order("id DESC").Limit(maxSiteMediaPerSite).Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không tải được thư viện ảnh."})
		return
	}
	out := make([]gin.H, 0, len(list))
	for _, m := range list {
		out = append(out, gin.H{
			"id": m.ID, "url": siteMediaURL(c, m.ID), "file_name": m.FileName,
			"size": m.Size, "width": m.Width, "height": m.Height, "created_at": m.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"media": out, "max": maxSiteMediaPerSite})
}

// DeleteSiteMedia DELETE /api/v1/storefront/sites/:id/media/:mid
func (h *StorefrontHandler) DeleteSiteMedia(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	res := h.DB.Where("id = ? AND storefront_id = ?", c.Param("mid"), sf.ID).Delete(&SiteMedia{})
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy ảnh."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ServeSiteMedia GET /api/v1/storefront/media/:mid (công khai)
func (h *StorefrontHandler) ServeSiteMedia(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("mid"), 10, 64)
	if err != nil || id == 0 {
		c.Status(http.StatusNotFound)
		return
	}
	var m SiteMedia
	if err := h.DB.Select("id, content_type, data").Where("id = ?", id).First(&m).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	c.Data(http.StatusOK, m.ContentType, m.Data)
}
