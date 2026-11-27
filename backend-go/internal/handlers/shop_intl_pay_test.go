package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

func signStripe(secret, payload string, ts int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.%s", ts, payload)))
	return fmt.Sprintf("t=%d,v1=%s", ts, hex.EncodeToString(mac.Sum(nil)))
}

func TestVerifyStripeSignature(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	body := `{"type":"checkout.session.completed"}`
	h := signStripe("whsec_x", body, now.Unix())
	if !verifyStripeSignature(h, []byte(body), "whsec_x", now) {
		t.Fatal("valid signature rejected")
	}
	if verifyStripeSignature(h, []byte(body+" "), "whsec_x", now) {
		t.Fatal("tampered payload accepted")
	}
	if verifyStripeSignature(h, []byte(body), "whsec_other", now) {
		t.Fatal("wrong secret accepted")
	}
	if verifyStripeSignature(h, []byte(body), "whsec_x", now.Add(10*time.Minute)) {
		t.Fatal("stale timestamp accepted")
	}
}

func TestQuoteForeign(t *testing.T) {
	rateCache.Lock()
	rateCache.rates = map[string]float64{"USD": 0.00004, "EUR": 0.0000351, "HUF": 0.0138, "RON": 0.00018}
	rateCache.fetched = time.Now()
	rateCache.Unlock()
	h := &StorefrontHandler{}

	fa, err := h.quoteForeign(828000, "eur", "paypal")
	if err != nil || fa.Currency != "EUR" || fa.Minor != 2906 || fa.Value != "29.06" {
		t.Fatalf("EUR: %+v %v", fa, err)
	}
	// RON không có trên PayPal -> USD
	fa, _ = h.quoteForeign(828000, "RON", "paypal")
	if fa.Currency != "USD" || fa.Value != "33.12" {
		t.Fatalf("fallback: %+v", fa)
	}
	// HUF: PayPal số nguyên; Stripe làm tròn forint
	fa, _ = h.quoteForeign(828000, "HUF", "paypal")
	if fa.Value != "11426" || fa.Minor != 11426 {
		t.Fatalf("HUF paypal: %+v", fa)
	}
	fa, _ = h.quoteForeign(828000, "HUF", "stripe")
	if fa.Minor%100 != 0 {
		t.Fatalf("HUF stripe must be whole forints: %+v", fa)
	}
	if !amountMatches("29.06", 2906, 2) || amountMatches("29.05", 2906, 2) {
		t.Fatal("amountMatches wrong")
	}
}
