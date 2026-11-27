package handlers

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestVnpSignAndVerifyRoundTrip(t *testing.T) {
	secret := "SECRETKEY123"
	p := url.Values{}
	p.Set("vnp_Version", "2.1.0")
	p.Set("vnp_Amount", "1000000")
	p.Set("vnp_OrderInfo", "Thanh toan don hang DH26112712345")
	p.Set("vnp_ReturnUrl", "https://shop.example/s/abc/order/DH1?t=x&y=1")
	p.Set("vnp_TxnRef", "DH26112712345")

	data := vnpHashData(p)
	if !strings.HasPrefix(data, "vnp_Amount=1000000&vnp_OrderInfo=Thanh+toan+don+hang") {
		t.Fatalf("hash data không đúng thứ tự / mã hoá: %s", data)
	}

	// Giả lập VNPay gửi lại: tham số đã được giải mã trong query, kèm chữ ký
	signed := p.Encode() + "&vnp_SecureHash=" + vnpSign(secret, data)
	back, err := url.ParseQuery(signed)
	if err != nil {
		t.Fatal(err)
	}
	if !vnpVerify(secret, back) {
		t.Fatal("chữ ký hợp lệ nhưng xác minh thất bại")
	}
	back.Set("vnp_Amount", "2000000")
	if vnpVerify(secret, back) {
		t.Fatal("sửa số tiền mà chữ ký vẫn hợp lệ")
	}
	if vnpVerify("WRONG", back) {
		t.Fatal("sai khoá mà vẫn hợp lệ")
	}
}

func TestShippingFor(t *testing.T) {
	ps := ShopPaymentSettings{ShippingFee: 30000, FreeShipFrom: 500000}
	if got := shippingFor(ps, 100000); got != 30000 {
		t.Fatalf("phí ship = %d", got)
	}
	if got := shippingFor(ps, 500000); got != 0 {
		t.Fatalf("đủ miễn phí ship mà vẫn tính %d", got)
	}
}

func TestWellnessTemplatePassesSanitizer(t *testing.T) {
	raw, tpl, ok := templateBlocksJSON("wellness")
	if !ok {
		t.Fatal("mẫu wellness không qua bộ lọc")
	}
	var blocks []StorefrontBlock
	if err := json.Unmarshal([]byte(raw), &blocks); err != nil {
		t.Fatal(err)
	}
	if len(blocks) < 10 || len(tpl.Products) == 0 {
		t.Fatalf("mẫu thiếu section/sản phẩm: %d section", len(blocks))
	}
	if _, _, ok := templateBlocksJSON("khong-ton-tai"); ok {
		t.Fatal("mẫu không tồn tại mà vẫn trả về")
	}
}

func TestSanitizerStripsDangerousLinksInNewBlocks(t *testing.T) {
	in := `[{"id":"f","type":"footer","enabled":true,"data":{"columns":[{"title":"A","links":[{"label":"x","link":"javascript:alert(1)"}]}],"socials":{"facebook_link":"javascript:x"}}}]`
	out, err := sanitizeBlocksJSON(in)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "javascript:") {
		t.Fatalf("còn link javascript: %s", out)
	}
}
