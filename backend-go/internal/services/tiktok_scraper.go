package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type TikTokVideoItem struct {
	VideoID      string `json:"video_id"`
	Title        string `json:"title"`
	Cover        string `json:"cover"`
	PlayCount    int64  `json:"play_count"`
	DiggCount    int64  `json:"digg_count"`
	CommentCount int64  `json:"comment_count"`
	ShareCount   int64  `json:"share_count"`
	CreateTime   int64  `json:"create_time"`
}

type tikwmUserPostsResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Videos  []TikTokVideoItem `json:"videos"`
		Cursor  string            `json:"cursor"`
		HasMore bool              `json:"hasMore"`
	} `json:"data"`
}

// buildHTTPClient tạo client có hỗ trợ HTTP/HTTPS/SOCKS5 Proxy từ biến môi trường OUTBOUND_PROXY_URL
func buildHTTPClient() *http.Client {
	transport := &http.Transport{}
	proxyRaw := strings.TrimSpace(os.Getenv("OUTBOUND_PROXY_URL"))
	if proxyRaw != "" {
		if proxyURL, err := url.Parse(proxyRaw); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}
	return &http.Client{
		Transport: transport,
		Timeout:   25 * time.Second,
	}
}

// FetchAllTikTokChannelVideos lặp qua toàn bộ cursor để lấy danh sách video của một username TikTok
func FetchAllTikTokChannelVideos(rawUsername string) ([]TikTokVideoItem, error) {
	username := strings.TrimSpace(rawUsername)
	username = strings.TrimPrefix(username, "https://www.tiktok.com/")
	username = strings.TrimPrefix(username, "https://tiktok.com/")
	username = strings.TrimPrefix(username, "@")
	if idx := strings.IndexAny(username, "/?"); idx != -1 {
		username = username[:idx]
	}
	if username == "" {
		return nil, fmt.Errorf("username TikTok không hợp lệ")
	}

	apiBase := strings.TrimSpace(os.Getenv("TIKTOK_SCRAPER_API_URL"))
	if apiBase == "" {
		apiBase = "https://www.tikwm.com/api/user/posts"
	}

	maxPages := 10
	if envPages := strings.TrimSpace(os.Getenv("TIKTOK_MAX_PAGES")); envPages != "" {
		if n, err := strconv.Atoi(envPages); err == nil {
			maxPages = n
		}
	}

	client := buildHTTPClient()
	cursor := "0"
	pageCount := 0
	var allVideos []TikTokVideoItem
	seenIDs := make(map[string]bool)

	for {
		pageCount++
		if maxPages > 0 && pageCount > maxPages {
			break
		}

		reqURL := fmt.Sprintf(
			"%s?unique_id=%s&count=30&cursor=%s",
			apiBase,
			url.QueryEscape(username),
			url.QueryEscape(cursor),
		)

		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		if err != nil {
			return allVideos, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return allVideos, fmt.Errorf("lỗi kết nối API TikTok (trang %d): %w", pageCount, err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return allVideos, err
		}

		var parsed tikwmUserPostsResp
		if err := json.Unmarshal(body, &parsed); err != nil {
			return allVideos, fmt.Errorf("không thể đọc phản hồi JSON từ API (trang %d): %w", pageCount, err)
		}

		if parsed.Code != 0 {
			// Nếu đã lấy được ít nhất 1 trang trước đó thì trả về dữ liệu đã có thay vì hủy toàn bộ
			if len(allVideos) > 0 {
				break
			}
			return nil, fmt.Errorf("API TikTok trả lỗi: %s", parsed.Msg)
		}

		for _, v := range parsed.Data.Videos {
			if v.VideoID == "" || seenIDs[v.VideoID] {
				continue
			}
			seenIDs[v.VideoID] = true
			allVideos = append(allVideos, v)
		}

		if !parsed.Data.HasMore || parsed.Data.Cursor == "" || parsed.Data.Cursor == cursor {
			break
		}

		cursor = parsed.Data.Cursor
		// Giãn cách 1.2 giây giữa các trang cursor để tránh bị chặn tần suất (rate limit)
		time.Sleep(1200 * time.Millisecond)
	}

	return allVideos, nil
}
