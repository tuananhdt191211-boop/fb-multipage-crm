package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func signFB(secret, payload string) string {
	body := base64.RawURLEncoding.EncodeToString([]byte(payload))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)) + "." + body
}

func TestParseSignedRequest(t *testing.T) {
	secret := "app-secret"
	good := signFB(secret, `{"algorithm":"HMAC-SHA256","user_id":"12345","issued_at":1}`)

	p, err := parseSignedRequest(good, secret)
	if err != nil || p.UserID != "12345" {
		t.Fatalf("chữ ký đúng phải được chấp nhận: %v %+v", err, p)
	}
	if _, err := parseSignedRequest(good, "other-secret"); err == nil {
		t.Fatal("sai App Secret phải bị từ chối")
	}
	if _, err := parseSignedRequest(good, ""); err == nil {
		t.Fatal("chưa cấu hình App Secret phải bị từ chối")
	}
	if _, err := parseSignedRequest("abc", secret); err == nil {
		t.Fatal("sai định dạng phải bị từ chối")
	}
	noUser := signFB(secret, `{"algorithm":"HMAC-SHA256","issued_at":1}`)
	if _, err := parseSignedRequest(noUser, secret); err == nil {
		t.Fatal("thiếu user_id phải bị từ chối")
	}
	badAlg := signFB(secret, `{"algorithm":"none","user_id":"1"}`)
	if _, err := parseSignedRequest(badAlg, secret); err == nil {
		t.Fatal("thuật toán lạ phải bị từ chối")
	}
}
