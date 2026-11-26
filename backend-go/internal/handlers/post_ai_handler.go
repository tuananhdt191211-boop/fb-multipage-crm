package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"fb-multipage-crm/internal/config"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PostAIHandler struct {
	DB         *gorm.DB
	Cfg        *config.Config
	HTTPClient *http.Client
}

type aiChatHistoryItem struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

type aiRewriteRequest struct {
	Provider      string              `json:"provider"`
	Content       string              `json:"content"`
	ReferenceForm string              `json:"reference_form"`
	Insight       string              `json:"insight"`
	Count         int                 `json:"count"` // 10 - 20 bài
	Prompt        string              `json:"prompt"`
	Conditions    []string            `json:"conditions"`
	History       []aiChatHistoryItem `json:"history"`
}

type aiContentSuggestion struct {
	Angle   string `json:"angle"`
	Content string `json:"content"`
}

type aiStructuredPayload struct {
	Reply          string                `json:"reply"`
	RevisedContent string                `json:"revised_content"`
	Suggestions    []aiContentSuggestion `json:"suggestions"`
}

func NewPostAIHandler(db *gorm.DB, cfg *config.Config) *PostAIHandler {
	return &PostAIHandler{
		DB:         db,
		Cfg:        cfg,
		HTTPClient: &http.Client{Timeout: 90 * time.Second},
	}
}

func normalizeSuggestionCount(c int) int {
	if c < 10 {
		return 10
	}
	if c > 20 {
		return 20
	}
	return c
}

// resolveAIConfig dùng trực tiếp AI_API_KEY, AI_BASE_URL, AI_MODEL giống phần Inbox
func (h *PostAIHandler) resolveAIConfig(preferredProvider string) (apiKey, baseURL, model string) {
	apiKey = strings.TrimSpace(os.Getenv("AI_API_KEY"))
	baseURL = strings.TrimRight(strings.TrimSpace(os.Getenv("AI_BASE_URL")), "/")
	model = strings.TrimSpace(os.Getenv("AI_MODEL"))

	// Nếu có cấu hình riêng lẻ cho Gemini / OpenAI và người dùng chọn provider tương ứng
	pref := strings.ToLower(strings.TrimSpace(preferredProvider))
	if pref == "gemini" && strings.TrimSpace(os.Getenv("GEMINI_API_KEY")) != "" {
		apiKey = strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
		baseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
		if gm := strings.TrimSpace(os.Getenv("GEMINI_MODEL")); gm != "" {
			model = gm
		} else {
			model = "gemini-2.5-flash"
		}
		return apiKey, baseURL, model
	}
	if pref == "openai" && strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) != "" {
		apiKey = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
		baseURL = "https://api.openai.com/v1"
		if om := strings.TrimSpace(os.Getenv("OPENAI_MODEL")); om != "" {
			model = om
		} else {
			model = "gpt-4o-mini"
		}
		return apiKey, baseURL, model
	}

	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "gpt-4o-mini"
	}
	return apiKey, baseURL, model
}

func (h *PostAIHandler) buildSystemPrompt(count int) string {
	return fmt.Sprintf(`Bạn là giám đốc sáng tạo nội dung (Creative Copywriter) chuyên viết bài bán hàng & xây dựng thương hiệu trên Facebook Fanpage.

QUY TẮC BẮT BUỘC:
1. KHÔNG dùng mẫu câu sáo rỗng hay lặp lại văn phong máy móc. Hãy tự đọc hiểu sâu sắc bất kỳ nội dung/sản phẩm/chủ đề nào người dùng đưa vào, tự suy nghĩ văn phong tự nhiên, sắc bén, giàu cảm xúc và thuyết phục như người viết chuyên nghiệp thực thụ.
2. Phân tích cấu trúc, nhịp điệu, cách trình bày (hook mở đầu, mạch dẫn insight, điểm nhấn lợi ích, ưu đãi, CTA, hashtag) từ "Form mẫu tham chiếu" (nếu có) và khai thác sâu "Insight người dùng" (nếu có).
3. Sáng tạo ĐÚNG %d bài content hoàn chỉnh khác nhau trong mảng "suggestions". Mỗi bài phải:
   - Có một góc tiếp cận / tâm lý khách hàng ("angle") hoàn toàn khác biệt (ví dụ: nỗi đau thực tế, tình huống sử dụng đời thường, so sánh thông minh, câu chuyện trải nghiệm, giải đáp băn khoăn, khan hiếm ưu đãi, quà tặng, lợi ích dài hạn...).
   - Tự nghĩ câu từ mới 100%%, tuyệt đối không copy-paste lại y nguyên các câu từ bài mẫu hay lặp lại giữa các bài.
   - Cuối mỗi bài luôn có một dòng gồm 5 đến 6 hashtag tự nhiên, đa dạng, sát với sản phẩm và góc insight của riêng bài đó.
4. Nếu người dùng chat yêu cầu chỉnh sửa cụ thể (đổi giọng văn, thêm ý, đổi ưu đãi, rút gọn, viết lại bài số mấy...), hãy thực hiện chính xác theo lịch sử chat.
5. Trả về DUY NHẤT JSON hợp lệ (không thêm giải thích ngoài JSON):
{
  "reply": "Nhận xét ngắn gọn cách bạn đã triển khai ý tưởng và các góc insight",
  "revised_content": "Bài content xuất sắc nhất trong danh sách để đưa lên khung đăng bài",
  "suggestions": [
    {
      "angle": "Tên góc insight / tâm lý khách hàng",
      "content": "Toàn bộ nội dung bài đăng hoàn chỉnh, xuống dòng rõ ràng, kèm 5-6 hashtag cuối bài"
    }
  ]
}`, count)
}

func (h *PostAIHandler) buildUserPrompt(req aiRewriteRequest, count int) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Hãy tự suy nghĩ và viết %d bài content khác nhau.\n\n", count))

	if strings.TrimSpace(req.Insight) != "" {
		sb.WriteString("=== INSIGHT KHÁCH HÀNG ===\n")
		sb.WriteString(strings.TrimSpace(req.Insight))
		sb.WriteString("\n==========================\n\n")
	}

	if len(req.Conditions) > 0 {
		sb.WriteString("Điều kiện ưu tiên: ")
		sb.WriteString(strings.Join(req.Conditions, ", "))
		sb.WriteString("\n\n")
	}

	if strings.TrimSpace(req.ReferenceForm) != "" {
		sb.WriteString("=== CONTENT / FORM MẪU NGƯỜI DÙNG ĐƯA VÀO ===\n")
		sb.WriteString(strings.TrimSpace(req.ReferenceForm))
		sb.WriteString("\n==============================================\n\n")
	}

	if strings.TrimSpace(req.Content) != "" && strings.TrimSpace(req.Content) != strings.TrimSpace(req.ReferenceForm) {
		sb.WriteString("=== NỘI DUNG ĐANG CÓ TRÊN KHUNG SOẠN THẢO ===\n")
		sb.WriteString(strings.TrimSpace(req.Content))
		sb.WriteString("\n=============================================\n\n")
	}

	if len(req.History) > 0 {
		sb.WriteString("=== LỊCH SỬ TRAO ĐỔI TRƯỚC ĐÓ ===\n")
		for _, item := range req.History {
			roleLabel := "Người dùng"
			if item.Role == "assistant" {
				roleLabel = "AI"
			}
			sb.WriteString(fmt.Sprintf("%s: %s\n", roleLabel, strings.TrimSpace(item.Text)))
		}
		sb.WriteString("=================================\n\n")
	}

	sb.WriteString("Yêu cầu cụ thể:\n")
	if strings.TrimSpace(req.Prompt) != "" {
		sb.WriteString(strings.TrimSpace(req.Prompt))
	} else {
		sb.WriteString("Tự phân tích sản phẩm/chủ đề trong nội dung trên và viết đủ số lượng bài với các góc insight đa dạng, tự nhiên, cuốn hút nhất.")
	}

	return sb.String()
}

func parseAIModelOutput(raw string) aiStructuredPayload {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var parsed aiStructuredPayload
	if err := json.Unmarshal([]byte(cleaned), &parsed); err == nil {
		if strings.TrimSpace(parsed.RevisedContent) == "" && len(parsed.Suggestions) > 0 {
			parsed.RevisedContent = parsed.Suggestions[0].Content
		}
		if strings.TrimSpace(parsed.RevisedContent) != "" || len(parsed.Suggestions) > 0 {
			if strings.TrimSpace(parsed.Reply) == "" {
				parsed.Reply = fmt.Sprintf("AI đã tự sáng tạo %d bài content theo ngữ cảnh của bạn.", len(parsed.Suggestions))
			}
			return parsed
		}
	}

	start := strings.Index(cleaned, "{")
	end := strings.LastIndex(cleaned, "}")
	if start != -1 && end > start {
		var parsedSub aiStructuredPayload
		if err := json.Unmarshal([]byte(cleaned[start:end+1]), &parsedSub); err == nil {
			if strings.TrimSpace(parsedSub.RevisedContent) == "" && len(parsedSub.Suggestions) > 0 {
				parsedSub.RevisedContent = parsedSub.Suggestions[0].Content
			}
			if strings.TrimSpace(parsedSub.RevisedContent) != "" || len(parsedSub.Suggestions) > 0 {
				if strings.TrimSpace(parsedSub.Reply) == "" {
					parsedSub.Reply = fmt.Sprintf("AI đã tự sáng tạo %d bài content theo ngữ cảnh của bạn.", len(parsedSub.Suggestions))
				}
				return parsedSub
			}
		}
	}

	return aiStructuredPayload{
		Reply:          "AI đã tạo nội dung mới cho bạn.",
		RevisedContent: cleaned,
		Suggestions: []aiContentSuggestion{
			{Angle: "Đề xuất từ AI", Content: cleaned},
		},
	}
}

func (h *PostAIHandler) callLLM(preferredProvider, systemPrompt, userPrompt string) (aiStructuredPayload, string, error) {
	apiKey, baseURL, model := h.resolveAIConfig(preferredProvider)
	if apiKey == "" {
		return aiStructuredPayload{}, "", errors.New("chưa cấu hình AI_API_KEY trên server")
	}

	endpoint := baseURL
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint = endpoint + "/chat/completions"
	}

	bodyMap := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.9,
		"max_tokens":  8192,
	}

	rawBody, err := json.Marshal(bodyMap)
	if err != nil {
		return aiStructuredPayload{}, model, err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(rawBody))
	if err != nil {
		return aiStructuredPayload{}, model, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := h.HTTPClient.Do(req)
	if err != nil {
		return aiStructuredPayload{}, model, err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return aiStructuredPayload{}, model, fmt.Errorf("AI HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	var chatRes struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBytes, &chatRes); err != nil {
		return aiStructuredPayload{}, model, err
	}
	if len(chatRes.Choices) == 0 || strings.TrimSpace(chatRes.Choices[0].Message.Content) == "" {
		return aiStructuredPayload{}, model, errors.New("AI trả về phản hồi rỗng")
	}

	return parseAIModelOutput(chatRes.Choices[0].Message.Content), model, nil
}

// RewriteContent POST /api/v1/posts/ai-rewrite
func (h *PostAIHandler) RewriteContent(c *gin.Context) {
	var req aiRewriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu yêu cầu AI không hợp lệ."})
		return
	}

	count := normalizeSuggestionCount(req.Count)
	sysPrompt := h.buildSystemPrompt(count)
	userPrompt := h.buildUserPrompt(req, count)

	result, usedModel, err := h.callLLM(req.Provider, sysPrompt, userPrompt)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": fmt.Sprintf("Không gọi được AI (%s): %v", usedModel, err),
		})
		return
	}

	// Nếu AI trả về ít hơn số lượng yêu cầu (ví dụ mới được 3-5 bài), tự động gọi bổ sung phần còn thiếu
	if len(result.Suggestions) > 0 && len(result.Suggestions) < count {
		missing := count - len(result.Suggestions)
		var existingAngles []string
		for _, s := range result.Suggestions {
			existingAngles = append(existingAngles, s.Angle)
		}
		followUpPrompt := fmt.Sprintf(
			"%s\n\nBạn đã viết %d bài với các góc: %s.\nHãy viết thêm ĐÚNG %d bài mới hoàn toàn với các góc insight KHÁC hẳn các góc trên.",
			userPrompt,
			len(result.Suggestions),
			strings.Join(existingAngles, "; "),
			missing,
		)
		if extra, _, errExtra := h.callLLM(req.Provider, h.buildSystemPrompt(missing), followUpPrompt); errExtra == nil && len(extra.Suggestions) > 0 {
			result.Suggestions = append(result.Suggestions, extra.Suggestions...)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"reply":           result.Reply,
		"revised_content": result.RevisedContent,
		"content":         result.RevisedContent,
		"suggestions":     result.Suggestions,
		"model":           usedModel,
	})
}
