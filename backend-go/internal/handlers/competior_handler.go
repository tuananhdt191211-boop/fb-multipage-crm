package handlers

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"fb-multipage-crm/internal/config"
	"fb-multipage-crm/internal/models"
	"fb-multipage-crm/internal/security"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CompetitorHandler struct {
	DB         *gorm.DB
	Cfg        *config.Config
	HTTPClient *http.Client
}

func NewCompetitorHandler(db *gorm.DB, cfg *config.Config) *CompetitorHandler {
	_ = db.AutoMigrate(&models.CompetitorPage{}, &models.CompetitorPost{})
	return &CompetitorHandler{
		DB:         db,
		Cfg:        cfg,
		HTTPClient: &http.Client{Timeout: 45 * time.Second},
	}
}

func (h *CompetitorHandler) encKey() string {
	if h.Cfg != nil && h.Cfg.JWTSecret != "" {
		return h.Cfg.JWTSecret
	}
	return "default-dev-master-key-change-in-prod"
}

func (h *CompetitorHandler) getAnyConnectedPageToken() string {
	var page models.FacebookPage
	if err := h.DB.Where("is_connected = ? AND encrypted_page_token <> ''", true).First(&page).Error; err != nil {
		return ""
	}
	tok, err := security.DecryptSecret(page.EncryptedPageToken, h.encKey())
	if err != nil {
		return ""
	}
	return tok
}

func detectPlatformAndID(platformHint, input string) (string, string, string) {
	raw := strings.TrimSpace(input)
	lower := strings.ToLower(raw)
	platform := strings.ToLower(strings.TrimSpace(platformHint))

	if strings.Contains(lower, "youtube.com") || strings.Contains(lower, "youtu.be") {
		platform = "youtube"
	} else if strings.Contains(lower, "tiktok.com") {
		platform = "tiktok"
	} else if platform == "" {
		platform = "facebook"
	}

	switch platform {
	case "youtube":
		if strings.HasPrefix(raw, "UC") && len(raw) >= 20 {
			return "youtube", raw, "https://www.youtube.com/channel/" + raw
		}
		if strings.HasPrefix(raw, "@") {
			return "youtube", raw, "https://www.youtube.com/" + raw
		}
		if u, err := url.Parse(raw); err == nil && u.Host != "" {
			path := strings.Trim(u.Path, "/")
			parts := strings.Split(path, "/")
			if len(parts) >= 2 && parts[0] == "channel" {
				return "youtube", parts[1], "https://www.youtube.com/channel/" + parts[1]
			}
			if len(parts) >= 1 && parts[0] != "" {
				return "youtube", parts[0], "https://www.youtube.com/" + parts[0]
			}
		}
		return "youtube", raw, "https://www.youtube.com/" + raw

	case "tiktok":
		username := strings.TrimPrefix(raw, "@")
		if u, err := url.Parse(raw); err == nil && u.Host != "" {
			path := strings.Trim(u.Path, "/")
			parts := strings.Split(path, "/")
			if len(parts) > 0 && parts[0] != "" {
				username = strings.TrimPrefix(parts[0], "@")
			}
		}
		re := regexp.MustCompile(`[^a-zA-Z0-9._]`)
		username = re.ReplaceAllString(username, "")
		return "tiktok", username, "https://www.tiktok.com/@" + username

	default:
		identifier := raw
		pageURL := fmt.Sprintf("https://www.facebook.com/%s", identifier)
		if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
			pageURL = raw
			if u, err := url.Parse(raw); err == nil {
				if idParam := u.Query().Get("id"); idParam != "" {
					identifier = idParam
				} else {
					path := strings.Trim(u.Path, "/")
					parts := strings.Split(path, "/")
					if len(parts) > 0 && parts[0] != "" {
						identifier = parts[0]
					}
				}
			}
		}
		re := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
		identifier = re.ReplaceAllString(identifier, "")
		return "facebook", identifier, pageURL
	}
}

// resolveYouTubeChannel tự động lấy Channel ID (UC...) và tên kênh từ @handle hoặc URL YouTube
func (h *CompetitorHandler) resolveYouTubeChannel(identifier, pageURL string) (string, string) {
	if strings.HasPrefix(identifier, "UC") && len(identifier) >= 20 {
		return identifier, identifier
	}

	candidates := []string{pageURL}
	base := strings.TrimSuffix(pageURL, "/")
	if !strings.HasSuffix(base, "/videos") {
		candidates = append(candidates, base+"/videos")
	}

	channelID := identifier
	channelName := strings.TrimPrefix(identifier, "@")

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`"channelId"\s*:\s*"(UC[a-zA-Z0-9_-]{20,})"`),
		regexp.MustCompile(`"externalId"\s*:\s*"(UC[a-zA-Z0-9_-]{20,})"`),
		regexp.MustCompile(`"browseId"\s*:\s*"(UC[a-zA-Z0-9_-]{20,})"`),
		regexp.MustCompile(`youtube\.com/channel/(UC[a-zA-Z0-9_-]{20,})`),
	}
	reTitle := regexp.MustCompile(`<meta property="og:title" content="([^"]+)"`)

	for _, targetURL := range candidates {
		req, err := http.NewRequest(http.MethodGet, targetURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
		req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,en;q=0.8")
		req.Header.Set("Cookie", "CONSENT=YES+cb.20210328-17-p0.en+FX+999; SOCS=CAISNQgQEitib3FfaWRlbnRpdHkabc")

		resp, err := h.HTTPClient.Do(req)
		if err != nil {
			continue
		}
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 2500000))
		resp.Body.Close()
		pageHTML := string(bodyBytes)

		if m := reTitle.FindStringSubmatch(pageHTML); len(m) > 1 && strings.TrimSpace(m[1]) != "" {
			channelName = html.UnescapeString(strings.TrimSpace(m[1]))
		}

		for _, re := range patterns {
			if m := re.FindStringSubmatch(pageHTML); len(m) > 1 {
				channelID = m[1]
				break
			}
		}

		if strings.HasPrefix(channelID, "UC") {
			break
		}
	}

	return channelID, channelName
}

// ListCompetitors trả về danh sách kênh đối thủ (FB, YouTube, TikTok) & các bài/video đã thu thập
func (h *CompetitorHandler) ListCompetitors(c *gin.Context) {
	var competitors []models.CompetitorPage
	_ = h.DB.Order("created_at DESC").Find(&competitors).Error

	var posts []models.CompetitorPost
	query := h.DB.Order("published_at DESC, id DESC").Limit(120)
	if compID := strings.TrimSpace(c.Query("competitor_page_id")); compID != "" && compID != "all" {
		query = query.Where("competitor_page_id = ?", compID)
	}
	if plat := strings.TrimSpace(c.Query("platform")); plat != "" && plat != "all" {
		query = query.Where("platform = ?", plat)
	}
	_ = query.Find(&posts).Error

	c.JSON(http.StatusOK, gin.H{
		"competitors": competitors,
		"posts":       posts,
	})
}

type addCompetitorReq struct {
	Platform  string `json:"platform"`
	PageInput string `json:"page_input" binding:"required"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	Notes     string `json:"notes"`
}

// AddCompetitor thêm kênh đối thủ (Facebook, YouTube hoặc TikTok)
func (h *CompetitorHandler) AddCompetitor(c *gin.Context) {
	var req addCompetitorReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng nhập Link hoặc ID kênh đối thủ"})
		return
	}

	platform, identifier, pageURL := detectPlatformAndID(req.Platform, req.PageInput)
	if identifier == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Không nhận diện được link hoặc username kênh đối thủ"})
		return
	}

	resolvedID := identifier
	resolvedName := strings.TrimSpace(req.Name)
	avatarURL := ""

	switch platform {
	case "youtube":
		ytID, ytName := h.resolveYouTubeChannel(identifier, pageURL)
		resolvedID = ytID
		if resolvedName == "" {
			resolvedName = ytName
		}
		if strings.HasPrefix(ytID, "UC") {
			pageURL = "https://www.youtube.com/channel/" + ytID
		}

	case "tiktok":
		if resolvedName == "" {
			resolvedName = "@" + identifier
		}

	case "facebook":
		if token := h.getAnyConnectedPageToken(); token != "" {
			apiURL := fmt.Sprintf(
				"https://graph.facebook.com/v21.0/%s?fields=id,name,category,link,picture.type(large)&access_token=%s",
				url.PathEscape(identifier),
				url.QueryEscape(token),
			)
			if resp, err := h.HTTPClient.Get(apiURL); err == nil {
				defer resp.Body.Close()
				body, _ := io.ReadAll(resp.Body)
				var meta struct {
					ID       string `json:"id"`
					Name     string `json:"name"`
					Category string `json:"category"`
					Link     string `json:"link"`
					Picture  struct {
						Data struct {
							URL string `json:"url"`
						} `json:"data"`
					} `json:"picture"`
				}
				if json.Unmarshal(body, &meta) == nil && meta.ID != "" {
					resolvedID = meta.ID
					if meta.Name != "" {
						resolvedName = meta.Name
					}
					if meta.Link != "" {
						pageURL = meta.Link
					}
					if req.Category == "" && meta.Category != "" {
						req.Category = meta.Category
					}
					avatarURL = meta.Picture.Data.URL
				}
			}
		}
	}

	if resolvedName == "" {
		resolvedName = identifier
	}

	uniqueKey := fmt.Sprintf("%s:%s", platform, resolvedID)
	comp := models.CompetitorPage{
		Platform:  platform,
		PageID:    uniqueKey,
		Name:      resolvedName,
		PageURL:   pageURL,
		AvatarURL: avatarURL,
		Category:  strings.TrimSpace(req.Category),
		Notes:     strings.TrimSpace(req.Notes),
	}

	if err := h.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "page_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"platform", "name", "page_url", "avatar_url", "category", "notes", "updated_at"}),
	}).Create(&comp).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể lưu kênh đối thủ"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    fmt.Sprintf("Đã thêm kênh %s (%s)!", resolvedName, strings.ToUpper(platform)),
		"competitor": comp,
	})
}

// SyncCompetitorPosts quét nội dung mới nhất theo từng nền tảng (YouTube RSS / TikTok Public / Facebook Graph)
func (h *CompetitorHandler) SyncCompetitorPosts(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID kênh đối thủ không hợp lệ"})
		return
	}

	var comp models.CompetitorPage
	if err := h.DB.First(&comp, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy kênh đối thủ"})
		return
	}

	rawID := comp.PageID
	if idx := strings.Index(rawID, ":"); idx != -1 {
		rawID = rawID[idx+1:]
	}

	switch comp.Platform {
	case "youtube":
		h.syncYouTubeFeed(c, &comp, rawID)
	case "tiktok":
		h.syncTikTokProfile(c, &comp, rawID)
	default:
		h.syncFacebookPage(c, &comp, rawID)
	}
}

// syncYouTubeFeed quét 15 video mới nhất của kênh YouTube qua RSS Feed công khai (100% không cần API key)
func (h *CompetitorHandler) syncYouTubeFeed(c *gin.Context, comp *models.CompetitorPage, channelID string) {
	if !strings.HasPrefix(channelID, "UC") {
		resolvedID, _ := h.resolveYouTubeChannel(channelID, comp.PageURL)
		channelID = resolvedID
	}

	feedURL := fmt.Sprintf("https://www.youtube.com/feeds/videos.xml?channel_id=%s", url.QueryEscape(channelID))
	resp, err := h.HTTPClient.Get(feedURL)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Không kết nối được YouTube RSS Feed"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Không đọc được RSS kênh YouTube này. Hãy thử dán link dạng https://www.youtube.com/channel/UC... của kênh.",
		})
		return
	}

	xmlBytes, _ := io.ReadAll(resp.Body)

	type ytEntry struct {
		VideoID   string `xml:"videoId"`
		Title     string `xml:"title"`
		Published string `xml:"published"`
		Link      struct {
			Href string `xml:"href,attr"`
		} `xml:"link"`
		Group struct {
			Description string `xml:"description"`
			Thumbnail   struct {
				URL string `xml:"url,attr"`
			} `xml:"thumbnail"`
			Community struct {
				StarRating struct {
					Count int `xml:"count,attr"`
				} `xml:"starRating"`
				Statistics struct {
					Views int `xml:"views,attr"`
				} `xml:"statistics"`
			} `xml:"community"`
		} `xml:"group"`
	}

	var feed struct {
		Title   string    `xml:"title"`
		Entries []ytEntry `xml:"entry"`
	}

	if err := xml.Unmarshal(xmlBytes, &feed); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi phân tích dữ liệu RSS YouTube"})
		return
	}

	if feed.Title != "" && comp.Name == channelID {
		comp.Name = feed.Title
	}

	savedCount := 0
	for _, entry := range feed.Entries {
		pubTime := time.Now()
		if t, err := time.Parse(time.RFC3339, entry.Published); err == nil {
			pubTime = t
		}

		fullText := strings.TrimSpace(entry.Title)
		desc := strings.TrimSpace(entry.Group.Description)
		if desc != "" {
			if len(desc) > 1200 {
				desc = desc[:1200] + "..."
			}
			fullText = fmt.Sprintf("🎬 %s\n\n%s", fullText, desc)
		}

		post := models.CompetitorPost{
			Platform:         "youtube",
			CompetitorPageID: comp.PageID,
			CompetitorName:   comp.Name,
			FBPostID:         "yt_" + entry.VideoID,
			Content:          fullText,
			PermalinkURL:     entry.Link.Href,
			FullPicture:      entry.Group.Thumbnail.URL,
			ViewsCount:       entry.Group.Community.Statistics.Views,
			LikesCount:       entry.Group.Community.StarRating.Count,
			PublishedAt:      pubTime,
		}

		if err := h.DB.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "fb_post_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"content", "full_picture", "views_count", "likes_count", "updated_at"}),
		}).Create(&post).Error; err == nil {
			savedCount++
		}
	}

	comp.LastSynced = time.Now()
	_ = h.DB.Save(comp).Error

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Đã quét %d video mới nhất từ kênh YouTube %s!", savedCount, comp.Name),
	})
}

// syncTikTokProfile quét video công khai từ kênh TikTok qua nhiều nguồn dự phòng (HTML SSR + oEmbed)
func (h *CompetitorHandler) syncTikTokProfile(c *gin.Context, comp *models.CompetitorPage, username string) {
	profileURL := fmt.Sprintf("https://www.tiktok.com/@%s", url.PathEscape(username))
	req, err := http.NewRequest(http.MethodGet, profileURL, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Link TikTok không hợp lệ"})
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Referer", "https://www.tiktok.com/")

	savedCount := 0
	if resp, err := h.HTTPClient.Do(req); err == nil {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 2500000))
		pageHTML := string(bodyBytes)

		reScript := regexp.MustCompile(`<script id="__UNIVERSAL_DATA_FOR_REHYDRATION__"[^>]*>(.*?)</script>`)
		if matches := reScript.FindStringSubmatch(pageHTML); len(matches) > 1 {
			var root map[string]interface{}
			if json.Unmarshal([]byte(matches[1]), &root) == nil {
				if defaultScope, ok := root["__DEFAULT_SCOPE__"].(map[string]interface{}); ok {
					if userDetail, ok := defaultScope["webapp.user-detail"].(map[string]interface{}); ok {
						if userInfo, ok := userDetail["userInfo"].(map[string]interface{}); ok {
							if userObj, ok := userInfo["user"].(map[string]interface{}); ok {
								if nick, ok := userObj["nickname"].(string); ok && nick != "" {
									comp.Name = nick
								}
								if ava, ok := userObj["avatarLarger"].(string); ok && ava != "" {
									comp.AvatarURL = ava
								}
							}
						}
						if items, ok := userDetail["itemList"].([]interface{}); ok {
							savedCount += h.saveTikTokItems(comp, username, items)
						}
					}
				}
			}
		}

		if savedCount == 0 {
			reVideoLink := regexp.MustCompile(fmt.Sprintf(`https?://(?:www\.|vm\.)?tiktok\.com/@%s/video/(\d+)`, regexp.QuoteMeta(username)))
			vidMatches := reVideoLink.FindAllStringSubmatch(pageHTML, 15)
			seen := map[string]bool{}
			for _, m := range vidMatches {
				if len(m) < 2 || seen[m[1]] {
					continue
				}
				seen[m[1]] = true
				vidID := m[1]
				vidURL := fmt.Sprintf("https://www.tiktok.com/@%s/video/%s", username, vidID)

				oembedURL := "https://www.tiktok.com/oembed?url=" + url.QueryEscape(vidURL)
				if oResp, oErr := h.HTTPClient.Get(oembedURL); oErr == nil {
					oBody, _ := io.ReadAll(oResp.Body)
					oResp.Body.Close()
					var oData struct {
						Title        string `json:"title"`
						AuthorName   string `json:"author_name"`
						ThumbnailURL string `json:"thumbnail_url"`
					}
					if json.Unmarshal(oBody, &oData) == nil && strings.TrimSpace(oData.Title) != "" {
						if oData.AuthorName != "" {
							comp.Name = oData.AuthorName
						}
						post := models.CompetitorPost{
							Platform:         "tiktok",
							CompetitorPageID: comp.PageID,
							CompetitorName:   comp.Name,
							FBPostID:         "tt_" + vidID,
							Content:          strings.TrimSpace(oData.Title),
							PermalinkURL:     vidURL,
							FullPicture:      oData.ThumbnailURL,
							PublishedAt:      time.Now(),
						}
						if err := h.DB.Clauses(clause.OnConflict{
							Columns:   []clause.Column{{Name: "fb_post_id"}},
							DoUpdates: clause.AssignmentColumns([]string{"content", "full_picture", "updated_at"}),
						}).Create(&post).Error; err == nil {
							savedCount++
						}
					}
				}
			}
		}
	}

	comp.LastSynced = time.Now()
	_ = h.DB.Save(comp).Error

	if savedCount == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("Kênh %s đang ẩn danh sách video với bot TikTok. Hãy bấm '+ Dán bài / link mẫu' và dán từng link video TikTok để tự kéo caption về ngay!", comp.Name),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Đã quét %d video TikTok mới nhất từ kênh %s!", savedCount, comp.Name),
	})
}

func (h *CompetitorHandler) saveTikTokItems(comp *models.CompetitorPage, username string, items []interface{}) int {
	count := 0
	for _, rawItem := range items {
		item, ok := rawItem.(map[string]interface{})
		if !ok {
			continue
		}
		vidID, _ := item["id"].(string)
		desc, _ := item["desc"].(string)
		if vidID == "" || strings.TrimSpace(desc) == "" {
			continue
		}

		var views, likes, comments, shares int
		if stats, ok := item["stats"].(map[string]interface{}); ok {
			if v, ok := stats["playCount"].(float64); ok {
				views = int(v)
			}
			if v, ok := stats["diggCount"].(float64); ok {
				likes = int(v)
			}
			if v, ok := stats["commentCount"].(float64); ok {
				comments = int(v)
			}
			if v, ok := stats["shareCount"].(float64); ok {
				shares = int(v)
			}
		}

		pubTime := time.Now()
		if ct, ok := item["createTime"].(float64); ok && ct > 0 {
			pubTime = time.Unix(int64(ct), 0)
		}

		cover := ""
		if videoObj, ok := item["video"].(map[string]interface{}); ok {
			cover, _ = videoObj["cover"].(string)
		}

		post := models.CompetitorPost{
			Platform:         "tiktok",
			CompetitorPageID: comp.PageID,
			CompetitorName:   comp.Name,
			FBPostID:         "tt_" + vidID,
			Content:          strings.TrimSpace(desc),
			PermalinkURL:     fmt.Sprintf("https://www.tiktok.com/@%s/video/%s", username, vidID),
			FullPicture:      cover,
			ViewsCount:       views,
			LikesCount:       likes,
			CommentsCount:    comments,
			SharesCount:      shares,
			PublishedAt:      pubTime,
		}

		if err := h.DB.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "fb_post_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"content", "views_count", "likes_count", "comments_count", "shares_count", "updated_at"}),
		}).Create(&post).Error; err == nil {
			count++
		}
	}
	return count
}

func (h *CompetitorHandler) syncFacebookPage(c *gin.Context, comp *models.CompetitorPage, pageID string) {
	token := h.getAnyConnectedPageToken()
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cần kết nối ít nhất 1 Fanpage của bạn trước để lấy token"})
		return
	}

	apiURL := fmt.Sprintf(
		"https://graph.facebook.com/v21.0/%s/posts?fields=id,message,created_time,permalink_url,full_picture,shares,likes.summary(true),comments.summary(true)&limit=15&access_token=%s",
		url.PathEscape(pageID),
		url.QueryEscape(token),
	)

	resp, err := h.HTTPClient.Get(apiURL)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Không kết nối được Facebook Graph API"})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var fbData struct {
		Data []struct {
			ID           string `json:"id"`
			Message      string `json:"message"`
			CreatedTime  string `json:"created_time"`
			PermalinkURL string `json:"permalink_url"`
			FullPicture  string `json:"full_picture"`
			Shares       struct {
				Count int `json:"count"`
			} `json:"shares"`
			Likes struct {
				Summary struct {
					TotalCount int `json:"total_count"`
				} `json:"summary"`
			} `json:"likes"`
			Comments struct {
				Summary struct {
					TotalCount int `json:"total_count"`
				} `json:"summary"`
			} `json:"comments"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	_ = json.Unmarshal(body, &fbData)
	if fbData.Error != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Meta hạn chế quét tự động Page người khác (cần Page Public Content Access). Bạn có thể bấm '+ Dán bài/video thủ công' hoặc bám kênh YouTube / TikTok để quét tự động!",
		})
		return
	}

	savedCount := 0
	for _, item := range fbData.Data {
		if strings.TrimSpace(item.Message) == "" && item.FullPicture == "" {
			continue
		}
		pubTime := time.Now()
		if t, err := time.Parse("2006-01-02T15:04:05-0700", item.CreatedTime); err == nil {
			pubTime = t
		}

		post := models.CompetitorPost{
			Platform:         "facebook",
			CompetitorPageID: comp.PageID,
			CompetitorName:   comp.Name,
			FBPostID:         item.ID,
			Content:          strings.TrimSpace(item.Message),
			PermalinkURL:     item.PermalinkURL,
			FullPicture:      item.FullPicture,
			LikesCount:       item.Likes.Summary.TotalCount,
			CommentsCount:    item.Comments.Summary.TotalCount,
			SharesCount:      item.Shares.Count,
			PublishedAt:      pubTime,
		}

		if err := h.DB.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "fb_post_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"content", "full_picture", "likes_count", "comments_count", "shares_count", "updated_at"}),
		}).Create(&post).Error; err == nil {
			savedCount++
		}
	}

	comp.LastSynced = time.Now()
	_ = h.DB.Save(comp).Error

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Đã cập nhật %d bài viết mới từ Fanpage %s!", savedCount, comp.Name),
	})
}

type manualCompetitorPostReq struct {
	CompetitorPageID string `json:"competitor_page_id" binding:"required"`
	Content          string `json:"content"`
	PermalinkURL     string `json:"permalink_url"`
	ViewsCount       int    `json:"views_count"`
	LikesCount       int    `json:"likes_count"`
	CommentsCount    int    `json:"comments_count"`
	SharesCount      int    `json:"shares_count"`
}

// CreateManualCompetitorPost lưu bài viết/video thủ công (tự động lấy caption nếu dán Link TikTok/YouTube)
func (h *CompetitorHandler) CreateManualCompetitorPost(c *gin.Context) {
	var req manualCompetitorPostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng chọn kênh đối thủ"})
		return
	}

	var comp models.CompetitorPage
	if err := h.DB.Where("page_id = ?", req.CompetitorPageID).First(&comp).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy kênh đối thủ"})
		return
	}

	content := strings.TrimSpace(req.Content)
	link := strings.TrimSpace(req.PermalinkURL)
	thumb := ""

	if content == "" && link != "" {
		oembedEndpoint := ""
		if strings.Contains(link, "tiktok.com") {
			oembedEndpoint = "https://www.tiktok.com/oembed?url=" + url.QueryEscape(link)
		} else if strings.Contains(link, "youtube.com") || strings.Contains(link, "youtu.be") {
			oembedEndpoint = "https://www.youtube.com/oembed?format=json&url=" + url.QueryEscape(link)
		}
		if oembedEndpoint != "" {
			if resp, err := h.HTTPClient.Get(oembedEndpoint); err == nil {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				var oData struct {
					Title        string `json:"title"`
					ThumbnailURL string `json:"thumbnail_url"`
				}
				if json.Unmarshal(body, &oData) == nil {
					content = strings.TrimSpace(oData.Title)
					thumb = oData.ThumbnailURL
				}
			}
		}
	}

	if content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng nhập nội dung hoặc dán link video TikTok/YouTube hợp lệ"})
		return
	}

	post := models.CompetitorPost{
		Platform:         comp.Platform,
		CompetitorPageID: comp.PageID,
		CompetitorName:   comp.Name,
		FBPostID:         fmt.Sprintf("manual_%s_%d", comp.Platform, time.Now().UnixNano()),
		Content:          content,
		PermalinkURL:     link,
		FullPicture:      thumb,
		ViewsCount:       req.ViewsCount,
		LikesCount:       req.LikesCount,
		CommentsCount:    req.CommentsCount,
		SharesCount:      req.SharesCount,
		PublishedAt:      time.Now(),
	}

	if err := h.DB.Create(&post).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể lưu nội dung đối thủ"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Đã lưu nội dung đối thủ! Bấm 'AI Phân tích & Viết lại' để khai thác.",
		"post":    post,
	})
}

type analyzePostReq struct {
	ProductContext string `json:"product_context"`
}

// AnalyzeAndRemixPost dùng Gemini AI phân tích nội dung từ FB/TikTok/YouTube & viết lại bài bán hàng Facebook
func (h *CompetitorHandler) AnalyzeAndRemixPost(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID không hợp lệ"})
		return
	}

	var post models.CompetitorPost
	if err := h.DB.First(&post, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy nội dung đối thủ"})
		return
	}

	var req analyzePostReq
	_ = c.ShouldBindJSON(&req)

	apiKey := ""
	if h.Cfg != nil {
		apiKey = strings.TrimSpace(h.Cfg.GeminiAPIKey)
	}
	if apiKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chưa cấu hình GEMINI_API_KEY trong file .env của Backend"})
		return
	}

	productHint := strings.TrimSpace(req.ProductContext)
	if productHint == "" {
		productHint = "Sản phẩm chủ lực của shop (giữ đúng ngành hàng như nội dung gốc nhưng tối ưu lợi thế cạnh tranh, ưu đãi và lời kêu gọi nhắn tin đặt hàng)."
	}

	prompt := fmt.Sprintf(`Bạn là chuyên gia Content Marketing đa nền tảng (Facebook, TikTok, YouTube) thực chiến tại Việt Nam.
Hãy phân tích nội dung dưới đây của kênh đối thủ [%s - nền tảng %s] và viết lại thành một bài đăng bán hàng Facebook cuốn hút cho shop của tôi.

Nội dung gốc của đối thủ:
"""
%s
"""
Chỉ số: %d Lượt xem, %d Like/Tim, %d Bình luận, %d Chia sẻ.
Thông tin sản phẩm/ưu đãi của shop tôi: %s

Hãy trả về kết quả JSON chính xác với 2 trường:
{
  "analysis": "Phân tích ngắn gọn 3 gạch đầu dòng: 1) Hook mở đầu giữ chân người xem là gì, 2) Điểm đắt giá khiến khách tương tác/mua hàng, 3) Gợi ý cách triển khai lại trên Fanpage Facebook của shop.",
  "remixed_post": "Nội dung bài đăng Facebook hoàn chỉnh viết mới 100%% (không đạo văn), mở đầu giật tít tự nhiên, nêu rõ lợi ích và CTA kêu gọi khách inbox ngay."
}`, post.CompetitorName, strings.ToUpper(post.Platform), post.Content, post.ViewsCount, post.LikesCount, post.CommentsCount, post.SharesCount, productHint)

	geminiPayload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"responseMimeType": "application/json",
		},
	}

	payloadBytes, _ := json.Marshal(geminiPayload)
	geminiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s", url.QueryEscape(apiKey))

	resp, err := h.HTTPClient.Post(geminiURL, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Không kết nối được Gemini AI"})
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var geminiRes struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	_ = json.Unmarshal(respBody, &geminiRes)
	if geminiRes.Error != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": geminiRes.Error.Message})
		return
	}
	if len(geminiRes.Candidates) == 0 || len(geminiRes.Candidates[0].Content.Parts) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gemini AI không trả về dữ liệu phân tích"})
		return
	}

	rawJSON := strings.TrimSpace(geminiRes.Candidates[0].Content.Parts[0].Text)
	var parsed struct {
		Analysis    string `json:"analysis"`
		RemixedPost string `json:"remixed_post"`
	}
	if err := json.Unmarshal([]byte(rawJSON), &parsed); err != nil {
		parsed.Analysis = "Đã phân tích cấu trúc nội dung đối thủ."
		parsed.RemixedPost = rawJSON
	}

	post.AIAnalysis = parsed.Analysis
	post.AIRemixedPost = parsed.RemixedPost
	_ = h.DB.Save(&post).Error

	c.JSON(http.StatusOK, gin.H{
		"message": "AI đã phân tích và viết lại bài mới thành công!",
		"post":    post,
	})
}

// SearchByKeyword tìm kiếm video/nội dung công khai theo từ khóa (hỗ trợ YouTube & TikTok)
// SearchByKeyword tìm kiếm video/nội dung theo từ khóa trên YouTube, TikTok (TikWM + HTML/oEmbed) và Facebook (DB/Page đã lưu)
func (h *CompetitorHandler) SearchByKeyword(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("q"))
	platform := strings.ToLower(strings.TrimSpace(c.Query("platform")))
	if keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng nhập từ khóa cần tìm"})
		return
	}
	if platform == "" {
		platform = "all"
	}

	savedCount := 0
	kwLower := strings.ToLower(keyword)

	// 1. Tìm trên YouTube
	if platform == "all" || platform == "youtube" {
		searchURL := "https://www.youtube.com/results?search_query=" + url.QueryEscape(keyword)
		req, err := http.NewRequest(http.MethodGet, searchURL, nil)
		if err == nil {
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36")
			req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,en;q=0.8")
			req.Header.Set("Cookie", "CONSENT=YES+cb.20210328-17-p0.en+FX+999; SOCS=CAISNQgQEitib3FfaWRlbnRpdHkabc")

			if resp, err := h.HTTPClient.Do(req); err == nil {
				bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 2500000))
				resp.Body.Close()
				pageHTML := string(bodyBytes)

				reVideoID := regexp.MustCompile(`"videoId"\s*:\s*"([a-zA-Z0-9_-]{11})"`)
				matches := reVideoID.FindAllStringSubmatch(pageHTML, 30)
				seen := map[string]bool{}
				ytAdded := 0

				for _, m := range matches {
					if len(m) < 2 || seen[m[1]] {
						continue
					}
					seen[m[1]] = true
					vidID := m[1]
					vidURL := "https://www.youtube.com/watch?v=" + vidID

					oembedURL := "https://www.youtube.com/oembed?format=json&url=" + url.QueryEscape(vidURL)
					if oResp, oErr := h.HTTPClient.Get(oembedURL); oErr == nil {
						oBody, _ := io.ReadAll(oResp.Body)
						oResp.Body.Close()
						var oData struct {
							Title        string `json:"title"`
							AuthorName   string `json:"author_name"`
							ThumbnailURL string `json:"thumbnail_url"`
						}
						if json.Unmarshal(oBody, &oData) == nil && strings.TrimSpace(oData.Title) != "" {
							author := oData.AuthorName
							if author == "" {
								author = "YouTube Search"
							}
							post := models.CompetitorPost{
								Platform:         "youtube",
								CompetitorPageID: "kw_yt_" + keyword,
								CompetitorName:   fmt.Sprintf("%s (Từ khóa: %s)", author, keyword),
								FBPostID:         "yt_kw_" + vidID,
								Content:          fmt.Sprintf("🎬 %s", strings.TrimSpace(oData.Title)),
								PermalinkURL:     vidURL,
								FullPicture:      oData.ThumbnailURL,
								PublishedAt:      time.Now(),
							}
							if err := h.DB.Clauses(clause.OnConflict{
								Columns:   []clause.Column{{Name: "fb_post_id"}},
								DoUpdates: clause.AssignmentColumns([]string{"content", "full_picture", "updated_at"}),
							}).Create(&post).Error; err == nil {
								savedCount++
								ytAdded++
								if ytAdded >= 6 {
									break
								}
							}
						}
					}
				}
			}
		}
	}

	// 2. Tìm trên TikTok qua API tìm kiếm công khai (TikWM) + fallback HTML/oEmbed
	if platform == "all" || platform == "tiktok" {
		ttAdded := 0
		apiURL := fmt.Sprintf("https://www.tikwm.com/api/feed/search?keywords=%s&count=10&cursor=0", url.QueryEscape(keyword))
		if resp, err := h.HTTPClient.Get(apiURL); err == nil {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			var ttRes struct {
				Code int `json:"code"`
				Data struct {
					Videos []struct {
						VideoID      string `json:"video_id"`
						Title        string `json:"title"`
						Cover        string `json:"cover"`
						PlayCount    int    `json:"play_count"`
						DiggCount    int    `json:"digg_count"`
						CommentCount int    `json:"comment_count"`
						ShareCount   int    `json:"share_count"`
						CreateTime   int64  `json:"create_time"`
						Author       struct {
							UniqueID string `json:"unique_id"`
							Nickname string `json:"nickname"`
						} `json:"author"`
					} `json:"videos"`
				} `json:"data"`
			}

			if json.Unmarshal(bodyBytes, &ttRes) == nil && ttRes.Code == 0 {
				for _, v := range ttRes.Data.Videos {
					if v.VideoID == "" || strings.TrimSpace(v.Title) == "" {
						continue
					}
					authorName := v.Author.Nickname
					if authorName == "" {
						authorName = "@" + v.Author.UniqueID
					}
					pubTime := time.Now()
					if v.CreateTime > 0 {
						pubTime = time.Unix(v.CreateTime, 0)
					}
					vidURL := ""
					if v.Author.UniqueID != "" {
						vidURL = fmt.Sprintf("https://www.tiktok.com/@%s/video/%s", v.Author.UniqueID, v.VideoID)
					}

					post := models.CompetitorPost{
						Platform:         "tiktok",
						CompetitorPageID: "kw_tt_" + keyword,
						CompetitorName:   fmt.Sprintf("%s (Từ khóa: %s)", authorName, keyword),
						FBPostID:         "tt_kw_" + v.VideoID,
						Content:          strings.TrimSpace(v.Title),
						PermalinkURL:     vidURL,
						FullPicture:      v.Cover,
						ViewsCount:       v.PlayCount,
						LikesCount:       v.DiggCount,
						CommentsCount:    v.CommentCount,
						SharesCount:      v.ShareCount,
						PublishedAt:      pubTime,
					}

					if err := h.DB.Clauses(clause.OnConflict{
						Columns:   []clause.Column{{Name: "fb_post_id"}},
						DoUpdates: clause.AssignmentColumns([]string{"content", "full_picture", "views_count", "likes_count", "comments_count", "shares_count", "updated_at"}),
					}).Create(&post).Error; err == nil {
						savedCount++
						ttAdded++
						if ttAdded >= 6 {
							break
						}
					}
				}
			}
		}

		// Fallback TikTok search HTML nếu API trên không trả về video
		if ttAdded == 0 {
			searchURL := "https://www.tiktok.com/search?q=" + url.QueryEscape(keyword)
			req, err := http.NewRequest(http.MethodGet, searchURL, nil)
			if err == nil {
				req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36")
				req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,en;q=0.8")
				if resp, err := h.HTTPClient.Do(req); err == nil {
					bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 2500000))
					resp.Body.Close()
					pageHTML := string(bodyBytes)

					reVid := regexp.MustCompile(`https?://(?:www\.|vm\.)?tiktok\.com/@([a-zA-Z0-9._]+)/video/(\d+)`)
					vMatches := reVid.FindAllStringSubmatch(pageHTML, 20)
					seen := map[string]bool{}
					for _, m := range vMatches {
						if len(m) < 3 || seen[m[2]] {
							continue
						}
						seen[m[2]] = true
						user := m[1]
						vidID := m[2]
						vidURL := fmt.Sprintf("https://www.tiktok.com/@%s/video/%s", user, vidID)

						oembedURL := "https://www.tiktok.com/oembed?url=" + url.QueryEscape(vidURL)
						if oResp, oErr := h.HTTPClient.Get(oembedURL); oErr == nil {
							oBody, _ := io.ReadAll(oResp.Body)
							oResp.Body.Close()
							var oData struct {
								Title        string `json:"title"`
								AuthorName   string `json:"author_name"`
								ThumbnailURL string `json:"thumbnail_url"`
							}
							if json.Unmarshal(oBody, &oData) == nil && strings.TrimSpace(oData.Title) != "" {
								author := oData.AuthorName
								if author == "" {
									author = "@" + user
								}
								post := models.CompetitorPost{
									Platform:         "tiktok",
									CompetitorPageID: "kw_tt_" + keyword,
									CompetitorName:   fmt.Sprintf("%s (Từ khóa: %s)", author, keyword),
									FBPostID:         "tt_kw_" + vidID,
									Content:          strings.TrimSpace(oData.Title),
									PermalinkURL:     vidURL,
									FullPicture:      oData.ThumbnailURL,
									PublishedAt:      time.Now(),
								}
								if err := h.DB.Clauses(clause.OnConflict{
									Columns:   []clause.Column{{Name: "fb_post_id"}},
									DoUpdates: clause.AssignmentColumns([]string{"content", "full_picture", "updated_at"}),
								}).Create(&post).Error; err == nil {
									savedCount++
									ttAdded++
									if ttAdded >= 6 {
										break
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// 3. Tìm trên Facebook (quét các bài viết đã thu thập hoặc bài từ các Fanpage đã kết nối khớp từ khóa)
	fbMatchedCount := 0
	if platform == "all" || platform == "facebook" {
		var existingFB []models.CompetitorPost
		_ = h.DB.Where("platform = ? AND LOWER(content) LIKE ?", "facebook", "%"+kwLower+"%").Find(&existingFB).Error
		fbMatchedCount = len(existingFB)

		if token := h.getAnyConnectedPageToken(); token != "" {
			var myPages []models.FacebookPage
			_ = h.DB.Where("is_connected = ?", true).Limit(10).Find(&myPages).Error
			for _, p := range myPages {
				apiURL := fmt.Sprintf(
					"https://graph.facebook.com/v21.0/%s/posts?fields=id,message,created_time,permalink_url,full_picture,shares,likes.summary(true),comments.summary(true)&limit=10&access_token=%s",
					url.PathEscape(p.PageID),
					url.QueryEscape(token),
				)
				resp, err := h.HTTPClient.Get(apiURL)
				if err != nil {
					continue
				}
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()

				var fbData struct {
					Data []struct {
						ID           string `json:"id"`
						Message      string `json:"message"`
						CreatedTime  string `json:"created_time"`
						PermalinkURL string `json:"permalink_url"`
						FullPicture  string `json:"full_picture"`
						Shares       struct {
							Count int `json:"count"`
						} `json:"shares"`
						Likes struct {
							Summary struct {
								TotalCount int `json:"total_count"`
							} `json:"summary"`
						} `json:"likes"`
						Comments struct {
							Summary struct {
								TotalCount int `json:"total_count"`
							} `json:"summary"`
						} `json:"comments"`
					} `json:"data"`
				}
				if json.Unmarshal(body, &fbData) == nil {
					for _, item := range fbData.Data {
						if !strings.Contains(strings.ToLower(item.Message), kwLower) {
							continue
						}
						pubTime := time.Now()
						if t, err := time.Parse("2006-01-02T15:04:05-0700", item.CreatedTime); err == nil {
							pubTime = t
						}
						post := models.CompetitorPost{
							Platform:         "facebook",
							CompetitorPageID: "fb_page_" + p.PageID,
							CompetitorName:   p.Name,
							FBPostID:         item.ID,
							Content:          strings.TrimSpace(item.Message),
							PermalinkURL:     item.PermalinkURL,
							FullPicture:      item.FullPicture,
							LikesCount:       item.Likes.Summary.TotalCount,
							CommentsCount:    item.Comments.Summary.TotalCount,
							SharesCount:      item.Shares.Count,
							PublishedAt:      pubTime,
						}
						if err := h.DB.Clauses(clause.OnConflict{
							Columns:   []clause.Column{{Name: "fb_post_id"}},
							DoUpdates: clause.AssignmentColumns([]string{"content", "full_picture", "likes_count", "comments_count", "shares_count", "updated_at"}),
						}).Create(&post).Error; err == nil {
							savedCount++
							fbMatchedCount++
						}
					}
				}
			}
		}
	}

	if savedCount == 0 && fbMatchedCount == 0 {
		msg := fmt.Sprintf("Không tìm thấy video/bài mới cho từ khóa '%s'.", keyword)
		if platform == "facebook" {
			msg = fmt.Sprintf("Facebook Graph API không hỗ trợ tìm kiếm công khai theo từ khóa cho mọi Page (cần Page Public Content Access). Hãy chọn tab TikTok hoặc YouTube để tìm tự động, hoặc bấm '+ Dán bài / link mẫu'.", keyword)
		}
		c.JSON(http.StatusOK, gin.H{
			"message": msg,
			"count":   0,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Đã tìm thấy & cập nhật kết quả cho từ khóa '%s' (%d mục mới)!", keyword, savedCount),
		"count":   savedCount,
	})
}

// DeleteCompetitor xóa kênh đối thủ cùng các bài viết đã lưu
func (h *CompetitorHandler) DeleteCompetitor(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID không hợp lệ"})
		return
	}
	var comp models.CompetitorPage
	if err := h.DB.First(&comp, id).Error; err == nil {
		_ = h.DB.Where("competitor_page_id = ?", comp.PageID).Delete(&models.CompetitorPost{}).Error
		_ = h.DB.Delete(&comp).Error
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã xóa kênh đối thủ"})
}
