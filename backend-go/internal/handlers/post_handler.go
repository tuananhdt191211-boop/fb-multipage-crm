package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fb-multipage-crm/internal/config"
	"fb-multipage-crm/internal/models"
	"fb-multipage-crm/internal/security"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PostHandler struct {
	DB         *gorm.DB
	Cfg        *config.Config
	HTTPClient *http.Client
}

func NewPostHandler(db *gorm.DB, cfg *config.Config) *PostHandler {
	_ = db.AutoMigrate(&models.ScheduledPost{})
	h := &PostHandler{
		DB:         db,
		Cfg:        cfg,
		HTTPClient: &http.Client{Timeout: 180 * time.Second},
	}
	go h.startSchedulerWorker()
	return h
}

func (h *PostHandler) encKey() string {
	if h.Cfg != nil && h.Cfg.JWTSecret != "" {
		return h.Cfg.JWTSecret
	}
	return "default-dev-master-key-change-in-prod"
}

type pagePublishResult struct {
	PageID   string `json:"page_id"`
	PageName string `json:"page_name"`
	Success  bool   `json:"success"`
	PostID   string `json:"post_id,omitempty"`
	Error    string `json:"error,omitempty"`
}

// startSchedulerWorker tự động đăng các bài viết đã đến giờ hẹn.
// Không hỏi database liên tục: ngủ tới đúng giờ bài kế tiếp (tối đa idleSleep),
// và được đánh thức ngay khi có bài hẹn giờ mới (schedulerWake) -> database Neon được nghỉ khi rảnh.
func (h *PostHandler) startSchedulerWorker() {
	time.Sleep(20 * time.Second)
	for {
		var duePosts []models.ScheduledPost
		now := time.Now()
		if err := h.DB.Where("status = ? AND scheduled_at <= ?", "pending", now).
			Order("scheduled_at ASC").
			Limit(10).
			Find(&duePosts).Error; err != nil {
			schedulerWake.Sleep(time.Minute)
			continue
		}

		for _, post := range duePosts {
			res := h.DB.Model(&models.ScheduledPost{}).
				Where("id = ? AND status = ?", post.ID, "pending").
				Update("status", "processing")
			if res.RowsAffected == 0 {
				continue
			}
			h.executePublishPost(post.ID)
		}
		if len(duePosts) == 10 {
			continue // còn bài đến hạn
		}

		// Ngủ tới giờ của bài kế tiếp
		wait := idleSleep
		var next models.ScheduledPost
		if h.DB.Where("status = ?", "pending").Order("scheduled_at ASC").Limit(1).Find(&next).RowsAffected > 0 {
			if d := time.Until(next.ScheduledAt); d < wait {
				wait = d
			}
		}
		if wait < 5*time.Second {
			wait = 5 * time.Second
		}
		schedulerWake.Sleep(wait)
	}
}

// executePublishPost thực thi đăng bài lên từng Fanpage qua Facebook Graph API
func (h *PostHandler) executePublishPost(postID uint) {
	var post models.ScheduledPost
	if err := h.DB.First(&post, postID).Error; err != nil {
		return
	}

	var pageIDs []string
	_ = json.Unmarshal([]byte(post.PageIDsJSON), &pageIDs)

	var results []pagePublishResult
	successCount := 0
	failedCount := 0

	for _, pid := range pageIDs {
		var page models.FacebookPage
		if err := h.DB.Where("page_id = ?", pid).First(&page).Error; err != nil {
			failedCount++
			results = append(results, pagePublishResult{
				PageID:  pid,
				Success: false,
				Error:   "Không tìm thấy Fanpage trong hệ thống",
			})
			continue
		}

		pageToken, err := security.DecryptSecret(page.EncryptedPageToken, h.encKey())
		if err != nil || pageToken == "" {
			failedCount++
			results = append(results, pagePublishResult{
				PageID:   page.PageID,
				PageName: page.Name,
				Success:  false,
				Error:    "Lỗi giải mã Page Access Token",
			})
			continue
		}

		fbPostID, errMsg := h.publishToSinglePage(page.PageID, pageToken, post.Content, post.MediaType, post.MediaURL)
		if errMsg != "" {
			failedCount++
			results = append(results, pagePublishResult{
				PageID:   page.PageID,
				PageName: page.Name,
				Success:  false,
				Error:    errMsg,
			})
		} else {
			successCount++
			results = append(results, pagePublishResult{
				PageID:   page.PageID,
				PageName: page.Name,
				Success:  true,
				PostID:   fbPostID,
			})
		}
	}

	finalStatus := "published"
	if successCount == 0 && failedCount > 0 {
		finalStatus = "failed"
	} else if successCount > 0 && failedCount > 0 {
		finalStatus = "partial"
	}

	resBytes, _ := json.Marshal(results)
	post.Status = finalStatus
	post.SuccessCount = successCount
	post.FailedCount = failedCount
	post.PublishResults = string(resBytes)
	_ = h.DB.Save(&post).Error
}

func (h *PostHandler) publishToSinglePage(pageID, pageToken, content, mediaType, mediaURL string) (string, string) {
	trimmedMedia := strings.TrimSpace(mediaURL)

	// 1. Đăng Ảnh (hỗ trợ cả file upload nội bộ /uploads/... lẫn URL công khai)
	if trimmedMedia != "" && mediaType == "image" {
		return h.uploadPhotoToPage(pageID, pageToken, content, trimmedMedia)
	}

	// 2. Đăng Video (hỗ trợ cả file upload nội bộ /uploads/... lẫn URL công khai)
	if trimmedMedia != "" && mediaType == "video" {
		return h.uploadVideoToPage(pageID, pageToken, content, trimmedMedia)
	}

	// 3. Đăng bài chỉ có văn bản
	endpoint := fmt.Sprintf("https://graph.facebook.com/v21.0/%s/feed", pageID)
	payload := map[string]interface{}{
		"message":      content,
		"access_token": pageToken,
	}

	bodyBytes, _ := json.Marshal(payload)
	resp, err := h.HTTPClient.Post(endpoint, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", "Không kết nối được Facebook Graph API"
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var fbRes struct {
		ID     string `json:"id"`
		PostID string `json:"post_id"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(respBody, &fbRes)
	if fbRes.Error != nil {
		return "", fbRes.Error.Message
	}

	if fbRes.PostID != "" {
		return fbRes.PostID, ""
	}
	return fbRes.ID, ""
}

func (h *PostHandler) uploadPhotoToPage(pageID, pageToken, caption, imageSource string) (string, string) {
	endpoint := fmt.Sprintf("https://graph.facebook.com/v21.0/%s/photos", pageID)
	localPath := strings.TrimPrefix(imageSource, "/")

	// Nếu là file ảnh upload nội bộ trên server -> đẩy trực tiếp bytes bằng multipart/form-data
	if strings.HasPrefix(localPath, "uploads/") {
		diskPath := filepath.FromSlash(localPath)
		fileBytes, err := os.ReadFile(diskPath)
		if err != nil {
			return "", fmt.Sprintf("Không tìm thấy file ảnh trên server (%s): %v", diskPath, err)
		}

		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		_ = writer.WriteField("access_token", pageToken)
		if strings.TrimSpace(caption) != "" {
			_ = writer.WriteField("caption", caption)
		}

		part, err := writer.CreateFormFile("source", filepath.Base(diskPath))
		if err != nil {
			return "", "Lỗi đóng gói file ảnh"
		}
		if _, err := part.Write(fileBytes); err != nil {
			return "", "Lỗi ghi dữ liệu file ảnh"
		}
		_ = writer.Close()

		req, err := http.NewRequest(http.MethodPost, endpoint, &body)
		if err != nil {
			return "", "Không tạo được request upload ảnh"
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := h.HTTPClient.Do(req)
		if err != nil {
			return "", "Không kết nối được Facebook Graph API khi tải ảnh"
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)

		var fbRes struct {
			ID     string `json:"id"`
			PostID string `json:"post_id"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(respBody, &fbRes)
		if fbRes.Error != nil {
			return "", fbRes.Error.Message
		}
		if fbRes.PostID != "" {
			return fbRes.PostID, ""
		}
		return fbRes.ID, ""
	}

	// Nếu là URL ảnh công khai
	payload := map[string]interface{}{
		"url":          imageSource,
		"caption":      caption,
		"access_token": pageToken,
	}
	bodyBytes, _ := json.Marshal(payload)
	resp, err := h.HTTPClient.Post(endpoint, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", "Không kết nối được Facebook Graph API"
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var fbRes struct {
		ID     string `json:"id"`
		PostID string `json:"post_id"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(respBody, &fbRes)
	if fbRes.Error != nil {
		return "", fbRes.Error.Message
	}
	if fbRes.PostID != "" {
		return fbRes.PostID, ""
	}
	return fbRes.ID, ""
}

func (h *PostHandler) uploadVideoToPage(pageID, pageToken, description, videoSource string) (string, string) {
	graphVideoURL := fmt.Sprintf("https://graph-video.facebook.com/v21.0/%s/videos", pageID)
	localPath := strings.TrimPrefix(videoSource, "/")

	// 1. Nếu là file video nội bộ trên server (/uploads/posts/...) -> upload trực tiếp file bytes bằng multipart/form-data
	if strings.HasPrefix(localPath, "uploads/") {
		diskPath := filepath.FromSlash(localPath)
		fileBytes, err := os.ReadFile(diskPath)
		if err != nil {
			return "", fmt.Sprintf("Không tìm thấy file video trên server (%s): %v", diskPath, err)
		}

		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		_ = writer.WriteField("access_token", pageToken)
		if strings.TrimSpace(description) != "" {
			_ = writer.WriteField("description", description)
		}

		part, err := writer.CreateFormFile("source", filepath.Base(diskPath))
		if err != nil {
			return "", "Lỗi đóng gói file video"
		}
		if _, err := part.Write(fileBytes); err != nil {
			return "", "Lỗi ghi dữ liệu file video"
		}
		_ = writer.Close()

		req, err := http.NewRequest(http.MethodPost, graphVideoURL, &body)
		if err != nil {
			return "", "Không tạo được request upload video"
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := h.HTTPClient.Do(req)
		if err != nil {
			return "", fmt.Sprintf("Lỗi kết nối khi tải video lên Facebook: %v", err)
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)

		var fbRes struct {
			ID    string `json:"id"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(respBody, &fbRes)
		if fbRes.Error != nil {
			return "", fbRes.Error.Message
		}
		if fbRes.ID == "" {
			return "", fmt.Sprintf("Facebook không trả về ID video: %s", string(respBody))
		}
		return fbRes.ID, ""
	}

	// 2. Nếu là URL video công khai (https://...mp4) -> gửi qua file_url
	payload := map[string]interface{}{
		"file_url":     videoSource,
		"description":  description,
		"access_token": pageToken,
	}
	bodyBytes, _ := json.Marshal(payload)
	resp, err := h.HTTPClient.Post(graphVideoURL, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", "Không kết nối được Facebook Graph API"
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var fbRes struct {
		ID    string `json:"id"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(respBody, &fbRes)
	if fbRes.Error != nil {
		return "", fbRes.Error.Message
	}
	return fbRes.ID, ""
}

// ListScheduledPosts trả về danh sách lịch đăng bài và danh sách Fanpage đang bật
func (h *PostHandler) ListScheduledPosts(c *gin.Context) {
	var posts []models.ScheduledPost
	if err := h.DB.Order("scheduled_at DESC, id DESC").Limit(100).Find(&posts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể tải danh sách bài đăng"})
		return
	}

	var pages []models.FacebookPage
	_ = h.DB.Where("is_connected = ?", true).Order("name ASC").Find(&pages).Error

	c.JSON(http.StatusOK, gin.H{
		"posts": posts,
		"pages": pages,
	})
}

type createScheduledPostReq struct {
	Content     string   `json:"content"`
	MediaType   string   `json:"media_type"`
	MediaURL    string   `json:"media_url"`
	PageIDs     []string `json:"page_ids" binding:"required"`
	ScheduledAt string   `json:"scheduled_at"` // ISO 8601 hoặc rỗng nếu publish_now = true
	PublishNow  bool     `json:"publish_now"`
}

// CreateScheduledPost tạo lịch đăng bài hoặc đăng ngay lập tức lên nhiều Page
func (h *PostHandler) CreateScheduledPost(c *gin.Context) {
	var req createScheduledPostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng chọn ít nhất 1 Fanpage"})
		return
	}

	content := strings.TrimSpace(req.Content)
	mediaURL := strings.TrimSpace(req.MediaURL)
	if (content == "" && mediaURL == "") || len(req.PageIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng nhập nội dung (hoặc đính kèm ảnh/video) và chọn ít nhất 1 Fanpage"})
		return
	}

	var pages []models.FacebookPage
	_ = h.DB.Where("page_id IN ?", req.PageIDs).Find(&pages).Error
	var pageNames []string
	for _, p := range pages {
		pageNames = append(pageNames, p.Name)
	}

	schedTime := time.Now()
	if !req.PublishNow && strings.TrimSpace(req.ScheduledAt) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(req.ScheduledAt))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Định dạng thời gian hẹn đăng không hợp lệ"})
			return
		}
		schedTime = parsed
	}

	mediaType := strings.TrimSpace(req.MediaType)
	if mediaType == "" {
		mediaType = "none"
	}

	idsBytes, _ := json.Marshal(req.PageIDs)
	namesBytes, _ := json.Marshal(pageNames)

	initialStatus := "pending"
	if req.PublishNow {
		initialStatus = "processing"
	}

	post := models.ScheduledPost{
		Content:       content,
		MediaType:     mediaType,
		MediaURL:      mediaURL,
		PageIDsJSON:   string(idsBytes),
		PageNamesJSON: string(namesBytes),
		ScheduledAt:   schedTime,
		Status:        initialStatus,
	}

	if err := h.DB.Create(&post).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể lưu lịch đăng bài"})
		return
	}
	// Có bài hẹn giờ mới -> đánh thức bộ lập lịch để tính lại giờ đăng kế tiếp
	schedulerWake.Wake()

	if req.PublishNow {
		h.executePublishPost(post.ID)
		_ = h.DB.First(&post, post.ID).Error
		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("Đã đăng bài lên %d/%d Fanpage!", post.SuccessCount, len(req.PageIDs)),
			"post":    post,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": fmt.Sprintf("Đã lên lịch đăng bài cho %d Fanpage!", len(req.PageIDs)),
		"post":    post,
	})
}

// PublishNowPost kích hoạt đăng ngay một bài đang chờ lịch
func (h *PostHandler) PublishNowPost(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID bài viết không hợp lệ"})
		return
	}

	var post models.ScheduledPost
	if err := h.DB.First(&post, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy bài viết"})
		return
	}

	post.Status = "processing"
	_ = h.DB.Save(&post).Error
	h.executePublishPost(post.ID)
	_ = h.DB.First(&post, post.ID).Error

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Hoàn tất đăng bài (%d thành công, %d lỗi)", post.SuccessCount, post.FailedCount),
		"post":    post,
	})
}

// DeleteScheduledPost xóa bài viết khỏi hàng đợi
func (h *PostHandler) DeleteScheduledPost(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID bài viết không hợp lệ"})
		return
	}
	if err := h.DB.Delete(&models.ScheduledPost{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể xóa bài viết"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã xóa bài viết khỏi lịch đăng"})
}
