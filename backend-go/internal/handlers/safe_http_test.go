package handlers

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestIsBlockedIP(t *testing.T) {
	for _, s := range []string{"127.0.0.1", "10.1.2.3", "172.16.0.9", "172.31.255.1", "192.168.1.1", "169.254.169.254", "100.64.0.1", "0.0.0.0", "::1", "fc00::1", "fe80::1", "::ffff:127.0.0.1", "::ffff:10.0.0.1", "224.0.0.1"} {
		if !isBlockedIP(net.ParseIP(s)) {
			t.Errorf("%s phải bị chặn", s)
		}
	}
	for _, s := range []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "172.32.0.1", "2606:4700:4700::1111"} {
		if isBlockedIP(net.ParseIP(s)) {
			t.Errorf("%s không được chặn", s)
		}
	}
}

func TestCheckFetchURL(t *testing.T) {
	bad := []string{"ftp://example.com/a", "file:///etc/passwd", "http://localhost/x", "http://foo.localhost/x", "http://svc.internal/x", "http://printer.local/x", "https://example.com:8443/x", "http:///nohost", "javascript:alert(1)"}
	for _, s := range bad {
		u, _ := url.Parse(s)
		if checkFetchURL(u) == nil {
			t.Errorf("%q phải bị từ chối", s)
		}
	}
	for _, s := range []string{"https://ae-pic-a1.aliexpress-media.com/kf/a.jpg", "http://example.com/a", "https://example.com:443/a"} {
		u, _ := url.Parse(s)
		if err := checkFetchURL(u); err != nil {
			t.Errorf("%q bị từ chối nhầm: %v", s, err)
		}
	}
}

// Máy chủ thử chạy ở 127.0.0.1 → client an toàn phải từ chối kết nối tới đó, kể cả khi bị chuyển hướng từ link "ngoài".
func TestSafeClientBlocksLoopbackAndRedirects(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.Write([]byte("bí mật nội bộ"))
	}))
	defer srv.Close()
	client := newSafeHTTPClient(5 * time.Second)

	if _, _, _, err := fetchLimited(client, srv.URL, 1<<20, nil); err == nil {
		t.Fatal("kết nối tới 127.0.0.1 phải bị chặn")
	}
	if hit {
		t.Fatal("yêu cầu đã tới máy chủ nội bộ")
	}
	// IP tuyệt đối cũng bị chặn ở bước Dial (cổng 80 để qua kiểm tra cổng)
	if _, _, _, err := fetchLimited(client, "http://127.0.0.1/", 1<<20, nil); err == nil || !strings.Contains(err.Error(), "không được phép") {
		t.Fatalf("127.0.0.1:80 phải bị chặn bằng lỗi rõ ràng, được: %v", err)
	}
	if _, _, _, err := fetchLimited(client, "http://169.254.169.254/latest/meta-data/", 1<<20, nil); err == nil {
		t.Fatal("metadata đám mây phải bị chặn")
	}
}
