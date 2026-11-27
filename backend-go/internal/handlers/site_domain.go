package handlers

// Gắn tên miền riêng (của khách) cho website bán hàng.
//
// Frontend chạy trên Cloudflare Pages, nên mỗi tên miền riêng được thêm vào Pages project
// qua Cloudflare API (Cloudflare tự cấp SSL). Khách chỉ cần trỏ CNAME về <project>.pages.dev.
// Biến môi trường:
//   CF_API_TOKEN      token có quyền "Cloudflare Pages: Edit"
//   CF_ACCOUNT_ID     ID tài khoản Cloudflare
//   CF_PAGES_PROJECT  tên Pages project (mặc định fb-multipage-crm)
// Không cấu hình -> vẫn lưu tên miền (trạng thái "manual"), quản trị viên tự thêm trong Cloudflare.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"fb-multipage-crm/internal/models"

	"github.com/gin-gonic/gin"
)

var hostnameRe = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+(xn--[a-z0-9-]{1,59}|[a-z]{2,63})$`)

// Hậu tố 2 cấp phổ biến (ví dụ shop.com.vn là tên miền gốc, không phải tên miền con)
var twoLevelSuffixes = map[string]bool{
	"com.vn": true, "net.vn": true, "org.vn": true, "edu.vn": true, "gov.vn": true, "info.vn": true,
	"biz.vn": true, "name.vn": true, "pro.vn": true, "health.vn": true, "ac.vn": true, "int.vn": true,
	"co.uk": true, "org.uk": true, "com.au": true, "net.au": true, "co.jp": true, "com.sg": true,
}

// cleanDomain chuẩn hoá chuỗi người dùng nhập ("https://Www.Shop.vn/abc" -> "www.shop.vn")
func cleanDomain(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if strings.Contains(s, "://") {
		if u, err := url.Parse(s); err == nil {
			s = u.Host
		}
	}
	if i := strings.IndexAny(s, "/?#"); i != -1 {
		s = s[:i]
	}
	s = strings.TrimSuffix(normalizeHost(s), ".")
	return s
}

func isApexDomain(d string) bool {
	parts := strings.Split(d, ".")
	if len(parts) <= 2 {
		return true
	}
	return len(parts) == 3 && twoLevelSuffixes[parts[1]+"."+parts[2]]
}

func cfPagesProject() string {
	if p := strings.TrimSpace(os.Getenv("CF_PAGES_PROJECT")); p != "" {
		return p
	}
	return "fb-multipage-crm"
}

func cfConfigured() bool {
	return strings.TrimSpace(os.Getenv("CF_API_TOKEN")) != "" && strings.TrimSpace(os.Getenv("CF_ACCOUNT_ID")) != ""
}

// cnameTarget: địa chỉ khách cần trỏ CNAME tới
func (h *StorefrontHandler) cnameTarget() string {
	if t := strings.TrimSpace(os.Getenv("PAGES_CNAME_TARGET")); t != "" {
		return t
	}
	if h.Cfg != nil {
		if u, err := url.Parse(strings.TrimSpace(h.Cfg.FrontendURL)); err == nil && strings.HasSuffix(u.Host, ".pages.dev") {
			return u.Host
		}
	}
	return cfPagesProject() + ".pages.dev"
}

// isSystemHost: tên miền của chính hệ thống, không cho gắn làm tên miền riêng
func (h *StorefrontHandler) isSystemHost(d string) bool {
	if d == "localhost" || strings.HasSuffix(d, ".pages.dev") || strings.HasSuffix(d, ".onrender.com") ||
		strings.HasSuffix(d, ".workers.dev") || d == normalizeHost(defaultDomainFromConfig(h.Cfg, "")) {
		return true
	}
	return false
}

// hasCustomDomain: site có tên miền riêng thật (bỏ qua giá trị cũ là domain hệ thống)
func (h *StorefrontHandler) hasCustomDomain(sf *models.Storefront) bool {
	d := normalizeHost(sf.Domain)
	return d != "" && !h.isSystemHost(d)
}

// ----------------------------------------------------------------- Cloudflare Pages API

type cfDomainResult struct {
	Name             string `json:"name"`
	Status           string `json:"status"` // initializing | pending | active | deactivated | blocked | error
	VerificationData struct {
		Status       string `json:"status"`
		ErrorMessage string `json:"error_message"`
	} `json:"verification_data"`
	ValidationData struct {
		Status       string `json:"status"`
		Method       string `json:"method"`
		ErrorMessage string `json:"error_message"`
		TxtName      string `json:"txt_name"`
		TxtValue     string `json:"txt_value"`
	} `json:"validation_data"`
}

var errCFNotFound = errors.New("cloudflare: domain not found")

func cfCall(method, path string, body any) (*cfDomainResult, error) {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	u := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/pages/projects/%s/domains%s",
		url.PathEscape(strings.TrimSpace(os.Getenv("CF_ACCOUNT_ID"))), url.PathEscape(cfPagesProject()), path)
	req, err := http.NewRequest(method, u, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(os.Getenv("CF_API_TOKEN")))
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("không kết nối được Cloudflare: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var env struct {
		Success bool `json:"success"`
		Errors  []struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
		Result json.RawMessage `json:"result"`
	}
	_ = json.Unmarshal(raw, &env)
	if resp.StatusCode == http.StatusNotFound {
		return nil, errCFNotFound
	}
	if !env.Success {
		msg := fmt.Sprintf("HTTP %d", resp.StatusCode)
		if len(env.Errors) > 0 {
			msg = env.Errors[0].Message
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			msg = "Token Cloudflare không hợp lệ hoặc thiếu quyền Cloudflare Pages: Edit (" + msg + ")"
		}
		return nil, errors.New(msg)
	}
	var r cfDomainResult
	if len(env.Result) > 0 && string(env.Result) != "null" {
		_ = json.Unmarshal(env.Result, &r)
	}
	return &r, nil
}

func cfAddDomain(d string) (*cfDomainResult, error) {
	r, err := cfCall(http.MethodPost, "", map[string]string{"name": d})
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "already") {
		// đã có trong project (ví dụ gắn lại) -> đọc trạng thái hiện tại
		return cfCall(http.MethodGet, "/"+url.PathEscape(d), nil)
	}
	return r, err
}

func cfGetDomain(d string) (*cfDomainResult, error) {
	return cfCall(http.MethodGet, "/"+url.PathEscape(d), nil)
}

func cfRemoveDomain(d string) error {
	_, err := cfCall(http.MethodDelete, "/"+url.PathEscape(d), nil)
	if errors.Is(err, errCFNotFound) {
		return nil
	}
	return err
}

func mapCFStatus(r *cfDomainResult) (status, errMsg string) {
	switch r.Status {
	case "active":
		return "active", ""
	case "error", "blocked", "deactivated":
		msg := r.ValidationData.ErrorMessage
		if msg == "" {
			msg = r.VerificationData.ErrorMessage
		}
		if msg == "" {
			msg = "Cloudflare báo trạng thái: " + r.Status
		}
		return "error", msg
	default:
		return "pending", r.VerificationData.ErrorMessage
	}
}

// ----------------------------------------------------------------- HTTP handlers

func (h *StorefrontHandler) domainInfo(sf *models.Storefront, cf *cfDomainResult) gin.H {
	out := gin.H{
		"domain":        "",
		"status":        "",
		"error":         "",
		"cname_target":  h.cnameTarget(),
		"cf_configured": cfConfigured(),
	}
	if !h.hasCustomDomain(sf) {
		return out
	}
	d := normalizeHost(sf.Domain)
	out["domain"] = d
	out["status"] = sf.DomainStatus
	out["error"] = sf.DomainError
	out["is_apex"] = isApexDomain(d)
	labels := strings.Split(d, ".")
	out["host_label"] = labels[0]
	if cf != nil && cf.ValidationData.TxtName != "" && cf.ValidationData.Status != "active" {
		out["txt_name"] = cf.ValidationData.TxtName
		out["txt_value"] = cf.ValidationData.TxtValue
	}
	return out
}

// GetSiteDomain GET /api/v1/storefront/sites/:id/domain — đọc & làm mới trạng thái
func (h *StorefrontHandler) GetSiteDomain(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	var cf *cfDomainResult
	if h.hasCustomDomain(sf) && cfConfigured() && sf.DomainStatus != "active" {
		if r, err := cfGetDomain(normalizeHost(sf.Domain)); err == nil {
			cf = r
			sf.DomainStatus, sf.DomainError = mapCFStatus(r)
		} else if errors.Is(err, errCFNotFound) {
			// chưa có trong Pages (ví dụ trước đó lưu thủ công) -> thêm lại
			if r, err := cfAddDomain(normalizeHost(sf.Domain)); err == nil {
				cf = r
				sf.DomainStatus, sf.DomainError = mapCFStatus(r)
			} else {
				sf.DomainStatus, sf.DomainError = "error", err.Error()
			}
		} else {
			sf.DomainError = err.Error()
		}
		h.DB.Model(sf).Updates(map[string]any{"domain_status": sf.DomainStatus, "domain_error": sf.DomainError})
	}
	c.JSON(http.StatusOK, h.domainInfo(sf, cf))
}

// SetSiteDomain PUT /api/v1/storefront/sites/:id/domain {domain}
func (h *StorefrontHandler) SetSiteDomain(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	var req struct {
		Domain string `json:"domain"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ."})
		return
	}
	d := cleanDomain(req.Domain)
	if len(d) > 253 || !hostnameRe.MatchString(d) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tên miền không hợp lệ. Ví dụ đúng: www.cuahangcuaban.vn"})
		return
	}
	if h.isSystemHost(d) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Không thể dùng tên miền của hệ thống. Hãy nhập tên miền bạn đã mua."})
		return
	}
	var cnt int64
	h.DB.Model(&models.Storefront{}).Where("LOWER(domain) = ? AND id <> ?", d, sf.ID).Count(&cnt)
	if cnt > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Tên miền này đang được gắn cho một website khác."})
		return
	}

	old := ""
	if h.hasCustomDomain(sf) {
		old = normalizeHost(sf.Domain)
	}
	if old == d && sf.DomainStatus == "active" {
		c.JSON(http.StatusOK, h.domainInfo(sf, nil))
		return
	}

	var cf *cfDomainResult
	status, errMsg := "manual", ""
	if cfConfigured() {
		r, err := cfAddDomain(d)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "Cloudflare từ chối thêm tên miền: " + err.Error()})
			return
		}
		cf = r
		status, errMsg = mapCFStatus(r)
	}
	if old != "" && old != d && cfConfigured() {
		_ = cfRemoveDomain(old)
	}
	sf.Domain, sf.DomainStatus, sf.DomainError = d, status, errMsg
	if err := h.DB.Model(sf).Updates(map[string]any{"domain": d, "domain_status": status, "domain_error": errMsg}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không lưu được tên miền."})
		return
	}
	c.JSON(http.StatusOK, h.domainInfo(sf, cf))
}

// RemoveSiteDomain DELETE /api/v1/storefront/sites/:id/domain
func (h *StorefrontHandler) RemoveSiteDomain(c *gin.Context) {
	sf, ok := h.loadOwned(c)
	if !ok {
		return
	}
	if h.hasCustomDomain(sf) && cfConfigured() {
		if err := cfRemoveDomain(normalizeHost(sf.Domain)); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "Không gỡ được tên miền trên Cloudflare: " + err.Error()})
			return
		}
	}
	sf.Domain, sf.DomainStatus, sf.DomainError = "", "", ""
	h.DB.Model(sf).Updates(map[string]any{"domain": "", "domain_status": "", "domain_error": ""})
	c.JSON(http.StatusOK, h.domainInfo(sf, nil))
}

// releaseDomainOnDelete: gọi khi xoá website để trả tên miền khỏi Pages project
func (h *StorefrontHandler) releaseDomainOnDelete(sf *models.Storefront) {
	if h.hasCustomDomain(sf) && cfConfigured() {
		_ = cfRemoveDomain(normalizeHost(sf.Domain))
	}
}

// orderReturnURL: địa chỉ trang đơn hàng sau khi thanh toán (tên miền riêng dùng đường dẫn ngắn)
func (h *StorefrontHandler) orderReturnURL(base string, sf *models.Storefront, code, token string) string {
	if u, err := url.Parse(base); err == nil && h.hasCustomDomain(sf) && normalizeHost(u.Host) == normalizeHost(sf.Domain) {
		return fmt.Sprintf("%s/order/%s?t=%s", base, code, url.QueryEscape(token))
	}
	return fmt.Sprintf("%s/s/%s/order/%s?t=%s", base, sf.Slug, code, url.QueryEscape(token))
}
