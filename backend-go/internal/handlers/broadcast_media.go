package handlers

// Ảnh / video đính kèm cho chiến dịch Broadcast
// ---------------------------------------------------------------------------
// - POST /api/v1/broadcasts/upload-media lưu file vào ./uploads/broadcast
// - Khi gửi, mỗi file chỉ được tải lên Meta 1 lần / Page (Attachment Upload API,
//   is_reusable = true), attachment_id được lưu cache và dùng lại cho mọi khách.
// - Tin chữ gửi trước, sau đó lần lượt từng ảnh/video với cùng messaging_type/tag.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	maxBroadcastAttachments = 5
	maxBroadcastImageBytes  = 8 << 20  // 8MB
	maxBroadcastVideoBytes  = 25 << 20 // 25MB
	broadcastUploadDir      = "uploads/broadcast"
	broadcastUploadURLPath  = "/uploads/broadcast/"
)

var (
	broadcastImageExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}
	broadcastVideoExts = map[string]bool{".mp4": true, ".mov": true, ".webm": true, ".m4v": true}
)

// BroadcastAttachment là 1 ảnh/video của chiến dịch
type BroadcastAttachment struct {
	Type string `json:"type"` // image | video
	URL  string `json:"url"`  // /uploads/broadcast/<file>
	Name string `json:"name"`
}

// BroadcastAttachmentCache lưu attachment_id Meta đã cấp cho từng file trên từng Page
type BroadcastAttachmentCache struct {
	ID           uint      `gorm:"primaryKey"`
	PageID       string    `gorm:"size:100;not null;uniqueIndex:idx_bc_att_page_url"`
	URL          string    `gorm:"size:500;not null;uniqueIndex:idx_bc_att_page_url"`
	AttachmentID string    `gorm:"size:100;not null"`
	CreatedAt    time.Time
}

func migrateBroadcastMediaTables(db *gorm.DB) {
	_ = db.AutoMigrate(&BroadcastAttachmentCache{})
}

// localPathFromBroadcastURL chuyển "/uploads/broadcast/x.jpg" thành đường dẫn file an toàn
func localPathFromBroadcastURL(u string) (string, bool) {
	if !strings.HasPrefix(u, broadcastUploadURLPath) {
		return "", false
	}
	name := strings.TrimPrefix(u, broadcastUploadURLPath)
	if name == "" || name != filepath.Base(name) || strings.Contains(name, "..") {
		return "", false
	}
	return filepath.Join(filepath.FromSlash(broadcastUploadDir), name), true
}

// validateBroadcastAttachments chỉ chấp nhận file đã tải lên qua upload-media
func validateBroadcastAttachments(list []BroadcastAttachment) ([]BroadcastAttachment, error) {
	if len(list) > maxBroadcastAttachments {
		return nil, fmt.Errorf("Tối đa %d ảnh/video cho mỗi chiến dịch", maxBroadcastAttachments)
	}
	out := make([]BroadcastAttachment, 0, len(list))
	for _, a := range list {
		a.URL = strings.TrimSpace(a.URL)
		if a.Type != "image" && a.Type != "video" {
			return nil, fmt.Errorf("Loại đính kèm không hợp lệ: %s", a.Type)
		}
		p, ok := localPathFromBroadcastURL(a.URL)
		if !ok {
			return nil, fmt.Errorf("Đường dẫn file không hợp lệ: %s", a.URL)
		}
		if _, err := os.Stat(p); err != nil {
			return nil, fmt.Errorf("Không tìm thấy file %s, hãy tải lên lại", a.Name)
		}
		out = append(out, a)
	}
	return out, nil
}

// UploadMedia POST /api/v1/broadcasts/upload-media (multipart, field "files")
func (h *BroadcastHandler) UploadMedia(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil || form == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Không đọc được file tải lên"})
		return
	}
	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng chọn ít nhất 1 ảnh hoặc video"})
		return
	}
	if err := os.MkdirAll(filepath.FromSlash(broadcastUploadDir), 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không tạo được thư mục lưu file"})
		return
	}

	uploaded := []BroadcastAttachment{}
	rejected := []string{}
	for idx, f := range files {
		if idx >= maxBroadcastAttachments {
			rejected = append(rejected, fmt.Sprintf("%s (vượt quá %d file)", f.Filename, maxBroadcastAttachments))
			continue
		}
		ext := strings.ToLower(filepath.Ext(f.Filename))
		kind := ""
		limit := int64(0)
		switch {
		case broadcastImageExts[ext]:
			kind, limit = "image", maxBroadcastImageBytes
		case broadcastVideoExts[ext]:
			kind, limit = "video", maxBroadcastVideoBytes
		default:
			rejected = append(rejected, f.Filename+" (định dạng không hỗ trợ)")
			continue
		}
		if f.Size > limit {
			rejected = append(rejected, fmt.Sprintf("%s (vượt %dMB)", f.Filename, limit>>20))
			continue
		}

		// Kiểm tra nội dung thật của ảnh (tránh đổi đuôi file)
		if kind == "image" {
			src, err := f.Open()
			if err != nil {
				rejected = append(rejected, f.Filename+" (không đọc được)")
				continue
			}
			head := make([]byte, 512)
			n, _ := io.ReadFull(src, head)
			src.Close()
			if !strings.HasPrefix(http.DetectContentType(head[:n]), "image/") {
				rejected = append(rejected, f.Filename+" (không phải file ảnh)")
				continue
			}
		}

		filename := fmt.Sprintf("%s_%d_%d%s", kind, time.Now().UnixNano(), idx, ext)
		dst := filepath.Join(filepath.FromSlash(broadcastUploadDir), filename)
		if err := c.SaveUploadedFile(f, dst); err != nil {
			rejected = append(rejected, f.Filename+" (lỗi lưu file)")
			continue
		}
		uploaded = append(uploaded, BroadcastAttachment{
			Type: kind,
			URL:  broadcastUploadURLPath + filename,
			Name: f.Filename,
		})
	}

	if len(uploaded) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Không có file hợp lệ: " + strings.Join(rejected, ", "), "rejected": rejected})
		return
	}
	c.JSON(http.StatusOK, gin.H{"files": uploaded, "rejected": rejected})
}

// resolveAttachmentID lấy attachment_id từ cache hoặc tải file lên Meta (1 lần / Page)
func (h *BroadcastHandler) resolveAttachmentID(pageID, pageToken string, att BroadcastAttachment) (string, error) {
	var cached BroadcastAttachmentCache
	if h.DB.Where("page_id = ? AND url = ?", pageID, att.URL).Limit(1).Find(&cached).RowsAffected > 0 && cached.AttachmentID != "" {
		return cached.AttachmentID, nil
	}

	p, ok := localPathFromBroadcastURL(att.URL)
	if !ok {
		return "", fmt.Errorf("đường dẫn file không hợp lệ")
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return "", fmt.Errorf("không đọc được file %s: %v", att.Name, err)
	}

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	msgJSON, _ := json.Marshal(map[string]any{
		"attachment": map[string]any{
			"type":    att.Type,
			"payload": map[string]any{"is_reusable": true},
		},
	})
	_ = w.WriteField("message", string(msgJSON))

	ctype := mime.TypeByExtension(strings.ToLower(filepath.Ext(p)))
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition", fmt.Sprintf(`form-data; name="filedata"; filename="%s"`, filepath.Base(p)))
	hdr.Set("Content-Type", ctype)
	part, err := w.CreatePart(hdr)
	if err != nil {
		return "", err
	}
	_, _ = part.Write(data)
	_ = w.Close()

	uploadURL := fmt.Sprintf("https://graph.facebook.com/v21.0/%s/message_attachments?access_token=%s", pageID, url.QueryEscape(pageToken))
	req, _ := http.NewRequest(http.MethodPost, uploadURL, &body)
	req.Header.Set("Content-Type", w.FormDataContentType())

	client := &http.Client{Timeout: 5 * time.Minute} // video có thể tải lâu
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("không tải được file lên Facebook: %v", err)
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)

	var res struct {
		AttachmentID string `json:"attachment_id"`
		Error        *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(respBytes, &res)
	if res.Error != nil {
		return "", fmt.Errorf("Meta từ chối file %s (Code %d): %s", att.Name, res.Error.Code, res.Error.Message)
	}
	if res.AttachmentID == "" {
		return "", fmt.Errorf("Meta không trả attachment_id cho %s", att.Name)
	}

	_ = h.DB.Where("page_id = ? AND url = ?", pageID, att.URL).Delete(&BroadcastAttachmentCache{}).Error
	_ = h.DB.Create(&BroadcastAttachmentCache{PageID: pageID, URL: att.URL, AttachmentID: res.AttachmentID}).Error
	log.Printf("[Broadcast] Đã tải %s lên Page %s (attachment_id %s)", att.Name, pageID, res.AttachmentID)
	return res.AttachmentID, nil
}

// sendBroadcastContent gửi chữ + ảnh/video cho 1 khách.
// Trả về: message_id đầu tiên, mã lỗi, thông báo lỗi (khi không gửi được gì), cảnh báo (khi lỗi một phần).
func (h *BroadcastHandler) sendBroadcastContent(pageID, pageToken, psid, text string, camp BroadcastCampaign) (string, int, string, string) {
	usedType, usedTag := "", ""
	firstMID := ""
	lastCode := 0

	if strings.TrimSpace(text) != "" {
		mid, code, errMsg, t, tg := h.sendDirectPayload(pageID, pageToken, psid, map[string]any{"text": text}, camp.OutOfWindowTag)
		if errMsg != "" {
			return "", code, errMsg, ""
		}
		firstMID, usedType, usedTag = mid, t, tg
	}

	var warns []string
	for i, att := range camp.Attachments {
		attID, err := h.resolveAttachmentID(pageID, pageToken, att)
		if err != nil {
			warns = append(warns, fmt.Sprintf("File %d: %v", i+1, err))
			continue
		}
		message := map[string]any{
			"attachment": map[string]any{
				"type":    att.Type,
				"payload": map[string]any{"attachment_id": attID},
			},
		}

		var mid, errMsg string
		var code int
		if usedType == "" {
			mid, code, errMsg, usedType, usedTag = h.sendDirectPayload(pageID, pageToken, psid, message, camp.OutOfWindowTag)
		} else {
			mid, code, errMsg = h.postFBMessage(pageID, pageToken, psid, message, usedType, usedTag)
		}
		if errMsg != "" {
			lastCode = code
			if code == 100 {
				// attachment_id có thể đã hết hạn -> xoá cache để lần sau tải lại
				_ = h.DB.Where("page_id = ? AND url = ?", pageID, att.URL).Delete(&BroadcastAttachmentCache{}).Error
			}
			if firstMID == "" && (code == 4 || code == 17 || code == 32 || code == 613) {
				return "", code, errMsg, ""
			}
			warns = append(warns, fmt.Sprintf("File %d (Code %d): %s", i+1, code, errMsg))
			continue
		}
		if firstMID == "" {
			firstMID = mid
		}
	}

	if firstMID == "" {
		if lastCode == 0 {
			lastCode = -1
		}
		return "", lastCode, "Không gửi được ảnh/video: " + strings.Join(warns, "; "), ""
	}
	if len(warns) > 0 {
		return firstMID, 0, "", "Đã gửi nhưng lỗi một số ảnh/video: " + truncateRunes(strings.Join(warns, "; "), 500)
	}
	return firstMID, 0, "", ""
}
