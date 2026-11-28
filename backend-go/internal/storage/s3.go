// Package storage lưu file (ảnh / video sản phẩm) lên kho tương thích S3: Cloudflare R2, AWS S3, Backblaze B2, MinIO...
//
// Vì sao cần: ổ đĩa của Render bị xoá mỗi lần deploy / ngủ dậy, còn nhét video vào PostgreSQL thì phá hạn mức Neon
// (dung lượng + băng thông). Kho ngoài lưu bền và phục vụ trực tiếp qua CDN, backend không phải gánh băng thông.
//
// Chỉ dùng thư viện chuẩn: ký yêu cầu theo AWS Signature V4 (payload UNSIGNED-PAYLOAD qua HTTPS nên không phải băm file).
package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

type S3 struct {
	Endpoint   string // vd. https://<account>.r2.cloudflarestorage.com
	Region     string // R2: "auto"
	Bucket     string
	AccessKey  string
	SecretKey  string
	PublicBase string // địa chỉ công khai để khách tải file, vd. https://pub-xxxx.r2.dev hoặc https://cdn.shop.com
	Client     *http.Client
}

// Config lấy từ biến môi trường (xem internal/config)
type Config struct {
	Endpoint, Region, Bucket, AccessKey, SecretKey, PublicBase string
}

// New trả về nil nếu chưa cấu hình đủ (khi đó hệ thống tự dùng cách lưu cũ)
func New(c Config) *S3 {
	c.Endpoint = strings.TrimRight(strings.TrimSpace(c.Endpoint), "/")
	c.PublicBase = strings.TrimRight(strings.TrimSpace(c.PublicBase), "/")
	if c.Endpoint == "" || strings.TrimSpace(c.Bucket) == "" || strings.TrimSpace(c.AccessKey) == "" || strings.TrimSpace(c.SecretKey) == "" || c.PublicBase == "" {
		return nil
	}
	region := strings.TrimSpace(c.Region)
	if region == "" {
		region = "auto"
	}
	return &S3{
		Endpoint: c.Endpoint, Region: region, Bucket: strings.TrimSpace(c.Bucket),
		AccessKey: strings.TrimSpace(c.AccessKey), SecretKey: strings.TrimSpace(c.SecretKey), PublicBase: c.PublicBase,
		Client: &http.Client{Timeout: 10 * time.Minute},
	}
}

// Khoá chỉ gồm ký tự an toàn (không cần mã hoá đường dẫn khi ký)
var keyRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9/_.-]{0,300}$`)

func (s *S3) PublicURL(key string) string { return s.PublicBase + "/" + key }

func (s *S3) objectURL(key string) (string, error) {
	if !keyRe.MatchString(key) || strings.Contains(key, "..") || strings.Contains(key, "//") {
		return "", fmt.Errorf("khoá file không hợp lệ")
	}
	return s.Endpoint + "/" + s.Bucket + "/" + key, nil
}

func hmacSHA256(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(data))
	return m.Sum(nil)
}

// sign ghi các header ký theo SigV4. Luôn ký host, x-amz-content-sha256, x-amz-date; extra là tên (chữ thường) các header
// khác muốn ký thêm (vd. "range") — dùng để kiểm chứng với ví dụ chính thức của AWS.
func (s *S3) sign(req *http.Request, payloadHash string, now time.Time, extra ...string) {
	now = now.UTC()
	amzDate := now.Format("20060102T150405Z")
	date := now.Format("20060102")
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)

	names := append([]string{"host", "x-amz-content-sha256", "x-amz-date"}, extra...)
	sort.Strings(names)
	var ch strings.Builder
	for _, n := range names {
		v := req.URL.Host
		if n != "host" {
			v = strings.Join(strings.Fields(req.Header.Get(n)), " ")
		}
		ch.WriteString(n + ":" + v + "\n")
	}
	signedHeaders := strings.Join(names, ";")
	canonicalRequest := strings.Join([]string{
		req.Method, req.URL.EscapedPath(), req.URL.RawQuery, ch.String(), signedHeaders, payloadHash,
	}, "\n")

	scope := date + "/" + s.Region + "/s3/aws4_request"
	sum := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(sum[:])

	k := hmacSHA256([]byte("AWS4"+s.SecretKey), date)
	k = hmacSHA256(k, s.Region)
	k = hmacSHA256(k, "s3")
	k = hmacSHA256(k, "aws4_request")
	sig := hex.EncodeToString(hmacSHA256(k, stringToSign))

	req.Header.Set("Authorization", fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s", s.AccessKey, scope, signedHeaders, sig))
}

// Put tải body (đúng size byte) lên kho. cacheControl rỗng thì dùng "public, max-age=31536000, immutable".
func (s *S3) Put(ctx context.Context, key, contentType string, body io.Reader, size int64) error {
	u, err := s.objectURL(key)
	if err != nil {
		return err
	}
	if size <= 0 {
		return fmt.Errorf("file rỗng")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, io.NopCloser(body))
	if err != nil {
		return err
	}
	req.ContentLength = size
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Cache-Control", "public, max-age=31536000, immutable")
	s.sign(req, "UNSIGNED-PAYLOAD", time.Now())
	resp, err := s.Client.Do(req)
	if err != nil {
		return fmt.Errorf("không kết nối được kho lưu trữ: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 600))
		return fmt.Errorf("kho lưu trữ từ chối (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

const emptyPayloadHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// Delete xoá file (không có cũng coi như xong)
func (s *S3) Delete(ctx context.Context, key string) error {
	u, err := s.objectURL(key)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return err
	}
	s.sign(req, emptyPayloadHash, time.Now())
	resp, err := s.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("kho lưu trữ từ chối xoá (HTTP %d)", resp.StatusCode)
	}
	return nil
}

// Get đọc lại nội dung file bằng yêu cầu có ký (dùng để kiểm tra / sao lưu; khách tải qua PublicURL)
func (s *S3) Get(ctx context.Context, key string) ([]byte, error) {
	u, err := s.objectURL(key)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	s.sign(req, emptyPayloadHash, time.Now())
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 200<<20))
}
