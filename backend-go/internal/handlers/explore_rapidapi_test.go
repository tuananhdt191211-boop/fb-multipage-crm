package handlers

import (
	"net/url"
	"strings"
	"testing"
)

func TestTikwmRequestUsesRapidAPIWhenKeySet(t *testing.T) {
	t.Setenv("RAPIDAPI_KEY", "k123")
	t.Setenv("RAPIDAPI_TIKTOK_HOST", "")
	u, h, via := tikwmRequest("/api/user/posts", url.Values{"unique_id": {"abc"}})
	if !via || !strings.HasPrefix(u, "https://tiktok-scraper7.p.rapidapi.com/user/posts?") {
		t.Fatalf("unexpected url %q via=%v", u, via)
	}
	if h["X-RapidAPI-Key"] != "k123" || h["X-RapidAPI-Host"] != "tiktok-scraper7.p.rapidapi.com" {
		t.Fatalf("bad headers %v", h)
	}
	u, _, _ = tikwmRequest("/api/", url.Values{"url": {"x"}})
	if !strings.HasPrefix(u, "https://tiktok-scraper7.p.rapidapi.com/?") {
		t.Fatalf("video info url %q", u)
	}
}

func TestTikwmRequestFallsBackToTikwm(t *testing.T) {
	t.Setenv("RAPIDAPI_KEY", "")
	t.Setenv("TIKWM_API_BASE", "")
	u, h, via := tikwmRequest("/api/feed/search", url.Values{"keywords": {"a"}})
	if via || !strings.HasPrefix(u, "https://www.tikwm.com/api/feed/search?") || h["Referer"] == "" {
		t.Fatalf("unexpected %q %v %v", u, h, via)
	}
}
