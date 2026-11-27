package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"fb-multipage-crm/internal/models"
	"fb-multipage-crm/internal/security"

	"github.com/gin-gonic/gin"
)

// DiagnosePage GET /api/v1/broadcasts/diagnose?page_id=...
// Chẩn đoán: Meta trả gì cho Page này, vì sao không lấy được PSID khách.
// Không bao giờ in token ra kết quả.
func (h *BroadcastHandler) DiagnosePage(c *gin.Context) {
	pageID := strings.TrimSpace(c.Query("page_id"))
	if pageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Thiếu page_id"})
		return
	}
	var page models.FacebookPage
	if err := h.DB.Where("page_id = ?", pageID).First(&page).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"buoc": "tìm page", "loi": "Không có Page này trong hệ thống"})
		return
	}
	token, err := security.DecryptSecret(page.EncryptedPageToken, h.getEncryptionKey())
	if err != nil || token == "" {
		c.JSON(http.StatusOK, gin.H{"buoc": "giải mã token", "loi": "Không giải mã được Page Token, hãy kết nối lại Page"})
		return
	}

	var inDB, withPSID int64
	h.DB.Model(&models.Conversation{}).Where("page_id = ?", pageID).Count(&inDB)
	h.DB.Model(&models.Conversation{}).Where("page_id = ? AND COALESCE(TRIM(customer_psid),'') <> ''", pageID).Count(&withPSID)

	out := gin.H{
		"page":               page.Name,
		"hoi_thoai_trong_db": inDB,
		"hoi_thoai_co_psid":  withPSID,
		"cac_cach_goi_meta":  []gin.H{},
	}
	var tries []gin.H

	for _, variant := range []struct{ name, query string }{
		{"messenger", "platform=messenger&fields=id,updated_time,participants,senders&limit=5"},
		{"mac_dinh", "fields=id,updated_time,participants,senders,messages.limit(3){from}&limit=5"},
	} {
		u := fmt.Sprintf("https://graph.facebook.com/v21.0/%s/conversations?%s&access_token=%s",
			url.PathEscape(pageID), variant.query, url.QueryEscape(token))
		r, reqErr := h.HTTPClient.Get(u)
		if reqErr != nil {
			tries = append(tries, gin.H{"cach": variant.name, "loi": "Lỗi mạng khi gọi Meta"})
			continue
		}
		raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		r.Body.Close()

		var parsed struct {
			Data []struct {
				ID           string `json:"id"`
				Participants struct {
					Data []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"data"`
				} `json:"participants"`
				Senders struct {
					Data []struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"senders"`
			} `json:"data"`
			Error *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(raw, &parsed)

		item := gin.H{"cach": variant.name, "http_status": r.StatusCode}
		if parsed.Error != nil {
			item["meta_bao_loi"] = fmt.Sprintf("code %d: %s", parsed.Error.Code, parsed.Error.Message)
		} else {
			noPart := 0
			for _, t := range parsed.Data {
				if len(t.Participants.Data) == 0 && len(t.Senders.Data) == 0 {
					noPart++
				}
			}
			item["so_hoi_thoai_mau"] = len(parsed.Data)
			item["so_hoi_thoai_khong_co_nguoi_tham_gia"] = noPart
			if len(parsed.Data) > 0 {
				first := parsed.Data[0]
				item["mau_dau_tien"] = gin.H{
					"so_participants": len(first.Participants.Data),
					"so_senders":      len(first.Senders.Data),
				}
			}
		}
		tries = append(tries, item)
	}
	out["cac_cach_goi_meta"] = tries
	c.JSON(http.StatusOK, out)
}
