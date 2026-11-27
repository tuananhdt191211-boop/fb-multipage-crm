package handlers

import (
	"testing"

	"fb-multipage-crm/internal/config"
	"fb-multipage-crm/internal/models"
)

func TestCleanDomainAndApex(t *testing.T) {
	cases := map[string]string{
		"https://Www.Shop.VN/abc?x=1": "www.shop.vn",
		"  shop.com.vn. ":             "shop.com.vn",
		"shop.vn:443":                 "shop.vn",
	}
	for in, want := range cases {
		if got := cleanDomain(in); got != want {
			t.Errorf("cleanDomain(%q)=%q want %q", in, got, want)
		}
	}
	for _, bad := range []string{"shop", "-a.com", "a..com", "a b.com", "ex_ample.com"} {
		if hostnameRe.MatchString(cleanDomain(bad)) {
			t.Errorf("%q should be invalid", bad)
		}
	}
	if !isApexDomain("shop.vn") || !isApexDomain("shop.com.vn") || isApexDomain("www.shop.vn") || isApexDomain("www.shop.com.vn") {
		t.Error("isApexDomain wrong")
	}
}

func TestSystemHostAndReturnURL(t *testing.T) {
	h := &StorefrontHandler{Cfg: &config.Config{FrontendURL: "https://fb-multipage-crm.pages.dev"}}
	if !h.isSystemHost("fb-multipage-crm.pages.dev") || !h.isSystemHost("x.onrender.com") || h.isSystemHost("www.shop.vn") {
		t.Error("isSystemHost wrong")
	}
	legacy := &models.Storefront{Slug: "a", Domain: "fb-multipage-crm.pages.dev"}
	if h.hasCustomDomain(legacy) {
		t.Error("legacy system domain must not count as custom")
	}
	sf := &models.Storefront{Slug: "hamax", Domain: "www.shop.vn"}
	if got := h.orderReturnURL("https://www.shop.vn", sf, "C1", "tk"); got != "https://www.shop.vn/order/C1?t=tk" {
		t.Errorf("custom return url %q", got)
	}
	if got := h.orderReturnURL("https://fb-multipage-crm.pages.dev", sf, "C1", "tk"); got != "https://fb-multipage-crm.pages.dev/s/hamax/order/C1?t=tk" {
		t.Errorf("system return url %q", got)
	}
}
