package handlers

// Thanh toán quốc tế bằng ngoại tệ: PayPal (Orders v2) và Stripe (Checkout Session).
// - Giá gốc của đơn luôn là VND; số tiền ngoại tệ được TÍNH Ở MÁY CHỦ theo tỉ giá (không tin trình duyệt).
// - PayPal: tạo đơn -> khách duyệt trên PayPal -> quay về -> máy chủ CAPTURE và kiểm tra số tiền.
// - Stripe: tạo Checkout Session -> khách trả trên trang Stripe -> quay về -> máy chủ đọc lại session;
//   thêm webhook có ký (Stripe-Signature) phòng khi khách đóng trình duyệt trước khi quay về.
// Khoá bí mật của từng shop được mã hoá (security.EncryptSecret) như khoá VNPay.

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"fb-multipage-crm/internal/models"
	"fb-multipage-crm/internal/security"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Tiền tệ PayPal nhận cho tài khoản Việt Nam (bỏ BRL, MYR, CNY chỉ dùng trong nước)
var paypalCurrencies = map[string]bool{
	"USD": true, "EUR": true, "GBP": true, "AUD": true, "CAD": true, "NZD": true, "CHF": true, "PLN": true,
	"CZK": true, "SEK": true, "DKK": true, "NOK": true, "HUF": true, "MXN": true, "JPY": true, "SGD": true,
	"HKD": true, "ILS": true, "PHP": true, "THB": true, "TWD": true,
}

// Tiền tệ Stripe (theo các thị trường của website)
var stripeCurrencies = map[string]bool{
	"USD": true, "EUR": true, "GBP": true, "AUD": true, "CAD": true, "NZD": true, "CHF": true, "PLN": true,
	"CZK": true, "SEK": true, "DKK": true, "NOK": true, "HUF": true, "RON": true, "UAH": true, "MXN": true,
	"BRL": true, "ARS": true, "CLP": true, "COP": true, "PEN": true, "UYU": true, "CRC": true, "DOP": true,
	"JPY": true, "SGD": true, "HKD": true,
}

// Số chữ số thập phân của từng loại tiền
func currencyDecimals(cur, gateway string) int {
	switch cur {
	case "JPY", "CLP", "VND", "KRW", "PYG":
		return 0
	case "HUF", "TWD":
		if gateway == "paypal" {
			return 0
		}
	}
	return 2
}

type foreignAmount struct {
	Currency string
	Minor    int64   // số nhỏ nhất (cent)
	Value    string  // dạng chuỗi cho PayPal, vd. "12.34"
	Rate     float64 // 1 VND = Rate ngoại tệ
}

// quoteForeign đổi tổng tiền VND sang ngoại tệ khách chọn (không hỗ trợ -> USD)
func (h *StorefrontHandler) quoteForeign(totalVND int64, want, gateway string) (foreignAmount, error) {
	want = strings.ToUpper(strings.TrimSpace(want))
	ok := paypalCurrencies
	if gateway == "stripe" {
		ok = stripeCurrencies
	}
	if !ok[want] {
		want = "USD"
	}
	rates, _, err := h.fetchRates()
	if err != nil || rates[want] <= 0 {
		return foreignAmount{}, errors.New("Chưa lấy được tỉ giá để quy đổi, vui lòng thử lại sau.")
	}
	rate := rates[want]
	dec := currencyDecimals(want, gateway)
	scale := math.Pow10(dec)
	minor := int64(math.Round(float64(totalVND) * rate * scale))
	if want == "HUF" && gateway == "stripe" { // Stripe: HUF phải là số forint tròn
		minor = int64(math.Round(float64(minor)/100) * 100)
	}
	if minor <= 0 {
		return foreignAmount{}, errors.New("Số tiền quá nhỏ để thanh toán quốc tế.")
	}
	val := strconv.FormatInt(minor, 10)
	if dec > 0 {
		val = strconv.FormatFloat(float64(minor)/scale, 'f', dec, 64)
	}
	return foreignAmount{Currency: want, Minor: minor, Value: val, Rate: rate}, nil
}

func (h *StorefrontHandler) paypalReady(ps ShopPaymentSettings) bool {
	return ps.PaypalEnabled && ps.PaypalClientID != "" && ps.PaypalSecretEnc != ""
}

func (h *StorefrontHandler) stripeReady(ps ShopPaymentSettings) bool {
	return ps.StripeEnabled && ps.StripeSecretEnc != ""
}

var intlHTTP = &http.Client{Timeout: 25 * time.Second}

// ----------------------------------------------------------------- PayPal

func paypalBase(sandbox bool) string {
	if sandbox {
		return "https://api-m.sandbox.paypal.com"
	}
	return "https://api-m.paypal.com"
}

func (h *StorefrontHandler) paypalToken(ps ShopPaymentSettings) (string, error) {
	secret, _ := security.DecryptSecret(ps.PaypalSecretEnc, h.encKey())
	if secret == "" {
		return "", errors.New("Chưa cấu hình khoá PayPal.")
	}
	req, _ := http.NewRequest(http.MethodPost, paypalBase(ps.PaypalSandbox)+"/v1/oauth2/token", strings.NewReader("grant_type=client_credentials"))
	req.SetBasicAuth(ps.PaypalClientID, secret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := intlHTTP.Do(req)
	if err != nil {
		return "", errors.New("Không kết nối được PayPal.")
	}
	defer resp.Body.Close()
	var out struct {
		AccessToken string `json:"access_token"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out)
	if resp.StatusCode != http.StatusOK || out.AccessToken == "" {
		return "", fmt.Errorf("PayPal từ chối Client ID / Secret (HTTP %d). Kiểm tra lại khoá và chế độ Sandbox/Live.", resp.StatusCode)
	}
	return out.AccessToken, nil
}

func (h *StorefrontHandler) paypalCall(ps ShopPaymentSettings, method, path string, body any) (int, []byte, error) {
	tok, err := h.paypalToken(ps)
	if err != nil {
		return 0, nil, err
	}
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, paypalBase(ps.PaypalSandbox)+path, rdr)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := intlHTTP.Do(req)
	if err != nil {
		return 0, nil, errors.New("Không kết nối được PayPal.")
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, raw, nil
}

func (h *StorefrontHandler) paypalCreate(ps ShopPaymentSettings, o *ShopOrder, sf *models.Storefront, fa foreignAmount, returnURL string) (string, string, error) {
	body := map[string]any{
		"intent": "CAPTURE",
		"purchase_units": []any{map[string]any{
			"reference_id": o.Code,
			"custom_id":    o.Code,
			"description":  trimTo(sf.SiteTitle+" #"+o.Code, 120),
			"amount":       map[string]string{"currency_code": fa.Currency, "value": fa.Value},
		}},
		"application_context": map[string]string{
			"brand_name":          trimTo(sf.SiteTitle, 120),
			"user_action":         "PAY_NOW",
			"shipping_preference": "NO_SHIPPING",
			"return_url":          returnURL + "&gw=paypal",
			"cancel_url":          returnURL + "&gw=paypal&cancel=1",
		},
	}
	status, raw, err := h.paypalCall(ps, http.MethodPost, "/v2/checkout/orders", body)
	if err != nil {
		return "", "", err
	}
	var out struct {
		ID    string `json:"id"`
		Links []struct {
			Href string `json:"href"`
			Rel  string `json:"rel"`
		} `json:"links"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(raw, &out)
	if status >= 300 || out.ID == "" {
		return "", "", fmt.Errorf("PayPal không tạo được thanh toán (HTTP %d) %s", status, out.Message)
	}
	for _, l := range out.Links {
		if l.Rel == "approve" || l.Rel == "payer-action" {
			return out.ID, l.Href, nil
		}
	}
	return "", "", errors.New("PayPal không trả về liên kết thanh toán.")
}

// paypalCapture thu tiền đơn PayPal đã được khách duyệt; trả (đã trả tiền?, mã giao dịch)
func (h *StorefrontHandler) paypalCapture(ps ShopPaymentSettings, o *ShopOrder) (bool, string, error) {
	type capture struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Amount struct {
			Currency string `json:"currency_code"`
			Value    string `json:"value"`
		} `json:"amount"`
	}
	type ppOrder struct {
		Status        string `json:"status"`
		PurchaseUnits []struct {
			Payments struct {
				Captures []capture `json:"captures"`
			} `json:"payments"`
		} `json:"purchase_units"`
	}
	status, raw, err := h.paypalCall(ps, http.MethodPost, "/v2/checkout/orders/"+url.PathEscape(o.GatewayRef)+"/capture", map[string]any{})
	if err != nil {
		return false, "", err
	}
	if status == http.StatusUnprocessableEntity { // đã capture trước đó, hoặc khách chưa duyệt
		status, raw, err = h.paypalCall(ps, http.MethodGet, "/v2/checkout/orders/"+url.PathEscape(o.GatewayRef), nil)
		if err != nil {
			return false, "", err
		}
	}
	if status >= 300 {
		return false, "", nil
	}
	var od ppOrder
	_ = json.Unmarshal(raw, &od)
	if od.Status != "COMPLETED" {
		return false, "", nil
	}
	for _, pu := range od.PurchaseUnits {
		for _, cp := range pu.Payments.Captures {
			if cp.Status == "COMPLETED" && cp.Amount.Currency == o.Currency && amountMatches(cp.Amount.Value, o.ForeignMinor, currencyDecimals(o.Currency, "paypal")) {
				return true, cp.ID, nil
			}
		}
	}
	return false, "", errors.New("Số tiền PayPal không khớp đơn hàng.")
}

func amountMatches(value string, minor int64, dec int) bool {
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return false
	}
	return int64(math.Round(f*math.Pow10(dec))) == minor
}

// ----------------------------------------------------------------- Stripe

func (h *StorefrontHandler) stripeCall(ps ShopPaymentSettings, method, path string, form url.Values) (int, []byte, error) {
	secret, _ := security.DecryptSecret(ps.StripeSecretEnc, h.encKey())
	if secret == "" {
		return 0, nil, errors.New("Chưa cấu hình khoá Stripe.")
	}
	var rdr io.Reader
	if form != nil {
		rdr = strings.NewReader(form.Encode())
	}
	req, _ := http.NewRequest(method, "https://api.stripe.com"+path, rdr)
	req.SetBasicAuth(secret, "")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := intlHTTP.Do(req)
	if err != nil {
		return 0, nil, errors.New("Không kết nối được Stripe.")
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, raw, nil
}

func (h *StorefrontHandler) stripeCreate(ps ShopPaymentSettings, o *ShopOrder, sf *models.Storefront, fa foreignAmount, returnURL string) (string, string, error) {
	f := url.Values{}
	f.Set("mode", "payment")
	f.Set("client_reference_id", o.Code)
	f.Set("metadata[order_code]", o.Code)
	f.Set("payment_intent_data[metadata][order_code]", o.Code)
	f.Set("line_items[0][quantity]", "1")
	f.Set("line_items[0][price_data][currency]", strings.ToLower(fa.Currency))
	f.Set("line_items[0][price_data][unit_amount]", strconv.FormatInt(fa.Minor, 10))
	f.Set("line_items[0][price_data][product_data][name]", trimTo(sf.SiteTitle+" #"+o.Code, 200))
	if o.Email != "" {
		f.Set("customer_email", o.Email)
	}
	f.Set("success_url", returnURL+"&gw=stripe")
	f.Set("cancel_url", returnURL+"&gw=stripe&cancel=1")
	status, raw, err := h.stripeCall(ps, http.MethodPost, "/v1/checkout/sessions", f)
	if err != nil {
		return "", "", err
	}
	var out struct {
		ID    string `json:"id"`
		URL   string `json:"url"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &out)
	if status >= 300 || out.ID == "" || out.URL == "" {
		msg := fmt.Sprintf("HTTP %d", status)
		if out.Error != nil {
			msg = out.Error.Message
		}
		return "", "", errors.New("Stripe không tạo được thanh toán: " + msg)
	}
	return out.ID, out.URL, nil
}

// stripeCheck đọc lại Checkout Session; trả (đã trả tiền?, mã PaymentIntent)
func (h *StorefrontHandler) stripeCheck(ps ShopPaymentSettings, o *ShopOrder) (bool, string, error) {
	status, raw, err := h.stripeCall(ps, http.MethodGet, "/v1/checkout/sessions/"+url.PathEscape(o.GatewayRef), nil)
	if err != nil {
		return false, "", err
	}
	if status >= 300 {
		return false, "", nil
	}
	var s struct {
		PaymentStatus string `json:"payment_status"`
		AmountTotal   int64  `json:"amount_total"`
		Currency      string `json:"currency"`
		PaymentIntent string `json:"payment_intent"`
		ClientRef     string `json:"client_reference_id"`
	}
	_ = json.Unmarshal(raw, &s)
	if s.PaymentStatus != "paid" {
		return false, "", nil
	}
	if s.ClientRef != o.Code || s.AmountTotal != o.ForeignMinor || !strings.EqualFold(s.Currency, o.Currency) {
		return false, "", errors.New("Số tiền Stripe không khớp đơn hàng.")
	}
	return true, s.PaymentIntent, nil
}

// ----------------------------------------------------------------- Dùng chung

// startIntlPayment tạo phiên thanh toán ngoại tệ cho đơn và lưu thông tin quy đổi; trả link thanh toán
func (h *StorefrontHandler) startIntlPayment(ps ShopPaymentSettings, sf *models.Storefront, o *ShopOrder, currency, returnURL string) (string, error) {
	fa, err := h.quoteForeign(o.Total, currency, o.PaymentMethod)
	if err != nil {
		return "", err
	}
	o.Currency, o.ForeignMinor, o.FxRate = fa.Currency, fa.Minor, fa.Rate
	var ref, payURL string
	if o.PaymentMethod == "paypal" {
		ref, payURL, err = h.paypalCreate(ps, o, sf, fa, returnURL)
	} else {
		ref, payURL, err = h.stripeCreate(ps, o, sf, fa, returnURL)
	}
	if err != nil {
		return "", err
	}
	o.GatewayRef = ref
	h.DB.Model(&ShopOrder{}).Where("id = ?", o.ID).Updates(map[string]any{
		"currency": fa.Currency, "foreign_minor": fa.Minor, "fx_rate": fa.Rate,
		"gateway_ref": ref, "payment_status": "pending",
	})
	return payURL, nil
}

// markOrderPaid: ghi nhận đã thanh toán (một lần), trừ kho
func (h *StorefrontHandler) markOrderPaid(o *ShopOrder, txn string) error {
	return h.DB.Transaction(func(tx *gorm.DB) error {
		var cur ShopOrder
		if err := tx.Where("id = ?", o.ID).First(&cur).Error; err != nil {
			return err
		}
		if cur.PaymentStatus == "paid" {
			return nil
		}
		now := time.Now()
		updates := map[string]any{"payment_status": "paid", "paid_at": &now, "gateway_txn": txn}
		if !cur.StockApplied {
			_ = applyStock(tx, cur.Items, -1)
			updates["stock_applied"] = true
		}
		return tx.Model(&ShopOrder{}).Where("id = ?", o.ID).Updates(updates).Error
	})
}

// ConfirmIntlPayment POST /api/v1/storefront/public/orders/:code/confirm?t=token
// Gọi khi khách quay về từ PayPal / Stripe (hoặc mở lại trang đơn): kiểm tra & ghi nhận thanh toán.
func (h *StorefrontHandler) ConfirmIntlPayment(c *gin.Context) {
	var o ShopOrder
	if h.DB.Where("code = ?", c.Param("code")).First(&o).Error != nil ||
		!hmac.Equal([]byte(o.AccessToken), []byte(c.Query("t"))) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy đơn hàng."})
		return
	}
	if o.PaymentStatus == "paid" || (o.PaymentMethod != "paypal" && o.PaymentMethod != "stripe") || o.GatewayRef == "" {
		c.JSON(http.StatusOK, gin.H{"payment_status": o.PaymentStatus})
		return
	}
	ps := h.paymentSettings(o.StorefrontID)
	var paid bool
	var txn string
	var err error
	if o.PaymentMethod == "paypal" {
		paid, txn, err = h.paypalCapture(ps, &o)
	} else {
		paid, txn, err = h.stripeCheck(ps, &o)
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "payment_status": o.PaymentStatus})
		return
	}
	if paid {
		if err := h.markOrderPaid(&o, txn); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Không ghi nhận được thanh toán."})
			return
		}
		o.PaymentStatus = "paid"
	}
	c.JSON(http.StatusOK, gin.H{"payment_status": o.PaymentStatus})
}

// StripeWebhook POST /api/v1/payments/stripe/webhook/:site — Stripe báo phiên thanh toán hoàn tất
func (h *StorefrontHandler) StripeWebhook(c *gin.Context) {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	siteID, _ := strconv.ParseUint(c.Param("site"), 10, 64)
	ps := h.paymentSettings(uint(siteID))
	secret, _ := security.DecryptSecret(ps.StripeWebhookSecretEnc, h.encKey())
	if secret == "" || !verifyStripeSignature(c.GetHeader("Stripe-Signature"), raw, secret, time.Now()) {
		c.Status(http.StatusBadRequest)
		return
	}
	var ev struct {
		Type string `json:"type"`
		Data struct {
			Object struct {
				ID        string `json:"id"`
				ClientRef string `json:"client_reference_id"`
			} `json:"object"`
		} `json:"data"`
	}
	_ = json.Unmarshal(raw, &ev)
	if ev.Type == "checkout.session.completed" || ev.Type == "checkout.session.async_payment_succeeded" {
		var o ShopOrder
		if h.DB.Where("storefront_id = ? AND gateway_ref = ?", siteID, ev.Data.Object.ID).First(&o).Error == nil {
			// Không tin dữ liệu trong sự kiện: đọc lại session từ Stripe rồi mới ghi nhận
			if paid, txn, err := h.stripeCheck(ps, &o); err == nil && paid {
				_ = h.markOrderPaid(&o, txn)
			}
		}
	}
	c.Status(http.StatusOK)
}

// verifyStripeSignature: header "t=...,v1=..." ; ký HMAC-SHA256 của "t.payload"; lệch giờ tối đa 5 phút
func verifyStripeSignature(header string, payload []byte, secret string, now time.Time) bool {
	var ts string
	var sigs []string
	for _, part := range strings.Split(header, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			ts = kv[1]
		case "v1":
			sigs = append(sigs, kv[1])
		}
	}
	t, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || len(sigs) == 0 || math.Abs(float64(now.Unix()-t)) > 300 {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "."))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	for _, s := range sigs {
		if hmac.Equal([]byte(s), []byte(expected)) {
			return true
		}
	}
	return false
}
