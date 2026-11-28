package handlers

// Marketing Messages on Messenger
// ---------------------------------------------------------------------------
// Luồng hợp lệ để gửi tin khuyến mãi cho khách cũ (kể cả ngoài 24h):
//   1. Trong 24h, Page gửi "lời mời nhận tin" (template notification_messages).
//   2. Khách bấm đồng ý -> webhook messaging_optins trả về notification_messages_token.
//   3. Tool lưu token vào bảng marketing_subscriptions.
//   4. Chiến dịch target_mode = "subscribers" gửi qua Marketing Messages API
//      (POST act_<AD_ACCOUNT_ID>/messages), tính phí theo tin được giao tới khách.
//
// Yêu cầu phía Meta (xem README / tin nhắn hướng dẫn):
//   - App kiểu Business, có Advanced Access: pages_messaging, ads_management,
//     paid_marketing_messages (hoặc marketing_messages_messenger).
//   - Tài khoản quảng cáo có phương thức thanh toán.
//   - System User Access Token (META_SYSTEM_USER_TOKEN) + META_AD_ACCOUNT_ID.
//   - Bật trường webhook "messaging_optins" cho app.
// Giới hạn: mỗi khách tối đa 1 tin marketing / ngày / Page.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"fb-multipage-crm/internal/models"
	"fb-multipage-crm/internal/security"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	defaultOptinTitle   = "Thông tin mới và khuyến mãi"
	maxOptinTitleRunes  = 65
	optinResendCooldown = 7 * 24 * time.Hour
	marketingCooldown   = 24 * time.Hour
	customerWindow      = 23*time.Hour + 45*time.Minute
)

// MarketingSubscription lưu token khách đã đồng ý nhận tin marketing
type MarketingSubscription struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	PageID         string     `gorm:"size:100;index;not null" json:"page_id"`
	ConversationID uint       `gorm:"index" json:"conversation_id"`
	RecipientPSID  string     `gorm:"size:120;index" json:"recipient_psid"`
	RecipientName  string     `gorm:"size:200" json:"recipient_name"`
	Token          string     `gorm:"size:512;uniqueIndex;not null" json:"-"`
	Title          string     `gorm:"size:200" json:"title"`
	Frequency      string     `gorm:"size:20" json:"frequency"`
	Timezone       string     `gorm:"size:60" json:"timezone"`
	Status         string     `gorm:"size:20;index;default:'active'" json:"status"` // active | stopped
	TokenExpiresAt *time.Time `json:"token_expires_at"`
	LastSentAt     *time.Time `json:"last_sent_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// MarketingOptinRequest ghi lại các lời mời đã gửi để không gửi trùng (Meta: 1 lời mời / tuần / tiêu đề)
type MarketingOptinRequest struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	PageID        string    `gorm:"size:100;index;not null" json:"page_id"`
	RecipientPSID string    `gorm:"size:120;index;not null" json:"recipient_psid"`
	Title         string    `gorm:"size:200" json:"title"`
	Status        string    `gorm:"size:20" json:"status"` // sent | failed
	ErrorMessage  string    `gorm:"type:text" json:"error_message"`
	CreatedAt     time.Time `gorm:"index" json:"created_at"`
}

// MarketingOptinEvent là object "optin" trong webhook messaging_optins
type MarketingOptinEvent struct {
	Type            string          `json:"type"`
	Payload         string          `json:"payload"`
	Title           string          `json:"title"`
	Token           string          `json:"notification_messages_token"`
	Frequency       string          `json:"notification_messages_frequency"`
	Timezone        string          `json:"notification_messages_timezone"`
	Status          string          `json:"notification_messages_status"`
	UserTokenStatus string          `json:"user_token_status"`
	TokenExpiry     json.RawMessage `json:"token_expiry_timestamp"`
}

func migrateMarketingTables(db *gorm.DB) {
	_ = db.AutoMigrate(&MarketingSubscription{}, &MarketingOptinRequest{})
}

// parseFlexibleUnix đọc timestamp dạng số hoặc chuỗi (giây hoặc mili-giây)
func parseFlexibleUnix(raw json.RawMessage) *time.Time {
	s := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	if s == "" || s == "null" {
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return nil
	}
	if n > 1e12 {
		n /= 1000
	}
	t := time.Unix(n, 0)
	return &t
}

func truncateRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	if max <= 1 {
		return string(r[:max])
	}
	return strings.TrimSpace(string(r[:max-1])) + "…"
}

// UpsertMarketingOptin xử lý sự kiện đăng ký / dừng / tiếp tục nhận tin từ webhook
func UpsertMarketingOptin(db *gorm.DB, pageID, psid string, o MarketingOptinEvent) {
	if o.Type != "notification_messages" {
		return
	}
	token := strings.TrimSpace(o.Token)
	status := strings.ToUpper(strings.TrimSpace(o.Status))

	scope := db.Model(&MarketingSubscription{}).Where("page_id = ?", pageID)
	if token != "" {
		scope = scope.Where("token = ?", token)
	} else {
		scope = scope.Where("recipient_ps_id = ?", psid)
	}

	switch status {
	case "STOP NOTIFICATIONS", "STOP_NOTIFICATIONS":
		_ = scope.Update("status", "stopped").Error
		cancelPendingMarketing(db, pageID, psid, token)
		log.Printf("[Marketing] Khách %s đã dừng nhận tin marketing của Page %s", psid, pageID)
		return
	case "RESUME NOTIFICATIONS", "RESUME_NOTIFICATIONS":
		_ = scope.Update("status", "active").Error
		log.Printf("[Marketing] Khách %s đã nhận tin marketing trở lại (Page %s)", psid, pageID)
		if token == "" {
			return
		}
	}

	if token == "" {
		return
	}

	var conv models.Conversation
	_ = db.Where("page_id = ? AND customer_ps_id = ?", pageID, psid).Limit(1).Find(&conv).Error

	var sub MarketingSubscription
	res := db.Where("token = ?", token).Limit(1).Find(&sub)
	sub.PageID = pageID
	sub.RecipientPSID = psid
	sub.Token = token
	sub.Status = "active"
	if conv.ID != 0 {
		sub.ConversationID = conv.ID
		sub.RecipientName = conv.CustomerName
	}
	if t := strings.TrimSpace(o.Title); t != "" {
		sub.Title = t
	}
	if f := strings.TrimSpace(o.Frequency); f != "" {
		sub.Frequency = f
	}
	if tz := strings.TrimSpace(o.Timezone); tz != "" {
		sub.Timezone = tz
	}
	if exp := parseFlexibleUnix(o.TokenExpiry); exp != nil {
		sub.TokenExpiresAt = exp
	}
	if res.RowsAffected > 0 {
		_ = db.Save(&sub).Error
	} else {
		_ = db.Create(&sub).Error
	}
	log.Printf("[Marketing] Khách %s (%s) đã đăng ký nhận tin của Page %s", sub.RecipientName, psid, pageID)
}

// StopMarketingForCustomer được gọi khi khách nhắn "ngừng nhận tin" / "stop"
func StopMarketingForCustomer(db *gorm.DB, pageID, psid string) {
	_ = db.Model(&MarketingSubscription{}).
		Where("page_id = ? AND recipient_ps_id = ?", pageID, psid).
		Update("status", "stopped").Error
	cancelPendingMarketing(db, pageID, psid, "")
}

func cancelPendingMarketing(db *gorm.DB, pageID, psid, token string) {
	q := db.Model(&BroadcastMessage{}).
		Where("page_id = ? AND status = ? AND COALESCE(subscription_token, '') <> ''", pageID, "pending")
	if token != "" {
		q = q.Where("subscription_token = ?", token)
	} else {
		q = q.Where("recipient_ps_id = ?", psid)
	}
	_ = q.Updates(map[string]any{"status": "cancelled", "error_message": "Khách đã dừng nhận tin marketing"}).Error
}

// ---------------------------------------------------------------------------
// Gọi Graph API
// ---------------------------------------------------------------------------

func (h *BroadcastHandler) graphVersion() string {
	if h.Cfg != nil && strings.TrimSpace(h.Cfg.MetaGraphVersion) != "" {
		return strings.TrimSpace(h.Cfg.MetaGraphVersion)
	}
	return "v23.0"
}

func (h *BroadcastHandler) adAccountID() string {
	if h.Cfg == nil {
		return ""
	}
	return strings.TrimPrefix(strings.TrimSpace(h.Cfg.MetaAdAccountID), "act_")
}

// marketingConfigError trả về lý do chưa dùng được Marketing Messages API (rỗng = sẵn sàng)
func (h *BroadcastHandler) marketingConfigError() string {
	var missing []string
	if h.adAccountID() == "" {
		missing = append(missing, "META_AD_ACCOUNT_ID")
	}
	if h.Cfg == nil || strings.TrimSpace(h.Cfg.MetaSystemUserToken) == "" {
		missing = append(missing, "META_SYSTEM_USER_TOKEN")
	}
	if len(missing) > 0 {
		return "Chưa cấu hình " + strings.Join(missing, ", ") + " trong file .env của backend"
	}
	return ""
}

// callGraph gửi request JSON tới Graph API; bearer rỗng thì dùng access_token trong URL
func (h *BroadcastHandler) callGraph(method, fullURL, bearer string, body any) ([]byte, int, string) {
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, fullURL, reader)
	if err != nil {
		return nil, -1, err.Error()
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := h.HTTPClient.Do(req)
	if err != nil {
		return nil, -1, "Không kết nối được Facebook: " + err.Error()
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)

	var fbErr struct {
		Error *struct {
			Code         int    `json:"code"`
			ErrorSubcode int    `json:"error_subcode"`
			Message      string `json:"message"`
			UserMsg      string `json:"error_user_msg"`
		} `json:"error"`
	}
	_ = json.Unmarshal(respBytes, &fbErr)
	if fbErr.Error != nil {
		code := fbErr.Error.Code
		if code == 0 {
			code = resp.StatusCode
		}
		msg := fbErr.Error.Message
		if strings.TrimSpace(fbErr.Error.UserMsg) != "" {
			msg += " – " + fbErr.Error.UserMsg
		}
		if fbErr.Error.ErrorSubcode != 0 {
			msg = fmt.Sprintf("%s (subcode %d)", msg, fbErr.Error.ErrorSubcode)
		}
		return respBytes, code, msg
	}
	if resp.StatusCode >= 400 {
		return respBytes, resp.StatusCode, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncateRunes(string(respBytes), 300))
	}
	return respBytes, 0, ""
}

// sendOptinRequest gửi template "notification_messages" (chỉ hợp lệ khi khách còn trong 24h)
func (h *BroadcastHandler) sendOptinRequest(pageID, pageToken, psid, title, imageURL string) (string, int, string) {
	payload := map[string]any{
		"template_type":                  "notification_messages",
		"title":                          title,
		"payload":                        "MARKETING_OPTIN",
		"notification_messages_timezone": "Asia/Ho_Chi_Minh",
		"notification_messages_cta_text": "GET_UPDATES",
	}
	if strings.TrimSpace(imageURL) != "" {
		payload["image_url"] = strings.TrimSpace(imageURL)
		payload["image_aspect_ratio"] = "HORIZONTAL"
	}
	body := map[string]any{
		"recipient":      map[string]string{"id": psid},
		"messaging_type": "RESPONSE",
		"message": map[string]any{
			"attachment": map[string]any{"type": "template", "payload": payload},
		},
	}
	u := fmt.Sprintf("https://graph.facebook.com/%s/%s/messages?access_token=%s",
		h.graphVersion(), pageID, url.QueryEscape(pageToken))
	respBytes, code, msg := h.callGraph(http.MethodPost, u, "", body)
	if msg != "" {
		return "", code, msg
	}
	var ok struct {
		MessageID string `json:"message_id"`
	}
	_ = json.Unmarshal(respBytes, &ok)
	return ok.MessageID, 0, ""
}

// ensureMetaCampaign tạo "message_campaign" bên Meta để gắn các tin marketing (chưa phát sinh phí)
func (h *BroadcastHandler) ensureMetaCampaign(camp *BroadcastCampaign) error {
	if camp.MetaCampaignID != "" {
		return nil
	}
	body := map[string]any{
		"name":    camp.Name,
		"page_id": camp.PageID,
	}
	if camp.MarketingDailyBudget > 0 {
		body["daily_budget"] = camp.MarketingDailyBudget
	}
	u := fmt.Sprintf("https://graph.facebook.com/%s/act_%s/message_campaign", h.graphVersion(), h.adAccountID())
	respBytes, code, msg := h.callGraph(http.MethodPost, u, h.Cfg.MetaSystemUserToken, body)
	if msg != "" {
		return fmt.Errorf("Meta từ chối tạo chiến dịch Marketing Messages (Code %d): %s", code, msg)
	}
	var res struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(respBytes, &res)
	if res.ID == "" {
		return fmt.Errorf("Meta không trả về ID chiến dịch: %s", truncateRunes(string(respBytes), 300))
	}
	camp.MetaCampaignID = res.ID
	return nil
}

// buildMarketingMessage dựng generic template (tiêu đề / mô tả tối đa 80 ký tự)
func buildMarketingMessage(camp BroadcastCampaign, text string) map[string]any {
	text = strings.TrimSpace(text)
	title := strings.TrimSpace(camp.MarketingTitle)
	subtitle := text
	if title == "" {
		lines := strings.SplitN(text, "\n", 2)
		title = lines[0]
		if len(lines) > 1 {
			subtitle = strings.TrimSpace(lines[1])
		} else if utf8.RuneCountInString(text) > 80 {
			subtitle = string([]rune(text)[79:])
		} else {
			subtitle = ""
		}
	}

	el := map[string]any{"title": truncateRunes(title, 80)}
	if s := truncateRunes(subtitle, 80); s != "" {
		el["subtitle"] = s
	}
	if img := strings.TrimSpace(camp.MarketingImageURL); img != "" {
		el["image_url"] = img
	}
	if link := strings.TrimSpace(camp.MarketingButtonURL); link != "" {
		btnTitle := strings.TrimSpace(camp.MarketingButtonTitle)
		if btnTitle == "" {
			btnTitle = "Xem ngay"
		}
		el["default_action"] = map[string]any{"type": "web_url", "url": link, "webview_height_ratio": "FULL"}
		el["buttons"] = []map[string]any{{"type": "web_url", "url": link, "title": truncateRunes(btnTitle, 20)}}
	}

	return map[string]any{
		"attachment": map[string]any{
			"type": "template",
			"payload": map[string]any{
				"template_type": "generic",
				"elements":      []map[string]any{el},
			},
		},
	}
}

// sendMarketingMessage gửi 1 tin marketing qua token đăng ký (không cần cửa sổ 24h)
func (h *BroadcastHandler) sendMarketingMessage(camp BroadcastCampaign, token, text string) (string, int, string) {
	if errMsg := h.marketingConfigError(); errMsg != "" {
		return "", -2, errMsg
	}
	body := map[string]any{
		"message_id":              camp.MetaCampaignID,
		"messenger_delivery_data": map[string]string{"subscription_token": token},
		"message":                 buildMarketingMessage(camp, text),
	}
	u := fmt.Sprintf("https://graph.facebook.com/%s/act_%s/messages", h.graphVersion(), h.adAccountID())
	respBytes, code, msg := h.callGraph(http.MethodPost, u, h.Cfg.MetaSystemUserToken, body)
	if msg != "" {
		if code == 2300052 {
			msg = "Khách vừa tương tác với Page gần đây nên Meta chưa cho gửi (2300052)"
		}
		return "", code, msg
	}
	var res struct {
		Success    bool   `json:"success"`
		TrackingID string `json:"marketing_message_tracking_id"`
	}
	_ = json.Unmarshal(respBytes, &res)
	if !res.Success && res.TrackingID == "" {
		return "", -1, "Meta không xác nhận đã gửi: " + truncateRunes(string(respBytes), 300)
	}
	return res.TrackingID, 0, ""
}

// processMarketingMessage được worker gọi cho các tin có subscription_token
func (h *BroadcastHandler) processMarketingMessage(camp *BroadcastCampaign, msg BroadcastMessage) {
	failMsg := func(code int, text string) {
		_ = h.DB.Model(&msg).Updates(map[string]any{
			"status": "failed", "error_code": code, "error_message": text,
		}).Error
		_ = h.DB.Model(camp).UpdateColumn("failed_count", gorm.Expr("failed_count + 1")).Error
	}

	var sub MarketingSubscription
	if h.DB.Where("token = ?", msg.SubscriptionToken).Limit(1).Find(&sub).RowsAffected == 0 || sub.Status != "active" {
		failMsg(0, "Khách đã dừng nhận tin marketing")
		return
	}
	if sub.LastSentAt != nil && time.Since(*sub.LastSentAt) < marketingCooldown {
		failMsg(0, "Khách đã nhận 1 tin marketing trong 24h qua (giới hạn của Meta)")
		return
	}

	if camp.MetaCampaignID == "" {
		if err := h.ensureMetaCampaign(camp); err != nil {
			_ = h.DB.Model(&msg).Update("status", "pending").Error
			_ = h.DB.Model(camp).Updates(map[string]any{"status": "paused", "pause_reason": err.Error()}).Error
			return
		}
		_ = h.DB.Model(camp).Update("meta_campaign_id", camp.MetaCampaignID).Error
	}

	trackingID, code, errMsg := h.sendMarketingMessage(*camp, msg.SubscriptionToken, msg.RenderedText)
	if errMsg != "" {
		log.Printf("[Marketing] Gửi thất bại tới %s (Code %d): %s", msg.RecipientPSID, code, errMsg)
		// Lỗi cấu hình / giới hạn tốc độ -> trả tin về hàng đợi và tạm dừng chiến dịch
		if code == -2 || code == 4 || code == 17 || code == 32 || code == 613 || code == 80006 {
			_ = h.DB.Model(&msg).Updates(map[string]any{"status": "pending", "error_code": code, "error_message": errMsg}).Error
			_ = h.DB.Model(camp).Updates(map[string]any{
				"status":       "paused",
				"pause_reason": fmt.Sprintf("Tạm dừng Marketing Messages (Code %d: %s)", code, errMsg),
			}).Error
			return
		}
		failMsg(code, errMsg)
		return
	}

	now := time.Now()
	_ = h.DB.Model(&msg).Updates(map[string]any{
		"status": "sent", "fb_message_id": trackingID, "sent_at": &now,
	}).Error
	_ = h.DB.Model(camp).UpdateColumn("sent_count", gorm.Expr("sent_count + 1")).Error
	_ = h.DB.Model(&MarketingSubscription{}).Where("id = ?", sub.ID).Update("last_sent_at", &now).Error

	if msg.ConversationID != 0 {
		var conv models.Conversation
		if h.DB.First(&conv, msg.ConversationID).Error == nil {
			conv.LastMessage = msg.RenderedText
			conv.LastMessageTime = now
			_ = h.DB.Save(&conv).Error
			fbID := "mm_" + trackingID
			if trackingID == "" {
				fbID = fmt.Sprintf("mm_%d_%d", msg.ID, now.UnixNano())
			}
			_ = h.DB.Create(&models.Message{
				ConversationID: conv.ID,
				FBMessageID:    fbID,
				SenderType:     "page",
				SenderName:     conv.PageName,
				Content:        "[Tin marketing] " + msg.RenderedText,
				SentAt:         now,
			}).Error
		}
	}
	log.Printf("[Marketing] Đã gửi tin marketing tới %s (%s)", msg.RecipientName, msg.RecipientPSID)
}

// ---------------------------------------------------------------------------
// API cho giao diện
// ---------------------------------------------------------------------------

type marketingCampaignInput struct {
	PageID        string
	Name          string
	Templates     []string
	MaxRecipients int
	MinDelaySec   int
	MaxDelaySec   int
	BatchSize     int
	BatchPauseSec int
	Title         string
	ImageURL      string
	ButtonURL     string
	ButtonTitle   string
	DailyBudget   int64
}

// createMarketingCampaign tạo chiến dịch gửi cho khách đã đăng ký nhận tin
func (h *BroadcastHandler) createMarketingCampaign(c *gin.Context, in marketingCampaignInput) {
	if errMsg := h.marketingConfigError(); errMsg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}

	var subs []MarketingSubscription
	if err := h.DB.Where("page_id = ? AND status = ?", in.PageID, "active").
		Where("token_expires_at IS NULL OR token_expires_at > ?", time.Now()).
		Where("last_sent_at IS NULL OR last_sent_at < ?", time.Now().Add(-marketingCooldown)).
		Order("updated_at DESC").
		Limit(in.MaxRecipients).
		Find(&subs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi đọc danh sách khách đăng ký: " + err.Error()})
		return
	}
	if len(subs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Page này chưa có khách nào đăng ký nhận tin (hoặc tất cả đã nhận tin trong 24h qua). Hãy gửi 'Lời mời nhận tin' cho khách trong 24h trước, hoặc bấm 'Đồng bộ người đăng ký'."})
		return
	}

	tplJSON, _ := json.Marshal(in.Templates)
	camp := BroadcastCampaign{
		PageID:               in.PageID,
		Name:                 in.Name,
		TemplatesJSON:        string(tplJSON),
		TargetMode:           "subscribers",
		DeliveryMode:         "marketing",
		OutOfWindowTag:       "",
		MinDelaySec:          in.MinDelaySec,
		MaxDelaySec:          in.MaxDelaySec,
		BatchSize:            in.BatchSize,
		BatchPauseSec:        in.BatchPauseSec,
		Status:               "running",
		TotalRecipients:      len(subs),
		MarketingTitle:       truncateRunes(in.Title, 80),
		MarketingImageURL:    strings.TrimSpace(in.ImageURL),
		MarketingButtonURL:   strings.TrimSpace(in.ButtonURL),
		MarketingButtonTitle: strings.TrimSpace(in.ButtonTitle),
		MarketingDailyBudget: in.DailyBudget,
	}

	if err := h.ensureMetaCampaign(&camp); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.DB.Create(&camp).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer broadcastWake.Wake() // đánh thức bộ gửi tin đang ngủ

	msgs := make([]BroadcastMessage, 0, len(subs))
	for _, s := range subs {
		name := strings.TrimSpace(s.RecipientName)
		if name == "" && s.ConversationID != 0 {
			var conv models.Conversation
			if h.DB.First(&conv, s.ConversationID).Error == nil {
				name = strings.TrimSpace(conv.CustomerName)
			}
		}
		display := name
		if display == "" {
			display = "anh/chị"
		}
		tpl := in.Templates[rand.Intn(len(in.Templates))]
		rendered := strings.ReplaceAll(tpl, "{{full_name}}", display)
		rendered = strings.ReplaceAll(rendered, "{customer_name}", display)

		msgs = append(msgs, BroadcastMessage{
			CampaignID:        camp.ID,
			PageID:            in.PageID,
			ConversationID:    s.ConversationID,
			RecipientPSID:     s.RecipientPSID,
			RecipientName:     name,
			RenderedText:      rendered,
			Status:            "pending",
			SubscriptionToken: s.Token,
		})
	}
	if err := h.DB.CreateInBatches(&msgs, 100).Error; err != nil {
		_ = h.DB.Delete(&camp).Error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không lưu được hàng đợi tin nhắn: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, camp)
}

// SendOptinRequests POST /api/v1/broadcasts/optin-requests
// Gửi lời mời nhận tin cho khách đã nhắn Page trong 24h qua (chưa đăng ký, chưa được mời tuần này)
func (h *BroadcastHandler) SendOptinRequests(c *gin.Context) {
	var req struct {
		PageID          string `json:"page_id" binding:"required"`
		Title           string `json:"title"`
		ImageURL        string `json:"image_url"`
		ConversationIDs []uint `json:"conversation_ids"`
		MaxRecipients   int    `json:"max_recipients"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	title := truncateRunes(req.Title, maxOptinTitleRunes)
	if title == "" {
		title = defaultOptinTitle
	}
	if req.MaxRecipients <= 0 || req.MaxRecipients > 1000 {
		req.MaxRecipients = 500
	}

	var page models.FacebookPage
	if err := h.DB.Where("page_id = ? AND workspace_id = ?", req.PageID, wsID(c)).First(&page).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy Page"})
		return
	}
	pageToken, err := security.DecryptSecret(page.EncryptedPageToken, h.getEncryptionKey())
	if err != nil || pageToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Không giải mã được Page Access Token"})
		return
	}

	if _, busy := h.optinRunning.LoadOrStore(req.PageID, true); busy {
		c.JSON(http.StatusConflict, gin.H{"error": "Page này đang gửi lời mời nhận tin, vui lòng đợi đợt trước chạy xong"})
		return
	}

	windowStart := time.Now().Add(-customerWindow)
	q := h.DB.Model(&models.Conversation{}).
		Where("page_id = ? AND COALESCE(TRIM(customer_ps_id), '') <> ''", req.PageID).
		Where("EXISTS (SELECT 1 FROM messages m WHERE m.conversation_id = conversations.id AND m.sender_type = 'customer' AND m.sent_at >= ?)", windowStart).
		Where("customer_ps_id NOT IN (?)", h.DB.Model(&MarketingSubscription{}).Select("recipient_ps_id").Where("page_id = ? AND status = ? AND recipient_ps_id IS NOT NULL", req.PageID, "active")).
		Where("customer_ps_id NOT IN (?)", h.DB.Model(&MarketingOptinRequest{}).Select("recipient_ps_id").Where("page_id = ? AND title = ? AND status = ? AND created_at >= ? AND recipient_ps_id IS NOT NULL", req.PageID, title, "sent", time.Now().Add(-optinResendCooldown)))
	if len(req.ConversationIDs) > 0 {
		q = q.Where("id IN ?", req.ConversationIDs)
	}

	var convs []models.Conversation
	if err := q.Order("last_message_time DESC").Limit(req.MaxRecipients).Find(&convs).Error; err != nil {
		h.optinRunning.Delete(req.PageID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi đọc danh sách khách: " + err.Error()})
		return
	}
	if len(convs) == 0 {
		h.optinRunning.Delete(req.PageID)
		c.JSON(http.StatusOK, gin.H{"queued": 0, "message": "Không có khách phù hợp: cần khách đã nhắn Page trong 24h qua, chưa đăng ký và chưa được mời với tiêu đề này trong 7 ngày."})
		return
	}

	go func(convs []models.Conversation) {
		defer h.optinRunning.Delete(req.PageID)
		sent, failed := 0, 0
		for i, conv := range convs {
			psid := strings.TrimSpace(conv.CustomerPSID)
			mid, code, errMsg := h.sendOptinRequest(page.PageID, pageToken, psid, title, req.ImageURL)
			rec := MarketingOptinRequest{PageID: page.PageID, RecipientPSID: psid, Title: title, Status: "sent"}
			if errMsg != "" {
				rec.Status = "failed"
				rec.ErrorMessage = fmt.Sprintf("Code %d: %s", code, errMsg)
				failed++
				log.Printf("[Marketing] Gửi lời mời thất bại tới %s: %s", psid, rec.ErrorMessage)
			} else {
				sent++
				now := time.Now()
				if mid == "" {
					mid = fmt.Sprintf("optin_%s_%d", psid, now.UnixNano())
				}
				_ = h.DB.Create(&models.Message{
					ConversationID: conv.ID,
					FBMessageID:    mid,
					SenderType:     "page",
					SenderName:     page.Name,
					Content:        "[Lời mời nhận tin] " + title,
					SentAt:         now,
				}).Error
			}
			_ = h.DB.Create(&rec).Error

			if code == 4 || code == 17 || code == 32 || code == 613 {
				log.Printf("[Marketing] Dừng gửi lời mời do giới hạn tốc độ Meta (Code %d)", code)
				break
			}
			if i < len(convs)-1 {
				time.Sleep(time.Duration(3+rand.Intn(4)) * time.Second)
			}
		}
		log.Printf("[Marketing] Page %s: đã gửi %d lời mời nhận tin, lỗi %d", page.Name, sent, failed)
	}(convs)

	c.JSON(http.StatusAccepted, gin.H{
		"queued":  len(convs),
		"message": fmt.Sprintf("Đang gửi lời mời nhận tin cho %d khách (giãn cách 3–6 giây/tin). Khách bấm đồng ý sẽ tự vào danh sách đăng ký.", len(convs)),
	})
}

// ListSubscribers GET /api/v1/broadcasts/subscribers?page_id=...
func (h *BroadcastHandler) ListSubscribers(c *gin.Context) {
	pageID := strings.TrimSpace(c.Query("page_id"))
	mine := wsPageIDs(h.DB, wsID(c))
	base := func() *gorm.DB {
		q := h.DB.Model(&MarketingSubscription{}).Where("page_id IN (?)", mine)
		if pageID != "" {
			q = q.Where("page_id = ?", pageID)
		}
		return q
	}
	var active, stopped, eligible int64
	base().Where("status = ?", "active").Count(&active)
	base().Where("status = ?", "stopped").Count(&stopped)
	base().Where("status = ?", "active").
		Where("token_expires_at IS NULL OR token_expires_at > ?", time.Now()).
		Where("last_sent_at IS NULL OR last_sent_at < ?", time.Now().Add(-marketingCooldown)).
		Count(&eligible)

	var list []MarketingSubscription
	_ = base().Order("updated_at DESC").Limit(200).Find(&list).Error

	var pendingInvites int64
	inv := h.DB.Model(&MarketingOptinRequest{}).Where("page_id IN (?)", mine).Where("status = ? AND created_at >= ?", "sent", time.Now().Add(-optinResendCooldown))
	if pageID != "" {
		inv = inv.Where("page_id = ?", pageID)
	}
	inv.Count(&pendingInvites)

	c.JSON(http.StatusOK, gin.H{
		"active":          active,
		"stopped":         stopped,
		"eligible_now":    eligible,
		"invites_7d":      pendingInvites,
		"marketing_ready": h.marketingConfigError() == "",
		"marketing_error": h.marketingConfigError(),
		"data":            list,
	})
}

// SyncSubscribers POST /api/v1/broadcasts/subscribers/sync?page_id=...
// Kéo toàn bộ token đăng ký hiện có của Page từ Meta về (phòng khi webhook bị lỡ)
func (h *BroadcastHandler) SyncSubscribers(c *gin.Context) {
	pageID := strings.TrimSpace(c.Query("page_id"))
	if pageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Thiếu page_id"})
		return
	}
	var page models.FacebookPage
	if err := h.DB.Where("page_id = ? AND workspace_id = ?", pageID, wsID(c)).First(&page).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy Page"})
		return
	}
	pageToken, err := security.DecryptSecret(page.EncryptedPageToken, h.getEncryptionKey())
	if err != nil || pageToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Không giải mã được Page Access Token"})
		return
	}

	nextURL := fmt.Sprintf("https://graph.facebook.com/%s/%s/notification_message_tokens?limit=1000&access_token=%s",
		h.graphVersion(), pageID, url.QueryEscape(pageToken))
	imported, pages := 0, 0
	for nextURL != "" && pages < 50 {
		pages++
		respBytes, code, errMsg := h.callGraph(http.MethodGet, nextURL, "", nil)
		if errMsg != "" {
			c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("Meta báo lỗi (Code %d): %s", code, errMsg), "imported": imported})
			return
		}
		var data struct {
			Data []struct {
				Token      string          `json:"notification_messages_token"`
				Recipient  string          `json:"recipient_id"`
				TopicTitle string          `json:"topic_title"`
				Timezone   string          `json:"notification_messages_timezone"`
				Expiry     json.RawMessage `json:"token_expiry_timestamp"`
			} `json:"data"`
			Paging struct {
				Next string `json:"next"`
			} `json:"paging"`
		}
		if err := json.Unmarshal(respBytes, &data); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "Không đọc được dữ liệu Meta trả về"})
			return
		}
		for _, t := range data.Data {
			token := strings.TrimSpace(t.Token)
			if token == "" {
				continue
			}
			var sub MarketingSubscription
			exists := h.DB.Where("token = ?", token).Limit(1).Find(&sub).RowsAffected > 0
			if exists && sub.Status == "stopped" {
				continue // tôn trọng lựa chọn dừng nhận tin của khách
			}
			sub.PageID = pageID
			sub.Token = token
			sub.Status = "active"
			if psid := strings.TrimSpace(t.Recipient); psid != "" {
				sub.RecipientPSID = psid
				var conv models.Conversation
				if h.DB.Where("page_id = ? AND customer_ps_id = ?", pageID, psid).Limit(1).Find(&conv).RowsAffected > 0 {
					sub.ConversationID = conv.ID
					sub.RecipientName = conv.CustomerName
				}
			}
			if s := strings.TrimSpace(t.TopicTitle); s != "" {
				sub.Title = s
			}
			if s := strings.TrimSpace(t.Timezone); s != "" {
				sub.Timezone = s
			}
			if exp := parseFlexibleUnix(t.Expiry); exp != nil {
				sub.TokenExpiresAt = exp
			}
			if exists {
				_ = h.DB.Save(&sub).Error
			} else {
				_ = h.DB.Create(&sub).Error
			}
			imported++
		}
		nextURL = data.Paging.Next
	}

	c.JSON(http.StatusOK, gin.H{
		"imported": imported,
		"message":  fmt.Sprintf("Đã đồng bộ %d người đăng ký nhận tin từ Meta", imported),
	})
}
