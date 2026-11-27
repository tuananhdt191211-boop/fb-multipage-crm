package handlers

// Khám phá nội dung: dán link kênh / video, @tên kênh, tên kênh hoặc từ khoá
// để xem ngay video / bài viết trên TikTok, YouTube, Facebook (không tự lưu).
// Người dùng chọn mục nào thì gọi /competitors/explore/save để lưu vào kho đối thủ.
//
// Nguồn dữ liệu (miễn phí):
//   - TikTok: API công khai tikwm.com (giới hạn ~1 yêu cầu/giây, có thể đổi qua TIKWM_API_BASE)
//   - YouTube: YouTube Data API v3 nếu có YOUTUBE_API_KEY; không có thì dùng RSS / trang tìm kiếm
//   - Facebook: Graph API bằng token của các Page đã gộp (chỉ đọc được Page của chính bạn)

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"fb-multipage-crm/internal/models"
	"fb-multipage-crm/internal/security"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

const exploreUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"

type exploreItem struct {
	ID           string `json:"id"`
	Platform     string `json:"platform"`
	Kind         string `json:"kind"` // video | photo | post
	Title        string `json:"title"`
	Thumbnail    string `json:"thumbnail"`
	URL          string `json:"url"`
	AuthorName   string `json:"author_name"`
	AuthorHandle string `json:"author_handle"`
	AuthorAvatar string `json:"author_avatar"`
	Views        int64  `json:"views"`
	Likes        int64  `json:"likes"`
	Comments     int64  `json:"comments"`
	Shares       int64  `json:"shares"`
	Duration     int64  `json:"duration"`
	PublishedAt  int64  `json:"published_at"`
}

type exploreChannel struct {
	Platform  string `json:"platform"`
	ID        string `json:"id"`
	Handle    string `json:"handle"`
	Name      string `json:"name"`
	Avatar    string `json:"avatar"`
	URL       string `json:"url"`
	Followers int64  `json:"followers"`
	Videos    int64  `json:"videos"`
}

type exploreResult struct {
	Mode       string           `json:"mode"` // channel | keyword | channels | single
	Platform   string           `json:"platform"`
	Query      string           `json:"query"`
	Channel    *exploreChannel  `json:"channel,omitempty"`
	Channels   []exploreChannel `json:"channels"`
	Items      []exploreItem    `json:"items"`
	NextCursor string           `json:"next_cursor"`
	HasMore    bool             `json:"has_more"`
	Notice     string           `json:"notice"`
}

// userError: lỗi có câu thông báo thân thiện để hiện thẳng cho người dùng
type userError struct {
	status int
	msg    string
}

func (e *userError) Error() string { return e.msg }

func badInput(msg string) error { return &userError{status: http.StatusBadRequest, msg: msg} }
func upstream(msg string) error { return &userError{status: http.StatusBadGateway, msg: msg} }

// flexInt đọc được số dạng 123, "123" hoặc 1.2e3
type flexInt int64

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		*f = flexInt(n)
		return nil
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		*f = flexInt(int64(v))
		return nil
	}
	*f = 0
	return nil
}

// looseString đọc chuỗi, hoặc object kiểu {"url_list": ["..."]}
type looseString string

func (l *looseString) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		*l = looseString(s)
		return nil
	}
	var obj struct {
		URLList []string `json:"url_list"`
		URL     string   `json:"url"`
	}
	if json.Unmarshal(b, &obj) == nil {
		if len(obj.URLList) > 0 {
			*l = looseString(obj.URLList[0])
		} else {
			*l = looseString(obj.URL)
		}
	}
	return nil
}

// rawCursor đổi cursor dạng số hoặc chuỗi thành chuỗi
func rawCursor(r json.RawMessage) string {
	s := strings.Trim(strings.TrimSpace(string(r)), `"`)
	if s == "null" || s == "0" {
		return ""
	}
	return s
}

// ---------------------------------------------------------------------------
// Nhận diện đầu vào
// ---------------------------------------------------------------------------

type exploreTarget struct {
	Platform string // tiktok | youtube | facebook
	Kind     string // channel | keyword | channels | single
	Value    string
}

var reHasScheme = regexp.MustCompile(`(?i)^[a-z][a-z0-9+.-]*://`)

func looksLikeURL(s string) bool {
	l := strings.ToLower(s)
	if reHasScheme.MatchString(l) || strings.HasPrefix(l, "www.") {
		return true
	}
	for _, host := range []string{"tiktok.com/", "youtube.com/", "youtu.be/", "facebook.com/", "fb.com/", "fb.watch/"} {
		if strings.Contains(l, host) {
			return true
		}
	}
	return false
}

func parseExploreInput(q, platformHint, modeHint string) (exploreTarget, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return exploreTarget{}, badInput("Hãy nhập link, @tên kênh hoặc từ khoá cần tìm.")
	}
	if looksLikeURL(q) {
		return parseExploreURL(q)
	}

	platform := strings.ToLower(strings.TrimSpace(platformHint))
	if platform != "youtube" && platform != "facebook" {
		platform = "tiktok"
	}
	mode := strings.ToLower(strings.TrimSpace(modeHint))

	if strings.HasPrefix(q, "@") && !strings.ContainsAny(q, " \t") {
		handle := strings.TrimPrefix(q, "@")
		if handle == "" {
			return exploreTarget{}, badInput("Tên kênh sau dấu @ đang trống.")
		}
		switch platform {
		case "youtube":
			return exploreTarget{"youtube", "channel", "@" + handle}, nil
		case "facebook":
			return exploreTarget{"facebook", "channel", handle}, nil
		default:
			return exploreTarget{"tiktok", "channel", handle}, nil
		}
	}
	if platform == "youtube" && strings.HasPrefix(q, "UC") && len(q) == 24 && !strings.ContainsAny(q, " \t") {
		return exploreTarget{"youtube", "channel", q}, nil
	}

	if mode == "channel" {
		if platform == "facebook" {
			if strings.ContainsAny(q, " \t") {
				return exploreTarget{}, badInput("Facebook không cho tìm Page theo tên. Hãy dán link Page (VD: facebook.com/tenpage).")
			}
			return exploreTarget{"facebook", "channel", q}, nil
		}
		return exploreTarget{platform, "channels", q}, nil
	}
	return exploreTarget{platform, "keyword", q}, nil
}

func parseExploreURL(raw string) (exploreTarget, error) {
	if !reHasScheme.MatchString(raw) {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return exploreTarget{}, badInput("Link không hợp lệ.")
	}
	host := strings.ToLower(u.Hostname())
	for _, p := range []string{"www.", "m.", "mobile.", "web.", "music."} {
		host = strings.TrimPrefix(host, p)
	}
	path := strings.Trim(u.Path, "/")
	parts := strings.Split(path, "/")
	first := ""
	if len(parts) > 0 {
		first = parts[0]
	}

	switch {
	case host == "tiktok.com" || strings.HasSuffix(host, ".tiktok.com"):
		if host != "tiktok.com" || strings.Contains(path, "/video/") || strings.Contains(path, "/photo/") || first == "t" {
			return exploreTarget{"tiktok", "single", u.String()}, nil
		}
		if strings.HasPrefix(first, "@") && len(first) > 1 {
			return exploreTarget{"tiktok", "channel", strings.TrimPrefix(first, "@")}, nil
		}
		return exploreTarget{}, badInput("Không nhận ra link TikTok. Hãy dán link kênh (tiktok.com/@ten) hoặc link video.")

	case host == "youtu.be":
		if first != "" {
			return exploreTarget{"youtube", "single", first}, nil
		}
	case host == "youtube.com" || host == "youtube-nocookie.com":
		if v := u.Query().Get("v"); v != "" {
			return exploreTarget{"youtube", "single", v}, nil
		}
		if (first == "shorts" || first == "live" || first == "embed") && len(parts) > 1 {
			return exploreTarget{"youtube", "single", parts[1]}, nil
		}
		if strings.HasPrefix(first, "@") && len(first) > 1 {
			return exploreTarget{"youtube", "channel", first}, nil
		}
		if first == "channel" && len(parts) > 1 {
			return exploreTarget{"youtube", "channel", parts[1]}, nil
		}
		if first == "user" && len(parts) > 1 {
			return exploreTarget{"youtube", "channel", "user:" + parts[1]}, nil
		}
		if first == "c" && len(parts) > 1 {
			return exploreTarget{"youtube", "channel", "c:" + parts[1]}, nil
		}
		if first != "" && first != "results" && first != "watch" {
			return exploreTarget{"youtube", "channel", "c:" + first}, nil
		}
		if sq := u.Query().Get("search_query"); sq != "" {
			return exploreTarget{"youtube", "keyword", sq}, nil
		}
		return exploreTarget{}, badInput("Không nhận ra link YouTube. Hãy dán link kênh (youtube.com/@ten) hoặc link video.")

	case host == "facebook.com" || host == "fb.com" || host == "fb.watch":
		if host == "fb.watch" {
			return exploreTarget{"facebook", "single", u.String()}, nil
		}
		if id := u.Query().Get("id"); first == "profile.php" && id != "" {
			return exploreTarget{"facebook", "channel", id}, nil
		}
		lp := "/" + strings.ToLower(path) + "/"
		for _, seg := range []string{"/posts/", "/videos/", "/reel/", "/photos/", "/photo/", "/permalink/", "/story.php/", "/watch/", "/share/", "/groups/"} {
			if strings.Contains(lp, seg) {
				return exploreTarget{"facebook", "single", u.String()}, nil
			}
		}
		if first == "people" && len(parts) > 2 {
			return exploreTarget{"facebook", "channel", parts[2]}, nil
		}
		if (first == "pg" || first == "pages") && len(parts) > 1 {
			last := parts[len(parts)-1]
			if regexp.MustCompile(`^\d{6,}$`).MatchString(last) {
				return exploreTarget{"facebook", "channel", last}, nil
			}
			return exploreTarget{"facebook", "channel", parts[1]}, nil
		}
		if first == "p" && len(parts) > 1 {
			if m := regexp.MustCompile(`(\d{6,})$`).FindStringSubmatch(parts[1]); len(m) > 1 {
				return exploreTarget{"facebook", "channel", m[1]}, nil
			}
		}
		if first != "" {
			return exploreTarget{"facebook", "channel", first}, nil
		}
		return exploreTarget{}, badInput("Không nhận ra link Facebook. Hãy dán link Page (facebook.com/tenpage).")
	}
	return exploreTarget{}, badInput("Mới hỗ trợ link TikTok, YouTube và Facebook.")
}

// ---------------------------------------------------------------------------
// Handler
// ---------------------------------------------------------------------------

// ExploreContent GET /competitors/explore?q=&platform=&mode=&cursor=
func (h *CompetitorHandler) ExploreContent(c *gin.Context) {
	target, err := parseExploreInput(c.Query("q"), c.Query("platform"), c.Query("mode"))
	if err != nil {
		h.exploreFail(c, err)
		return
	}
	cursor := strings.TrimSpace(c.Query("cursor"))

	res := &exploreResult{
		Mode:     target.Kind,
		Platform: target.Platform,
		Query:    target.Value,
		Channels: []exploreChannel{},
		Items:    []exploreItem{},
	}

	switch target.Platform {
	case "tiktok":
		err = h.exploreTikTok(target, cursor, res)
	case "youtube":
		err = h.exploreYouTube(target, cursor, res)
	case "facebook":
		err = h.exploreFacebook(target, cursor, res)
	}
	if err != nil {
		h.exploreFail(c, err)
		return
	}
	if res.Items == nil {
		res.Items = []exploreItem{}
	}
	if res.Channels == nil {
		res.Channels = []exploreChannel{}
	}
	c.JSON(http.StatusOK, res)
}

func (h *CompetitorHandler) exploreFail(c *gin.Context, err error) {
	var ue *userError
	if errors.As(err, &ue) {
		c.JSON(ue.status, gin.H{"error": ue.msg})
		return
	}
	c.JSON(http.StatusBadGateway, gin.H{"error": "Quét thất bại: " + err.Error()})
}

func (h *CompetitorHandler) getJSON(rawURL string, headers map[string]string) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", exploreUA)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,en;q=0.8")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{Transport: h.HTTPClient.Transport, Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return body, resp.StatusCode, err
}

// ---------------------------------------------------------------------------
// TikTok (tikwm)
// ---------------------------------------------------------------------------

var (
	tikwmMu   sync.Mutex
	tikwmLast time.Time
)

func tikwmBase() string {
	if b := strings.TrimRight(strings.TrimSpace(os.Getenv("TIKWM_API_BASE")), "/"); b != "" {
		return b
	}
	return "https://www.tikwm.com"
}

func tikwmAbs(u string) string {
	u = strings.TrimSpace(u)
	if strings.HasPrefix(u, "/") && !strings.HasPrefix(u, "//") {
		return tikwmBase() + u
	}
	return u
}

// tikwmGet gọi tikwm, tự giãn cách ~1.1s giữa các lần gọi và thử lại khi bị giới hạn tần suất
func (h *CompetitorHandler) tikwmGet(path string, params url.Values) (json.RawMessage, error) {
	full := tikwmBase() + path + "?" + params.Encode()
	var lastMsg string
	for attempt := 0; attempt < 3; attempt++ {
		tikwmMu.Lock()
		if wait := 1100*time.Millisecond - time.Since(tikwmLast); wait > 0 {
			time.Sleep(wait)
		}
		tikwmLast = time.Now()
		tikwmMu.Unlock()

		body, status, err := h.getJSON(full, map[string]string{"Referer": "https://www.tikwm.com/"})
		if err != nil {
			return nil, upstream("Không kết nối được máy chủ dữ liệu TikTok (tikwm): " + err.Error())
		}
		var env struct {
			Code int             `json:"code"`
			Msg  string          `json:"msg"`
			Data json.RawMessage `json:"data"`
		}
		if jerr := json.Unmarshal(body, &env); jerr != nil {
			if status == http.StatusTooManyRequests && attempt < 2 {
				time.Sleep(1500 * time.Millisecond)
				continue
			}
			return nil, upstream(fmt.Sprintf("Máy chủ dữ liệu TikTok (tikwm) từ chối yêu cầu từ server (HTTP %d). Thử lại sau ít phút, hoặc cấu hình OUTBOUND_PROXY_URL.", status))
		}
		if env.Code == 0 {
			return env.Data, nil
		}
		lastMsg = env.Msg
		if strings.Contains(strings.ToLower(env.Msg), "limit") && attempt < 2 {
			time.Sleep(1500 * time.Millisecond)
			continue
		}
		break
	}
	if lastMsg == "" {
		lastMsg = "không rõ nguyên nhân"
	}
	return nil, upstream("TikTok (tikwm) báo lỗi: " + lastMsg)
}

type tikwmAuthor struct {
	ID       string      `json:"id"`
	UniqueID string      `json:"unique_id"`
	Nickname string      `json:"nickname"`
	Avatar   looseString `json:"avatar"`
}

type tikwmVideo struct {
	VideoID      string      `json:"video_id"`
	ID           string      `json:"id"`
	AwemeID      string      `json:"aweme_id"`
	Title        string      `json:"title"`
	Cover        looseString `json:"cover"`
	OriginCover  looseString `json:"origin_cover"`
	AIDynamic    looseString `json:"ai_dynamic_cover"`
	Duration     flexInt     `json:"duration"`
	PlayCount    flexInt     `json:"play_count"`
	DiggCount    flexInt     `json:"digg_count"`
	CommentCount flexInt     `json:"comment_count"`
	ShareCount   flexInt     `json:"share_count"`
	CreateTime   flexInt     `json:"create_time"`
	Images       []string    `json:"images"`
	Author       tikwmAuthor `json:"author"`
}

func (v tikwmVideo) toItem(fallbackHandle, fallbackName, fallbackAvatar string) exploreItem {
	id := v.VideoID
	if id == "" {
		id = v.ID
	}
	if id == "" {
		id = v.AwemeID
	}
	handle := v.Author.UniqueID
	if handle == "" {
		handle = fallbackHandle
	}
	name := v.Author.Nickname
	if name == "" {
		name = fallbackName
	}
	if name == "" && handle != "" {
		name = "@" + handle
	}
	avatar := tikwmAbs(string(v.Author.Avatar))
	if avatar == "" {
		avatar = fallbackAvatar
	}
	thumb := tikwmAbs(string(v.Cover))
	if thumb == "" {
		thumb = tikwmAbs(string(v.OriginCover))
	}
	kind := "video"
	seg := "video"
	if len(v.Images) > 0 {
		kind = "photo"
		seg = "photo"
		if thumb == "" {
			thumb = tikwmAbs(v.Images[0])
		}
	}
	link := ""
	if handle != "" && id != "" {
		link = fmt.Sprintf("https://www.tiktok.com/@%s/%s/%s", handle, seg, id)
	} else if id != "" {
		link = "https://www.tiktok.com/video/" + id
	}
	return exploreItem{
		ID:           id,
		Platform:     "tiktok",
		Kind:         kind,
		Title:        strings.TrimSpace(v.Title),
		Thumbnail:    thumb,
		URL:          link,
		AuthorName:   name,
		AuthorHandle: handle,
		AuthorAvatar: avatar,
		Views:        int64(v.PlayCount),
		Likes:        int64(v.DiggCount),
		Comments:     int64(v.CommentCount),
		Shares:       int64(v.ShareCount),
		Duration:     int64(v.Duration),
		PublishedAt:  int64(v.CreateTime),
	}
}

func (h *CompetitorHandler) exploreTikTok(t exploreTarget, cursor string, res *exploreResult) error {
	switch t.Kind {
	case "single":
		data, err := h.tikwmGet("/api/", url.Values{"url": {t.Value}, "hd": {"0"}})
		if err != nil {
			return err
		}
		var v tikwmVideo
		if err := json.Unmarshal(data, &v); err != nil {
			return upstream("Không đọc được dữ liệu video TikTok.")
		}
		item := v.toItem("", "", "")
		if item.ID == "" {
			return upstream("Không tìm thấy video TikTok này (có thể đã bị xoá hoặc để riêng tư).")
		}
		res.Items = append(res.Items, item)
		if item.AuthorHandle != "" {
			res.Channel = &exploreChannel{
				Platform: "tiktok", Handle: item.AuthorHandle, Name: item.AuthorName,
				Avatar: item.AuthorAvatar, URL: "https://www.tiktok.com/@" + item.AuthorHandle,
			}
		}
		return nil

	case "channel":
		handle := strings.TrimPrefix(t.Value, "@")
		handle = regexp.MustCompile(`[^a-zA-Z0-9._]`).ReplaceAllString(handle, "")
		if handle == "" {
			return badInput("Tên kênh TikTok không hợp lệ.")
		}
		res.Query = handle
		ch := &exploreChannel{Platform: "tiktok", Handle: handle, Name: "@" + handle, URL: "https://www.tiktok.com/@" + handle}
		if cursor == "" {
			if data, err := h.tikwmGet("/api/user/info", url.Values{"unique_id": {handle}}); err == nil {
				var info struct {
					User struct {
						ID           string      `json:"id"`
						UniqueID     string      `json:"uniqueId"`
						Nickname     string      `json:"nickname"`
						AvatarMedium looseString `json:"avatarMedium"`
						AvatarThumb  looseString `json:"avatarThumb"`
					} `json:"user"`
					Stats struct {
						FollowerCount flexInt `json:"followerCount"`
						VideoCount    flexInt `json:"videoCount"`
					} `json:"stats"`
				}
				if json.Unmarshal(data, &info) == nil {
					ch.ID = info.User.ID
					if info.User.Nickname != "" {
						ch.Name = info.User.Nickname
					}
					ch.Avatar = tikwmAbs(string(info.User.AvatarMedium))
					if ch.Avatar == "" {
						ch.Avatar = tikwmAbs(string(info.User.AvatarThumb))
					}
					ch.Followers = int64(info.Stats.FollowerCount)
					ch.Videos = int64(info.Stats.VideoCount)
				}
			} else if ue, ok := err.(*userError); ok && strings.Contains(strings.ToLower(ue.msg), "user") {
				return badInput("Không tìm thấy kênh TikTok @" + handle + ".")
			}
		}
		res.Channel = ch

		params := url.Values{"unique_id": {handle}, "count": {"30"}, "cursor": {cursorOr(cursor, "0")}}
		data, err := h.tikwmGet("/api/user/posts", params)
		if err != nil {
			return err
		}
		var page struct {
			Videos  []tikwmVideo    `json:"videos"`
			Cursor  json.RawMessage `json:"cursor"`
			HasMore bool            `json:"hasMore"`
		}
		if err := json.Unmarshal(data, &page); err != nil {
			return upstream("Không đọc được danh sách video của kênh TikTok.")
		}
		for _, v := range page.Videos {
			if it := v.toItem(handle, ch.Name, ch.Avatar); it.ID != "" {
				res.Items = append(res.Items, it)
			}
		}
		res.NextCursor = rawCursor(page.Cursor)
		res.HasMore = page.HasMore && res.NextCursor != "" && res.NextCursor != cursor
		if len(res.Items) == 0 && cursor == "" {
			res.Notice = "Kênh này chưa có video công khai, hoặc đang để riêng tư."
		}
		return nil

	case "channels":
		params := url.Values{"keywords": {t.Value}, "count": {"20"}, "cursor": {cursorOr(cursor, "0")}}
		data, err := h.tikwmGet("/api/user/search", params)
		if err != nil {
			return err
		}
		var page struct {
			UserList []struct {
				UserInfo struct {
					UID           string      `json:"uid"`
					UniqueID      string      `json:"unique_id"`
					Nickname      string      `json:"nickname"`
					AvatarThumb   looseString `json:"avatar_thumb"`
					AvatarMedium  looseString `json:"avatar_medium"`
					FollowerCount flexInt     `json:"follower_count"`
					AwemeCount    flexInt     `json:"aweme_count"`
				} `json:"user_info"`
			} `json:"user_list"`
			Cursor  json.RawMessage `json:"cursor"`
			HasMore bool            `json:"hasMore"`
		}
		if err := json.Unmarshal(data, &page); err != nil {
			return upstream("Không đọc được kết quả tìm kênh TikTok.")
		}
		for _, u := range page.UserList {
			ui := u.UserInfo
			if ui.UniqueID == "" {
				continue
			}
			avatar := tikwmAbs(string(ui.AvatarMedium))
			if avatar == "" {
				avatar = tikwmAbs(string(ui.AvatarThumb))
			}
			name := ui.Nickname
			if name == "" {
				name = "@" + ui.UniqueID
			}
			res.Channels = append(res.Channels, exploreChannel{
				Platform: "tiktok", ID: ui.UID, Handle: ui.UniqueID, Name: name, Avatar: avatar,
				URL: "https://www.tiktok.com/@" + ui.UniqueID, Followers: int64(ui.FollowerCount), Videos: int64(ui.AwemeCount),
			})
		}
		res.NextCursor = rawCursor(page.Cursor)
		res.HasMore = page.HasMore && res.NextCursor != "" && res.NextCursor != cursor
		if len(res.Channels) == 0 && cursor == "" {
			res.Notice = "Không tìm thấy kênh TikTok nào khớp tên này."
		}
		return nil

	default: // keyword
		params := url.Values{"keywords": {t.Value}, "count": {"30"}, "cursor": {cursorOr(cursor, "0")}, "HD": {"0"}}
		data, err := h.tikwmGet("/api/feed/search", params)
		if err != nil {
			return err
		}
		var page struct {
			Videos  []tikwmVideo    `json:"videos"`
			Cursor  json.RawMessage `json:"cursor"`
			HasMore bool            `json:"hasMore"`
		}
		if err := json.Unmarshal(data, &page); err != nil {
			return upstream("Không đọc được kết quả tìm kiếm TikTok.")
		}
		for _, v := range page.Videos {
			if it := v.toItem("", "", ""); it.ID != "" {
				res.Items = append(res.Items, it)
			}
		}
		res.NextCursor = rawCursor(page.Cursor)
		res.HasMore = page.HasMore && res.NextCursor != "" && res.NextCursor != cursor
		if len(res.Items) == 0 && cursor == "" {
			res.Notice = "TikTok không trả về video nào cho từ khoá này. Thử từ khoá ngắn hơn hoặc không dấu."
		}
		return nil
	}
}

func cursorOr(c, def string) string {
	if c == "" {
		return def
	}
	return c
}

// ---------------------------------------------------------------------------
// YouTube
// ---------------------------------------------------------------------------

func youtubeKey() string { return strings.TrimSpace(os.Getenv("YOUTUBE_API_KEY")) }

const ytNoKeyNotice = "Đang chạy chế độ không có YouTube API key nên kết quả bị giới hạn. Thêm biến YOUTUBE_API_KEY (miễn phí từ Google Cloud) trên Render để xem đầy đủ."

func (h *CompetitorHandler) ytAPI(endpoint string, params url.Values, out interface{}) error {
	params.Set("key", youtubeKey())
	body, status, err := h.getJSON("https://www.googleapis.com/youtube/v3/"+endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return upstream("Không kết nối được YouTube API: " + err.Error())
	}
	if status != http.StatusOK {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &e)
		msg := e.Error.Message
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", status)
		}
		if strings.Contains(strings.ToLower(msg), "quota") {
			msg = "đã hết lượt miễn phí trong ngày (quota), thử lại vào ngày mai"
		}
		return upstream("YouTube API báo lỗi: " + html.UnescapeString(msg))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return upstream("Không đọc được dữ liệu YouTube.")
	}
	return nil
}

type ytThumbs struct {
	Default struct {
		URL string `json:"url"`
	} `json:"default"`
	Medium struct {
		URL string `json:"url"`
	} `json:"medium"`
	High struct {
		URL string `json:"url"`
	} `json:"high"`
	Standard struct {
		URL string `json:"url"`
	} `json:"standard"`
}

func (t ytThumbs) best() string {
	for _, u := range []string{t.High.URL, t.Standard.URL, t.Medium.URL, t.Default.URL} {
		if u != "" {
			return u
		}
	}
	return ""
}

type ytChannelItem struct {
	ID      string `json:"id"`
	Snippet struct {
		Title      string   `json:"title"`
		CustomURL  string   `json:"customUrl"`
		Thumbnails ytThumbs `json:"thumbnails"`
	} `json:"snippet"`
	Statistics struct {
		SubscriberCount flexInt `json:"subscriberCount"`
		VideoCount      flexInt `json:"videoCount"`
	} `json:"statistics"`
}

func (c ytChannelItem) toChannel() exploreChannel {
	handle := strings.TrimPrefix(c.Snippet.CustomURL, "@")
	link := "https://www.youtube.com/channel/" + c.ID
	if handle != "" {
		link = "https://www.youtube.com/@" + handle
	}
	return exploreChannel{
		Platform: "youtube", ID: c.ID, Handle: handle, Name: c.Snippet.Title,
		Avatar: c.Snippet.Thumbnails.best(), URL: link,
		Followers: int64(c.Statistics.SubscriberCount), Videos: int64(c.Statistics.VideoCount),
	}
}

var reISODur = regexp.MustCompile(`^P(?:(\d+)D)?T?(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$`)

func parseISODuration(s string) int64 {
	m := reISODur.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	n := func(i int) int64 { v, _ := strconv.ParseInt(m[i], 10, 64); return v }
	return n(1)*86400 + n(2)*3600 + n(3)*60 + n(4)
}

// ytVideosByIDs lấy chi tiết (lượt xem, thời lượng...) theo đúng thứ tự ID
func (h *CompetitorHandler) ytVideosByIDs(ids []string) ([]exploreItem, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var resp struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title        string   `json:"title"`
				PublishedAt  string   `json:"publishedAt"`
				ChannelID    string   `json:"channelId"`
				ChannelTitle string   `json:"channelTitle"`
				Thumbnails   ytThumbs `json:"thumbnails"`
			} `json:"snippet"`
			Statistics struct {
				ViewCount    flexInt `json:"viewCount"`
				LikeCount    flexInt `json:"likeCount"`
				CommentCount flexInt `json:"commentCount"`
			} `json:"statistics"`
			ContentDetails struct {
				Duration string `json:"duration"`
			} `json:"contentDetails"`
		} `json:"items"`
	}
	err := h.ytAPI("videos", url.Values{
		"part": {"snippet,statistics,contentDetails"},
		"id":   {strings.Join(ids, ",")},
	}, &resp)
	if err != nil {
		return nil, err
	}
	byID := map[string]exploreItem{}
	for _, v := range resp.Items {
		pub := int64(0)
		if t, err := time.Parse(time.RFC3339, v.Snippet.PublishedAt); err == nil {
			pub = t.Unix()
		}
		byID[v.ID] = exploreItem{
			ID: v.ID, Platform: "youtube", Kind: "video",
			Title: v.Snippet.Title, Thumbnail: v.Snippet.Thumbnails.best(),
			URL:          "https://www.youtube.com/watch?v=" + v.ID,
			AuthorName:   v.Snippet.ChannelTitle,
			AuthorHandle: v.Snippet.ChannelID,
			Views:        int64(v.Statistics.ViewCount),
			Likes:        int64(v.Statistics.LikeCount),
			Comments:     int64(v.Statistics.CommentCount),
			Duration:     parseISODuration(v.ContentDetails.Duration),
			PublishedAt:  pub,
		}
	}
	out := make([]exploreItem, 0, len(ids))
	for _, id := range ids {
		if it, ok := byID[id]; ok {
			out = append(out, it)
		}
	}
	return out, nil
}

func (h *CompetitorHandler) ytResolveChannel(value string) (*ytChannelItem, error) {
	params := url.Values{"part": {"snippet,statistics"}}
	switch {
	case strings.HasPrefix(value, "UC") && len(value) == 24:
		params.Set("id", value)
	case strings.HasPrefix(value, "@"):
		params.Set("forHandle", value)
	case strings.HasPrefix(value, "user:"):
		params.Set("forUsername", strings.TrimPrefix(value, "user:"))
	default:
		name := strings.TrimPrefix(value, "c:")
		// thử như @handle trước (rẻ), không được thì tìm theo tên
		var byHandle struct {
			Items []ytChannelItem `json:"items"`
		}
		if err := h.ytAPI("channels", url.Values{"part": {"snippet,statistics"}, "forHandle": {"@" + name}}, &byHandle); err == nil && len(byHandle.Items) > 0 {
			return &byHandle.Items[0], nil
		}
		var search struct {
			Items []struct {
				ID struct {
					ChannelID string `json:"channelId"`
				} `json:"id"`
			} `json:"items"`
		}
		if err := h.ytAPI("search", url.Values{"part": {"snippet"}, "type": {"channel"}, "q": {name}, "maxResults": {"1"}}, &search); err != nil {
			return nil, err
		}
		if len(search.Items) == 0 {
			return nil, badInput("Không tìm thấy kênh YouTube này.")
		}
		params.Set("id", search.Items[0].ID.ChannelID)
	}
	var resp struct {
		Items []ytChannelItem `json:"items"`
	}
	if err := h.ytAPI("channels", params, &resp); err != nil {
		return nil, err
	}
	if len(resp.Items) == 0 {
		return nil, badInput("Không tìm thấy kênh YouTube này.")
	}
	return &resp.Items[0], nil
}

func (h *CompetitorHandler) exploreYouTube(t exploreTarget, cursor string, res *exploreResult) error {
	if youtubeKey() == "" {
		return h.exploreYouTubeNoKey(t, res)
	}
	switch t.Kind {
	case "single":
		items, err := h.ytVideosByIDs([]string{t.Value})
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return badInput("Không tìm thấy video YouTube này.")
		}
		res.Items = items
		return nil

	case "channel":
		ch, err := h.ytResolveChannel(t.Value)
		if err != nil {
			return err
		}
		info := ch.toChannel()
		res.Channel = &info
		uploads := "UU" + strings.TrimPrefix(ch.ID, "UC")
		var pl struct {
			NextPageToken string `json:"nextPageToken"`
			Items         []struct {
				ContentDetails struct {
					VideoID string `json:"videoId"`
				} `json:"contentDetails"`
			} `json:"items"`
		}
		params := url.Values{"part": {"contentDetails"}, "playlistId": {uploads}, "maxResults": {"24"}}
		if cursor != "" {
			params.Set("pageToken", cursor)
		}
		if err := h.ytAPI("playlistItems", params, &pl); err != nil {
			return err
		}
		ids := make([]string, 0, len(pl.Items))
		for _, it := range pl.Items {
			if it.ContentDetails.VideoID != "" {
				ids = append(ids, it.ContentDetails.VideoID)
			}
		}
		items, err := h.ytVideosByIDs(ids)
		if err != nil {
			return err
		}
		for i := range items {
			items[i].AuthorAvatar = info.Avatar
		}
		res.Items = items
		res.NextCursor = pl.NextPageToken
		res.HasMore = pl.NextPageToken != ""
		if len(items) == 0 && cursor == "" {
			res.Notice = "Kênh này chưa có video công khai."
		}
		return nil

	case "channels":
		var search struct {
			NextPageToken string `json:"nextPageToken"`
			Items         []struct {
				ID struct {
					ChannelID string `json:"channelId"`
				} `json:"id"`
			} `json:"items"`
		}
		params := url.Values{"part": {"snippet"}, "type": {"channel"}, "q": {t.Value}, "maxResults": {"12"}, "regionCode": {"VN"}}
		if cursor != "" {
			params.Set("pageToken", cursor)
		}
		if err := h.ytAPI("search", params, &search); err != nil {
			return err
		}
		ids := []string{}
		for _, it := range search.Items {
			if it.ID.ChannelID != "" {
				ids = append(ids, it.ID.ChannelID)
			}
		}
		if len(ids) > 0 {
			var chs struct {
				Items []ytChannelItem `json:"items"`
			}
			if err := h.ytAPI("channels", url.Values{"part": {"snippet,statistics"}, "id": {strings.Join(ids, ",")}}, &chs); err != nil {
				return err
			}
			byID := map[string]ytChannelItem{}
			for _, c := range chs.Items {
				byID[c.ID] = c
			}
			for _, id := range ids {
				if c, ok := byID[id]; ok {
					res.Channels = append(res.Channels, c.toChannel())
				}
			}
		}
		res.NextCursor = search.NextPageToken
		res.HasMore = search.NextPageToken != ""
		if len(res.Channels) == 0 && cursor == "" {
			res.Notice = "Không tìm thấy kênh YouTube nào khớp tên này."
		}
		return nil

	default: // keyword
		var search struct {
			NextPageToken string `json:"nextPageToken"`
			Items         []struct {
				ID struct {
					VideoID string `json:"videoId"`
				} `json:"id"`
			} `json:"items"`
		}
		params := url.Values{
			"part": {"snippet"}, "type": {"video"}, "q": {t.Value}, "maxResults": {"24"},
			"regionCode": {"VN"}, "relevanceLanguage": {"vi"},
		}
		if cursor != "" {
			params.Set("pageToken", cursor)
		}
		if err := h.ytAPI("search", params, &search); err != nil {
			return err
		}
		ids := []string{}
		for _, it := range search.Items {
			if it.ID.VideoID != "" {
				ids = append(ids, it.ID.VideoID)
			}
		}
		items, err := h.ytVideosByIDs(ids)
		if err != nil {
			return err
		}
		res.Items = items
		res.NextCursor = search.NextPageToken
		res.HasMore = search.NextPageToken != ""
		if len(items) == 0 && cursor == "" {
			res.Notice = "YouTube không trả về video nào cho từ khoá này."
		}
		return nil
	}
}

// ytOEmbed lấy tiêu đề + tên kênh của 1 video khi không có API key
func (h *CompetitorHandler) ytOEmbed(videoID string) (exploreItem, bool) {
	link := "https://www.youtube.com/watch?v=" + videoID
	body, status, err := h.getJSON("https://www.youtube.com/oembed?format=json&url="+url.QueryEscape(link), nil)
	if err != nil || status != http.StatusOK {
		return exploreItem{}, false
	}
	var o struct {
		Title      string `json:"title"`
		AuthorName string `json:"author_name"`
		AuthorURL  string `json:"author_url"`
	}
	if json.Unmarshal(body, &o) != nil || strings.TrimSpace(o.Title) == "" {
		return exploreItem{}, false
	}
	return exploreItem{
		ID: videoID, Platform: "youtube", Kind: "video", Title: o.Title,
		Thumbnail:  "https://i.ytimg.com/vi/" + videoID + "/hqdefault.jpg",
		URL:        link,
		AuthorName: o.AuthorName, AuthorHandle: strings.TrimPrefix(strings.TrimPrefix(o.AuthorURL, "https://www.youtube.com/"), "@"),
	}, true
}

func (h *CompetitorHandler) exploreYouTubeNoKey(t exploreTarget, res *exploreResult) error {
	res.Notice = ytNoKeyNotice
	switch t.Kind {
	case "single":
		it, ok := h.ytOEmbed(t.Value)
		if !ok {
			return badInput("Không tìm thấy video YouTube này.")
		}
		res.Items = append(res.Items, it)
		return nil

	case "channels":
		return badInput("Tìm kênh YouTube theo tên cần YOUTUBE_API_KEY. Bạn có thể dán link kênh (youtube.com/@ten) hoặc gõ @ten.")

	case "channel":
		ident := t.Value
		pageURL := "https://www.youtube.com/" + ident
		switch {
		case strings.HasPrefix(ident, "UC"):
			pageURL = "https://www.youtube.com/channel/" + ident
		case strings.HasPrefix(ident, "user:"):
			pageURL = "https://www.youtube.com/user/" + strings.TrimPrefix(ident, "user:")
		case strings.HasPrefix(ident, "c:"):
			pageURL = "https://www.youtube.com/c/" + strings.TrimPrefix(ident, "c:")
		}
		channelID, channelName := h.resolveYouTubeChannel(ident, pageURL)
		if !strings.HasPrefix(channelID, "UC") {
			return upstream("Không xác định được kênh YouTube từ máy chủ. Hãy dán link dạng youtube.com/channel/UC..., hoặc thêm YOUTUBE_API_KEY.")
		}
		body, status, err := h.getJSON("https://www.youtube.com/feeds/videos.xml?channel_id="+url.QueryEscape(channelID), nil)
		if err != nil || status != http.StatusOK {
			return upstream("Không đọc được danh sách video của kênh YouTube. Thử lại sau hoặc thêm YOUTUBE_API_KEY.")
		}
		var feed struct {
			Title   string `xml:"title"`
			Entries []struct {
				VideoID   string `xml:"videoId"`
				Title     string `xml:"title"`
				Published string `xml:"published"`
				Group     struct {
					Thumbnail struct {
						URL string `xml:"url,attr"`
					} `xml:"thumbnail"`
					Community struct {
						StarRating struct {
							Count int64 `xml:"count,attr"`
						} `xml:"starRating"`
						Statistics struct {
							Views int64 `xml:"views,attr"`
						} `xml:"statistics"`
					} `xml:"community"`
				} `xml:"group"`
			} `xml:"entry"`
		}
		if err := xml.Unmarshal(body, &feed); err != nil {
			return upstream("Không đọc được danh sách video của kênh YouTube.")
		}
		if strings.TrimSpace(feed.Title) != "" {
			channelName = feed.Title
		}
		res.Channel = &exploreChannel{Platform: "youtube", ID: channelID, Name: channelName, URL: "https://www.youtube.com/channel/" + channelID}
		for _, e := range feed.Entries {
			pub := int64(0)
			if tm, err := time.Parse(time.RFC3339, e.Published); err == nil {
				pub = tm.Unix()
			}
			thumb := e.Group.Thumbnail.URL
			if thumb == "" {
				thumb = "https://i.ytimg.com/vi/" + e.VideoID + "/hqdefault.jpg"
			}
			res.Items = append(res.Items, exploreItem{
				ID: e.VideoID, Platform: "youtube", Kind: "video", Title: e.Title, Thumbnail: thumb,
				URL: "https://www.youtube.com/watch?v=" + e.VideoID, AuthorName: channelName, AuthorHandle: channelID,
				Views: e.Group.Community.Statistics.Views, Likes: e.Group.Community.StarRating.Count, PublishedAt: pub,
			})
		}
		return nil

	default: // keyword: đọc trang tìm kiếm rồi lấy tiêu đề qua oEmbed
		body, _, err := h.getJSON("https://www.youtube.com/results?search_query="+url.QueryEscape(t.Value), map[string]string{
			"Accept": "text/html",
			"Cookie": "CONSENT=YES+cb.20210328-17-p0.en+FX+999; SOCS=CAISNQgQEitib3FfaWRlbnRpdHkabc",
		})
		if err != nil {
			return upstream("Không kết nối được YouTube: " + err.Error())
		}
		matches := regexp.MustCompile(`"videoRenderer":\{"videoId":"([a-zA-Z0-9_-]{11})"`).FindAllStringSubmatch(string(body), -1)
		if len(matches) == 0 {
			matches = regexp.MustCompile(`"videoId":"([a-zA-Z0-9_-]{11})"`).FindAllStringSubmatch(string(body), -1)
		}
		ids := []string{}
		seen := map[string]bool{}
		for _, m := range matches {
			if !seen[m[1]] {
				seen[m[1]] = true
				ids = append(ids, m[1])
			}
			if len(ids) >= 16 {
				break
			}
		}
		if len(ids) == 0 {
			return upstream("YouTube chặn tìm kiếm từ máy chủ. Hãy thêm YOUTUBE_API_KEY trên Render để tìm từ khoá ổn định.")
		}
		out := make([]exploreItem, len(ids))
		ok := make([]bool, len(ids))
		var wg sync.WaitGroup
		for i, id := range ids {
			wg.Add(1)
			go func(i int, id string) {
				defer wg.Done()
				out[i], ok[i] = h.ytOEmbed(id)
			}(i, id)
		}
		wg.Wait()
		for i := range ids {
			if ok[i] {
				res.Items = append(res.Items, out[i])
			}
		}
		return nil
	}
}

// ---------------------------------------------------------------------------
// Facebook
// ---------------------------------------------------------------------------

const fbNoAccess = "Facebook không cho đọc bài của Page không thuộc bạn qua API chính thức (cần quyền Page Public Content Access). Bạn chỉ quét được các Page đã gộp; với Page khác hãy dùng mục 'Dán bài / link mẫu' để lưu tay."

// pageTokenFor trả token của đúng Page đã gộp (nếu có)
func (h *CompetitorHandler) pageTokenFor(pageID string) string {
	if pageID == "" {
		return ""
	}
	var page models.FacebookPage
	if err := h.DB.Where("page_id = ? AND encrypted_page_token <> ''", pageID).First(&page).Error; err != nil {
		return ""
	}
	tok, err := security.DecryptSecret(page.EncryptedPageToken, h.encKey())
	if err != nil {
		return ""
	}
	return tok
}

func (h *CompetitorHandler) fbGet(path string, params url.Values, out interface{}) (int, error) {
	body, status, err := h.getJSON("https://graph.facebook.com/v21.0/"+path+"?"+params.Encode(), nil)
	if err != nil {
		return 0, upstream("Không kết nối được Facebook: " + err.Error())
	}
	var e struct {
		Error struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &e) == nil && e.Error.Message != "" {
		return e.Error.Code, fmt.Errorf("%s", e.Error.Message)
	}
	if status != http.StatusOK {
		return 0, fmt.Errorf("HTTP %d", status)
	}
	return 0, json.Unmarshal(body, out)
}

type fbPost struct {
	ID           string `json:"id"`
	Message      string `json:"message"`
	Story        string `json:"story"`
	CreatedTime  string `json:"created_time"`
	PermalinkURL string `json:"permalink_url"`
	FullPicture  string `json:"full_picture"`
	StatusType   string `json:"status_type"`
	Shares       struct {
		Count flexInt `json:"count"`
	} `json:"shares"`
	Reactions struct {
		Summary struct {
			TotalCount flexInt `json:"total_count"`
		} `json:"summary"`
	} `json:"reactions"`
	Comments struct {
		Summary struct {
			TotalCount flexInt `json:"total_count"`
		} `json:"summary"`
	} `json:"comments"`
}

const fbPostFields = "id,message,story,created_time,permalink_url,full_picture,status_type,shares,reactions.limit(0).summary(true),comments.limit(0).summary(true)"

func (p fbPost) toItem(pageID, pageName, avatar string) exploreItem {
	pub := int64(0)
	if t, err := time.Parse("2006-01-02T15:04:05-0700", p.CreatedTime); err == nil {
		pub = t.Unix()
	}
	title := strings.TrimSpace(p.Message)
	if title == "" {
		title = strings.TrimSpace(p.Story)
	}
	kind := "post"
	if strings.Contains(p.StatusType, "video") {
		kind = "video"
	}
	return exploreItem{
		ID: p.ID, Platform: "facebook", Kind: kind, Title: title, Thumbnail: p.FullPicture,
		URL: p.PermalinkURL, AuthorName: pageName, AuthorHandle: pageID, AuthorAvatar: avatar,
		Likes: int64(p.Reactions.Summary.TotalCount), Comments: int64(p.Comments.Summary.TotalCount),
		Shares: int64(p.Shares.Count), PublishedAt: pub,
	}
}

func (h *CompetitorHandler) exploreFacebook(t exploreTarget, cursor string, res *exploreResult) error {
	var pages []models.FacebookPage
	_ = h.DB.Where("is_connected = ? AND encrypted_page_token <> ''", true).Find(&pages).Error
	if len(pages) == 0 {
		return badInput("Cần gộp ít nhất 1 Fanpage của bạn trước khi quét Facebook.")
	}

	switch t.Kind {
	case "single", "channels":
		return badInput(fbNoAccess)

	case "channel":
		ident := t.Value
		// Tìm trong các Page đã gộp theo ID hoặc tên
		var own *models.FacebookPage
		for i := range pages {
			if pages[i].PageID == ident || strings.EqualFold(pages[i].Name, ident) {
				own = &pages[i]
				break
			}
		}
		token := ""
		if own != nil {
			token = h.pageTokenFor(own.PageID)
		}
		if own == nil {
			// Thử hỏi Facebook ID thật của username, rồi so với Page đã gộp
			anyTok := h.getAnyConnectedPageToken()
			var meta struct {
				ID string `json:"id"`
			}
			if _, err := h.fbGet(url.PathEscape(ident), url.Values{"fields": {"id"}, "access_token": {anyTok}}, &meta); err == nil && meta.ID != "" {
				for i := range pages {
					if pages[i].PageID == meta.ID {
						own = &pages[i]
						token = h.pageTokenFor(own.PageID)
						break
					}
				}
				if own == nil {
					ident = meta.ID
					token = anyTok
				}
			} else {
				return badInput(fbNoAccess)
			}
		}
		if token == "" {
			return badInput(fbNoAccess)
		}

		ch := &exploreChannel{Platform: "facebook", ID: ident, Name: ident, URL: "https://www.facebook.com/" + ident}
		if own != nil {
			ch.ID, ch.Name, ch.Avatar = own.PageID, own.Name, own.AvatarURL
			ch.URL = "https://www.facebook.com/" + own.PageID
		}
		if cursor == "" {
			var meta struct {
				Name     string  `json:"name"`
				Link     string  `json:"link"`
				FanCount flexInt `json:"fan_count"`
				Picture  struct {
					Data struct {
						URL string `json:"url"`
					} `json:"data"`
				} `json:"picture"`
			}
			if _, err := h.fbGet(url.PathEscape(ch.ID), url.Values{"fields": {"name,link,fan_count,picture.type(large)"}, "access_token": {token}}, &meta); err == nil {
				if meta.Name != "" {
					ch.Name = meta.Name
				}
				if meta.Picture.Data.URL != "" {
					ch.Avatar = meta.Picture.Data.URL
				}
				if meta.Link != "" {
					ch.URL = meta.Link
				}
				ch.Followers = int64(meta.FanCount)
			}
		}
		res.Channel = ch

		var feed struct {
			Data   []fbPost `json:"data"`
			Paging struct {
				Cursors struct {
					After string `json:"after"`
				} `json:"cursors"`
				Next string `json:"next"`
			} `json:"paging"`
		}
		params := url.Values{"fields": {fbPostFields}, "limit": {"24"}, "access_token": {token}}
		if cursor != "" {
			params.Set("after", cursor)
		}
		if _, err := h.fbGet(url.PathEscape(ch.ID)+"/posts", params, &feed); err != nil {
			if own == nil {
				return badInput(fbNoAccess)
			}
			return upstream("Facebook báo lỗi khi đọc bài của Page: " + err.Error())
		}
		for _, p := range feed.Data {
			res.Items = append(res.Items, p.toItem(ch.ID, ch.Name, ch.Avatar))
		}
		res.NextCursor = feed.Paging.Cursors.After
		res.HasMore = feed.Paging.Next != "" && res.NextCursor != ""
		if len(res.Items) == 0 && cursor == "" {
			res.Notice = "Page này chưa có bài viết nào."
		}
		return nil

	default: // keyword: tìm trong bài mới của các Page đã gộp
		kw := strings.ToLower(t.Value)
		res.Notice = "Facebook chỉ cho tìm trong các Page bạn đã gộp (50 bài mới nhất mỗi Page)."
		limit := len(pages)
		if limit > 15 {
			limit = 15
		}
		type pageResult struct{ items []exploreItem }
		results := make([]pageResult, limit)
		var wg sync.WaitGroup
		for i := 0; i < limit; i++ {
			wg.Add(1)
			go func(i int, p models.FacebookPage) {
				defer wg.Done()
				tok := h.pageTokenFor(p.PageID)
				if tok == "" {
					return
				}
				var feed struct {
					Data []fbPost `json:"data"`
				}
				if _, err := h.fbGet(url.PathEscape(p.PageID)+"/posts", url.Values{"fields": {fbPostFields}, "limit": {"50"}, "access_token": {tok}}, &feed); err != nil {
					return
				}
				for _, post := range feed.Data {
					if strings.Contains(strings.ToLower(post.Message+" "+post.Story), kw) {
						results[i].items = append(results[i].items, post.toItem(p.PageID, p.Name, p.AvatarURL))
					}
				}
			}(i, pages[i])
		}
		wg.Wait()
		for _, r := range results {
			res.Items = append(res.Items, r.items...)
		}
		sort.SliceStable(res.Items, func(a, b int) bool { return res.Items[a].PublishedAt > res.Items[b].PublishedAt })
		if len(res.Items) == 0 {
			res.Notice = "Không có bài nào trong các Page đã gộp chứa từ khoá này. (Facebook chỉ cho tìm trong Page của bạn.)"
		}
		return nil
	}
}

// ---------------------------------------------------------------------------
// Lưu các mục đã chọn vào kho đối thủ
// ---------------------------------------------------------------------------

type exploreSaveReq struct {
	Items   []exploreItem `json:"items"`
	Keyword string        `json:"keyword"`
}

func httpOnly(raw string) string {
	raw = strings.TrimSpace(raw)
	if u, err := url.Parse(raw); err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
		return raw
	}
	return ""
}

func clampInt(v int64) int {
	const max = int64(^uint32(0) >> 1)
	if v > max {
		return int(max)
	}
	if v < 0 {
		return 0
	}
	return int(v)
}

// SaveExploreItems POST /competitors/explore/save
func (h *CompetitorHandler) SaveExploreItems(c *gin.Context) {
	var req exploreSaveReq
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chưa chọn video / bài nào để lưu."})
		return
	}
	if len(req.Items) > 200 {
		req.Items = req.Items[:200]
	}
	keyword := strings.TrimSpace(req.Keyword)
	saved := 0
	for _, it := range req.Items {
		it.ID = strings.TrimSpace(it.ID)
		if it.ID == "" {
			continue
		}
		var postKey, pageKey string
		switch it.Platform {
		case "tiktok":
			postKey, pageKey = "tt_"+it.ID, "tiktok_"+it.AuthorHandle
		case "youtube":
			postKey, pageKey = "yt_"+it.ID, "youtube_"+it.AuthorHandle
		case "facebook":
			postKey, pageKey = it.ID, "fb_page_"+it.AuthorHandle
		default:
			continue
		}
		name := strings.TrimSpace(it.AuthorName)
		if name == "" {
			name = it.AuthorHandle
		}
		if keyword != "" {
			name = fmt.Sprintf("%s (Từ khóa: %s)", name, keyword)
		}
		pub := time.Now()
		if it.PublishedAt > 0 {
			pub = time.Unix(it.PublishedAt, 0)
		}
		post := models.CompetitorPost{
			Platform:         it.Platform,
			CompetitorPageID: truncateRunes(pageKey, 120),
			CompetitorName:   truncateRunes(name, 255),
			FBPostID:         truncateRunes(postKey, 180),
			Content:          strings.TrimSpace(it.Title),
			PermalinkURL:     httpOnly(it.URL),
			FullPicture:      httpOnly(it.Thumbnail),
			ViewsCount:       clampInt(it.Views),
			LikesCount:       clampInt(it.Likes),
			CommentsCount:    clampInt(it.Comments),
			SharesCount:      clampInt(it.Shares),
			PublishedAt:      pub,
		}
		err := h.DB.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "fb_post_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"content", "full_picture", "permalink_url", "views_count", "likes_count", "comments_count", "shares_count", "updated_at"}),
		}).Create(&post).Error
		if err == nil {
			saved++
		}
	}
	if saved == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không lưu được mục nào, thử lại sau."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Đã lưu %d mục vào kho nội dung đối thủ.", saved), "count": saved})
}
