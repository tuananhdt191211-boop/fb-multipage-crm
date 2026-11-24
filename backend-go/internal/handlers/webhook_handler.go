package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
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

type WebhookHandler struct {
	DB          *gorm.DB
	Cfg         *config.Config
	AI          *AIHandler
	HTTPClient  *http.Client
	mu          sync.Mutex
	inFlight    map[uint]bool      // Chống gửi trùng 2 tin AI cùng lúc cho 1 hội thoại
	cooldownMap map[uint]time.Time // Tạm nghỉ gọi AI nếu hội thoại vừa gặp lỗi (429 / quota)
}

func NewWebhookHandler(db *gorm.DB, cfg *config.Config, ai *AIHandler) *WebhookHandler {
	h := &WebhookHandler{
		DB:          db,
		Cfg:         cfg,
		AI:          ai,
		HTTPClient:  &http.Client{Timeout: 15 * time.Second},
		inFlight:    make(map[uint]bool),
		cooldownMap: make(map[uint]time.Time),
	}
	// Chạy worker quét nền mỗi 30 giây để xử lý các tin nhắn trong vòng 24h nhưng quá 2 phút chưa có nhân viên trả lời
	go h.startDelayedAutoReplyWorker()
	return h
}

func (h *WebhookHandler) getEncryptionKey() string {
	if h.Cfg != nil && h.Cfg.JWTSecret != "" {
		return h.Cfg.JWTSecret
	}
	return "default-dev-master-key-change-in-prod"
}

// VerifyWebhook xác thực URL Webhook với Meta Developers (Verify Token: pagepilot_verify_token)
func (h *WebhookHandler) VerifyWebhook(c *gin.Context) {
	mode := c.Query("hub.mode")
	token := c.Query("hub.verify_token")
	challenge := c.Query("hub.challenge")

	verifyToken := "pagepilot_verify_token"
	if mode == "subscribe" && token == verifyToken {
		c.String(http.StatusOK, challenge)
		return
	}
	c.Status(http.StatusForbidden)
}

// HandleWebhook nhận sự kiện tin nhắn Messenger thời gian thực
func (h *WebhookHandler) HandleWebhook(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	var payload struct {
		Object string `json:"object"`
		Entry  []struct {
			ID        string `json:"id"`
			Messaging []struct {
				Sender struct {
					ID string `json:"id"`
				} `json:"sender"`
				Recipient struct {
					ID string `json:"id"`
				} `json:"recipient"`
				Timestamp int64 `json:"timestamp"`
				Message   *struct {
					MID         string `json:"mid"`
					Text        string `json:"text"`
					IsEcho      bool   `json:"is_echo"`
					Attachments []struct {
						Type    string `json:"type"`
						Payload struct {
							URL string `json:"url"`
						} `json:"payload"`
					} `json:"attachments"`
				} `json:"message"`
			} `json:"messaging"`
		} `json:"entry"`
	}

	if err := json.Unmarshal(body, &payload); err != nil || payload.Object != "page" {
		c.Status(http.StatusOK)
		return
	}

	c.Status(http.StatusOK)

	go func() {
		for _, entry := range payload.Entry {
			pageID := entry.ID
			var page models.FacebookPage
			if h.DB.Where("page_id = ? AND is_connected = ?", pageID, true).First(&page).Error != nil {
				continue
			}

			for _, event := range entry.Messaging {
				if event.Message == nil {
					continue
				}

				// Nếu là tin nhắn echo do nhân viên trực tiếp nhắn trên Facebook Page -> lưu vào DB và hủy chờ AI
				if event.Message.IsEcho {
					customerPSID := event.Recipient.ID
					text := strings.TrimSpace(event.Message.Text)
					if customerPSID != "" && text != "" {
						var conv models.Conversation
						if h.DB.Where("page_id = ? AND customer_psid = ?", pageID, customerPSID).First(&conv).Error == nil {
							now := time.Now()
							_ = h.DB.Create(&models.Message{
								ConversationID: conv.ID,
								FBMessageID:    event.Message.MID,
								SenderType:     "page",
								SenderName:     page.Name,
								Content:        text,
								SentAt:         now,
							}).Error
							conv.LastMessage = text
							conv.LastMessageTime = now
							conv.UnreadCount = 0
							_ = h.DB.Save(&conv).Error
						}
					}
					continue
				}

				customerPSID := event.Sender.ID
				if customerPSID == "" || customerPSID == pageID {
					continue
				}

				var existMsg models.Message
				if event.Message.MID != "" && h.DB.Where("fb_message_id = ?", event.Message.MID).First(&existMsg).Error == nil {
					continue
				}

				content := strings.TrimSpace(event.Message.Text)
				attURL := ""
				attType := ""
				if len(event.Message.Attachments) > 0 {
					attURL = event.Message.Attachments[0].Payload.URL
					attType = event.Message.Attachments[0].Type
					if content == "" {
						content = "[Hình ảnh / Đính kèm]"
					}
				}
				if content == "" {
					continue
				}

				now := time.Now()
				var conv models.Conversation
				if h.DB.Where("page_id = ? AND customer_psid = ?", pageID, customerPSID).First(&conv).Error != nil {
					suffix := customerPSID
					if len(customerPSID) > 4 {
						suffix = customerPSID[len(customerPSID)-4:]
					}
					conv = models.Conversation{
						PageID:          pageID,
						PageName:        page.Name,
						ThreadID:        "t_" + customerPSID,
						CustomerPSID:    customerPSID,
						CustomerName:    "Khách hàng #" + suffix,
						LastMessage:     content,
						LastMessageTime: now,
						UnreadCount:     1,
					}
					_ = h.DB.Create(&conv).Error
				} else {
					conv.LastMessage = content
					conv.LastMessageTime = now
					conv.UnreadCount += 1
					_ = h.DB.Save(&conv).Error
				}

				customerMsg := models.Message{
					ConversationID: conv.ID,
					FBMessageID:    event.Message.MID,
					SenderType:     "customer",
					SenderName:     conv.CustomerName,
					Content:        content,
					AttachmentURL:  attURL,
					AttachmentType: attType,
					SentAt:         now,
				}
				_ = h.DB.Create(&customerMsg).Error

				// Ưu tiên tự động trả lời ngay lập tức bằng tin nhắn mẫu đã lưu nếu khớp từ khóa
				if sentTemplate := h.tryAutoReplyWithSavedTemplate(page, &conv, content); sentTemplate {
					continue
				}

				// Nếu không khớp mẫu tự động: hẹn giờ 2 phút để AI phản hồi nếu nhân viên chưa nhắn
				go h.scheduleAutoReplyAfter2Minutes(page, conv.ID, customerMsg.ID)
			}
		}
	}()
}

// tryAutoReplyWithSavedTemplate đối chiếu nội dung tin nhắn khách với các mẫu có autoTrigger=true của Fanpage
func (h *WebhookHandler) tryAutoReplyWithSavedTemplate(page models.FacebookPage, conv *models.Conversation, customerMsg string) bool {
	if conv == nil || strings.TrimSpace(customerMsg) == "" {
		return false
	}

	var setting models.PageAISetting
	if err := h.DB.Where("page_id = ?", page.PageID).First(&setting).Error; err != nil {
		return false
	}
	if strings.TrimSpace(setting.QuickReplies) == "" {
		return false
	}

	var templates []quickReplyTemplate
	if err := json.Unmarshal([]byte(setting.QuickReplies), &templates); err != nil || len(templates) == 0 {
		return false
	}

	msgLower := strings.ToLower(customerMsg)
	var matched *quickReplyTemplate

	for i := range templates {
		tpl := &templates[i]
		if !tpl.AutoTrigger || strings.TrimSpace(tpl.Content) == "" || strings.TrimSpace(tpl.Keywords) == "" {
			continue
		}
		for _, kw := range strings.Split(tpl.Keywords, ",") {
			k := strings.ToLower(strings.TrimSpace(kw))
			if k != "" && strings.Contains(msgLower, k) {
				matched = tpl
				break
			}
		}
		if matched != nil {
			break
		}
	}

	if matched == nil {
		return false
	}

	// Tránh gửi trùng mẫu liên tiếp cho cùng hội thoại trong 30 giây
	var recentAuto models.Message
	if err := h.DB.Where("conversation_id = ? AND sender_type = ? AND sent_at >= ?",
		conv.ID, "page", time.Now().Add(-30*time.Second)).Order("sent_at DESC").First(&recentAuto).Error; err == nil {
		return false
	}

	custName := strings.TrimSpace(conv.CustomerName)
	if custName == "" {
		custName = "anh/chị"
	}
	pgName := strings.TrimSpace(conv.PageName)
	if pgName == "" {
		pgName = page.Name
	}

	replyText := strings.ReplaceAll(matched.Content, "{customer_name}", custName)
	replyText = strings.ReplaceAll(replyText, "{page_name}", pgName)
	replyText = strings.TrimSpace(replyText)
	if replyText == "" {
		return false
	}

	pageToken, err := security.DecryptSecret(page.EncryptedPageToken, h.getEncryptionKey())
	if err != nil || pageToken == "" {
		return false
	}

	payload := map[string]interface{}{
		"recipient":      map[string]string{"id": conv.CustomerPSID},
		"messaging_type": "RESPONSE",
		"message":        map[string]string{"text": replyText},
	}
	bodyBytes, _ := json.Marshal(payload)

	sendURL := fmt.Sprintf("https://graph.facebook.com/v21.0/%s/messages?access_token=%s", page.PageID, pageToken)
	resp, err := h.HTTPClient.Post(sendURL, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var fbRes struct {
		MessageID string `json:"message_id"`
		Error     *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(respBody, &fbRes)

	if fbRes.Error != nil || fbRes.MessageID == "" {
		return false
	}

	now := time.Now()
	_ = h.DB.Create(&models.Message{
		ConversationID: conv.ID,
		FBMessageID:    fbRes.MessageID,
		SenderType:     "page",
		SenderName:     fmt.Sprintf("%s (Mẫu: %s)", page.Name, matched.Title),
		Content:        replyText,
		SentAt:         now,
	}).Error

	conv.LastMessage = replyText
	conv.LastMessageTime = now
	conv.UnreadCount = 0
	_ = h.DB.Save(conv).Error

	log.Printf("[Auto-Template] Đã tự động gửi mẫu '%s' cho khách %s trên Page %s", matched.Title, conv.CustomerName, page.Name)
	return true
}

// scheduleAutoReplyAfter2Minutes chờ 2 phút, nếu nhân viên chưa nhắn gì thì kích hoạt AI trả lời
func (h *WebhookHandler) scheduleAutoReplyAfter2Minutes(page models.FacebookPage, convID uint, triggerMsgID uint) {
	time.Sleep(2 * time.Minute)
	h.tryAutoReplyConversation(page, convID, triggerMsgID)
}

// startDelayedAutoReplyWorker quét định kỳ mỗi 30s chỉ trong khung 24h hợp lệ của Meta để tiết kiệm quota Gemini
func (h *WebhookHandler) startDelayedAutoReplyWorker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		var settings []models.PageAISetting
		if err := h.DB.Where("auto_reply = ?", true).Find(&settings).Error; err != nil || len(settings) == 0 {
			continue
		}

		now := time.Now()
		twoMinsAgo := now.Add(-2 * time.Minute)
		within24Hours := now.Add(-24 * time.Hour)

		for _, st := range settings {
			var page models.FacebookPage
			if h.DB.Where("page_id = ? AND is_connected = ?", st.PageID, true).First(&page).Error != nil {
				continue
			}

			// Chỉ lấy tối đa 5 hội thoại mới nhất trong khung 24h để tránh đốt quota Gemini trên tin cũ
			var pendingConvs []models.Conversation
			_ = h.DB.Where(
				"page_id = ? AND unread_count > 0 AND last_message_time <= ? AND last_message_time >= ?",
				st.PageID, twoMinsAgo, within24Hours,
			).Order("last_message_time DESC").Limit(5).Find(&pendingConvs).Error

			for _, conv := range pendingConvs {
				var lastMsg models.Message
				if err := h.DB.Where("conversation_id = ?", conv.ID).Order("sent_at DESC, id DESC").First(&lastMsg).Error; err != nil {
					continue
				}
				// Chỉ tự động trả lời nếu tin nhắn cuối cùng là của khách, đã chờ đủ 2 phút và vẫn nằm trong 24h
				if lastMsg.SenderType == "customer" && lastMsg.SentAt.Before(twoMinsAgo) && lastMsg.SentAt.After(within24Hours) {
					h.tryAutoReplyConversation(page, conv.ID, lastMsg.ID)
				}
			}
		}
	}
}

func (h *WebhookHandler) tryAutoReplyConversation(page models.FacebookPage, convID uint, expectedLastMsgID uint) {
	h.mu.Lock()
	if h.inFlight[convID] {
		h.mu.Unlock()
		return
	}
	if until, ok := h.cooldownMap[convID]; ok && time.Now().Before(until) {
		h.mu.Unlock()
		return
	}
	h.inFlight[convID] = true
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.inFlight, convID)
		h.mu.Unlock()
	}()

	// Kiểm tra lại Fanpage có đang bật AutoReply không
	var aiSetting models.PageAISetting
	if h.DB.Where("page_id = ? AND auto_reply = ?", page.PageID, true).First(&aiSetting).Error != nil {
		return
	}

	var conv models.Conversation
	if err := h.DB.First(&conv, convID).Error; err != nil {
		return
	}

	// Bỏ qua nếu hội thoại đã quá khung 24h của Meta
	if time.Since(conv.LastMessageTime) > 24*time.Hour {
		return
	}

	// Kiểm tra tin nhắn cuối cùng trong hội thoại:
	// Nếu nhân viên đã nhắn (sender_type == "page") hoặc khách vừa nhắn thêm tin mới hơn -> bỏ qua
	var latestMsg models.Message
	if err := h.DB.Where("conversation_id = ?", conv.ID).Order("sent_at DESC, id DESC").First(&latestMsg).Error; err != nil {
		return
	}
	if latestMsg.SenderType != "customer" {
		return
	}
	if expectedLastMsgID > 0 && latestMsg.ID != expectedLastMsgID {
		return
	}

	if h.AI == nil {
		return
	}

	replyText, err := h.AI.GenerateReplyForConversation(conv)
	if err != nil {
		log.Printf("[AI Auto-Reply 2m] Lỗi sinh câu trả lời cho Page %s: %v", page.Name, err)
		// Nghỉ 5 phút cho hội thoại này nếu gặp lỗi (như 429 hết quota) để tránh lặp vô ích mỗi 30s
		h.mu.Lock()
		h.cooldownMap[convID] = time.Now().Add(5 * time.Minute)
		h.mu.Unlock()
		return
	}

	h.sendAutoReplyToFacebook(page, conv, replyText)
}

func (h *WebhookHandler) sendAutoReplyToFacebook(page models.FacebookPage, conv models.Conversation, replyText string) {
	pageToken, err := security.DecryptSecret(page.EncryptedPageToken, h.getEncryptionKey())
	if err != nil || pageToken == "" {
		return
	}

	payload := map[string]interface{}{
		"recipient":      map[string]string{"id": conv.CustomerPSID},
		"messaging_type": "RESPONSE",
		"message":        map[string]string{"text": replyText},
	}
	bodyBytes, _ := json.Marshal(payload)

	sendURL := fmt.Sprintf("https://graph.facebook.com/v21.0/%s/messages?access_token=%s", page.PageID, pageToken)
	resp, err := h.HTTPClient.Post(sendURL, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var fbRes struct {
		MessageID string `json:"message_id"`
		Error     *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(respBody, &fbRes)

	if fbRes.Error != nil {
		log.Printf("[AI Auto-Reply 2m] Facebook từ chối gửi cho %s (%s): %s", conv.CustomerName, page.Name, fbRes.Error.Message)
		h.mu.Lock()
		h.cooldownMap[conv.ID] = time.Now().Add(10 * time.Minute)
		h.mu.Unlock()
		return
	}

	if fbRes.MessageID != "" {
		now := time.Now()
		_ = h.DB.Create(&models.Message{
			ConversationID: conv.ID,
			FBMessageID:    fbRes.MessageID,
			SenderType:     "page",
			SenderName:     page.Name + " (AI)",
			Content:        replyText,
			SentAt:         now,
		}).Error

		conv.LastMessage = replyText
		conv.LastMessageTime = now
		conv.UnreadCount = 0
		_ = h.DB.Save(&conv).Error
		log.Printf("[AI Auto-Reply 2m] Đã tự động trả lời khách %s trên Page %s", conv.CustomerName, page.Name)
	}
}
