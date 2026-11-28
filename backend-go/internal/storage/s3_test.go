package storage

// - TestSignMatchesAWSDocumentationVector: ví dụ ký chính thức trong tài liệu AWS (GET Object có Range) — bằng chứng
//   cách ký đúng chuẩn mà không cần máy chủ thật.
// - TestS3AgainstRealServer: chạy với một máy chủ tương thích S3 thật (MinIO, Scality, R2...), bỏ qua nếu không đặt biến:
//     TEST_S3_ENDPOINT=http://localhost:18000 TEST_S3_ACCESS=accessKey1 TEST_S3_SECRET=verySecretKey1 \
//     TEST_S3_BUCKET=pp-test TEST_S3_REGION=us-east-1 go test ./internal/storage -run RealServer -v

import (
	"bytes"
	"context"
	"crypto/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSignMatchesAWSDocumentationVector(t *testing.T) {
	// https://docs.aws.amazon.com/AmazonS3/latest/API/sig-v4-header-based-auth.html — "Example: GET Object"
	s := &S3{Region: "us-east-1", AccessKey: "AKIAIOSFODNN7EXAMPLE", SecretKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"}
	u, _ := url.Parse("https://examplebucket.s3.amazonaws.com/test.txt")
	req, _ := http.NewRequest(http.MethodGet, u.String(), nil)
	req.Header.Set("Range", "bytes=0-9")
	now := time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC)
	s.sign(req, emptyPayloadHash, now, "range")
	got := req.Header.Get("Authorization")
	want := "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request, SignedHeaders=host;range;x-amz-content-sha256;x-amz-date, Signature=f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41"
	if got != want {
		t.Fatalf("chữ ký không khớp ví dụ AWS:\n got: %s\nwant: %s", got, want)
	}
}

func TestNewRequiresFullConfig(t *testing.T) {
	full := Config{Endpoint: "https://x.r2.cloudflarestorage.com/", Bucket: "b", AccessKey: "a", SecretKey: "s", PublicBase: "https://cdn.x.com/"}
	if New(full) == nil {
		t.Fatal("đủ cấu hình mà không tạo được")
	}
	for name, mut := range map[string]func(*Config){
		"thiếu endpoint": func(c *Config) { c.Endpoint = "" },
		"thiếu bucket":   func(c *Config) { c.Bucket = " " },
		"thiếu key":      func(c *Config) { c.AccessKey = "" },
		"thiếu secret":   func(c *Config) { c.SecretKey = "" },
		"thiếu public":   func(c *Config) { c.PublicBase = "" },
	} {
		c := full
		mut(&c)
		if New(c) != nil {
			t.Errorf("%s mà vẫn bật kho lưu trữ", name)
		}
	}
	s := New(full)
	if s.Region != "auto" || s.Endpoint != "https://x.r2.cloudflarestorage.com" || s.PublicURL("a/b.mp4") != "https://cdn.x.com/a/b.mp4" {
		t.Fatalf("chuẩn hoá sai: %+v / %s", s, s.PublicURL("a/b.mp4"))
	}
}

func TestObjectKeyValidation(t *testing.T) {
	s := New(Config{Endpoint: "https://e", Bucket: "b", AccessKey: "a", SecretKey: "s", PublicBase: "https://p"})
	for _, k := range []string{"sites/1/202611/abc123.mp4", "a.jpg", "x/y_z-1.webp"} {
		if _, err := s.objectURL(k); err != nil {
			t.Errorf("khoá hợp lệ bị từ chối %q: %v", k, err)
		}
	}
	for _, k := range []string{"", "../etc/passwd", "a//b", "a b.jpg", "a?x=1", "/abs", "a/../b", "tiếng-việt.jpg", "a%2e.jpg"} {
		if _, err := s.objectURL(k); err == nil {
			t.Errorf("khoá không hợp lệ được chấp nhận %q", k)
		}
	}
}

func TestS3AgainstRealServer(t *testing.T) {
	ep := os.Getenv("TEST_S3_ENDPOINT")
	if ep == "" {
		t.Skip("đặt TEST_S3_ENDPOINT/ACCESS/SECRET/BUCKET để thử với máy chủ S3 thật")
	}
	cfg := Config{Endpoint: ep, Region: os.Getenv("TEST_S3_REGION"), Bucket: os.Getenv("TEST_S3_BUCKET"),
		AccessKey: os.Getenv("TEST_S3_ACCESS"), SecretKey: os.Getenv("TEST_S3_SECRET"), PublicBase: ep + "/" + os.Getenv("TEST_S3_BUCKET")}
	s := New(cfg)
	if s == nil {
		t.Fatal("cấu hình thử không đủ")
	}
	ctx := context.Background()

	// tạo bucket (PUT /bucket, không có trong API sản phẩm vì bucket do người dùng tạo sẵn)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, ep+"/"+cfg.Bucket, nil)
	s.sign(req, emptyPayloadHash, time.Now())
	if resp, err := s.Client.Do(req); err != nil {
		t.Fatal(err)
	} else {
		resp.Body.Close()
		if resp.StatusCode/100 != 2 && resp.StatusCode != http.StatusConflict {
			t.Fatalf("tạo bucket: HTTP %d", resp.StatusCode)
		}
	}

	// file nhỏ + file lớn (5MB ngẫu nhiên, giống video ngắn)
	small := []byte("xin chào video")
	big := make([]byte, 5<<20)
	_, _ = rand.Read(big)
	for key, data := range map[string][]byte{"sites/1/202611/small.mp4": small, "sites/1/202611/big.webm": big} {
		if err := s.Put(ctx, key, "video/mp4", bytes.NewReader(data), int64(len(data))); err != nil {
			t.Fatalf("Put %s: %v", key, err)
		}
		got, err := s.Get(ctx, key)
		if err != nil || !bytes.Equal(got, data) {
			t.Fatalf("Get %s: %v (khác nội dung: %v)", key, err, !bytes.Equal(got, data))
		}
		if err := s.Delete(ctx, key); err != nil {
			t.Fatalf("Delete %s: %v", key, err)
		}
		if _, err := s.Get(ctx, key); err == nil || !strings.Contains(err.Error(), "404") {
			t.Fatalf("sau khi xoá phải 404, được: %v", err)
		}
	}
	if err := s.Delete(ctx, "sites/1/202611/khong-ton-tai.mp4"); err != nil {
		t.Fatalf("xoá file không tồn tại phải êm: %v", err)
	}

	// sai khoá bí mật → bị từ chối, và thông báo lỗi nêu mã HTTP
	bad := New(Config{Endpoint: ep, Region: cfg.Region, Bucket: cfg.Bucket, AccessKey: cfg.AccessKey, SecretKey: "sai-khoa", PublicBase: cfg.PublicBase})
	if err := bad.Put(ctx, "sites/1/x.mp4", "video/mp4", bytes.NewReader(small), int64(len(small))); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("sai khoá bí mật phải bị 403, được: %v", err)
	}
	// size không khớp nội dung → máy chủ từ chối thay vì lưu file hỏng
	if err := s.Put(ctx, "sites/1/short.mp4", "video/mp4", bytes.NewReader(small[:3]), int64(len(small))); err == nil {
		_ = s.Delete(ctx, "sites/1/short.mp4")
		t.Fatal("size lớn hơn nội dung thật mà vẫn lưu được")
	}
}
