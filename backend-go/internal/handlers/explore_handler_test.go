package handlers

import (
	"encoding/json"
	"testing"
)

func TestParseExploreInput(t *testing.T) {
	cases := []struct {
		q, platform, mode string
		want              exploreTarget
	}{
		// TikTok
		{"https://www.tiktok.com/@noithat.tungthuy", "", "", exploreTarget{"tiktok", "channel", "noithat.tungthuy"}},
		{"tiktok.com/@abc?lang=vi", "", "", exploreTarget{"tiktok", "channel", "abc"}},
		{"https://www.tiktok.com/@abc/video/7312345678901234567", "", "", exploreTarget{"tiktok", "single", "https://www.tiktok.com/@abc/video/7312345678901234567"}},
		{"https://vt.tiktok.com/ZSabc123/", "", "", exploreTarget{"tiktok", "single", "https://vt.tiktok.com/ZSabc123/"}},
		{"@shopxinh", "", "", exploreTarget{"tiktok", "channel", "shopxinh"}},
		{"nước hoa nữ", "tiktok", "", exploreTarget{"tiktok", "keyword", "nước hoa nữ"}},
		{"nội thất tùng thuỷ", "tiktok", "channel", exploreTarget{"tiktok", "channels", "nội thất tùng thuỷ"}},
		{"micro karaoke", "", "", exploreTarget{"tiktok", "keyword", "micro karaoke"}},
		// YouTube
		{"https://www.youtube.com/@MrBeast/videos", "", "", exploreTarget{"youtube", "channel", "@MrBeast"}},
		{"https://youtube.com/channel/UCX6OQ3DkcsbYNE6H8uQQuVA", "", "", exploreTarget{"youtube", "channel", "UCX6OQ3DkcsbYNE6H8uQQuVA"}},
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=10", "", "", exploreTarget{"youtube", "single", "dQw4w9WgXcQ"}},
		{"https://youtu.be/dQw4w9WgXcQ", "", "", exploreTarget{"youtube", "single", "dQw4w9WgXcQ"}},
		{"https://m.youtube.com/shorts/abcdefghijk", "", "", exploreTarget{"youtube", "single", "abcdefghijk"}},
		{"https://www.youtube.com/user/oldname", "", "", exploreTarget{"youtube", "channel", "user:oldname"}},
		{"@mixigaming", "youtube", "", exploreTarget{"youtube", "channel", "@mixigaming"}},
		{"review điện thoại", "youtube", "channel", exploreTarget{"youtube", "channels", "review điện thoại"}},
		// Facebook
		{"https://www.facebook.com/noithattungthuy", "", "", exploreTarget{"facebook", "channel", "noithattungthuy"}},
		{"https://www.facebook.com/profile.php?id=100064", "", "", exploreTarget{"facebook", "channel", "100064"}},
		{"https://www.facebook.com/people/Shop-ABC/61550000000000/", "", "", exploreTarget{"facebook", "channel", "61550000000000"}},
		{"https://www.facebook.com/abc/posts/pfbid0xyz", "", "", exploreTarget{"facebook", "single", "https://www.facebook.com/abc/posts/pfbid0xyz"}},
		{"https://fb.watch/abc123/", "", "", exploreTarget{"facebook", "single", "https://fb.watch/abc123/"}},
		{"giường ngủ", "facebook", "", exploreTarget{"facebook", "keyword", "giường ngủ"}},
	}
	for _, c := range cases {
		got, err := parseExploreInput(c.q, c.platform, c.mode)
		if err != nil {
			t.Errorf("%q: lỗi không mong đợi: %v", c.q, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q: được %+v, muốn %+v", c.q, got, c.want)
		}
	}

	for _, bad := range []struct{ q, platform, mode string }{
		{"", "", ""},
		{"https://example.com/abc", "", ""},
		{"tên page có dấu cách", "facebook", "channel"},
	} {
		if _, err := parseExploreInput(bad.q, bad.platform, bad.mode); err == nil {
			t.Errorf("%q: lẽ ra phải báo lỗi", bad.q)
		}
	}
}

func TestTikwmVideoDecoding(t *testing.T) {
	// counts dạng số hoặc chuỗi, cover dạng đường dẫn tương đối, cursor dạng số
	raw := `{"videos":[{"video_id":"1","title":" Hello ","cover":"/video/cover/1.webp","play_count":"1200","digg_count":3.0,
	  "duration":15,"create_time":1700000000,"author":{"unique_id":"abc","nickname":"ABC","avatar":"https://x/a.jpg"}},
	  {"video_id":"2","title":"Ảnh","cover":"","images":["https://x/i.jpg"],"author":{"unique_id":"abc"}}],
	  "cursor":1700000000000,"hasMore":true}`
	var page struct {
		Videos  []tikwmVideo    `json:"videos"`
		Cursor  json.RawMessage `json:"cursor"`
		HasMore bool            `json:"hasMore"`
	}
	if err := json.Unmarshal([]byte(raw), &page); err != nil {
		t.Fatal(err)
	}
	a := page.Videos[0].toItem("", "", "")
	if a.Views != 1200 || a.Likes != 3 || a.Title != "Hello" || a.Thumbnail != "https://www.tikwm.com/video/cover/1.webp" ||
		a.URL != "https://www.tiktok.com/@abc/video/1" || a.Kind != "video" {
		t.Errorf("video sai: %+v", a)
	}
	b := page.Videos[1].toItem("", "", "")
	if b.Kind != "photo" || b.Thumbnail != "https://x/i.jpg" || b.URL != "https://www.tiktok.com/@abc/photo/2" || b.AuthorName != "@abc" {
		t.Errorf("bài ảnh sai: %+v", b)
	}
	if rawCursor(page.Cursor) != "1700000000000" {
		t.Errorf("cursor sai: %s", rawCursor(page.Cursor))
	}
}

func TestParseISODuration(t *testing.T) {
	for in, want := range map[string]int64{"PT45S": 45, "PT1M2S": 62, "PT1H0M5S": 3605, "P1DT1S": 86401, "": 0} {
		if got := parseISODuration(in); got != want {
			t.Errorf("%s: %d, muốn %d", in, got, want)
		}
	}
}
