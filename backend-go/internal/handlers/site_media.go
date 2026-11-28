package handlers

// Thư viện ảnh / video của website
// ---------------------------------------------------------------------------
// Hai chế độ lưu trữ:
//   - Kho ngoài S3/R2 (khi cấu hình S3_* hoặc R2_*): ảnh VÀ video lưu ở bucket, khách tải trực tiếp qua CDN của kho.
//     Đây là cách duy nhất phù hợp cho video, và cũng không tốn dung lượng / băng thông PostgreSQL.
//   - Mặc định (chưa cấu hình): ảnh lưu thẳng trong PostgreSQL (bytea) vì ổ đĩa của Render bị xoá mỗi lần deploy / ngủ dậy.
//     Ảnh công khai qua GET /api/v1/storefront/media/:mid (cache 1 năm). Chế độ này KHÔNG nhận video.

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	maxSiteMediaBytes   = 5 << 20  // 5MB / ảnh (giao diện đã nén trước khi tải lên)
	maxSiteVideoBytes   = 80 << 20 // 80MB / video
	maxSiteMediaDB      = 600      // số ảnh tối đa mỗi website khi lưu trong PostgreSQL
	maxSiteMediaStorage = 5000     // số file tối đa mỗi website khi có kho ngoài
	maxImportImages     = 12       // số ảnh tải về từ link trong một lần gọi
)

// Chỉ nhận ảnh raster; không nhận SVG (có thể chứa script)
var allowedSiteMediaTypes = map[string]string{
	"image/jpeg": "jpg", "image/png": "png", "image/gif": "gif", "image/webp": "webp", "image/avif": "avif",
}

// detectImageType nhận dạng ảnh theo nội dung (không tin đuôi file / Content-Type của nguồn).
// http.DetectContentType chưa biết AVIF (nhiều CDN, vd. AliExpress, trả AVIF) nên kiểm tra thêm hộp "ftyp avif/avis".
func detectImageType(data []byte) (string, bool) {
	ct := http.DetectContentType(data)
	if _, ok := allowedSiteMediaTypes[ct]; ok {
		return ct, true
	}
	if len(data) >= 12 && string(data[4:8]) == "ftyp" && (string(data[8:12]) == "avif" || string(data[8:12]) == "avis") {
		return "image/avif", true
	}
	return "", false
}

type SiteMedia struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"index;not null" json:"-"`
	StorefrontID uint      `gorm:"index;not null" json:"storefront_id"`
	Kind         string    `gorm:"size:10;default:image" json:"kind"` // image | video
	FileName     string    `gorm:"size:255" json:"file_name"`
	ContentType  string    `gorm:"size:50;not null" json:"content_type"`
	Size         int64     `json:"size"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	Data         []byte    `gorm:"type:bytea" json:"-"` // chỉ dùng khi lưu trong PostgreSQL
	StorageKey   string    `gorm:"size:300" json:"-"`   // khoá trong kho ngoài
	URL          string    `gorm:"type:text" json:"-"`  // địa chỉ công khai khi lưu ở kho ngoài
	CreatedAt    time.Time `json:"created_at"`
}

func migrateSiteMedia(db *gorm.DB) {
	_ = db.AutoMigrate(&SiteMedia{})
	// Bản cũ khai báo cột data NOT NULL; file ở kho ngoài không có dữ liệu trong DB
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(&SiteMedia{}); err == nil {
		tbl := stmt.Schema.Table
		_ = db.Exec(fmt.Sprintf(`ALTER TABLE "%s" ALTER COLUMN data DROP NOT NULL`, tbl)).Error
		_ = db.Exec(fmt.Sprintf(`UPDATE "%s" SET kind = 'image' WHERE kind IS NULL OR kind = ''`, tbl)).Error
	}
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

func mediaURL(c *gin.Context, m SiteMedia) string {
	if m.URL != "" {
		return m.URL
	}
	return fmt.Sprintf("%s/api/v1/storefront/media/%d", publicBaseURL(c), m.ID)
}

func mediaJSON(c *gin.Context, m SiteMedia) gin.H {
	return gin.H{
		"id": m.ID, "kind": m.Kind, "url": mediaURL(c, m), "file_name": m.FileName,
		"size": m.Size, "width": m.Width, "height": m.Height, "created_at": m.CreatedAt,
	}
}

func (h *StorefrontHandler) mediaLimit() int64 {
	if h.Store != nil {
		return maxSiteMediaStorage
	}
	return maxSiteMediaDB
}

func randomKeyPart() string {
	b := make([]byte, 10)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// storeKey: sites/<id>/<yyyymm>/<ngẫu nhiên>.<đuôi> — chỉ gồm ký tự an toàn
func storeKey(siteID uint, ext string) string {
	return fmt.Sprintf("sites/%d/%s/%s.%s", siteID, time.Now().UTC().Format("200601"), randomKeyPart(), ext)
}

func cleanFileName(name string) string {
	name = strings.TrimSpace(path.Base(strings.ReplaceAll(name, "\\", "/")))
	if r := []rune(name); len(r) > 120 {
		name = string(r[:120])
	}
	return name
}

func (h *StorefrontHandler) siteMediaCount(siteID uint) int64 {
	var n int64
	h.DB.Model(&SiteMedia{}).Where("storefront_id = ?", siteID).Count(&n)
	return n
}

// saveImage lưu ảnh đã kiểm tra (kho ngoài nếu có, không thì PostgreSQL) và ghi 1 dòng vào thư viện
func (h *StorefrontHandler) saveImage(ctx context.Context, userID, siteID uint, data []byte, ctype, fileName string, w, hgt int) (SiteMedia, error) {
	m := SiteMedia{UserID: userID, StorefrontID: siteID, Kind: "image", FileName: cleanFileName(fileName), ContentType: ctype, Size: int64(len(data)), Width: w, Height: hgt}
	if h.Store != nil {
		key := storeKey(siteID, allowedSiteMediaTypes[ctype])
		if err := h.Store.Put(ctx, key, ctype, bytes.NewReader(data), int64(len(data))); err != nil {
			return m, err
		}
		m.StorageKey, m.URL = key, h.Store.PublicURL(key)
	} else {
		m.Data = data
	}
	if err := h.DB.Create(&m).Error; err != nil {
		if m.StorageKey != "" {
			_ = h.Store.Delete(ctx, m.StorageKey)
		}
		return m, fmt.Errorf("lưu ảnh thất bại")
	}
	return m, nil
}

// UploadSiteMedia POST /api/v1/storefront/sites/:id/media (multipart, field "file") — ảnh
func (h *StorefrontHandler) UploadSiteMedia(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	if h.siteMediaCount(sf.ID) >= h.mediaLimit() {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Mỗi website tối đa %d ảnh. Hãy xoá bớt ảnh không dùng.", h.mediaLimit())})
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

	ctype, isImage := detectImageType(data)
	if !isImage {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chỉ nhận ảnh JPG, PNG, GIF hoặc WEBP."})
		return
	}
	w, _ := strconv.Atoi(c.PostForm("width"))
	hgt, _ := strconv.Atoi(c.PostForm("height"))

	m, err := h.saveImage(c.Request.Context(), sf.UserID, sf.ID, data, ctype, fh.Filename, w, hgt)
	if err != nil {
		log.Printf("[media] lưu ảnh lỗi: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lưu ảnh thất bại."})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"media": mediaJSON(c, m)})
}

// sniffVideo nhận MP4/MOV/M4V (hộp "ftyp") và WebM (EBML). Trả về content-type chuẩn + đuôi file.
func sniffVideo(head []byte) (ctype, ext string, ok bool) {
	if len(head) >= 12 && string(head[4:8]) == "ftyp" {
		return "video/mp4", "mp4", true
	}
	if len(head) >= 4 && head[0] == 0x1A && head[1] == 0x45 && head[2] == 0xDF && head[3] == 0xA3 {
		return "video/webm", "webm", true
	}
	return "", "", false
}

// UploadSiteVideo POST /api/v1/storefront/sites/:id/media/video (multipart, field "file") — chỉ khi đã có kho ngoài
func (h *StorefrontHandler) UploadSiteVideo(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	if h.Store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Chưa cấu hình kho lưu trữ video (Cloudflare R2 / S3) nên chưa tải video lên được. Bạn vẫn có thể dán link YouTube hoặc link file .mp4."})
		return
	}
	if h.siteMediaCount(sf.ID) >= h.mediaLimit() {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Mỗi website tối đa %d ảnh. Hãy xoá bớt ảnh không dùng.", h.mediaLimit())})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSiteVideoBytes+(2<<20))
	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File video quá lớn (tối đa 80MB) hoặc dữ liệu tải lên không hợp lệ."})
		return
	}
	if fh.Size <= 0 || fh.Size > maxSiteVideoBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Video tối đa %dMB.", maxSiteVideoBytes>>20)})
		return
	}
	f, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Không đọc được file tải lên"})
		return
	}
	defer f.Close()
	head := make([]byte, 16)
	n, _ := io.ReadFull(f, head)
	ctype, ext, ok := sniffVideo(head[:n])
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Định dạng video không hỗ trợ (chỉ nhận MP4, MOV, WEBM, M4V)."})
		return
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Không đọc được file tải lên"})
		return
	}
	key := storeKey(sf.ID, ext)
	if err := h.Store.Put(c.Request.Context(), key, ctype, f, fh.Size); err != nil {
		log.Printf("[media] tải video lên kho lỗi: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Tải video lên kho lưu trữ thất bại: " + shortErr(err)})
		return
	}
	m := SiteMedia{UserID: sf.UserID, StorefrontID: sf.ID, Kind: "video", FileName: cleanFileName(fh.Filename), ContentType: ctype, Size: fh.Size, StorageKey: key, URL: h.Store.PublicURL(key)}
	if err := h.DB.Create(&m).Error; err != nil {
		_ = h.Store.Delete(c.Request.Context(), key)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lưu video thất bại."})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"media": mediaJSON(c, m)})
}

func shortErr(err error) string {
	s := err.Error()
	if r := []rune(s); len(r) > 200 {
		s = string(r[:200])
	}
	return s
}

// mediaFetchClient: client tải ảnh từ link ngoài (test thay bằng bản cho phép máy chủ thử ở 127.0.0.1)
var mediaFetchClient = func() *http.Client { return newSafeHTTPClient(25 * time.Second) }

// ImportSiteMedia POST /api/v1/storefront/sites/:id/media/import {urls:[...]} — máy chủ tải ảnh từ link về thư viện của shop
func (h *StorefrontHandler) ImportSiteMedia(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	var req struct {
		URLs []string `json:"urls"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.URLs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ."})
		return
	}
	if len(req.URLs) > maxImportImages {
		req.URLs = req.URLs[:maxImportImages]
	}
	room := h.mediaLimit() - h.siteMediaCount(sf.ID)
	client := mediaFetchClient()
	type result struct {
		Source string `json:"source"`
		Media  gin.H  `json:"media,omitempty"`
		Error  string `json:"error,omitempty"`
	}
	results := make([]result, 0, len(req.URLs))
	for _, src := range req.URLs {
		if room <= 0 {
			results = append(results, result{Source: src, Error: fmt.Sprintf("Mỗi website tối đa %d ảnh.", h.mediaLimit())})
			continue
		}
		// Ưu tiên WebP/JPEG/PNG (tương thích rộng); AVIF chỉ khi nguồn không có định dạng khác
		data, _, final, err := fetchLimited(client, src, maxSiteMediaBytes, map[string]string{"Accept": "image/webp,image/jpeg,image/png,image/gif,image/avif;q=0.5"})
		if err != nil {
			results = append(results, result{Source: src, Error: "Không tải được ảnh: " + shortErr(err)})
			continue
		}
		ctype, isImage := detectImageType(data)
		if !isImage {
			results = append(results, result{Source: src, Error: "Không phải ảnh JPG, PNG, GIF, WEBP hoặc AVIF."})
			continue
		}
		name := path.Base(strings.SplitN(final, "?", 2)[0])
		m, err := h.saveImage(c.Request.Context(), sf.UserID, sf.ID, data, ctype, name, 0, 0)
		if err != nil {
			results = append(results, result{Source: src, Error: "Lưu ảnh thất bại."})
			continue
		}
		room--
		results = append(results, result{Source: src, Media: mediaJSON(c, m)})
	}
	c.JSON(http.StatusOK, gin.H{"results": results})
}

// MediaConfig GET /api/v1/storefront/media-config — giao diện dùng để biết có bật tải video lên không
func (h *StorefrontHandler) MediaConfig(c *gin.Context) {
	storageMode, publicBase := "db", ""
	if h.Store != nil {
		storageMode, publicBase = "s3", h.Store.PublicBase
	}
	c.JSON(http.StatusOK, gin.H{
		"storage":      storageMode,
		"video_upload": h.Store != nil,
		"max_image_mb": maxSiteMediaBytes >> 20,
		"max_video_mb": maxSiteVideoBytes >> 20,
		"limit":        h.mediaLimit(),
		"public_base":  publicBase,
	})
}

// ListSiteMedia GET /api/v1/storefront/sites/:id/media[?kind=image|video|all] (mặc định: ảnh)
func (h *StorefrontHandler) ListSiteMedia(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	q := h.DB.Select("id, storefront_id, kind, file_name, content_type, size, width, height, url, created_at").Where("storefront_id = ?", sf.ID)
	switch c.DefaultQuery("kind", "image") {
	case "video":
		q = q.Where("kind = ?", "video")
	case "all":
	default:
		q = q.Where("kind = ? OR kind IS NULL OR kind = ''", "image")
	}
	var list []SiteMedia
	if err := q.Order("id DESC").Limit(int(h.mediaLimit())).Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không tải được thư viện ảnh."})
		return
	}
	out := make([]gin.H, 0, len(list))
	for _, m := range list {
		if m.Kind == "" {
			m.Kind = "image"
		}
		out = append(out, mediaJSON(c, m))
	}
	c.JSON(http.StatusOK, gin.H{"media": out, "max": h.mediaLimit()})
}

// DeleteSiteMedia DELETE /api/v1/storefront/sites/:id/media/:mid
func (h *StorefrontHandler) DeleteSiteMedia(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	var m SiteMedia
	if err := h.DB.Select("id, storage_key").Where("id = ? AND storefront_id = ?", c.Param("mid"), sf.ID).First(&m).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy ảnh."})
		return
	}
	if err := h.DB.Delete(&SiteMedia{}, m.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Xoá ảnh thất bại"})
		return
	}
	if m.StorageKey != "" && h.Store != nil {
		if err := h.Store.Delete(c.Request.Context(), m.StorageKey); err != nil {
			log.Printf("[media] xoá file %s khỏi kho lỗi: %v", m.StorageKey, err)
		}
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
	if err := h.DB.Select("id, content_type, data, url").Where("id = ?", id).First(&m).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	if m.URL != "" { // file nằm ở kho ngoài
		c.Header("Cache-Control", "public, max-age=3600")
		c.Redirect(http.StatusFound, m.URL)
		return
	}
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	c.Data(http.StatusOK, m.ContentType, m.Data)
}
