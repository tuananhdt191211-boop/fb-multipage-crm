package services

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

type BroadcastWorker struct {
	db         *sql.DB
	httpClient *http.Client
	vnLocation *time.Location
}

func NewBroadcastWorker(db *sql.DB) *BroadcastWorker {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.FixedZone("ICT", 7*3600)
	}
	return &BroadcastWorker{
		db:         db,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		vnLocation: loc,
	}
}

// Start chạy worker nền xử lý hàng đợi tin nhắn
func (w *BroadcastWorker) Start(ctx context.Context) {
	go func() {
		log.Println("[BroadcastWorker] Started anti-spam message worker")
		sentInCurrentBatch := 0

		for {
			select {
			case <-ctx.Done():
				log.Println("[BroadcastWorker] Stopped")
				return
			default:
			}

			// 1. Chỉ gửi trong khung giờ 08:30 - 21:00 giờ Việt Nam
			if !w.isWithinAllowedHours() {
				time.Sleep(1 * time.Minute)
				continue
			}

			// 2. Lấy 1 tin nhắn pending của chiến dịch đang running
			processed, minDelay, maxDelay, batchSize, batchPause := w.processNextMessage(ctx)
			if !processed {
				sentInCurrentBatch = 0
				time.Sleep(5 * time.Second)
				continue
			}

			sentInCurrentBatch++

			// 3. Nghỉ dài sau mỗi lô (batch) để tránh bị Meta đánh dấu hành vi bot
			if batchSize > 0 && sentInCurrentBatch >= batchSize {
				sentInCurrentBatch = 0
				pause := time.Duration(maxInt(batchPause, 30)) * time.Second
				log.Printf("[BroadcastWorker] Batch limit reached, pausing %v", pause)
				time.Sleep(pause)
				continue
			}

			// 4. Độ trễ ngẫu nhiên (Jitter) giữa từng tin nhắn
			delay := randomSeconds(maxInt(minDelay, 3), maxInt(maxDelay, 6))
			time.Sleep(delay)
		}
	}()
}

func (w *BroadcastWorker) isWithinAllowedHours() bool {
	now := time.Now().In(w.vnLocation)
	minutes := now.Hour()*60 + now.Minute()
	start := 8*60 + 30 // 08:30
	end := 21*60 + 0   // 21:00
	return minutes >= start && minutes <= end
}

type queuedMessage struct {
	ID            string
	CampaignID    string
	PageID        string
	RecipientPSID string
	RenderedText  string
	MessagingType string
	PageToken     string
	MinDelaySec   int
	MaxDelaySec   int
	BatchSize     int
	BatchPauseSec int
}

func (w *BroadcastWorker) processNextMessage(ctx context.Context) (bool, int, int, int, int) {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return false, 0, 0, 0, 0
	}
	defer tx.Rollback()

	var msg queuedMessage
	// Khóa hàng đợi an toàn với FOR UPDATE SKIP LOCKED
	query := `
		SELECT
			m.id, m.campaign_id, m.page_id, m.recipient_psid, m.rendered_text,
			c.messaging_type, p.access_token,
			c.min_delay_sec, c.max_delay_sec, c.batch_size, c.batch_pause_sec
		FROM broadcast_messages m
		JOIN broadcast_campaigns c ON c.id = m.campaign_id
		JOIN pages p ON p.page_id = m.page_id
		WHERE m.status = 'pending'
		  AND m.scheduled_at <= NOW()
		  AND c.status = 'running'
		ORDER BY m.scheduled_at ASC
		LIMIT 1
		FOR UPDATE OF m SKIP LOCKED
	`
	err = tx.QueryRowContext(ctx, query).Scan(
		&msg.ID, &msg.CampaignID, &msg.PageID, &msg.RecipientPSID, &msg.RenderedText,
		&msg.MessagingType, &msg.PageToken,
		&msg.MinDelaySec, &msg.MaxDelaySec, &msg.BatchSize, &msg.BatchPauseSec,
	)
	if err == sql.ErrNoRows {
		return false, 0, 0, 0, 0
	}
	if err != nil {
		log.Printf("[BroadcastWorker] Query error: %v", err)
		return false, 0, 0, 0, 0
	}

	if _, err := tx.ExecContext(ctx, `UPDATE broadcast_messages SET status = 'processing' WHERE id = $1`, msg.ID); err != nil {
		return false, 0, 0, 0, 0
	}
	if err := tx.Commit(); err != nil {
		return false, 0, 0, 0, 0
	}

	// Gọi Facebook Graph API gửi tin
	fbMsgID, fbErrCode, fbErrMsg := w.sendFacebookMessage(ctx, msg.PageToken, msg.RecipientPSID, msg.RenderedText, msg.MessagingType)

	if fbErrCode == 0 && fbErrMsg == "" {
		_, _ = w.db.ExecContext(ctx, `
			UPDATE broadcast_messages
			SET status = 'sent', fb_message_id = $1, sent_at = NOW()
			WHERE id = $2
		`, fbMsgID, msg.ID)

		_, _ = w.db.ExecContext(ctx, `
			UPDATE broadcast_campaigns
			SET sent_count = sent_count + 1, updated_at = NOW()
			WHERE id = $1
		`, msg.CampaignID)

		_, _ = w.db.ExecContext(ctx, `
			UPDATE conversations
			SET last_auto_sent_at = NOW()
			WHERE page_id = $1 AND customer_psid = $2
		`, msg.PageID, msg.RecipientPSID)
	} else {
		_, _ = w.db.ExecContext(ctx, `
			UPDATE broadcast_messages
			SET status = 'failed', error_code = $1, error_message = $2
			WHERE id = $3
		`, fbErrCode, fbErrMsg, msg.ID)

		_, _ = w.db.ExecContext(ctx, `
			UPDATE broadcast_campaigns
			SET failed_count = failed_count + 1, updated_at = NOW()
			WHERE id = $1
		`, msg.CampaignID)

		// Circuit Breaker: Dừng ngay chiến dịch nếu dính rate-limit hoặc cảnh báo chính sách của Meta
		if isCriticalSpamOrRateLimitError(fbErrCode) {
			reason := fmt.Sprintf("Tự động tạm dừng để bảo vệ Page (FB Error %d: %s)", fbErrCode, fbErrMsg)
			_, _ = w.db.ExecContext(ctx, `
				UPDATE broadcast_campaigns
				SET status = 'paused', pause_reason = $1, updated_at = NOW()
				WHERE id = $2
			`, reason, msg.CampaignID)
			log.Printf("[BroadcastWorker] Circuit breaker triggered for campaign %s: %s", msg.CampaignID, reason)
		}
	}

	// Kiểm tra xem chiến dịch đã gửi hết toàn bộ tin chưa
	_, _ = w.db.ExecContext(ctx, `
		UPDATE broadcast_campaigns c
		SET status = 'completed', updated_at = NOW()
		WHERE c.id = $1
		  AND c.status = 'running'
		  AND NOT EXISTS (
			  SELECT 1 FROM broadcast_messages m
			  WHERE m.campaign_id = c.id AND m.status IN ('pending', 'processing')
		  )
	`, msg.CampaignID)

	return true, msg.MinDelaySec, msg.MaxDelaySec, msg.BatchSize, msg.BatchPauseSec
}

func (w *BroadcastWorker) sendFacebookMessage(ctx context.Context, pageToken, psid, text, messagingType string) (string, int, string) {
	url := fmt.Sprintf("https://graph.facebook.com/v21.0/me/messages?access_token=%s", pageToken)

	// Kèm nút Quick Reply để khách dễ phản hồi hoặc bấm hủy nhận tin
	payload := map[string]any{
		"messaging_type": messagingType,
		"recipient": map[string]string{
			"id": psid,
		},
		"message": map[string]any{
			"text": text,
			"quick_replies": []map[string]string{
				{
					"content_type": "text",
					"title":        "Tư vấn thêm",
					"payload":      "REENGAGE_INTERESTED",
				},
				{
					"content_type": "text",
					"title":        "Ngừng nhận tin",
					"payload":      "OPT_OUT_BROADCAST",
				},
			},
		},
	}

	bodyBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", -1, err.Error()
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return "", -1, err.Error()
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var okRes struct {
			MessageID string `json:"message_id"`
		}
		_ = json.Unmarshal(respBytes, &okRes)
		return okRes.MessageID, 0, ""
	}

	var errRes struct {
		Error struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(respBytes, &errRes)
	if errRes.Error.Code == 0 {
		errRes.Error.Code = resp.StatusCode
	}
	if errRes.Error.Message == "" {
		errRes.Error.Message = string(respBytes)
	}
	return "", errRes.Error.Code, errRes.Error.Message
}

func isCriticalSpamOrRateLimitError(code int) bool {
	switch code {
	case 4, 17, 32, 613, 2022, 10, 200:
		// 4, 17, 32, 613: App/Page rate limit
		// 2022 / 10 / 200: Chính sách nhắn tin hoặc chặn quyền gửi ngoài 24h
		return true
	default:
		return false
	}
}

// RenderPersonalizedMessage chọn ngẫu nhiên 1 mẫu câu và thay biến {{full_name}}
func RenderPersonalizedMessage(templates []string, fullName string) string {
	if len(templates) == 0 {
		return ""
	}
	tpl := templates[rand.Intn(len(templates))]
	name := strings.TrimSpace(fullName)
	if name == "" {
		name = "bạn"
	}
	return strings.ReplaceAll(tpl, "{{full_name}}", name)
}

func randomSeconds(minSec, maxSec int) time.Duration {
	if maxSec <= minSec {
		return time.Duration(minSec) * time.Second
	}
	return time.Duration(minSec+rand.Intn(maxSec-minSec+1)) * time.Second
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
