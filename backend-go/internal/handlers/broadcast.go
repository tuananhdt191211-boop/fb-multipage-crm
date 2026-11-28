package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"fb-multipage-crm/internal/config"
	"fb-multipage-crm/internal/models"
	"fb-multipage-crm/internal/security"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type BroadcastCampaign struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	PageID          string    `gorm:"index;not null" json:"page_id"`
	Name            string    `gorm:"not null" json:"name"`
	TemplatesJSON   string    `gorm:"type:text;not null" json:"-"`
	TargetMode      string    `gorm:"default:'all'" json:"target_mode"`
	OutOfWindowTag  string    `gorm:"default:'HUMAN_AGENT'" json:"out_of_window_tag"`
	MinDelaySec     int       `gorm:"default:4" json:"min_delay_sec"`
	MaxDelaySec     int       `gorm:"default:9" json:"max_delay_sec"`
	BatchSize       int       `gorm:"default:20" json:"batch_size"`
	BatchPauseSec   int       `gorm:"default:60" json:"batch_pause_sec"`
	Status          string    `gorm:"index;default:'running'" json:"status"`
	PauseReason     string    `json:"pause_reason"`
	TotalRecipients int       `json:"total_recipients"`
	SentCount       int       `json:"sent_count"`
	FailedCount     int       `json:"failed_count"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	// Marketing Messages (target_mode = "subscribers")
	DeliveryMode         string `gorm:"default:'standard'" json:"delivery_mode"` // standard | marketing
	MetaCampaignID       string `json:"meta_campaign_id"`
	MarketingTitle       string `json:"marketing_title"`
	MarketingImageURL    string `gorm:"type:text" json:"marketing_image_url"`
	MarketingButtonURL   string `gorm:"type:text" json:"marketing_button_url"`
	MarketingButtonTitle string `json:"marketing_button_title"`
	MarketingDailyBudget int64  `json:"marketing_daily_budget"`

	// Ảnh/video gửi kèm sau nội dung chữ (lưu JSON, trả ra dạng mảng)
	AttachmentsJSON string                `gorm:"type:text" json:"-"`
	Attachments     []BroadcastAttachment `gorm:"-" json:"attachments"`
}

// AfterFind giải mã danh sách ảnh/video mỗi khi đọc chiến dịch từ DB
func (c *BroadcastCampaign) AfterFind(tx *gorm.DB) error {
	c.Attachments = nil
	if strings.TrimSpace(c.AttachmentsJSON) != "" {
		_ = json.Unmarshal([]byte(c.AttachmentsJSON), &c.Attachments)
	}
	return nil
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

	// Có giá trị => gửi qua Marketing Messages API bằng token đăng ký thay vì PSID
	SubscriptionToken string `gorm:"type:text" json:"-"`
}

type BroadcastHandler struct {
	DB           *gorm.DB
	Cfg          *config.Config
	HTTPClient   *http.Client
	vnLoc        *time.Location
	optinRunning sync.Map // page_id -> true khi đang gửi lời mời nhận tin
}

func NewBroadcastHandler(db *gorm.DB, cfg *config.Config) *BroadcastHandler {
	_ = db.AutoMigrate(&BroadcastCampaign{}, &BroadcastMessage{}, &models.Conversation{})
	migrateMarketingTables(db)
	migrateBroadcastMediaTables(db)

	// Cột cũ recipient_psid (NOT NULL) không còn được ghi: GORM lưu PSID vào recipient_ps_id.
	// Bỏ ràng buộc để insert không bị lỗi 23502.
	if err := db.Exec(`
		DO $$
		BEGIN
		  IF EXISTS (
		    SELECT 1 FROM information_schema.columns
		    WHERE table_name = 'broadcast_messages' AND column_name = 'recipient_psid' AND is_nullable = 'NO'
		  ) THEN
		    ALTER TABLE broadcast_messages ALTER COLUMN recipient_psid DROP NOT NULL;
		  END IF;
		END $$;
	`).Error; err != nil {
		log.Printf("[Broadcast] Không bỏ được NOT NULL của broadcast_messages.recipient_psid: %v", err)
	}

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

// pullAllConversationsFromMeta quét phân trang toàn bộ khách hàng có trong Meta Business Inbox của Page
func (h *BroadcastHandler) pullAllConversationsFromMeta(page models.FacebookPage, pageToken string, maxCount int) error {
	if maxCount <= 0 {
		maxCount = 2000
	}
	nextURL := fmt.Sprintf(
		"https://graph.facebook.com/v21.0/%s/conversations?fields=id,updated_time,participants,senders,messages.limit(5){from}&limit=100&access_token=%s",
		page.PageID, pageToken,
	)

	fetched := 0
	skipped := 0
	totalThreads := 0
	for nextURL != "" && fetched < maxCount {
		resp, err := h.HTTPClient.Get(nextURL)
		if err != nil {
			return fmt.Errorf("không kết nối được Graph API")
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
				Senders struct {
					Data []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"data"`
				} `json:"senders"`
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
			return fmt.Errorf("lỗi đọc dữ liệu từ Meta: %v", err)
		}
		if fbData.Error != nil {
			return fmt.Errorf("Meta báo lỗi: %s", fbData.Error.Message)
		}

		totalThreads += len(fbData.Data)
		for _, thread := range fbData.Data {
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
				for _, sd := range thread.Senders.Data {
					if sd.ID != page.PageID && strings.TrimSpace(sd.ID) != "" {
						customerPSID = strings.TrimSpace(sd.ID)
						if strings.TrimSpace(sd.Name) != "" {
							customerName = strings.TrimSpace(sd.Name)
						}
						break
					}
				}
			}
			if customerPSID == "" {
				skipped++
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
			res := h.DB.Where("thread_id = ? OR (page_id = ? AND customer_ps_id = ?)", threadID, page.PageID, customerPSID).Limit(1).Find(&conv)
			if res.RowsAffected > 0 {
				conv.PageName = page.Name
				conv.CustomerPSID = customerPSID
				if customerName != "Khách hàng Facebook" || conv.CustomerName == "" {
					conv.CustomerName = customerName
				}
				if updatedAt.After(conv.LastMessageTime) {
					conv.LastMessageTime = updatedAt
				}
				_ = h.DB.Save(&conv).Error
			} else {
				conv = models.Conversation{
					WorkspaceID:     page.WorkspaceID,
					PageID:          page.PageID,
					PageName:        page.Name,
					ThreadID:        threadID,
					CustomerPSID:    customerPSID,
					CustomerName:    customerName,
					LastMessageTime: updatedAt,
				}
				_ = h.DB.Create(&conv).Error
			}
			fetched++
		}

		nextURL = fbData.Paging.Next
	}

	log.Printf("[Broadcast] Page %s: Meta trả %d hội thoại, lấy được %d khách, bỏ qua %d", page.Name, totalThreads, fetched, skipped)
	if fetched == 0 {
		return fmt.Errorf("Meta trả %d hội thoại nhưng không xác định được khách nào (%d bị bỏ qua)", totalThreads, skipped)
	}
	return nil
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
			if err != nil {
				broadcastWake.Sleep(time.Minute)
			} else {
				// Không có chiến dịch nào đang chạy: ngủ dài cho database được nghỉ,
				// tạo / chạy lại chiến dịch sẽ đánh thức ngay (broadcastWake)
				broadcastWake.Sleep(idleSleep)
			}
			continue
		}

		var msgs []BroadcastMessage
		if err := h.DB.Where("status = ? AND campaign_id IN ?", "pending", runningCampaignIDs).
			Order("id ASC").
			Limit(1).
			Find(&msgs).Error; err != nil {
			broadcastWake.Sleep(30 * time.Second)
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
			// Chiến dịch đang nạp danh sách người nhận (chạy nền) -> hỏi lại sau ít giây
			broadcastWake.Sleep(20 * time.Second)
			continue
		}

		msg := msgs[0]
		if res := h.DB.Model(&BroadcastMessage{}).
			Where("id = ? AND status = ?", msg.ID, "pending").
			Update("status", "processing"); res.RowsAffected == 0 {
			continue
		}

		// Tin marketing: gửi qua Marketing Messages API bằng token khách đã đăng ký nhận tin
		if strings.TrimSpace(msg.SubscriptionToken) != "" {
			var mcamp BroadcastCampaign
			if h.DB.First(&mcamp, msg.CampaignID).Error != nil {
				_ = h.DB.Model(&msg).Updates(map[string]any{"status": "failed", "error_message": "Không tìm thấy chiến dịch"}).Error
				continue
			}
			h.processMarketingMessage(&mcamp, msg)
			h.afterMessageStep(mcamp, &sentInBatch)
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

		// Gửi chữ trước, sau đó lần lượt ảnh/video đính kèm của chiến dịch
		fbMsgID, errCode, errMsg, mediaWarn := h.sendBroadcastContent(page.PageID, pageToken, psid, msg.RenderedText, camp)

		if errCode == 0 && errMsg == "" {
			now := time.Now()
			updates := map[string]any{
				"status":        "sent",
				"fb_message_id": fbMsgID,
				"sent_at":       &now,
			}
			if mediaWarn != "" {
				updates["error_message"] = mediaWarn
			}
			_ = h.DB.Model(&msg).Updates(updates).Error

			_ = h.DB.Model(&camp).UpdateColumn("sent_count", gorm.Expr("sent_count + 1")).Error

			inboxText := msg.RenderedText
			if n := len(camp.Attachments); n > 0 {
				if strings.TrimSpace(inboxText) == "" {
					inboxText = fmt.Sprintf("[%d ảnh/video]", n)
				} else {
					inboxText += fmt.Sprintf(" [+%d ảnh/video]", n)
				}
			}

			conv.LastMessage = inboxText
			conv.LastMessageTime = now
			_ = h.DB.Save(&conv).Error

			_ = h.DB.Create(&models.Message{
				ConversationID: conv.ID,
				FBMessageID:    fbMsgID,
				SenderType:     "page",
				SenderName:     page.Name,
				Content:        inboxText,
				SentAt:         now,
			}).Error

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

		h.afterMessageStep(camp, &sentInBatch)
	}
}

// afterMessageStep: đánh dấu hoàn tất chiến dịch nếu hết tin, rồi nghỉ lô / giãn cách ngẫu nhiên
func (h *BroadcastHandler) afterMessageStep(camp BroadcastCampaign, sentInBatch *int) {
	var remaining int64
	h.DB.Model(&BroadcastMessage{}).Where("campaign_id = ? AND status IN ?", camp.ID, []string{"pending", "processing"}).Count(&remaining)
	if remaining == 0 {
		_ = h.DB.Model(&camp).Where("status = ?", "running").Update("status", "completed").Error
	}

	*sentInBatch++
	if camp.BatchSize > 0 && *sentInBatch >= camp.BatchSize {
		*sentInBatch = 0
		pause := camp.BatchPauseSec
		if pause < 30 {
			pause = 60
		}
		time.Sleep(time.Duration(pause) * time.Second)
		return
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

// postFBMessage gửi 1 tin (chữ hoặc đính kèm) với messaging_type / tag chỉ định
func (h *BroadcastHandler) postFBMessage(pageID, pageToken, psid string, message map[string]any, msgType, tag string) (string, int, string) {
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
	return fbRes.MessageID, 0, ""
}

// sendDirectPayload: thử RESPONSE trước, nếu quá 24h (#10) thì thử lần lượt các Message Tag.
// Trả thêm messaging_type / tag đã thành công để các tin tiếp theo (ảnh/video) dùng lại.
func (h *BroadcastHandler) sendDirectPayload(pageID, pageToken, psid string, message map[string]any, preferredTag string) (string, int, string, string, string) {
	// 1. Gửi thẳng cho khách trong 24h (RESPONSE)
	mid, code, errMsg := h.postFBMessage(pageID, pageToken, psid, message, "RESPONSE", "")
	if errMsg == "" {
		return mid, 0, "", "RESPONSE", ""
	}

	// 2. Nếu quá 24h (#10), tự động thử lần lượt các Message Tag hợp lệ
	if code == 10 || strings.Contains(errMsg, "(#10)") {
		tags := []string{}
		if strings.TrimSpace(preferredTag) != "" {
			tags = append(tags, strings.TrimSpace(preferredTag))
		}
		for _, t := range []string{"HUMAN_AGENT", "POST_PURCHASE_UPDATE", "CONFIRMED_EVENT_UPDATE"} {
			if t != strings.TrimSpace(preferredTag) {
				tags = append(tags, t)
			}
		}
		for _, tag := range tags {
			mid, code, errMsg = h.postFBMessage(pageID, pageToken, psid, message, "MESSAGE_TAG", tag)
			if errMsg == "" {
				return mid, 0, "", "MESSAGE_TAG", tag
			}
		}
	}

	return "", code, errMsg, "", ""
}

func (h *BroadcastHandler) sendDirectMessage(pageID, pageToken, psid, text, preferredTag string) (string, int, string) {
	mid, code, errMsg, _, _ := h.sendDirectPayload(pageID, pageToken, psid, map[string]any{"text": text}, preferredTag)
	return mid, code, errMsg
}

// CreateCampaign POST /api/v1/broadcasts
func (h *BroadcastHandler) CreateCampaign(c *gin.Context) {
	var req struct {
		PageID         string   `json:"page_id" binding:"required"`
		Name           string   `json:"name" binding:"required"`
		Templates      []string              `json:"templates"`
		Attachments    []BroadcastAttachment `json:"attachments"`
		TargetMode     string   `json:"target_mode"`
		OutOfWindowTag string   `json:"out_of_window_tag"`
		CooldownDays   int      `json:"cooldown_days"`
		MaxRecipients  int      `json:"max_recipients"`
		MinDelaySec    int      `json:"min_delay_sec"`
		MaxDelaySec    int      `json:"max_delay_sec"`
		BatchSize      int      `json:"batch_size"`
		BatchPauseSec  int      `json:"batch_pause_sec"`

		// Chỉ dùng khi target_mode = "subscribers" (Marketing Messages)
		MarketingTitle       string `json:"marketing_title"`
		MarketingImageURL    string `json:"marketing_image_url"`
		MarketingButtonURL   string `json:"marketing_button_url"`
		MarketingButtonTitle string `json:"marketing_button_title"`
		MarketingDailyBudget int64  `json:"marketing_daily_budget"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Chỉ được chạy chiến dịch trên Fanpage của chính mình
	if _, ok := ownedPage(h.DB, wsID(c), req.PageID); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy Fanpage"})
		return
	}

	var validTemplates []string
	for _, t := range req.Templates {
		if s := strings.TrimSpace(t); s != "" {
			validTemplates = append(validTemplates, s)
		}
	}
	attachments, attErr := validateBroadcastAttachments(req.Attachments)
	if attErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": attErr.Error()})
		return
	}
	if len(validTemplates) == 0 && (len(attachments) == 0 || req.TargetMode == "subscribers") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cần ít nhất 1 mẫu nội dung hoặc 1 ảnh/video đính kèm (tin Marketing bắt buộc có nội dung chữ)"})
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

	// Khách đã đăng ký nhận tin -> gửi qua Marketing Messages API (không phụ thuộc cửa sổ 24h)
	if req.TargetMode == "subscribers" {
		h.createMarketingCampaign(c, marketingCampaignInput{
			PageID:        req.PageID,
			Name:          req.Name,
			Templates:     validTemplates,
			MaxRecipients: req.MaxRecipients,
			MinDelaySec:   req.MinDelaySec,
			MaxDelaySec:   req.MaxDelaySec,
			BatchSize:     req.BatchSize,
			BatchPauseSec: req.BatchPauseSec,
			Title:         req.MarketingTitle,
			ImageURL:      req.MarketingImageURL,
			ButtonURL:     req.MarketingButtonURL,
			ButtonTitle:   req.MarketingButtonTitle,
			DailyBudget:   req.MarketingDailyBudget,
		})
		return
	}

	// Bước 1: Quét toàn bộ khách hàng có trong hộp thư Meta Business của Page về CRM
	var metaSyncErr string
	var page models.FacebookPage
	if err := h.DB.Where("page_id = ?", req.PageID).First(&page).Error; err == nil {
		if pageToken, decErr := security.DecryptSecret(page.EncryptedPageToken, h.getEncryptionKey()); decErr == nil && pageToken != "" {
			var existing int64
			h.DB.Model(&models.Conversation{}).
				Where("page_id = ? AND COALESCE(TRIM(customer_ps_id), '') <> ''", req.PageID).
				Count(&existing)
			if existing > 0 {
				// Đã có khách trong CRM: quét Meta chạy ngầm để không bắt giao diện chờ quá lâu (timeout)
				go func(p models.FacebookPage, tok string, max int) {
					if syncErr := h.pullAllConversationsFromMeta(p, tok, max); syncErr != nil {
						log.Printf("[Broadcast] Cảnh báo quét ngầm từ Meta Business: %v", syncErr)
					}
				}(page, pageToken, req.MaxRecipients)
			} else if syncErr := h.pullAllConversationsFromMeta(page, pageToken, req.MaxRecipients); syncErr != nil {
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
		Where("page_id = ? AND COALESCE(TRIM(customer_ps_id), '') <> ''", req.PageID)

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
		msgErr := "Không tìm thấy khách hàng có PSID hợp lệ trong Page này. Hãy mở mục Hộp thư bấm 'Đồng bộ tin nhắn' rồi thử lại!"
		if metaSyncErr != "" {
			msgErr += " (Chi tiết: " + metaSyncErr + ")"
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": msgErr})
		return
	}

	tplJSON, _ := json.Marshal(validTemplates)
	attJSON := ""
	if len(attachments) > 0 {
		b, _ := json.Marshal(attachments)
		attJSON = string(b)
	}
	camp := BroadcastCampaign{
		PageID:          req.PageID,
		Name:            req.Name,
		TemplatesJSON:   string(tplJSON),
		TargetMode:      req.TargetMode,
		OutOfWindowTag:  req.OutOfWindowTag,
		MinDelaySec:     req.MinDelaySec,
		MaxDelaySec:     req.MaxDelaySec,
		BatchSize:       req.BatchSize,
		BatchPauseSec:   req.BatchPauseSec,
		Status:          "running",
		TotalRecipients: len(validConvs),
		AttachmentsJSON: attJSON,
		Attachments:     attachments,
	}

	if err := h.DB.Create(&camp).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer broadcastWake.Wake() // đánh thức bộ gửi tin đang ngủ

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

	c.JSON(http.StatusCreated, camp)
}

// ListCampaigns GET /api/v1/broadcasts
func (h *BroadcastHandler) ListCampaigns(c *gin.Context) {
	var list []BroadcastCampaign
	_ = h.DB.Where("page_id IN (?)", wsPageIDs(h.DB, wsID(c))).Order("created_at DESC").Limit(30).Find(&list).Error
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
	_ = h.DB.Model(&BroadcastCampaign{}).Where("id = ? AND page_id IN (?)", id, wsPageIDs(h.DB, wsID(c))).Updates(updates).Error
	if body.Status == "running" {
		broadcastWake.Wake()
	}
	c.JSON(http.StatusOK, gin.H{"status": body.Status})
}
