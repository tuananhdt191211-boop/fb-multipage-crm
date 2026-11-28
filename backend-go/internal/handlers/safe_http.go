package handlers

// HTTP client "an toàn" để máy chủ tải nội dung từ một địa chỉ do người dùng nhập (nhập sản phẩm từ link, tải ảnh về thư viện).
// Chống SSRF: từ chối địa chỉ nội bộ / loopback / link-local (kể cả metadata đám mây 169.254.169.254), kiểm tra lại ở MỖI lần
// chuyển hướng, và nối thẳng tới IP đã kiểm tra (không để DNS đổi đáp án giữa lúc kiểm tra và lúc kết nối).

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var blockedCIDRs = func() []*net.IPNet {
	var out []*net.IPNet
	for _, c := range []string{
		"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24",
		"192.168.0.0/16", "198.18.0.0/15", "224.0.0.0/4", "240.0.0.0/4", "::1/128", "fc00::/7", "fe80::/10", "ff00::/8", "::/128",
	} {
		_, n, _ := net.ParseCIDR(c)
		out = append(out, n)
	}
	return out
}()

func isBlockedIP(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
	}
	for _, n := range blockedCIDRs {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

var errBlockedHost = errors.New("địa chỉ không được phép truy cập")

// allowCustomFetchPorts chỉ bật trong test (máy chủ thử chạy ở cổng ngẫu nhiên)
var allowCustomFetchPorts = false

func checkFetchURL(u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("chỉ hỗ trợ link http/https")
	}
	host := strings.ToLower(u.Hostname())
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return errBlockedHost
	}
	if p := u.Port(); p != "" && p != "80" && p != "443" && !allowCustomFetchPorts {
		return fmt.Errorf("chỉ hỗ trợ cổng 80/443")
	}
	return nil
}

// newSafeHTTPClient: lưu ý Proxy: nil để không đi vòng qua proxy của hệ thống
func newSafeHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	tr := &http.Transport{
		Proxy:               nil,
		TLSHandshakeTimeout: 10 * time.Second,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			var ips []net.IP
			if ip := net.ParseIP(host); ip != nil {
				ips = []net.IP{ip}
			} else {
				addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
				if err != nil {
					return nil, err
				}
				for _, a := range addrs {
					ips = append(ips, a.IP)
				}
			}
			var lastErr error = errBlockedHost
			for _, ip := range ips {
				if isBlockedIP(ip) {
					continue
				}
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			return nil, lastErr
		},
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("quá nhiều lần chuyển hướng")
			}
			return checkFetchURL(req.URL)
		},
	}
}

// fetchLimited tải tối đa max byte (quá giới hạn thì báo lỗi), trả về nội dung + Content-Type + URL cuối sau chuyển hướng.
func fetchLimited(client *http.Client, raw string, max int64, headers map[string]string) ([]byte, string, string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, "", "", fmt.Errorf("link không hợp lệ")
	}
	if err := checkFetchURL(u); err != nil {
		return nil, "", "", err
	}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, "", resp.Request.URL.String(), fmt.Errorf("máy chủ nguồn trả HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > max {
		return nil, "", resp.Request.URL.String(), fmt.Errorf("file quá lớn")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, "", resp.Request.URL.String(), err
	}
	if int64(len(data)) > max {
		return nil, "", resp.Request.URL.String(), fmt.Errorf("file quá lớn")
	}
	return data, resp.Header.Get("Content-Type"), resp.Request.URL.String(), nil
}
