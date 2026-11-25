package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"fb-multipage-crm/internal/config"
	"fb-multipage-crm/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SystemAIConfig lưu cấu hình AI động trong Database (ưu tiên hơn file .env)
type SystemAIConfig struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	BaseURL   string    `gorm:"type:text" json:"base_url"`
	APIKeys   string    `gorm:"type:text" json:"api_keys"`
	Model     string    `gorm:"type:varchar(150)" json:"model"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AIHandler struct {
	DB         *gorm.DB
	Cfg        *config.Config
	HTTPClient *http.Client
}

func NewAIHandler(db *gorm.DB, cfg *config.Config) *AIHandler {
	_ = db.AutoMigrate(&models.PageAISetting{}, &SystemAIConfig{})
	return &AIHandler{
		DB:  db,
		Cfg: cfg,
		HTTPClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &http.Transport{Proxy: nil},
		},
	}
}

func (h *AIHandler) resolveAIConfig() (baseURL string, apiKeys []string, model string) {
	// 1. Ưu tiên đọc cấu hình đã lưu từ giao diện web trong Database
	var dbCfg SystemAIConfig
	if h.DB != nil {
		if err := h.DB.First(&dbCfg, 1).Error; err == nil && strings.TrimSpace(dbCfg.APIKeys) != "" {
			baseURL = strings.TrimRight(strings.TrimSpace(dbCfg.BaseURL), "/")
			model = strings.TrimSpace(dbCfg.Model)
			for _, k := range strings.Split(dbCfg.APIKeys, ",") {
				if trimmed := strings.TrimSpace(k); trimmed != "" {
					apiKeys = append(apiKeys, trimmed)
				}
			}
		}
	}

	// 2. Nếu Database chưa cấu hình thì đọc từ file .env
	if len(apiKeys) == 0 {
		if baseURL == "" {
			baseURL = strings.TrimRight(strings.TrimSpace(os.Getenv("AI_BASE_URL")), "/")
		}
		if model == "" {
			model = strings.TrimSpace(os.Getenv("AI_MODEL"))
		}
		rawKeyStr := strings.TrimSpace(os.Getenv("AI_API_KEY"))

		if rawKeyStr == "" && h.Cfg != nil {
			rawKeyStr = strings.TrimSpace(h.Cfg.GeminiAPIKey)
			if baseURL == "" {
				baseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
			}
			if model == "" {
				model = "gemini-3.5-flash-lite"
			}
		}

		for _, k := range strings.Split(rawKeyStr, ",") {
			if trimmed := strings.TrimSpace(k); trimmed != "" {
				apiKeys = append(apiKeys, trimmed)
			}
		}
	}

	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
	}
	if model == "" {
		model = "gemini-3.5-flash-lite"
	}

	// Cho phép Ollama local chạy không cần API key
	if len(apiKeys) == 0 && (strings.Contains(baseURL, "localhost") || strings.Contains(baseURL, "127.0.0.1")) {
		apiKeys = []string{"ollama-local"}
	}
	return baseURL, apiKeys, model
}

// GetGlobalAIConfig lấy cấu hình AI hiện tại hiển thị lên giao diện cài đặt
func (h *AIHandler) GetGlobalAIConfig(c *gin.Context) {
	baseURL, apiKeys, model := h.resolveAIConfig()
	rawKeys := strings.Join(apiKeys, ", ")
	if rawKeys == "ollama-local" {
		rawKeys = ""
	}
	c.JSON(http.StatusOK, gin.H{
		"base_url":  baseURL,
		"api_keys":  rawKeys,
		"model":     model,
		"key_count": len(apiKeys),
	})
}

// SaveGlobalAIConfig lưu API Key mới từ giao diện vào Database (áp dụng ngay lập tức)
func (h *AIHandler) SaveGlobalAIConfig(c *gin.Context) {
	var req struct {
		BaseURL string `json:"base_url"`
		APIKeys string `json:"api_keys"`
		Model   string `json:"model"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu cấu hình không hợp lệ"})
		return
	}

	// Chuẩn hóa danh sách key (chấp nhận cả xuống dòng hoặc dấu phẩy)
	normalized := strings.ReplaceAll(req.APIKeys, "\n", ",")
	normalized = strings.ReplaceAll(normalized, "\r", ",")
	var cleanKeys []string
	for _, k := range strings.Split(normalized, ",") {
		if t := strings.TrimSpace(k); t != "" {
			cleanKeys = append(cleanKeys, t)
		}
	}

	if len(cleanKeys) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng nhập ít nhất 1 mã API Key hợp lệ"})
		return
	}

	baseURL := strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = "gemini-3.5-flash-lite"
	}

	cfg := SystemAIConfig{
		ID:      1,
		BaseURL: baseURL,
		APIKeys: strings.Join(cleanKeys, ","),
		Model:   model,
	}
	if err := h.DB.Save(&cfg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể lưu cấu hình vào Database"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   fmt.Sprintf("Đã lưu thành công %d API Key! Hệ thống đã áp dụng ngay.", len(cleanKeys)),
		"key_count": len(cleanKeys),
	})
}

// TestGlobalAIConfig kiểm tra thử kết nối AI ngay sau khi nhập key
func (h *AIHandler) TestGlobalAIConfig(c *gin.Context) {
	reply, err := h.callUniversalChatAI(
		"Bạn là trợ lý kiểm tra hệ thống.",
		"Trả lời ngắn gọn 1 câu tiếng Việt: Kết nối AI thành công!",
		false,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Kết nối AI hoạt động tốt!",
		"reply":   reply,
	})
}

type chatMessagePayload struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (h *AIHandler) callUniversalChatAI(systemPrompt, userPrompt string, jsonMode bool) (string, error) {
	baseURL, apiKeys, model := h.resolveAIConfig()
	if len(apiKeys) == 0 {
		return "", fmt.Errorf("chưa cấu hình API Key AI. Hãy bấm nút Cài đặt API Key để thêm mã")
	}

	messages := []chatMessagePayload{}
	if strings.TrimSpace(systemPrompt) != "" {
		messages = append(messages, chatMessagePayload{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, chatMessagePayload{Role: "user", Content: userPrompt})

	reqPayload := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"temperature": 0.65,
	}
	if jsonMode && !strings.Contains(baseURL, "openrouter.ai") {
		reqPayload["response_format"] = map[string]string{"type": "json_object"}
	}
	bodyBytes, _ := json.Marshal(reqPayload)

	endpoint := baseURL + "/chat/completions"
	referer := strings.TrimSpace(os.Getenv("FRONTEND_URL"))
	if referer == "" {
		referer = "http://localhost:5173"
	}

	var lastErr error

	for idx, apiKey := range apiKeys {
		for attempt := 1; attempt <= 2; attempt++ {
			req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(bodyBytes))
			if err != nil {
				return "", err
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+apiKey)
			req.Header.Set("HTTP-Referer", referer)
			req.Header.Set("X-Title", "PagePilot CRM")
			req.Header.Set("User-Agent", "PagePilot-CRM/1.0")

			resp, err := h.HTTPClient.Do(req)
			if err != nil {
				lastErr = fmt.Errorf("lỗi kết nối AI (%s): %v", model, err)
				log.Printf("[AI][URL: %s][Key #%d][Attempt %d] %v", endpoint, idx+1, attempt, lastErr)
				time.Sleep(1 * time.Second)
				continue
			}

			respData, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			var chatResp struct {
				Choices []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				} `json:"choices"`
				Error *struct {
					Message string      `json:"message"`
					Code    interface{} `json:"code"`
				} `json:"error"`
			}

			if err := json.Unmarshal(respData, &chatResp); err != nil {
				lastErr = fmt.Errorf("phản hồi AI không hợp lệ (HTTP %d): %s", resp.StatusCode, string(respData))
				log.Printf("[AI][URL: %s] %v", endpoint, lastErr)
				break
			}

			if chatResp.Error != nil || resp.StatusCode >= 400 {
				errMsg := fmt.Sprintf("HTTP %d", resp.StatusCode)
				if chatResp.Error != nil && chatResp.Error.Message != "" {
					errMsg = chatResp.Error.Message
				}
				lastErr = fmt.Errorf("AI báo lỗi (%d): %s", resp.StatusCode, errMsg)
				log.Printf("[AI][URL: %s][Model: %s][Key #%d][Attempt %d] %v", endpoint, model, idx+1, attempt, lastErr)

				if resp.StatusCode == http.StatusTooManyRequests {
					break
				}
				if resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusBadGateway {
					time.Sleep(1500 * time.Millisecond)
					continue
				}
				break
			}

			if len(chatResp.Choices) > 0 {
				replyText := strings.TrimSpace(chatResp.Choices[0].Message.Content)
				if replyText != "" {
					return replyText, nil
				}
			}
			lastErr = fmt.Errorf("AI trả về nội dung rỗng")
		}
	}

	if lastErr != nil && strings.Contains(lastErr.Error(), "(429)") {
		return "", fmt.Errorf("Tất cả API Key đã chạm giới hạn lượt gọi (429). Hãy mở Cài đặt AI để thêm key dự phòng.")
	}
	return "", lastErr
}

// ListAllPageSettings trả về danh sách nhãn sản phẩm & màu sắc của tất cả Fanpage
func (h *AIHandler) ListAllPageSettings(c *gin.Context) {
	var settings []models.PageAISetting
	_ = h.DB.Find(&settings).Error
	c.JSON(http.StatusOK, gin.H{"settings": settings})
}

func (h *AIHandler) GetPageSetting(c *gin.Context) {
	pageID := strings.TrimSpace(c.Param("page_id"))
	var setting models.PageAISetting
	res := h.DB.Where("page_id = ?", pageID).Limit(1).Find(&setting)
	if res.Error != nil || res.RowsAffected == 0 {
		setting = models.PageAISetting{
			PageID:       pageID,
			Enabled:      true,
			AutoReply:    false,
			TagColor:     "emerald",
			Tone:         "Thân thiện, nhiệt tình, xưng em - anh/chị, ngắn gọn tự nhiên",
			SystemPrompt: "Tư vấn sản phẩm rõ ràng, hỏi thăm nhu cầu và xin số điện thoại/địa chỉ khi khách muốn đặt hàng.",
		}
	}
	c.JSON(http.StatusOK, gin.H{"setting": setting})
}

type updateAISettingReq struct {
	Enabled      bool   `json:"enabled"`
	AutoReply    bool   `json:"auto_reply"`
	ProductTag   string `json:"product_tag"`
	TagColor     string `json:"tag_color"`
	QuickReplies string `json:"quick_replies"`
	Tone         string `json:"tone"`
	SystemPrompt string `json:"system_prompt"`
}

func (h *AIHandler) SavePageSetting(c *gin.Context) {
	pageID := strings.TrimSpace(c.Param("page_id"))
	var req updateAISettingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu cấu hình AI không hợp lệ"})
		return
	}

	var setting models.PageAISetting
	res := h.DB.Where("page_id = ?", pageID).Limit(1).Find(&setting)
	if res.RowsAffected == 0 {
		setting = models.PageAISetting{PageID: pageID}
	}
	setting.Enabled = req.Enabled
	setting.AutoReply = req.AutoReply
	setting.ProductTag = strings.TrimSpace(req.ProductTag)
	if strings.TrimSpace(req.TagColor) != "" {
		setting.TagColor = strings.TrimSpace(req.TagColor)
	}
	setting.QuickReplies = req.QuickReplies
	setting.Tone = strings.TrimSpace(req.Tone)
	setting.SystemPrompt = strings.TrimSpace(req.SystemPrompt)
	_ = h.DB.Save(&setting).Error

	c.JSON(http.StatusOK, gin.H{
		"message": "Đã lưu cấu hình sản phẩm, tin nhắn mẫu & AI cho Fanpage!",
		"setting": setting,
	})
}

func (h *AIHandler) GenerateReplyForConversation(conv models.Conversation) (string, error) {
	return h.GenerateReplyWithContext(conv, "", "")
}

func (h *AIHandler) GenerateReplyWithContext(conv models.Conversation, selectedMessage, customInstruction string) (string, error) {
	var setting models.PageAISetting
	_ = h.DB.Where("page_id = ?", conv.PageID).Limit(1).Find(&setting).Error

	productInfo := ""
	if strings.TrimSpace(setting.ProductTag) != "" {
		productInfo = fmt.Sprintf("Sản phẩm chính của Fanpage này: %s.\n", strings.TrimSpace(setting.ProductTag))
	}

	tone := setting.Tone
	if tone == "" {
		tone = "Thân thiện, lịch sự, xưng em - anh/chị, ngắn gọn dễ hiểu"
	}
	knowledge := setting.SystemPrompt
	if knowledge == "" {
		knowledge = "Hỗ trợ giải đáp thắc mắc của khách hàng và xin số điện thoại để tư vấn chi tiết."
	}

	var recentMsgs []models.Message
	_ = h.DB.Where("conversation_id = ?", conv.ID).Order("sent_at DESC, id DESC").Limit(15).Find(&recentMsgs).Error

	var transcript strings.Builder
	for i := len(recentMsgs) - 1; i >= 0; i-- {
		m := recentMsgs[i]
		roleLabel := "Khách hàng (" + conv.CustomerName + ")"
		if m.SenderType == "page" {
			roleLabel = "Nhân viên Fanpage"
		}
		text := m.Content
		if text == "" && m.AttachmentURL != "" {
			text = "[Khách gửi một hình ảnh]"
		}
		transcript.WriteString(fmt.Sprintf("- %s: %s\n", roleLabel, text))
	}

	focusBlock := ""
	if strings.TrimSpace(selectedMessage) != "" {
		focusBlock += fmt.Sprintf("\n[TIN NHẮN KHÁCH HÀNG ĐƯỢC CHỌN ĐỂ TRẢ LỜI TRỌNG TÂM]: \"%s\"", strings.TrimSpace(selectedMessage))
	}
	if strings.TrimSpace(customInstruction) != "" {
		focusBlock += fmt.Sprintf("\n[CHỈ ĐẠO / GỢI Ý CỦA NHÂN VIÊN TRỰC PAGE]: %s", strings.TrimSpace(customInstruction))
	}

	systemPrompt := fmt.Sprintf(`Bạn là nhân viên chăm sóc khách hàng của Fanpage "%s".
%sPhong cách giao tiếp: %s.
Thông tin sản phẩm, bảng giá và quy tắc tư vấn của shop:
%s
Quy tắc bắt buộc: Viết DUY NHẤT 1 câu trả lời tự nhiên để gửi ngay cho khách hàng. Không giải thích, không đặt trong dấu ngoặc kép, không tự bịa giá nếu không có trong thông tin shop hoặc chỉ đạo của nhân viên.`,
		conv.PageName,
		productInfo,
		tone,
		knowledge,
	)

	userPrompt := fmt.Sprintf(
		"Lịch sử hội thoại gần nhất với khách hàng \"%s\":\n%s%s\n\nHãy viết câu phản hồi tiếp theo:",
		conv.CustomerName,
		transcript.String(),
		focusBlock,
	)

	return h.callUniversalChatAI(systemPrompt, userPrompt, false)
}

func (h *AIHandler) SuggestReply(c *gin.Context) {
	convID, err := strconv.Atoi(c.Param("conversation_id"))
	if err != nil || convID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID hội thoại không hợp lệ"})
		return
	}

	var req struct {
		SelectedMessage string `json:"selected_message"`
		Instruction     string `json:"instruction"`
	}
	_ = c.ShouldBindJSON(&req)

	var conv models.Conversation
	if err := h.DB.First(&conv, convID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy hội thoại"})
		return
	}

	replyText, err := h.GenerateReplyWithContext(conv, req.SelectedMessage, req.Instruction)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"reply": replyText})
}

type generateScriptReq struct {
	PageName    string `json:"page_name"`
	ProductName string `json:"product_name" binding:"required"`
	ExtraNotes  string `json:"extra_notes"`
}

func (h *AIHandler) GenerateSalesScript(c *gin.Context) {
	var req generateScriptReq
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.ProductName) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng nhập tên sản phẩm (ở ô Nhãn nhận diện sản phẩm) trước khi tạo kịch bản"})
		return
	}

	prod := strings.TrimSpace(req.ProductName)
	userPrompt := fmt.Sprintf(`Hãy tạo trọn bộ kịch bản tư vấn bán hàng và 4 tin nhắn mẫu cho Fanpage "%s" chuyên bán sản phẩm: "%s".
Ghi chú thêm của shop: %s

Trả về kết quả JSON chính xác theo cấu trúc sau (không thêm markdown):
{
  "tone": "Mô tả ngắn gọn giọng điệu xưng hô phù hợp ngành hàng (VD: Thân thiện, nhiệt tình, xưng em - anh/chị, tư vấn như chuyên gia)",
  "system_prompt": "Kịch bản tư vấn 4 bước rõ ràng cho AI: 1) Chào hỏi & hỏi nhu cầu sử dụng, 2) Nêu 3 điểm nổi bật của sản phẩm & gợi ý khung giá/ưu đãi, 3) Xử lý băn khoăn thường gặp (độ bền, bảo hành, ship), 4) Kêu gọi xin Số điện thoại + Địa chỉ để lên đơn.",
  "quick_replies": [
    {
      "title": "Chào khách mới",
      "category": "Chào hỏi",
      "keywords": "hi, hello, xin chào, shop ơi, tư vấn",
      "autoTrigger": true,
      "content": "Dạ em chào {customer_name} ạ! Cảm ơn mình đã quan tâm đến %s tại {page_name}. Mình đang tìm dòng dùng cho nhu cầu nào để em tư vấn mẫu phù hợp nhất ạ?"
    },
    {
      "title": "Báo giá & Ưu đãi",
      "category": "Báo giá",
      "keywords": "giá, bao nhiêu, bn, báo giá",
      "autoTrigger": true,
      "content": "Dạ mẫu %s bên em đang có chương trình ưu đãi giảm giá trực tiếp kèm miễn phí giao hàng toàn quốc ạ. Anh/chị để lại SĐT em gửi bảng giá chi tiết từng phiên bản nhé!"
    },
    {
      "title": "Chính sách bảo hành",
      "category": "Chính sách",
      "keywords": "bảo hành, đổi trả, ship, giao hàng",
      "autoTrigger": false,
      "content": "Dạ sản phẩm bên em được kiểm tra hàng trước khi thanh toán, bảo hành chính hãng 12 tháng và lỗi 1 đổi 1 trong 30 ngày nên {customer_name} hoàn toàn yên tâm ạ!"
    },
    {
      "title": "Xin thông tin chốt đơn",
      "category": "Chốt đơn",
      "keywords": "đặt hàng, lấy 1, chốt, mua",
      "autoTrigger": false,
      "content": "Dạ vâng ạ! Để em lên đơn ưu đãi cho {customer_name}, mình cho em xin Tên người nhận + Số điện thoại + Địa chỉ nhận hàng nhé ạ!"
    }
  ]
}`, req.PageName, prod, strings.TrimSpace(req.ExtraNotes), prod, prod)

	rawOut, err := h.callUniversalChatAI(
		"Bạn là chuyên gia xây dựng kịch bản chốt đơn Messenger thực chiến tại Việt Nam. Chỉ trả về JSON thuần túy, không giải thích thêm.",
		userPrompt,
		false,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rawOut = strings.TrimPrefix(rawOut, "```json")
	rawOut = strings.TrimPrefix(rawOut, "```")
	rawOut = strings.TrimSuffix(rawOut, "```")
	rawOut = strings.TrimSpace(rawOut)

	var parsed struct {
		Tone         string               `json:"tone"`
		SystemPrompt string               `json:"system_prompt"`
		QuickReplies []quickReplyTemplate `json:"quick_replies"`
	}
	if err := json.Unmarshal([]byte(rawOut), &parsed); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "AI trả về định dạng JSON chưa chuẩn, vui lòng bấm thử lại."})
		return
	}

	for i := range parsed.QuickReplies {
		if parsed.QuickReplies[i].ID == "" {
			parsed.QuickReplies[i].ID = fmt.Sprintf("qr_ai_%d_%d", time.Now().UnixNano(), i)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "AI đã tạo xong kịch bản Sale & 4 mẫu tin nhắn cho sản phẩm!",
		"tone":          parsed.Tone,
		"system_prompt": parsed.SystemPrompt,
		"quick_replies": parsed.QuickReplies,
	})
}
