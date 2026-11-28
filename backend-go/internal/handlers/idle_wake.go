package handlers

// Tiết kiệm database (Neon): Neon chỉ tự "ngủ" (không tính giờ chạy) khi KHÔNG có truy vấn nào trong ~5 phút.
// Các vòng lặp nền trước đây hỏi database mỗi 5–30 giây suốt 24/24 nên database không bao giờ ngủ,
// làm hết hạn mức gói miễn phí. Giờ: khi rảnh thì ngủ dài, khi có việc thì được đánh thức ngay.

import (
	"sync"
	"sync/atomic"
	"time"
)

// idleSleep: thời gian ngủ khi không có việc (dài hơn 5 phút để Neon kịp ngủ)
const idleSleep = 15 * time.Minute

type waker struct{ ch chan struct{} }

func newWaker() *waker { return &waker{ch: make(chan struct{}, 1)} }

// Wake đánh thức vòng lặp đang ngủ (không chặn; gọi nhiều lần cũng chỉ tính 1)
func (w *waker) Wake() {
	select {
	case w.ch <- struct{}{}:
	default:
	}
}

// Sleep ngủ tối đa d, hoặc dậy sớm khi có Wake()
func (w *waker) Sleep(d time.Duration) {
	if d <= 0 {
		return
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
	case <-w.ch:
	}
}

var (
	broadcastWake = newWaker() // có chiến dịch gửi tin mới / chạy lại
	schedulerWake = newWaker() // có bài hẹn giờ mới
)

// Thời điểm (unix giây) có người mở hộp thư / có tin nhắn webhook gần nhất
var (
	lastInboxUse   atomic.Int64
	lastWebhookMsg atomic.Int64
)

// Mỗi khách hàng (workspace) có mốc "đang xem hộp thư" riêng: đồng bộ nền chỉ chạy cho khách đang xem
var inboxUseByWS sync.Map // workspace id (int64) -> unix giây

func markInboxUse(ws int64) {
	now := time.Now().Unix()
	lastInboxUse.Store(now)
	inboxUseByWS.Store(ws, now)
}

func inboxActive(ws int64, d time.Duration) bool {
	v, ok := inboxUseByWS.Load(ws)
	if !ok {
		return false
	}
	return time.Since(time.Unix(v.(int64), 0)) < d
}

func markWebhookMsg() { lastWebhookMsg.Store(time.Now().Unix()) }

func within(ts *atomic.Int64, d time.Duration) bool {
	v := ts.Load()
	return v > 0 && time.Since(time.Unix(v, 0)) < d
}
