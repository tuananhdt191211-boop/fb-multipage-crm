package handlers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type VideoDownloaderHandler struct{}

func NewVideoDownloaderHandler() *VideoDownloaderHandler {
	return &VideoDownloaderHandler{}
}

type fetchSocialVideoReq struct {
	URL string `json:"url" binding:"required"`
}

func isSupportedSocialVideoHost(rawURL string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", false
	}
	host := strings.ToLower(u.Hostname())
	switch {
	case strings.Contains(host, "tiktok.com"):
		return "TikTok", true
	case strings.Contains(host, "youtube.com") || strings.Contains(host, "youtu.be"):
		return "YouTube", true
	case strings.Contains(host, "facebook.com") || strings.Contains(host, "fb.watch") || strings.Contains(host, "fb.com"):
		return "Facebook", true
	case strings.Contains(host, "instagram.com"):
		return "Instagram", true
	default:
		return "", false
	}
}

// FetchSocialVideo POST /api/v1/posts/fetch-social-video
// Tải video từ TikTok (không watermark), YouTube, Facebook, Instagram về server dưới dạng MP4
func (h *VideoDownloaderHandler) FetchSocialVideo(c *gin.Context) {
	var req fetchSocialVideoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng nhập đường dẫn video hợp lệ."})
		return
	}

	targetURL := strings.TrimSpace(req.URL)
	platform, ok := isSupportedSocialVideoHost(targetURL)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Chỉ hỗ trợ link từ TikTok, YouTube, Facebook hoặc Instagram.",
		})
		return
	}

	if _, err := exec.LookPath("yt-dlp"); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Server chưa cài đặt công cụ yt-dlp để tải video mạng xã hội.",
		})
		return
	}

	uploadDir := filepath.Join("uploads", "posts")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không tạo được thư mục lưu video."})
		return
	}

	fileID := fmt.Sprintf("social_%d", time.Now().UnixNano())
	outputTemplate := filepath.Join(uploadDir, fileID+".%(ext)s")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	// Cấu hình yt-dlp: ưu tiên bản không watermark (TikTok) và định dạng MP4 chuẩn (H.264/AAC) tương thích Facebook
	args := []string{
		"--no-playlist",
		"--no-warnings",
		"--max-filesize", "150M",
		"-f", "bv*[ext=mp4]+ba[ext=m4a]/b[ext=mp4]/best",
		"--merge-output-format", "mp4",
		"-o", outputTemplate,
		targetURL,
	}

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errDetail := strings.TrimSpace(stderr.String())
		if errDetail == "" {
			errDetail = err.Error()
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Không tải được video từ %s: %s", platform, errDetail),
		})
		return
	}

	// Tìm file .mp4 vừa tạo
	matches, _ := filepath.Glob(filepath.Join(uploadDir, fileID+".*"))
	if len(matches) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tải video thất bại, không tìm thấy file đầu ra."})
		return
	}

	savedPath := matches[0]
	fileName := filepath.Base(savedPath)
	info, _ := os.Stat(savedPath)
	var sizeBytes int64
	if info != nil {
		sizeBytes = info.Size()
	}

	publicPath := "/uploads/posts/" + fileName
	c.JSON(http.StatusOK, gin.H{
		"message":    fmt.Sprintf("Đã tải xong video từ %s (không watermark)!", platform),
		"platform":   platform,
		"url":        publicPath,
		"file_name":  fileName,
		"size_bytes": sizeBytes,
	})
}
