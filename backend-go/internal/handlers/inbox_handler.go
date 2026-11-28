package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fb-multipage-crm/internal/config"
	"fb-multipage-crm/internal/inboxrev"
	"fb-multipage-crm/internal/models"
	"fb-multipage-crm/internal/security"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type InboxHandler struct {
	DB         *gorm.DB
	Cfg        *config.Config
	HTTPClient *http.Client
}

type quickReplyTemplate struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Category    string   `json:"category"`
	Keywords    string   `json:"keywords"`
	AutoTrigger bool     `json:"autoTrigger"`
	Content     string   `json:"content"`
	Images      []string `json:"images"`
}

// conversationWithCRM mở rộng models.Conversation với các trường theo dõi & chăm sóc khách hàng
type conversationWithCRM struct {
	models.Conversation
	LeadStatus      string     `gorm:"type:varchar(30);default:'new'" json:"lead_status"`
	CustomerPhone   string     `gorm:"type:varchar(30)" json:"customer_phone"`
	CustomerAddress string     `gorm:"type:text" json:"customer_address"`
	CareNote        string     `gorm:"type:text" json:"care_note"`
	NextFollowUpAt  *time.Time `json:"next_follow_up_at"`
}

func (conversationWithCRM) TableName() string {
	return "conversations"
}

func NewInboxHandler(db *gorm.DB, cfg *config.Config) *InboxHandler {
	_ = db.AutoMigrate(&models.Conversation{}, &models.Message{}, &conversationWithCRM{})
	h := &InboxHandler{
		DB:         db,
		Cfg:        cfg,
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
	}
	go h.startAutoSyncWorker()
	return h
}

// startAutoSyncWorker kéo tin mới từ Facebook để hộp thư luôn cập nhật (lưới an toàn khi webhook chưa/không chạy).
//
// Chỉ chạy khi có người đang XEM hộp thư: tab đang hiện và người dùng đang thao tác (xem InboxRev).
// Không ai xem thì không chạm database -> Neon được ngủ, không hết hạn mức gói miễn phí.
// Fanpage đã đăng ký webhook thì tin mới vốn về ngay nên chỉ đối soát thưa hơn (mỗi 60 giây).
func (h *InboxHandler) startAutoSyncWorker() {
	const webhookPageEvery = 60 * time.Second
	lastSync := map[string]time.Time{}

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if !within(&lastInboxUse, 2*time.Minute) {
			continue
		}
		var pages []models.FacebookPage
		if err := h.DB.Where("is_connected = ?", true).Find(&pages).Error; err != nil || len(pages) == 0 {
			continue
		}
		for _, page := range pages {
			if page.WebhookSubscribed && time.Since(lastSync[page.PageID]) < webhookPageEvery {
				continue
			}
			lastSync[page.PageID] = time.Now()
			_, _ = h.syncSinglePageConversations(page, 25)
		}
	}
}

// InboxRev GET /api/v1/inbox/rev?bg=1 — "số phiên bản" của hộp thư, đọc từ RAM (KHÔNG truy vấn database).
// Trình duyệt hỏi số này thường xuyên và chỉ tải lại danh sách khi số đổi.
// bg=1: tab đang ẩn / người dùng đã rảnh -> không tính là "đang xem", nên không bật đồng bộ nền.
func (h *InboxHandler) InboxRev(c *gin.Context) {
	if c.Query("bg") != "1" {
		markInboxUse()
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"rev": inboxrev.Current()})
}

func (h *InboxHandler) encKey() string {
	if h.Cfg != nil && h.Cfg.JWTSecret != "" {
		return h.Cfg.JWTSecret
	}
	return "default-dev-master-key-change-in-prod"
}

// sanitizeAvatarURL loại bỏ các đường dẫn /picture? chứa access_token hoặc gây lỗi 400 trên trình duyệt
func sanitizeAvatarURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.Contains(trimmed, "/picture?") || strings.Contains(trimmed, "access_token=") {
		return ""
	}
	return trimmed
}

// ListConversations trả về danh sách hội thoại đã gộp từ tất cả Page đang bật (kèm trạng thái chăm sóc khách)
func (h *InboxHandler) ListConversations(c *gin.Context) {
	markInboxUse() // đang có người xem hộp thư -> bật đồng bộ nền
	pageID := strings.TrimSpace(c.Query("page_id"))
	search := strings.TrimSpace(c.Query("q"))

	query := h.DB.Model(&conversationWithCRM{})
	if pageID != "" && pageID != "all" {
		query = query.Where("page_id = ?", pageID)
	}
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("customer_name ILIKE ? OR customer_phone ILIKE ? OR last_message ILIKE ?", like, like, like)
	}

	// Trần an toàn để một lần tải không kéo ra lượng dữ liệu khổng lồ (băng thông Neon gói miễn phí chỉ 5 GB/tháng)
	limit := 2000
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 && v <= 5000 {
		limit = v
	}

	var conversations []conversationWithCRM
	if err := query.Order("last_message_time DESC").Limit(limit).Find(&conversations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể tải danh sách hội thoại"})
		return
	}

	for i := range conversations {
		conversations[i].CustomerAvatar = sanitizeAvatarURL(conversations[i].CustomerAvatar)
	}

	var pages []models.FacebookPage
	_ = h.DB.Where("is_connected = ?", true).Order("name ASC").Find(&pages).Error
	for i := range pages {
		pages[i].AvatarURL = sanitizeAvatarURL(pages[i].AvatarURL)
	}

	c.JSON(http.StatusOK, gin.H{
		"conversations": conversations,
		"pages":         pages,
	})
}

// TryAutoReplyWithTemplates kiểm tra và tự động gửi tin nhắn mẫu đã lưu (kèm ảnh nếu có) nếu khớp từ khóa
func TryAutoReplyWithTemplates(db *gorm.DB, encKey string, conv *models.Conversation, customerMsg string) bool {
	if conv == nil || strings.TrimSpace(customerMsg) == "" {
		return false
	}

	var setting models.PageAISetting
	res := db.Where("page_id = ? AND enabled = ?", conv.PageID, true).Limit(1).Find(&setting)
	if res.Error != nil || res.RowsAffected == 0 {
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
		hasBody := strings.TrimSpace(tpl.Content) != "" || len(tpl.Images) > 0
		if !tpl.AutoTrigger || !hasBody || strings.TrimSpace(tpl.Keywords) == "" {
			continue
		}
		kws := strings.Split(tpl.Keywords, ",")
		for _, kw := range kws {
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

	// Tránh gửi trùng liên tiếp cùng một nội dung mẫu cho cùng 1 hội thoại trong vòng 30 giây
	var recentSame models.Message
	recentRes := db.Where("conversation_id = ? AND sender_type = ? AND sent_at >= ?",
		conv.ID, "page", time.Now().Add(-30*time.Second)).Order("sent_at DESC").Limit(1).Find(&recentSame)
	if recentRes.Error == nil && recentRes.RowsAffected > 0 {
		return false
	}

	var page models.FacebookPage
	pageRes := db.Where("page_id = ? AND is_connected = ?", conv.PageID, true).Limit(1).Find(&page)
	if pageRes.Error != nil || pageRes.RowsAffected == 0 {
		return false
	}
	pageToken, err := security.DecryptSecret(page.EncryptedPageToken, encKey)
	if err != nil || pageToken == "" {
		return false
	}

	custName := conv.CustomerName
	if strings.TrimSpace(custName) == "" {
		custName = "anh/chị"
	}
	replyText := strings.ReplaceAll(matched.Content, "{customer_name}", custName)
	replyText = strings.ReplaceAll(replyText, "{page_name}", conv.PageName)
	replyText = strings.TrimSpace(replyText)

	sentAny := false

	if replyText != "" {
		payload := map[string]interface{}{
			"recipient": map[string]string{
				"id": conv.CustomerPSID,
			},
			"message": map[string]string{
				"text": replyText,
			},
			"messaging_type": "RESPONSE",
		}
		bodyBytes, _ := json.Marshal(payload)

		graphURL := fmt.Sprintf("https://graph.facebook.com/v21.0/%s/messages?access_token=%s", conv.PageID, pageToken)
		resp, err := http.Post(graphURL, "application/json", bytes.NewBuffer(bodyBytes))
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				now := time.Now()
				savedMsg := models.Message{
					ConversationID: conv.ID,
					FBMessageID:    fmt.Sprintf("auto_tpl_%d_%d", conv.ID, now.UnixNano()),
					SenderType:     "page",
					SenderName:     fmt.Sprintf("Auto • %s", matched.Title),
					Content:        replyText,
					SentAt:         now,
				}
				_ = db.Create(&savedMsg).Error

				conv.LastMessage = replyText
				conv.LastMessageTime = now
				conv.UnreadCount = 0
				_ = db.Save(conv).Error
				sentAny = true
			} else {
				_, _ = io.ReadAll(resp.Body)
			}
		}
	}

	for _, imgURL := range matched.Images {
		if strings.TrimSpace(imgURL) == "" {
			continue
		}
		imgMID, _, err := sendImageAttachmentToFacebook(http.DefaultClient, conv.PageID, pageToken, conv.CustomerPSID, imgURL, "")
		if err == nil && imgMID != "" {
			now := time.Now()
			_ = db.Create(&models.Message{
				ConversationID: conv.ID,
				FBMessageID:    imgMID,
				SenderType:     "page",
				SenderName:     fmt.Sprintf("Auto • %s", matched.Title),
				Content:        "",
				AttachmentURL:  imgURL,
				AttachmentType: "image",
				SentAt:         now,
			}).Error

			if replyText == "" {
				conv.LastMessage = "[Đã gửi hình ảnh mẫu]"
			}
			conv.LastMessageTime = now
			conv.UnreadCount = 0
			_ = db.Save(conv).Error
			sentAny = true
		}
	}

	return sentAny
}

// syncSinglePageConversations đồng bộ hội thoại & tin nhắn mới nhất của một Fanpage từ Graph API
func (h *InboxHandler) syncSinglePageConversations(page models.FacebookPage, limitThreads int) (int, string) {
	pageToken, err := security.DecryptSecret(page.EncryptedPageToken, h.encKey())
	if err != nil || pageToken == "" {
		return 0, page.Name + ": lỗi giải mã token"
	}

	apiURL := fmt.Sprintf(
		"https://graph.facebook.com/v21.0/%s/conversations?fields=id,updated_time,unread_count,participants,messages.limit(10){id,message,from,created_time,attachments{image_data,file_url,mime_type,name}}&limit=%d&access_token=%s",
		page.PageID, limitThreads, pageToken,
	)
	resp, err := h.HTTPClient.Get(apiURL)
	if err != nil {
		return 0, page.Name + ": không kết nối được Facebook"
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var fbData struct {
		Data []struct {
			ID           string `json:"id"`
			UpdatedTime  string `json:"updated_time"`
			UnreadCount  int    `json:"unread_count"`
			Participants struct {
				Data []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"data"`
			} `json:"participants"`
			Messages struct {
				Data []struct {
					ID          string `json:"id"`
					Message     string `json:"message"`
					CreatedTime string `json:"created_time"`
					From        struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"from"`
					Attachments struct {
						Data []struct {
							Name      string `json:"name"`
							MimeType  string `json:"mime_type"`
							FileURL   string `json:"file_url"`
							ImageData *struct {
								URL        string `json:"url"`
								PreviewURL string `json:"preview_url"`
							} `json:"image_data"`
						} `json:"data"`
					} `json:"attachments"`
				} `json:"data"`
			} `json:"messages"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &fbData); err != nil {
		return 0, page.Name + ": lỗi đọc dữ liệu"
	}
	if fbData.Error != nil {
		return 0, page.Name + ": " + fbData.Error.Message
	}

	// Khách (id, tên) của hội thoại thứ i
	custOf := func(i int) (string, string) {
		thread := fbData.Data[i]
		customerID := ""
		customerName := "Khách hàng Facebook"
		for _, p := range thread.Participants.Data {
			if p.ID != page.PageID && strings.TrimSpace(p.ID) != "" {
				customerID = strings.TrimSpace(p.ID)
				if strings.TrimSpace(p.Name) != "" {
					customerName = strings.TrimSpace(p.Name)
				}
				break
			}
		}
		if customerID == "" {
			for _, m := range thread.Messages.Data {
				if m.From.ID != "" && m.From.ID != page.PageID {
					customerID = strings.TrimSpace(m.From.ID)
					if strings.TrimSpace(m.From.Name) != "" {
						customerName = strings.TrimSpace(m.From.Name)
					}
					break
				}
			}
		}
		return customerID, customerName
	}

	// ---- Tải sẵn dữ liệu đã có bằng 2 truy vấn gộp ----
	// Trước đây: với MỖI hội thoại có 1 SELECT + 1 UPDATE (luôn ghi, kể cả khi không có gì mới) và với MỖI tin nhắn có 1 SELECT
	// -> ~300 truy vấn cho 1 Fanpage mỗi 15 giây. Đó là nguyên nhân chính làm Neon không bao giờ ngủ và hết hạn mức.
	threadIDs := make([]string, 0, len(fbData.Data))
	psids := make([]string, 0, len(fbData.Data))
	msgIDs := make([]string, 0, len(fbData.Data)*10)
	for i, thread := range fbData.Data {
		if id, _ := custOf(i); id != "" {
			threadIDs = append(threadIDs, thread.ID)
			psids = append(psids, id)
		}
		for _, m := range thread.Messages.Data {
			if m.ID != "" {
				msgIDs = append(msgIDs, m.ID)
			}
		}
	}

	convByThread := map[string]*models.Conversation{}
	convByPSID := map[string]*models.Conversation{}
	if len(threadIDs) > 0 {
		// Chỉ nạp các cột cần so sánh; cập nhật bên dưới dùng Updates(map) nên không ghi đè cột nào khác
		var existing []models.Conversation
		h.DB.Select("id", "page_id", "page_name", "thread_id", "customer_ps_id", "customer_name",
			"customer_avatar", "last_message", "last_message_time", "unread_count").
			Where("thread_id IN ? OR (page_id = ? AND customer_ps_id IN ?)", threadIDs, page.PageID, psids).
			Find(&existing)
		for i := range existing {
			c := &existing[i]
			convByThread[c.ThreadID] = c
			if c.PageID == page.PageID {
				convByPSID[c.CustomerPSID] = c
			}
		}
	}
	msgByFBID := map[string]*models.Message{}
	if len(msgIDs) > 0 {
		var existing []models.Message
		h.DB.Select("id", "fb_message_id", "content", "attachment_url", "attachment_type").
			Where("fb_message_id IN ?", msgIDs).Find(&existing)
		for i := range existing {
			msgByFBID[existing[i].FBMessageID] = &existing[i]
		}
	}

	synced := 0
	for i, thread := range fbData.Data {
		customerID, customerName := custOf(i)
		if customerID == "" {
			continue
		}

		updatedAt, updatedOK := time.Now(), false
		if t, err := time.Parse("2006-01-02T15:04:05-0700", thread.UpdatedTime); err == nil {
			updatedAt, updatedOK = t, true
		}

		lastMsgText := ""
		if len(thread.Messages.Data) > 0 {
			lastMsgText = strings.TrimSpace(thread.Messages.Data[0].Message)
			if lastMsgText == "" {
				lastMsgText = "[Hình ảnh / Đính kèm]"
			}
		}

		conv := convByThread[thread.ID]
		if conv == nil {
			conv = convByPSID[customerID]
		}
		if conv != nil {
			// Chỉ ghi những cột THỰC SỰ thay đổi (trước đây ghi lại cả dòng mỗi 15 giây dù không có gì mới)
			changes := map[string]any{}
			if conv.PageName != page.Name {
				changes["page_name"], conv.PageName = page.Name, page.Name
			}
			if conv.CustomerPSID != customerID {
				changes["customer_ps_id"], conv.CustomerPSID = customerID, customerID
			}
			if conv.CustomerName != customerName {
				changes["customer_name"], conv.CustomerName = customerName, customerName
			}
			if av := sanitizeAvatarURL(conv.CustomerAvatar); av != conv.CustomerAvatar {
				changes["customer_avatar"], conv.CustomerAvatar = av, av
			}
			if conv.LastMessage != lastMsgText {
				changes["last_message"], conv.LastMessage = lastMsgText, lastMsgText
			}
			// Không đọc được updated_time thì giữ nguyên giờ cũ (không ghi "bây giờ" mỗi lần như trước)
			if updatedOK && !conv.LastMessageTime.Equal(updatedAt) {
				changes["last_message_time"], conv.LastMessageTime = updatedAt, updatedAt
			}
			if conv.UnreadCount != thread.UnreadCount {
				changes["unread_count"], conv.UnreadCount = thread.UnreadCount, thread.UnreadCount
			}
			if len(changes) > 0 {
				_ = h.DB.Model(&models.Conversation{}).Where("id = ?", conv.ID).Updates(changes).Error
			}
		} else {
			nc := models.Conversation{
				PageID:          page.PageID,
				PageName:        page.Name,
				ThreadID:        thread.ID,
				CustomerPSID:    customerID,
				CustomerName:    customerName,
				CustomerAvatar:  "",
				LastMessage:     lastMsgText,
				LastMessageTime: updatedAt,
				UnreadCount:     thread.UnreadCount,
			}
			if err := h.DB.Create(&nc).Error; err != nil {
				// Có thể webhook vừa tạo đúng hội thoại này: lấy lại bản đó, không có thì bỏ qua
				var ex models.Conversation
				if h.DB.Where("thread_id = ? OR (page_id = ? AND customer_ps_id = ?)", thread.ID, page.PageID, customerID).
					Limit(1).Find(&ex).RowsAffected == 0 {
					continue
				}
				nc = ex
			}
			conv = &nc
			convByThread[thread.ID] = conv
			convByPSID[customerID] = conv
		}

		var newestCustomerText string
		for _, m := range thread.Messages.Data {
			if m.ID == "" {
				continue
			}

			attURL := ""
			attType := ""
			if len(m.Attachments.Data) > 0 {
				for _, att := range m.Attachments.Data {
					if att.ImageData != nil && att.ImageData.URL != "" {
						attURL = att.ImageData.URL
						attType = "image"
						break
					}
					if att.ImageData != nil && att.ImageData.PreviewURL != "" {
						attURL = att.ImageData.PreviewURL
						attType = "image"
						break
					}
					if att.FileURL != "" {
						attURL = att.FileURL
						if strings.HasPrefix(att.MimeType, "image/") {
							attType = "image"
						} else {
							attType = "file"
						}
						break
					}
				}
			}

			content := strings.TrimSpace(m.Message)
			if content == "" && attURL == "" {
				content = "[Đính kèm / Hình ảnh]"
			}

			if ex := msgByFBID[m.ID]; ex != nil {
				if attURL != "" && (ex.AttachmentURL != attURL || ex.Content == "[Đính kèm / Hình ảnh]") {
					_ = h.DB.Model(&models.Message{}).Where("id = ?", ex.ID).Updates(map[string]any{
						"attachment_url":  attURL,
						"attachment_type": attType,
						"content":         content,
					}).Error
					ex.AttachmentURL, ex.AttachmentType, ex.Content = attURL, attType, content
				}
				continue
			}

			senderType := "customer"
			if m.From.ID == page.PageID {
				senderType = "page"
			}
			sentAt := time.Now()
			if t, err := time.Parse("2006-01-02T15:04:05-0700", m.CreatedTime); err == nil {
				sentAt = t
			}

			_ = h.DB.Create(&models.Message{
				ConversationID: conv.ID,
				FBMessageID:    m.ID,
				SenderType:     senderType,
				SenderName:     m.From.Name,
				Content:        content,
				AttachmentURL:  attURL,
				AttachmentType: attType,
				SentAt:         sentAt,
			}).Error

			if senderType == "customer" && newestCustomerText == "" && time.Since(sentAt) <= 5*time.Minute && strings.TrimSpace(m.Message) != "" {
				newestCustomerText = strings.TrimSpace(m.Message)
			}
		}

		if newestCustomerText != "" {
			// TryAutoReplyWithTemplates lưu lại TOÀN BỘ dòng hội thoại -> phải nạp bản đầy đủ
			// (ở trên chỉ nạp vài cột cho nhẹ; lưu bản thiếu sẽ xoá ghi chú / trạng thái chăm sóc khách)
			var full models.Conversation
			if h.DB.First(&full, conv.ID).Error == nil {
				_ = TryAutoReplyWithTemplates(h.DB, h.encKey(), &full, newestCustomerText)
			}
		}
		synced++
	}

	return synced, ""
}

// SyncMessages kéo hội thoại và tin nhắn mới nhất (bao gồm hình ảnh/đính kèm) từ Graph API
func (h *InboxHandler) SyncMessages(c *gin.Context) {
	targetPageID := strings.TrimSpace(c.Query("page_id"))

	query := h.DB.Where("is_connected = ?", true)
	if targetPageID != "" && targetPageID != "all" {
		query = query.Where("page_id = ?", targetPageID)
	}

	var pages []models.FacebookPage
	if err := query.Find(&pages).Error; err != nil || len(pages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chưa có Fanpage nào đang hoạt động để đồng bộ tin nhắn"})
		return
	}

	totalThreads := 0
	var syncErrors []string

	for _, page := range pages {
		count, errStr := h.syncSinglePageConversations(page, 100)
		if errStr != "" {
			syncErrors = append(syncErrors, errStr)
		}
		totalThreads += count
	}

	if totalThreads == 0 && len(syncErrors) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Không đồng bộ được tin nhắn: " + strings.Join(syncErrors, " | "),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Đã đồng bộ %d hội thoại từ các Fanpage!", totalThreads),
		"synced":  totalThreads,
	})
}

// GetMessages lấy lịch sử tin nhắn của một hội thoại và đánh dấu đã đọc
func (h *InboxHandler) GetMessages(c *gin.Context) {
	markInboxUse()
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID hội thoại không hợp lệ"})
		return
	}

	var conv conversationWithCRM
	convRes := h.DB.Where("id = ?", id).Limit(1).Find(&conv)
	if convRes.Error != nil || convRes.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy hội thoại"})
		return
	}

	conv.CustomerAvatar = sanitizeAvatarURL(conv.CustomerAvatar)
	if conv.UnreadCount > 0 {
		conv.UnreadCount = 0
		_ = h.DB.Save(&conv).Error
	}

	var messages []models.Message
	if err := h.DB.Where("conversation_id = ?", conv.ID).Order("sent_at ASC, id ASC").Find(&messages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không tải được lịch sử tin nhắn"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"conversation": conv,
		"messages":     messages,
	})
}

type sendMessageReq struct {
	Content    string   `json:"content"`
	ImageURLs  []string `json:"image_urls"`
	MessageTag string   `json:"message_tag"`
}

// SendMessage gửi tin nhắn văn bản và/hoặc nhiều hình ảnh từ Page tới khách hàng qua Facebook Send API (hỗ trợ Message Tag sau 24h)
func (h *InboxHandler) SendMessage(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID hội thoại không hợp lệ"})
		return
	}

	var req sendMessageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu gửi tin nhắn không hợp lệ"})
		return
	}

	trimmedText := strings.TrimSpace(req.Content)
	requestedTag := strings.TrimSpace(req.MessageTag)
	var cleanImages []string
	for _, img := range req.ImageURLs {
		if u := strings.TrimSpace(img); u != "" {
			cleanImages = append(cleanImages, u)
		}
	}

	if trimmedText == "" && len(cleanImages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nội dung tin nhắn hoặc hình ảnh không được để trống"})
		return
	}

	var conv models.Conversation
	convRes := h.DB.Where("id = ?", id).Limit(1).Find(&conv)
	if convRes.Error != nil || convRes.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy hội thoại"})
		return
	}

	var page models.FacebookPage
	pageRes := h.DB.Where("page_id = ?", conv.PageID).Limit(1).Find(&page)
	if pageRes.Error != nil || pageRes.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy Fanpage quản lý hội thoại này"})
		return
	}

	pageToken, err := security.DecryptSecret(page.EncryptedPageToken, h.encKey())
	if err != nil || pageToken == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi giải mã Page Access Token"})
		return
	}

	var createdMessages []models.Message
	usedTag := ""

	// 1. Gửi phần nội dung chữ nếu có
	if trimmedText != "" {
		callFBSend := func(msgType, tag string) (string, string) {
			payload := map[string]interface{}{
				"recipient": map[string]string{
					"id": conv.CustomerPSID,
				},
				"messaging_type": msgType,
				"message": map[string]string{
					"text": trimmedText,
				},
			}
			if tag != "" {
				payload["tag"] = tag
			}
			payloadBytes, _ := json.Marshal(payload)

			sendURL := fmt.Sprintf("https://graph.facebook.com/v21.0/%s/messages?access_token=%s", page.PageID, pageToken)
			resp, err := h.HTTPClient.Post(sendURL, "application/json", bytes.NewBuffer(payloadBytes))
			if err != nil {
				return "", "Không gửi được request tới Facebook"
			}
			defer resp.Body.Close()
			respBody, _ := io.ReadAll(resp.Body)

			var fbSendResp struct {
				MessageID string `json:"message_id"`
				Error     *struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			_ = json.Unmarshal(respBody, &fbSendResp)
			if fbSendResp.Error != nil {
				return "", fbSendResp.Error.Message
			}
			return fbSendResp.MessageID, ""
		}

		var fbMsgID, errMsg string
		if requestedTag != "" {
			fbMsgID, errMsg = callFBSend("MESSAGE_TAG", requestedTag)
			if errMsg == "" {
				usedTag = requestedTag
			}
		} else {
			fbMsgID, errMsg = callFBSend("RESPONSE", "")
			// Nếu quá 24h (#10), tự động thử lần lượt các Message Tag hợp lệ của Meta
			if errMsg != "" && strings.Contains(errMsg, "(#10)") {
				fallbackTags := []string{"HUMAN_AGENT", "POST_PURCHASE_UPDATE", "CONFIRMED_EVENT_UPDATE"}
				for _, tag := range fallbackTags {
					fbMsgID, errMsg = callFBSend("MESSAGE_TAG", tag)
					if errMsg == "" {
						usedTag = tag
						break
					}
				}
			}
		}

		if errMsg != "" {
			if strings.Contains(errMsg, "(#10)") || requestedTag != "" {
				mbSuiteURL := fmt.Sprintf("https://business.facebook.com/latest/inbox/all?asset_id=%s&selected_item_id=%s", page.PageID, conv.CustomerPSID)
				c.JSON(http.StatusBadRequest, gin.H{
					"error":        "Hội thoại đã quá 24 giờ và App chưa được cấp quyền Message Tag. Bấm nút bên dưới để trả lời trực tiếp trên Meta Business Suite.",
					"code":         "OUTSIDE_24H_WINDOW",
					"fb_inbox_url": mbSuiteURL,
				})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": "Facebook từ chối gửi tin: " + errMsg})
			return
		}

		now := time.Now()
		textMsg := models.Message{
			ConversationID: conv.ID,
			FBMessageID:    fbMsgID,
			SenderType:     "page",
			SenderName:     page.Name,
			Content:        trimmedText,
			SentAt:         now,
		}
		_ = h.DB.Create(&textMsg).Error
		createdMessages = append(createdMessages, textMsg)

		conv.LastMessage = trimmedText
		conv.LastMessageTime = now
		_ = h.DB.Save(&conv).Error
	}

	// 2. Gửi lần lượt từng hình ảnh nếu có (tự động kèm Message Tag nếu quá 24h)
	activeImgTag := requestedTag
	if activeImgTag == "" {
		activeImgTag = usedTag
	}

	for _, imgURL := range cleanImages {
		imgMsgID, tagUsedForImg, imgErr := sendImageAttachmentToFacebook(h.HTTPClient, page.PageID, pageToken, conv.CustomerPSID, imgURL, activeImgTag)
		if imgErr != nil {
			if len(createdMessages) == 0 {
				if strings.Contains(imgErr.Error(), "(#10)") {
					mbSuiteURL := fmt.Sprintf("https://business.facebook.com/latest/inbox/all?asset_id=%s&selected_item_id=%s", page.PageID, conv.CustomerPSID)
					c.JSON(http.StatusBadRequest, gin.H{
						"error":        "Hội thoại đã quá 24 giờ nên Meta khóa gửi ảnh qua API. Bấm nút bên dưới để mở Meta Business Suite.",
						"code":         "OUTSIDE_24H_WINDOW",
						"fb_inbox_url": mbSuiteURL,
					})
					return
				}
				c.JSON(http.StatusBadRequest, gin.H{"error": "Facebook từ chối gửi ảnh: " + imgErr.Error()})
				return
			}
			break
		}
		if tagUsedForImg != "" {
			usedTag = tagUsedForImg
		}

		now := time.Now()
		imgMsg := models.Message{
			ConversationID: conv.ID,
			FBMessageID:    imgMsgID,
			SenderType:     "page",
			SenderName:     page.Name,
			Content:        "",
			AttachmentURL:  imgURL,
			AttachmentType: "image",
			SentAt:         now,
		}
		_ = h.DB.Create(&imgMsg).Error
		createdMessages = append(createdMessages, imgMsg)

		if trimmedText == "" {
			conv.LastMessage = "[Đã gửi hình ảnh]"
		}
		conv.LastMessageTime = now
		_ = h.DB.Save(&conv).Error
	}

	var firstMsg models.Message
	if len(createdMessages) > 0 {
		firstMsg = createdMessages[0]
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  firstMsg,
		"messages": createdMessages,
		"used_tag": usedTag,
	})
}

type updateConversationCRMReq struct {
	LeadStatus      string `json:"lead_status"`
	CustomerPhone   string `json:"customer_phone"`
	CustomerAddress string `json:"customer_address"`
	CareNote        string `json:"care_note"`
	NextFollowUpAt  string `json:"next_follow_up_at"`
}

// UpdateConversationCRM cập nhật trạng thái khách (Phân vân / Chốt đơn / Chăm lại), SĐT, địa chỉ và lịch hẹn
func (h *InboxHandler) UpdateConversationCRM(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID hội thoại không hợp lệ"})
		return
	}

	var conv conversationWithCRM
	if err := h.DB.First(&conv, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy hội thoại"})
		return
	}

	var req updateConversationCRMReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu cập nhật không hợp lệ"})
		return
	}

	allowedStatus := map[string]bool{
		"new":       true,
		"hesitant":  true,
		"closed":    true,
		"follow_up": true,
	}
	status := strings.TrimSpace(req.LeadStatus)
	if allowedStatus[status] {
		conv.LeadStatus = status
	}

	conv.CustomerPhone = strings.TrimSpace(req.CustomerPhone)
	conv.CustomerAddress = strings.TrimSpace(req.CustomerAddress)
	conv.CareNote = strings.TrimSpace(req.CareNote)

	rawTime := strings.TrimSpace(req.NextFollowUpAt)
	if rawTime == "" {
		conv.NextFollowUpAt = nil
	} else {
		if t, err := time.ParseInLocation("2006-01-02T15:04", rawTime, time.Local); err == nil {
			conv.NextFollowUpAt = &t
		} else if t2, err2 := time.Parse(time.RFC3339, rawTime); err2 == nil {
			conv.NextFollowUpAt = &t2
		}
	}

	if err := h.DB.Save(&conv).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể lưu thông tin chăm sóc khách hàng"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Đã cập nhật trạng thái & lịch chăm sóc khách hàng!",
		"conversation": conv,
	})
}

// GetCustomerAvatar lấy trực tiếp ảnh đại diện mới nhất của khách hàng từ Graph API bằng Page Token
func (h *InboxHandler) GetCustomerAvatar(c *gin.Context) {
	pageID := strings.TrimSpace(c.Param("page_id"))
	psid := strings.TrimSpace(c.Param("psid"))
	if pageID == "" || psid == "" {
		c.Status(http.StatusBadRequest)
		return
	}

	var page models.FacebookPage
	res := h.DB.Where("page_id = ? AND is_connected = ?", pageID, true).Limit(1).Find(&page)
	if res.Error != nil || res.RowsAffected == 0 {
		c.Status(http.StatusNotFound)
		return
	}

	pageToken, err := security.DecryptSecret(page.EncryptedPageToken, h.encKey())
	if err != nil || pageToken == "" {
		c.Status(http.StatusNotFound)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}

	// Cách 1: User Profile API (dành cho PSID Messenger) -> lấy link profile_pic
	imgURL := ""
	profURL := fmt.Sprintf(
		"https://graph.facebook.com/v21.0/%s?fields=profile_pic&access_token=%s",
		url.PathEscape(psid),
		url.QueryEscape(pageToken),
	)
	if r, reqErr := client.Get(profURL); reqErr == nil {
		var body struct {
			ProfilePic string `json:"profile_pic"`
		}
		if r.StatusCode == http.StatusOK {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		r.Body.Close()
		if strings.HasPrefix(body.ProfilePic, "https://") {
			imgURL = body.ProfilePic
		}
	}

	// Cách 2: dự phòng bằng /picture
	if imgURL == "" {
		imgURL = fmt.Sprintf(
			"https://graph.facebook.com/v21.0/%s/picture?type=normal&access_token=%s",
			url.PathEscape(psid),
			url.QueryEscape(pageToken),
		)
	}

	resp, err := client.Get(imgURL)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.Status(http.StatusNotFound)
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	c.Header("Cache-Control", "public, max-age=86400")
	c.DataFromReader(http.StatusOK, resp.ContentLength, contentType, resp.Body, nil)
}

// UploadInboxImages nhận nhiều file ảnh (chọn file hoặc dán Ctrl+V) và lưu vào ./uploads/inbox
func (h *InboxHandler) UploadInboxImages(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil || form == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Không đọc được dữ liệu ảnh tải lên"})
		return
	}

	files := form.File["images"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng chọn ít nhất 1 hình ảnh"})
		return
	}

	uploadDir := filepath.Join("uploads", "inbox")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không tạo được thư mục lưu ảnh"})
		return
	}

	var urls []string
	for idx, file := range files {
		ext := strings.ToLower(filepath.Ext(file.Filename))
		if ext == "" {
			ext = ".png"
		}
		allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}
		if !allowed[ext] {
			continue
		}

		filename := fmt.Sprintf("img_%d_%d%s", time.Now().UnixNano(), idx, ext)
		dstPath := filepath.Join(uploadDir, filename)
		if err := c.SaveUploadedFile(file, dstPath); err == nil {
			urls = append(urls, "/uploads/inbox/"+filename)
		}
	}

	if len(urls) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Định dạng ảnh không hỗ trợ (chỉ nhận JPG, PNG, WEBP, GIF)"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"urls": urls})
}

// sendImageAttachmentToFacebook đẩy file ảnh trực tiếp qua multipart/form-data lên Messenger Graph API (hỗ trợ fallback Message Tag sau 24h)
func sendImageAttachmentToFacebook(client *http.Client, pageID, pageToken, recipientPSID, imageURL, requestedTag string) (string, string, error) {
	var fileBytes []byte
	var fileName string

	cleanPath := strings.TrimPrefix(imageURL, "/")
	if strings.HasPrefix(cleanPath, "uploads/") {
		data, err := os.ReadFile(filepath.FromSlash(cleanPath))
		if err != nil {
			return "", "", fmt.Errorf("không tìm thấy file ảnh nội bộ: %v", err)
		}
		fileBytes = data
		fileName = filepath.Base(cleanPath)
	} else if strings.HasPrefix(imageURL, "http://") || strings.HasPrefix(imageURL, "https://") {
		resp, err := client.Get(imageURL)
		if err != nil {
			return "", "", err
		}
		defer resp.Body.Close()
		fileBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			return "", "", err
		}
		fileName = "attachment.jpg"
	} else {
		return "", "", fmt.Errorf("đường dẫn ảnh không hợp lệ")
	}

	attemptUpload := func(msgType, tag string) (string, error) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)

		recipientJSON, _ := json.Marshal(map[string]string{"id": recipientPSID})
		_ = writer.WriteField("recipient", string(recipientJSON))
		_ = writer.WriteField("messaging_type", msgType)
		if tag != "" {
			_ = writer.WriteField("tag", tag)
		}
		_ = writer.WriteField("message", `{"attachment":{"type":"image","payload":{"is_reusable":true}}}`)

		part, err := writer.CreateFormFile("filedata", fileName)
		if err != nil {
			return "", err
		}
		if _, err := part.Write(fileBytes); err != nil {
			return "", err
		}
		_ = writer.Close()

		sendURL := fmt.Sprintf("https://graph.facebook.com/v21.0/%s/messages?access_token=%s", pageID, pageToken)
		req, err := http.NewRequest(http.MethodPost, sendURL, &body)
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := client.Do(req)
		if err != nil {
			return "", err
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
			return "", fmt.Errorf("%s", fbRes.Error.Message)
		}
		return fbRes.MessageID, nil
	}

	if requestedTag != "" {
		mid, err := attemptUpload("MESSAGE_TAG", requestedTag)
		return mid, requestedTag, err
	}

	mid, err := attemptUpload("RESPONSE", "")
	if err != nil && strings.Contains(err.Error(), "(#10)") {
		fallbackTags := []string{"HUMAN_AGENT", "POST_PURCHASE_UPDATE", "CONFIRMED_EVENT_UPDATE"}
		for _, tag := range fallbackTags {
			mid, err = attemptUpload("MESSAGE_TAG", tag)
			if err == nil {
				return mid, tag, nil
			}
		}
	}
	return mid, "", err
}

// CheckCustomerAvatar: TẠM THỜI, chỉ để chẩn đoán. Xóa sau khi xong.
func (h *InboxHandler) CheckCustomerAvatar(c *gin.Context) {
	pageID := strings.TrimSpace(c.Param("page_id"))
	psid := strings.TrimSpace(c.Param("psid"))

	var page models.FacebookPage
	res := h.DB.Where("page_id = ? AND is_connected = ?", pageID, true).Limit(1).Find(&page)
	if res.Error != nil || res.RowsAffected == 0 {
		c.JSON(http.StatusOK, gin.H{"buoc": "tìm page", "loi": "không có page này hoặc page chưa kết nối"})
		return
	}
	pageToken, err := security.DecryptSecret(page.EncryptedPageToken, h.encKey())
	if err != nil || pageToken == "" {
		c.JSON(http.StatusOK, gin.H{"buoc": "giải mã token", "loi": "không giải mã được Page Token"})
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	call := func(u string) gin.H {
		r, reqErr := client.Get(u)
		if reqErr != nil {
			return gin.H{"loi": "lỗi mạng khi gọi Facebook"} // không in err vì có thể chứa token
		}
		defer r.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(r.Body, 2048))
		out := gin.H{"status": r.StatusCode, "content_type": r.Header.Get("Content-Type")}
		if r.StatusCode != http.StatusOK {
			out["facebook_tra_ve"] = string(raw)
		}
		return out
	}

	c.JSON(http.StatusOK, gin.H{
		"profile_pic_api": call(fmt.Sprintf(
			"https://graph.facebook.com/v21.0/%s?fields=name,profile_pic&access_token=%s",
			url.PathEscape(psid), url.QueryEscape(pageToken))),
		"picture_api": call(fmt.Sprintf(
			"https://graph.facebook.com/v21.0/%s/picture?type=normal&access_token=%s",
			url.PathEscape(psid), url.QueryEscape(pageToken))),
	})
}
