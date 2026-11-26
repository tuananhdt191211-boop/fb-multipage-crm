package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fb-multipage-crm/internal/config"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PostAIHandler struct {
	DB  *gorm.DB
	Cfg *config.Config
	AI  *AIHandler
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
	Count         int                 `json:"count"`
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

func NewPostAIHandler(db *gorm.DB, cfg *config.Config, aiHandler *AIHandler) *PostAIHandler {
	if aiHandler == nil {
		aiHandler = NewAIHandler(db, cfg)
	}
	return &PostAIHandler{
		DB:  db,
		Cfg: cfg,
		AI:  aiHandler,
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

func (h *PostAIHandler) buildSystemPrompt(count int) string {
	return fmt.Sprintf(`Bạn là giám đốc sáng tạo nội dung (Creative Copywriter) chuyên viết bài bán hàng & truyền thông trên Facebook Fanpage.

QUY TẮC BẮT BUỘC:
1. Đọc hiểu sâu sắc nội dung/sản phẩm/chủ đề người dùng đưa vào, tự suy nghĩ văn phong tự nhiên, sắc bén, giàu cảm xúc và thuyết phục.
2. Phân tích cấu trúc, nhịp điệu, cách trình bày từ "Form mẫu tham chiếu" (nếu có) và khai thác sâu "Insight người dùng" (nếu có).
3. Sáng tạo ĐÚNG %d bài content hoàn chỉnh khác nhau trong mảng "suggestions". Mỗi bài phải:
   - Có một góc tiếp cận / tâm lý khách hàng ("angle") hoàn toàn khác biệt.
   - Tự nghĩ câu từ mới 100%%, không lặp lại máy móc giữa các bài.
   - Cuối mỗi bài luôn có 5 đến 6 hashtag tự nhiên, sát với nội dung và góc insight của riêng bài đó.
4. Đặt "revised_content" là bài nổi bật nhất để đưa lên khung soạn thảo chính.
5. Trả về DUY NHẤT JSON hợp lệ (không bọc markdown):
{
  "reply": "Tóm tắt ngắn gọn các góc insight bạn vừa triển khai",
  "revised_content": "Bài content xuất sắc nhất",
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
		sb.WriteString("Tự phân tích nội dung trên và viết đủ số lượng bài với các góc insight đa dạng, tự nhiên, cuốn hút nhất.")
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

// RewriteContent POST /api/v1/posts/ai-rewrite (dùng chung bộ cấu hình & xoay vòng key của AIHandler)
func (h *PostAIHandler) RewriteContent(c *gin.Context) {
	var req aiRewriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu yêu cầu AI không hợp lệ."})
		return
	}

	count := normalizeSuggestionCount(req.Count)
	sysPrompt := h.buildSystemPrompt(count)
	userPrompt := h.buildUserPrompt(req, count)

	_, _, usedModel := h.AI.resolveAIConfig()

	rawOut, err := h.AI.callUniversalChatAI(sysPrompt, userPrompt, true)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": err.Error(),
		})
		return
	}

	result := parseAIModelOutput(rawOut)

	// Nếu AI trả về ít hơn số lượng yêu cầu, gọi bổ sung cho đủ
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
		if extraRaw, errExtra := h.AI.callUniversalChatAI(h.buildSystemPrompt(missing), followUpPrompt, true); errExtra == nil {
			extra := parseAIModelOutput(extraRaw)
			if len(extra.Suggestions) > 0 {
				result.Suggestions = append(result.Suggestions, extra.Suggestions...)
			}
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

// UploadPostImages POST /api/v1/posts/upload-images
func (h *PostAIHandler) UploadPostImages(c *gin.Context) {
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

	uploadDir := filepath.Join("uploads", "posts")
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

		filename := fmt.Sprintf("post_%d_%d%s", time.Now().UnixNano(), idx, ext)
		dstPath := filepath.Join(uploadDir, filename)
		if err := c.SaveUploadedFile(file, dstPath); err == nil {
			urls = append(urls, "/uploads/posts/"+filename)
		}
	}

	if len(urls) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Định dạng ảnh không hỗ trợ (chỉ nhận JPG, PNG, WEBP, GIF)"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"urls": urls})
}
