package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fb-multipage-crm/internal/config"
	"fb-multipage-crm/internal/models"
	"fb-multipage-crm/internal/security"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// BroadcastAttachment là 1 file ảnh/video đính kèm chiến dịch.
// AttachmentID được Facebook cấp sau lần upload đầu tiên và dùng lại cho mọi khách.
type BroadcastAttachment struct {
	Type         string `json:"type"` // image | video
	URL          string `json:"url"`
	Name         string `json:"name,omitempty"`
	AttachmentID string `json:"attachment_id,omitempty"`
}

const (
	broadcastUploadDir     = "uploads/broadcast"
	broadcastMaxAttach     = 5
	broadcastMaxImageBytes = 8 << 20  // Messenger: ảnh tối đa 8MB
	broadcastMaxVideoBytes = 25 << 20 // Messenger: video tối đa 25MB
)

type BroadcastCampaign struct {
	ID              uint                  `gorm:"primaryKey" json:"id"`
	PageID          string                `gorm:"index;not null" json:"page_id"`
	Name            string                `gorm:"not null" json:"name"`
	TemplatesJSON   string                `gorm:"type:text;not null" json:"-"`
	AttachmentsJSON string                `gorm:"type:text" json:"-"`
	Attachments     []BroadcastAttachment `gorm:"-" json:"attachments"`
	TargetMode      string                `gorm:"default:'all'" json:"target_mode"`
	OutOfWindowTag  string                `gorm:"default:'HUMAN_AGENT'" json:"out_of_window_tag"`
	MinDelaySec     int                   `gorm:"default:4" json:"min_delay_sec"`
	MaxDelaySec     int                   `gorm:"default:9" json:"max_delay_sec"`
	BatchSize       int                   `gorm:"default:20" json:"batch_size"`
	BatchPauseSec   int                   `gorm:"default:60" json:"batch_pause_sec"`
	Status          string                `gorm:"index;default:'running'" json:"status"`
	PauseReason     string                `json:"pause_reason"`
	TotalRecipients int                   `json:"total_recipients"`
	SentCount       int                   `json:"sent_count"`
	FailedCount     int                   `json:"failed_count"`
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
}

type BroadcastMessage struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	CampaignID     uint       `gorm:"index;not null" json:"campaign_id"`
	PageID         string     `gorm:"index;not null" json:"page_id"`
	ConversationID uint       `gorm:"index;not null" json:"conversation_id"`
	RecipientPSID  string     `gorm:"not null" json:"recipient_psid"`
	RecipientName  string     `json:"recipient_name"`
	RenderedText   string     `gorm:"type:text;not null" json:"rendered_text"`
	Status         string     `gorm:"index;default:'pending'" json:"status"`
	FBMessageID    string     `json:"fb_message_id"`
	ErrorCode      int        `json:"error_code"`
	ErrorMessage   string     `json:"error_message"`
	SentAt         *time.Time `json:"sent_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

type BroadcastHandler struct {
	DB         *gorm.DB
	Cfg        *config.Config
	HTTPClient *http.Client
	vnLoc      *time.Location
}

func NewBroadcastHandler(db *gorm.DB, cfg *config.Config) *BroadcastHandler {
	_ = db.AutoMigrate(&BroadcastCampaign{}, &BroadcastMessage{}, &models.Conversation{})

	_ = db.Exec(`
		ALTER TABLE broadcast_messages
		  ADD COLUMN IF NOT EXISTS page_id TEXT,
		  ADD COLUMN IF NOT EXISTS conversation_id BIGINT,
		  ADD COLUMN IF NOT EXISTS recipient_psid TEXT,
		  ADD COLUMN IF NOT EXISTS recipient_name TEXT,
		  ADD COLUMN IF NOT EXISTS rendered_text TEXT,
		  ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'pending',
		  ADD COLUMN IF NOT EXISTS fb_message_id TEXT,
		  ADD COLUMN IF NOT EXISTS error_code INT DEFAULT 0,
		  ADD COLUMN IF NOT EXISTS error_message TEXT,
		  ADD COLUMN IF NOT EXISTS sent_at TIMESTAMPTZ,
		  ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ DEFAULT NOW()
	`).Error

	_ = db.Exec(`
		DO $$
		BEGIN
		  IF EXISTS (
		    SELECT 1 FROM information_schema.columns
		    WHERE table_name = 'conversations' AND column_name = 'psid'
		  ) THEN
		    EXECUTE 'UPDATE conversations SET customer_psid = psid WHERE COALESCE(customer_psid, '''') = '''' AND COALESCE(psid, '''') <> ''''';
		  ELSIF EXISTS (
		    SELECT 1 FROM information_schema.columns
		    WHERE table_name = 'conversations' AND column_name = 'sender_id'
		  ) THEN
		    EXECUTE 'UPDATE conversations SET customer_psid = sender_id WHERE COALESCE(customer_psid, '''') = '''' AND COALESCE(sender_id, '''') <> ''''';
		  END IF;
		END $$;
	`).Error

	_ = db.Model(&BroadcastMessage{}).Where("status = ?", "processing").Update("status", "pending").Error

	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.FixedZone("ICT", 7*3600)
	}

	h := &BroadcastHandler{
		DB:         db,
		Cfg:        cfg,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		vnLoc:      loc,
	}
	go h.startWorker()
	return h
}

func (h *BroadcastHandler) getEncryptionKey() string {
	if h.Cfg != nil && h.Cfg.JWTSecret != "" {
		return h.Cfg.JWTSecret
	}
	return "default-dev-master-key-change-in-prod"
}

// metaSyncStats thống kê kết quả quét để báo lỗi rõ ràng cho người dùng
type metaSyncStats struct {
	Threads   int    // số hội thoại Meta trả về
	NoPSID    int    // hội thoại không xác định được khách
	Saved     int    // số khách lưu được vào CRM
	SaveError string // lỗi lưu DB đầu tiên (nếu có)
}

// pullAllConversationsFromMeta quét phân trang toàn bộ khách hàng có trong Meta Business Inbox của Page
func (h *BroadcastHandler) pullAllConversationsFromMeta(page models.FacebookPage, pageToken string, maxCount int) (metaSyncStats, error) {
	var stats metaSyncStats
	if maxCount <= 0 {
		maxCount = 2000
	}
	nextURL := fmt.Sprintf(
		"https://graph.facebook.com/v21.0/%s/conversations?fields=id,updated_time,participants,messages.limit(5){from}&limit=100&access_token=%s",
		page.PageID, pageToken,
	)

	fetched := 0
	for nextURL != "" && fetched < maxCount {
		resp, err := h.HTTPClient.Get(nextURL)
		if err != nil {
			return stats, fmt.Errorf("không kết nối được Graph API: %v", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var fbData struct {
			Data []struct {
				ID           string `json:"id"`
				UpdatedTime  string `json:"updated_time"`
				Participants struct {
					Data []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"data"`
				} `json:"participants"`
				Messages struct {
					Data []struct {
						From struct {
							ID   string `json:"id"`
							Name string `json:"name"`
						} `json:"from"`
					} `json:"data"`
				} `json:"messages"`
			} `json:"data"`
			Paging struct {
				Next string `json:"next"`
			} `json:"paging"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}

		if err := json.Unmarshal(body, &fbData); err != nil {
			return stats, fmt.Errorf("lỗi đọc dữ liệu từ Meta: %v", err)
		}
		if fbData.Error != nil {
			return stats, fmt.Errorf("Meta báo lỗi: %s", fbData.Error.Message)
		}

		for _, thread := range fbData.Data {
			stats.Threads++
			customerPSID := ""
			customerName := "Khách hàng Facebook"
			for _, p := range thread.Participants.Data {
				if p.ID != page.PageID && strings.TrimSpace(p.ID) != "" {
					customerPSID = strings.TrimSpace(p.ID)
					if strings.TrimSpace(p.Name) != "" {
						customerName = strings.TrimSpace(p.Name)
					}
					break
				}
			}
			if customerPSID == "" {
				for _, m := range thread.Messages.Data {
					if m.From.ID != page.PageID && strings.TrimSpace(m.From.ID) != "" {
						customerPSID = strings.TrimSpace(m.From.ID)
						if strings.TrimSpace(m.From.Name) != "" {
							customerName = strings.TrimSpace(m.From.Name)
						}
						break
					}
				}
			}
			if customerPSID == "" {
				stats.NoPSID++
				continue
			}

			updatedAt := time.Now()
			if t, err := time.Parse("2006-01-02T15:04:05-0700", thread.UpdatedTime); err == nil {
				updatedAt = t
			} else if t, err := time.Parse(time.RFC3339, thread.UpdatedTime); err == nil {
				updatedAt = t
			}

			threadID := strings.TrimSpace(thread.ID)
			if threadID == "" {
				threadID = fmt.Sprintf("t_%s_%s", page.PageID, customerPSID)
			}

			var conv models.Conversation
			var saveErr error
			res := h.DB.Where("thread_id = ? OR (page_id = ? AND customer_psid = ?)", threadID, page.PageID, customerPSID).Limit(1).Find(&conv)
			if res.Error != nil {
				saveErr = res.Error
			} else if res.RowsAffected > 0 {
				conv.PageID = page.PageID
				conv.PageName = page.Name
				conv.CustomerPSID = customerPSID
				if customerName != "Khách hàng Facebook" || conv.CustomerName == "" {
					conv.CustomerName = customerName
				}
				if updatedAt.After(conv.LastMessageTime) {
					conv.LastMessageTime = updatedAt
				}
				saveErr = h.DB.Save(&conv).Error
			} else {
				conv = models.Conversation{
					PageID:          page.PageID,
					PageName:        page.Name,
					ThreadID:        threadID,
					CustomerPSID:    customerPSID,
					CustomerName:    customerName,
					LastMessageTime: updatedAt,
				}
				saveErr = h.DB.Create(&conv).Error
			}
			if saveErr != nil {
				if stats.SaveError == "" {
					stats.SaveError = saveErr.Error()
				}
				log.Printf("[Broadcast] Lỗi lưu khách %s của Page %s: %v", customerPSID, page.Name, saveErr)
				continue
			}
			stats.Saved++
			fetched++
		}

		nextURL = fbData.Paging.Next
	}

	log.Printf("[Broadcast] Page %s: Meta trả %d hội thoại, lưu %d khách, %d không có PSID", page.Name, stats.Threads, stats.Saved, stats.NoPSID)
	return stats, nil
}

func (h *BroadcastHandler) startWorker() {
	sentInBatch := 0

	for {
		nowVN := time.Now().In(h.vnLoc)
		mins := nowVN.Hour()*60 + nowVN.Minute()
		if mins < 8*60+30 || mins > 21*60 {
			time.Sleep(1 * time.Minute)
			continue
		}

		var runningCampaignIDs []uint
		if err := h.DB.Model(&BroadcastCampaign{}).
			Where("status = ?", "running").
			Pluck("id", &runningCampaignIDs).Error; err != nil || len(runningCampaignIDs) == 0 {
			sentInBatch = 0
			time.Sleep(5 * time.Second)
			continue
		}

		var msgs []BroadcastMessage
		if err := h.DB.Where("status = ? AND campaign_id IN ?", "pending", runningCampaignIDs).
			Order("id ASC").
			Limit(1).
			Find(&msgs).Error; err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		if len(msgs) == 0 {
			cutoff := time.Now().Add(-3 * time.Second)
			for _, cid := range runningCampaignIDs {
				var remaining int64
				h.DB.Model(&BroadcastMessage{}).
					Where("campaign_id = ? AND status IN ?", cid, []string{"pending", "processing"}).
					Count(&remaining)
				if remaining == 0 {
					_ = h.DB.Model(&BroadcastCampaign{}).
						Where("id = ? AND status = ? AND created_at < ?", cid, "running", cutoff).
						Update("status", "completed").Error
				}
			}
			sentInBatch = 0
			time.Sleep(5 * time.Second)
			continue
		}

		msg := msgs[0]
		if res := h.DB.Model(&BroadcastMessage{}).
			Where("id = ? AND status = ?", msg.ID, "pending").
			Update("status", "processing"); res.RowsAffected == 0 {
			continue
		}

		var camp BroadcastCampaign
		var page models.FacebookPage
		var conv models.Conversation
		if h.DB.First(&camp, msg.CampaignID).Error != nil ||
			h.DB.Where("page_id = ?", msg.PageID).First(&page).Error != nil ||
			h.DB.First(&conv, msg.ConversationID).Error != nil {
			_ = h.DB.Model(&msg).Updates(map[string]any{"status": "failed", "error_message": "Không tìm thấy Page hoặc hội thoại"}).Error
			_ = h.DB.Model(&camp).UpdateColumn("failed_count", gorm.Expr("failed_count + 1")).Error
			continue
		}

		psid := strings.TrimSpace(msg.RecipientPSID)
		if psid == "" {
			psid = strings.TrimSpace(conv.CustomerPSID)
		}
		if psid == "" {
			_ = h.DB.Model(&msg).Updates(map[string]any{"status": "failed", "error_message": "Thiếu PSID khách hàng"}).Error
			_ = h.DB.Model(&camp).UpdateColumn("failed_count", gorm.Expr("failed_count + 1")).Error
			continue
		}

		pageToken, err := security.DecryptSecret(page.EncryptedPageToken, h.getEncryptionKey())
		if err != nil || pageToken == "" {
			_ = h.DB.Model(&msg).Updates(map[string]any{"status": "failed", "error_message": "Lỗi giải mã Page Access Token"}).Error
			_ = h.DB.Model(&camp).UpdateColumn("failed_count", gorm.Expr("failed_count + 1")).Error
			continue
		}

		atts, attErr := h.ensureCampaignAttachments(&camp, pageToken)
		if attErr != nil {
			log.Printf("[Broadcast] Lỗi upload media chiến dịch %d: %v", camp.ID, attErr)
			_ = h.DB.Model(&msg).Updates(map[string]any{"status": "pending"}).Error
			_ = h.DB.Model(&camp).Updates(map[string]any{
				"status":       "paused",
				"pause_reason": "Không tải được ảnh/video lên Facebook: " + attErr.Error(),
			}).Error
			continue
		}

		fbMsgID, sentAtts, errCode, errMsg := h.sendBroadcastContent(page.PageID, pageToken, psid, msg.RenderedText, atts, camp.OutOfWindowTag)

		if errCode == 0 && errMsg == "" {
			now := time.Now()
			_ = h.DB.Model(&msg).Updates(map[string]any{
				"status":        "sent",
				"fb_message_id": fbMsgID,
				"sent_at":       &now,
			}).Error

			_ = h.DB.Model(&camp).UpdateColumn("sent_count", gorm.Expr("sent_count + 1")).Error

			conv.LastMessage = msg.RenderedText
			if strings.TrimSpace(conv.LastMessage) == "" && len(sentAtts) > 0 {
				conv.LastMessage = attachmentPreviewLabel(sentAtts[len(sentAtts)-1].Type)
			}
			conv.LastMessageTime = now
			_ = h.DB.Save(&conv).Error

			if strings.TrimSpace(msg.RenderedText) != "" {
				_ = h.DB.Create(&models.Message{
					ConversationID: conv.ID,
					FBMessageID:    fbMsgID,
					SenderType:     "page",
					SenderName:     page.Name,
					Content:        msg.RenderedText,
					SentAt:         now,
				}).Error
			}
			for _, sa := range sentAtts {
				_ = h.DB.Create(&models.Message{
					ConversationID: conv.ID,
					FBMessageID:    sa.MessageID,
					SenderType:     "page",
					SenderName:     page.Name,
					Content:        "",
					AttachmentURL:  sa.URL,
					AttachmentType: sa.Type,
					SentAt:         now,
				}).Error
			}

			log.Printf("[Broadcast] Đã gửi thành công tới %s (%s)", msg.RecipientName, psid)
		} else {
			log.Printf("[Broadcast] Gửi thất bại tới %s (Code %d): %s", psid, errCode, errMsg)
			_ = h.DB.Model(&msg).Updates(map[string]any{
				"status":        "failed",
				"error_code":    errCode,
				"error_message": errMsg,
			}).Error
			_ = h.DB.Model(&camp).UpdateColumn("failed_count", gorm.Expr("failed_count + 1")).Error

			if errCode == 4 || errCode == 17 || errCode == 32 || errCode == 613 {
				_ = h.DB.Model(&camp).Updates(map[string]any{
					"status":       "paused",
					"pause_reason": fmt.Sprintf("Tự dừng do giới hạn tốc độ Meta (Code %d: %s)", errCode, errMsg),
				}).Error
			}
		}

		var remaining int64
		h.DB.Model(&BroadcastMessage{}).Where("campaign_id = ? AND status IN ?", camp.ID, []string{"pending", "processing"}).Count(&remaining)
		if remaining == 0 {
			_ = h.DB.Model(&camp).Where("status = ?", "running").Update("status", "completed").Error
		}

		sentInBatch++
		if camp.BatchSize > 0 && sentInBatch >= camp.BatchSize {
			sentInBatch = 0
			pause := camp.BatchPauseSec
			if pause < 30 {
				pause = 60
			}
			time.Sleep(time.Duration(pause) * time.Second)
			continue
		}

		minD := camp.MinDelaySec
		if minD < 3 {
			minD = 4
		}
		maxD := camp.MaxDelaySec
		if maxD < minD {
			maxD = minD + 5
		}
		time.Sleep(time.Duration(minD+rand.Intn(maxD-minD+1)) * time.Second)
	}
}

// callFBSend gửi 1 tin (text hoặc attachment) qua Send API.
func (h *BroadcastHandler) callFBSend(pageID, pageToken, psid string, message any, msgType, tag string) (string, int, string) {
	payload := map[string]any{
		"recipient":      map[string]string{"id": psid},
		"messaging_type": msgType,
		"message":        message,
	}
	if tag != "" {
		payload["tag"] = tag
	}
	bodyBytes, _ := json.Marshal(payload)

	sendURL := fmt.Sprintf("https://graph.facebook.com/v21.0/%s/messages?access_token=%s", pageID, pageToken)
	resp, err := h.HTTPClient.Post(sendURL, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", -1, "Không kết nối được Facebook: " + err.Error()
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	var fbRes struct {
		MessageID string `json:"message_id"`
		Error     *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(respBytes, &fbRes)
	if fbRes.Error != nil {
		code := fbRes.Error.Code
		if code == 0 {
			code = resp.StatusCode
		}
		return "", code, fbRes.Error.Message
	}
	if resp.StatusCode >= 300 {
		return "", resp.StatusCode, string(respBytes)
	}
	return fbRes.MessageID, 0, ""
}

// sendWithFallback thử gửi trong cửa sổ 24h (RESPONSE), nếu quá 24h (#10) thì lần lượt thử các Message Tag.
// Trả về thêm messaging_type/tag đã thành công để các tin tiếp theo của cùng khách dùng lại.
func (h *BroadcastHandler) sendWithFallback(pageID, pageToken, psid string, message any, preferredTag string) (mid string, code int, errMsg, usedType, usedTag string) {
	mid, code, errMsg = h.callFBSend(pageID, pageToken, psid, message, "RESPONSE", "")
	if errMsg == "" {
		return mid, 0, "", "RESPONSE", ""
	}

	if code == 10 || strings.Contains(errMsg, "(#10)") {
		preferredTag = strings.TrimSpace(preferredTag)
		tags := []string{}
		if preferredTag != "" {
			tags = append(tags, preferredTag)
		}
		for _, t := range []string{"HUMAN_AGENT", "POST_PURCHASE_UPDATE", "CONFIRMED_EVENT_UPDATE"} {
			if t != preferredTag {
				tags = append(tags, t)
			}
		}
		for _, tag := range tags {
			mid, code, errMsg = h.callFBSend(pageID, pageToken, psid, message, "MESSAGE_TAG", tag)
			if errMsg == "" {
				return mid, 0, "", "MESSAGE_TAG", tag
			}
		}
	}
	return "", code, errMsg, "", ""
}

type sentAttachment struct {
	Type      string
	URL       string
	MessageID string
}

// sendBroadcastContent gửi nội dung chữ (nếu có) rồi lần lượt từng ảnh/video cho 1 khách.
// Tin đầu tiên quyết định messaging_type/tag, các tin sau dùng lại để không phải thử lại.
func (h *BroadcastHandler) sendBroadcastContent(pageID, pageToken, psid, text string, atts []BroadcastAttachment, preferredTag string) (string, []sentAttachment, int, string) {
	var (
		firstMID string
		usedType string
		usedTag  string
		sent     []sentAttachment
	)

	send := func(message any) (string, int, string) {
		if usedType == "" {
			mid, code, errMsg, t, tag := h.sendWithFallback(pageID, pageToken, psid, message, preferredTag)
			if errMsg == "" {
				usedType, usedTag = t, tag
			}
			return mid, code, errMsg
		}
		return h.callFBSend(pageID, pageToken, psid, message, usedType, usedTag)
	}

	if strings.TrimSpace(text) != "" {
		mid, code, errMsg := send(map[string]string{"text": text})
		if errMsg != "" {
			return "", nil, code, errMsg
		}
		firstMID = mid
	}

	for _, a := range atts {
		message := map[string]any{
			"attachment": map[string]any{
				"type":    a.Type,
				"payload": map[string]string{"attachment_id": a.AttachmentID},
			},
		}
		mid, code, errMsg := send(message)
		if errMsg != "" {
			// Chữ đã tới khách nhưng media lỗi: vẫn báo lỗi để người dùng thấy trong thống kê
			return firstMID, sent, code, "Gửi " + attachmentPreviewLabel(a.Type) + " thất bại: " + errMsg
		}
		if firstMID == "" {
			firstMID = mid
		}
		sent = append(sent, sentAttachment{Type: a.Type, URL: a.URL, MessageID: mid})
	}

	return firstMID, sent, 0, ""
}

// ensureCampaignAttachments upload ảnh/video của chiến dịch lên Facebook đúng 1 lần
// (Attachment Upload API) rồi lưu attachment_id để tái sử dụng cho toàn bộ khách.
func (h *BroadcastHandler) ensureCampaignAttachments(camp *BroadcastCampaign, pageToken string) ([]BroadcastAttachment, error) {
	atts := decodeAttachments(camp.AttachmentsJSON)
	if len(atts) == 0 {
		return nil, nil
	}

	changed := false
	for i := range atts {
		if atts[i].AttachmentID != "" {
			continue
		}
		id, err := h.uploadAttachmentToFacebook(camp.PageID, pageToken, atts[i])
		if err != nil {
			return nil, err
		}
		atts[i].AttachmentID = id
		changed = true
	}

	if changed {
		b, _ := json.Marshal(atts)
		camp.AttachmentsJSON = string(b)
		_ = h.DB.Model(&BroadcastCampaign{}).Where("id = ?", camp.ID).Update("attachments_json", camp.AttachmentsJSON).Error
	}
	return atts, nil
}

func (h *BroadcastHandler) uploadAttachmentToFacebook(pageID, pageToken string, att BroadcastAttachment) (string, error) {
	localPath, ok := broadcastLocalPath(att.URL)
	if !ok {
		return "", fmt.Errorf("đường dẫn file không hợp lệ")
	}
	f, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("không tìm thấy file %s trên server (hãy tải lại file và tạo chiến dịch mới)", filepath.Base(localPath))
	}
	defer f.Close()

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	go func() {
		msgJSON, _ := json.Marshal(map[string]any{
			"attachment": map[string]any{
				"type":    att.Type,
				"payload": map[string]bool{"is_reusable": true},
			},
		})
		_ = writer.WriteField("message", string(msgJSON))
		part, err := writer.CreateFormFile("filedata", filepath.Base(localPath))
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(part, f); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_ = pw.CloseWithError(writer.Close())
	}()

	uploadURL := fmt.Sprintf("https://graph.facebook.com/v21.0/%s/message_attachments?access_token=%s", pageID, pageToken)
	req, err := http.NewRequest(http.MethodPost, uploadURL, pr)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("không kết nối được Facebook: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var res struct {
		AttachmentID string `json:"attachment_id"`
		Error        *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &res)
	if res.Error != nil {
		return "", fmt.Errorf("%s", res.Error.Message)
	}
	if res.AttachmentID == "" {
		return "", fmt.Errorf("Facebook không trả về attachment_id")
	}
	return res.AttachmentID, nil
}

func decodeAttachments(raw string) []BroadcastAttachment {
	atts := []BroadcastAttachment{}
	if strings.TrimSpace(raw) == "" {
		return atts
	}
	_ = json.Unmarshal([]byte(raw), &atts)
	return atts
}

func attachmentPreviewLabel(t string) string {
	if t == "video" {
		return "[Video]"
	}
	return "[Hình ảnh]"
}

var (
	broadcastImageExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}
	broadcastVideoExts = map[string]bool{".mp4": true, ".mov": true, ".m4v": true, ".webm": true}
)

// broadcastLocalPath chỉ chấp nhận file nằm trong thư mục uploads/broadcast (chặn đọc file tùy ý).
func broadcastLocalPath(url string) (string, bool) {
	clean := strings.TrimPrefix(strings.TrimSpace(url), "/")
	if !strings.HasPrefix(clean, broadcastUploadDir+"/") {
		return "", false
	}
	name := strings.TrimPrefix(clean, broadcastUploadDir+"/")
	if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return "", false
	}
	return filepath.Join(filepath.FromSlash(broadcastUploadDir), name), true
}

func sanitizeBroadcastAttachment(t, url, name string) (BroadcastAttachment, bool) {
	t = strings.ToLower(strings.TrimSpace(t))
	localPath, ok := broadcastLocalPath(url)
	if !ok {
		return BroadcastAttachment{}, false
	}
	ext := strings.ToLower(filepath.Ext(localPath))
	switch t {
	case "image":
		if !broadcastImageExts[ext] {
			return BroadcastAttachment{}, false
		}
	case "video":
		if !broadcastVideoExts[ext] {
			return BroadcastAttachment{}, false
		}
	default:
		return BroadcastAttachment{}, false
	}
	if _, err := os.Stat(localPath); err != nil {
		return BroadcastAttachment{}, false
	}
	return BroadcastAttachment{Type: t, URL: "/" + filepath.ToSlash(localPath), Name: strings.TrimSpace(name)}, true
}

// UploadMedia POST /api/v1/broadcasts/upload-media (multipart, field "files")
func (h *BroadcastHandler) UploadMedia(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, int64(broadcastMaxAttach)*broadcastMaxVideoBytes+(1<<20))

	form, err := c.MultipartForm()
	if err != nil || form == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File quá lớn hoặc dữ liệu tải lên không hợp lệ"})
		return
	}
	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng chọn ít nhất 1 ảnh hoặc video"})
		return
	}
	if len(files) > broadcastMaxAttach {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Tối đa %d file mỗi lần", broadcastMaxAttach)})
		return
	}

	if err := os.MkdirAll(filepath.FromSlash(broadcastUploadDir), 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không tạo được thư mục lưu file"})
		return
	}

	var out []BroadcastAttachment
	var rejected []string
	for idx, file := range files {
		ext := strings.ToLower(filepath.Ext(file.Filename))
		var t string
		var limit int64
		switch {
		case broadcastImageExts[ext]:
			t, limit = "image", broadcastMaxImageBytes
		case broadcastVideoExts[ext]:
			t, limit = "video", broadcastMaxVideoBytes
		default:
			rejected = append(rejected, file.Filename+" (định dạng không hỗ trợ)")
			continue
		}
		if file.Size > limit {
			rejected = append(rejected, fmt.Sprintf("%s (vượt %dMB)", file.Filename, limit>>20))
			continue
		}

		filename := fmt.Sprintf("%s_%d_%d%s", t, time.Now().UnixNano(), idx, ext)
		dst := filepath.Join(filepath.FromSlash(broadcastUploadDir), filename)
		if err := c.SaveUploadedFile(file, dst); err != nil {
			rejected = append(rejected, file.Filename+" (lỗi lưu file)")
			continue
		}
		out = append(out, BroadcastAttachment{
			Type: t,
			URL:  "/" + broadcastUploadDir + "/" + filename,
			Name: file.Filename,
		})
	}

	if len(out) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Không có file hợp lệ. Ảnh: JPG/PNG/GIF/WEBP ≤ 8MB; Video: MP4/MOV/M4V/WEBM ≤ 25MB. " + strings.Join(rejected, ", ")})
		return
	}
	c.JSON(http.StatusOK, gin.H{"files": out, "rejected": rejected})
}

// CreateCampaign POST /api/v1/broadcasts
func (h *BroadcastHandler) CreateCampaign(c *gin.Context) {
	var req struct {
		PageID      string   `json:"page_id" binding:"required"`
		Name        string   `json:"name" binding:"required"`
		Templates   []string `json:"templates"`
		Attachments []struct {
			Type string `json:"type"`
			URL  string `json:"url"`
			Name string `json:"name"`
		} `json:"attachments"`
		TargetMode     string `json:"target_mode"`
		OutOfWindowTag string `json:"out_of_window_tag"`
		CooldownDays   int    `json:"cooldown_days"`
		MaxRecipients  int    `json:"max_recipients"`
		MinDelaySec    int    `json:"min_delay_sec"`
		MaxDelaySec    int    `json:"max_delay_sec"`
		BatchSize      int    `json:"batch_size"`
		BatchPauseSec  int    `json:"batch_pause_sec"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var validTemplates []string
	for _, t := range req.Templates {
		if s := strings.TrimSpace(t); s != "" {
			validTemplates = append(validTemplates, s)
		}
	}

	var validAtts []BroadcastAttachment
	for _, a := range req.Attachments {
		att, ok := sanitizeBroadcastAttachment(a.Type, a.URL, a.Name)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "File đính kèm không hợp lệ, vui lòng tải lại ảnh/video"})
			return
		}
		validAtts = append(validAtts, att)
	}
	if len(validAtts) > broadcastMaxAttach {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Tối đa %d ảnh/video cho mỗi chiến dịch", broadcastMaxAttach)})
		return
	}

	if len(validTemplates) == 0 && len(validAtts) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cần ít nhất 1 mẫu nội dung hoặc 1 ảnh/video"})
		return
	}

	if req.MaxRecipients <= 0 || req.MaxRecipients > 2000 {
		req.MaxRecipients = 1000
	}
	if req.MinDelaySec < 3 {
		req.MinDelaySec = 4
	}
	if req.MaxDelaySec < req.MinDelaySec {
		req.MaxDelaySec = req.MinDelaySec + 5
	}
	if req.BatchSize <= 0 {
		req.BatchSize = 20
	}
	if req.BatchPauseSec < 30 {
		req.BatchPauseSec = 60
	}
	if req.OutOfWindowTag == "" {
		req.OutOfWindowTag = "HUMAN_AGENT"
	}

	// Bước 1: Quét toàn bộ khách hàng có trong hộp thư Meta Business của Page về CRM
	var metaSyncErr string
	var syncStats metaSyncStats
	synced := false
	var page models.FacebookPage
	if err := h.DB.Where("page_id = ?", req.PageID).First(&page).Error; err == nil {
		if pageToken, decErr := security.DecryptSecret(page.EncryptedPageToken, h.getEncryptionKey()); decErr == nil && pageToken != "" {
			st, syncErr := h.pullAllConversationsFromMeta(page, pageToken, req.MaxRecipients)
			syncStats, synced = st, syncErr == nil
			if syncErr != nil {
				metaSyncErr = syncErr.Error()
				log.Printf("[Broadcast] Cảnh báo quét từ Meta Business: %v", syncErr)
			}
		} else {
			metaSyncErr = "Không giải mã được Page Token"
		}
	} else {
		metaSyncErr = "Không tìm thấy Page trong DB"
	}

	// Bước 2: Lấy TẤT CẢ khách hàng thuộc Page đó có CustomerPSID hợp lệ
	q := h.DB.Model(&models.Conversation{}).
		Where("page_id = ? AND COALESCE(TRIM(customer_psid), '') <> ''", req.PageID)

	switch req.TargetMode {
	case "within_24h":
		q = q.Where("last_message_time >= ?", time.Now().Add(-23*time.Hour-45*time.Minute))
	case "over_24h":
		q = q.Where("last_message_time < ?", time.Now().Add(-23*time.Hour-45*time.Minute))
	default:
		req.TargetMode = "all"
	}

	var convs []models.Conversation
	if err := q.Order("last_message_time DESC").Limit(req.MaxRecipients).Find(&convs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi đọc danh sách khách hàng: " + err.Error()})
		return
	}

	var validConvs []models.Conversation
	for _, cv := range convs {
		if strings.TrimSpace(cv.CustomerPSID) != "" {
			validConvs = append(validConvs, cv)
		}
	}

	if len(validConvs) == 0 {
		msgErr := "Không tìm thấy khách hàng có PSID hợp lệ trong Page này."
		switch {
		case metaSyncErr != "":
			msgErr += " Chi tiết: " + metaSyncErr
		case synced && syncStats.Threads == 0:
			msgErr += " Meta trả về 0 hội thoại cho Page này. Kiểm tra: Page đã có khách nhắn tin chưa; khi kết nối Page đã cấp quyền đọc tin nhắn (pages_messaging, pages_read_engagement) chưa — nếu chưa hãy kết nối lại Page; App Facebook có đang ở chế độ Development không (chế độ này chỉ thấy tin của tài khoản tester)."
		case synced && syncStats.SaveError != "":
			msgErr += fmt.Sprintf(" Meta trả về %d hội thoại nhưng lưu vào CRM bị lỗi: %s", syncStats.Threads, syncStats.SaveError)
		case synced && syncStats.Saved > 0 && req.TargetMode != "all":
			msgErr += fmt.Sprintf(" Đã quét được %d khách nhưng không ai thuộc nhóm \"%s\". Hãy chọn \"Tất cả khách\".", syncStats.Saved, req.TargetMode)
		case synced:
			msgErr += fmt.Sprintf(" Meta trả về %d hội thoại, %d hội thoại không xác định được khách.", syncStats.Threads, syncStats.NoPSID)
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": msgErr})
		return
	}

	tplJSON, _ := json.Marshal(validTemplates)
	if validAtts == nil {
		validAtts = []BroadcastAttachment{}
	}
	attJSON, _ := json.Marshal(validAtts)
	camp := BroadcastCampaign{
		PageID:          req.PageID,
		Name:            req.Name,
		TemplatesJSON:   string(tplJSON),
		AttachmentsJSON: string(attJSON),
		TargetMode:      req.TargetMode,
		OutOfWindowTag:  req.OutOfWindowTag,
		MinDelaySec:     req.MinDelaySec,
		MaxDelaySec:     req.MaxDelaySec,
		BatchSize:       req.BatchSize,
		BatchPauseSec:   req.BatchPauseSec,
		Status:          "running",
		TotalRecipients: len(validConvs),
	}

	if err := h.DB.Create(&camp).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var msgs []BroadcastMessage
	for _, cv := range validConvs {
		psid := strings.TrimSpace(cv.CustomerPSID)
		if psid == "" {
			continue
		}
		tpl := ""
		if len(validTemplates) > 0 {
			tpl = validTemplates[rand.Intn(len(validTemplates))]
		}
		name := strings.TrimSpace(cv.CustomerName)
		if name == "" {
			name = "anh/chị"
		}
		rendered := strings.ReplaceAll(tpl, "{{full_name}}", name)
		rendered = strings.ReplaceAll(rendered, "{customer_name}", name)

		msgs = append(msgs, BroadcastMessage{
			CampaignID:     camp.ID,
			PageID:         req.PageID,
			ConversationID: cv.ID,
			RecipientPSID:  psid,
			RecipientName:  cv.CustomerName,
			RenderedText:   rendered,
			Status:         "pending",
		})
	}

	if len(msgs) == 0 {
		_ = h.DB.Delete(&camp).Error
		c.JSON(http.StatusBadRequest, gin.H{"error": "Các hội thoại hiện tại chưa có CustomerPSID hợp lệ."})
		return
	}

	if err := h.DB.CreateInBatches(&msgs, 100).Error; err != nil {
		_ = h.DB.Delete(&camp).Error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không lưu được hàng đợi tin nhắn: " + err.Error()})
		return
	}

	camp.Attachments = validAtts
	c.JSON(http.StatusCreated, camp)
}

// ListCampaigns GET /api/v1/broadcasts
func (h *BroadcastHandler) ListCampaigns(c *gin.Context) {
	var list []BroadcastCampaign
	_ = h.DB.Order("created_at DESC").Limit(30).Find(&list).Error
	for i := range list {
		list[i].Attachments = decodeAttachments(list[i].AttachmentsJSON)
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

// ToggleStatus PATCH /api/v1/broadcasts/:id/status
func (h *BroadcastHandler) ToggleStatus(c *gin.Context) {
	id := c.Param("id")
	var body struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || (body.Status != "running" && body.Status != "paused") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trạng thái không hợp lệ"})
		return
	}
	updates := map[string]any{"status": body.Status}
	if body.Status == "running" {
		updates["pause_reason"] = ""
	}
	_ = h.DB.Model(&BroadcastCampaign{}).Where("id = ?", id).Updates(updates).Error
	c.JSON(http.StatusOK, gin.H{"status": body.Status})
}
