package handlers

import (
	"testing"

	"fb-multipage-crm/internal/config"
	"fb-multipage-crm/internal/utils"
)

// Token khách hàng KHÔNG được dùng làm token quản trị (khoá ký khác nhau)
func TestCustomerTokenIsNotAdminToken(t *testing.T) {
	h := &StorefrontHandler{Cfg: &config.Config{JWTSecret: "admin-secret"}}
	tok, err := h.issueCustomerToken(&SiteCustomer{ID: 7, StorefrontID: 3})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := utils.ParseJWT(tok, "admin-secret"); err == nil {
		t.Fatal("customer token accepted as admin JWT")
	}
	admin, _ := utils.GenerateJWT(1, "admin", "admin-secret")
	if string(h.customerKey()) == "admin-secret" || admin == tok {
		t.Fatal("keys must differ")
	}
}

func TestNormPhone(t *testing.T) {
	if got := normPhone(" 091.234-5678 "); got != "0912345678" {
		t.Fatalf("got %q", got)
	}
}
