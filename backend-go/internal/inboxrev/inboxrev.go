// Package inboxrev giữ "số phiên bản" của hộp thư trong RAM.
//
// Vì sao cần: trình duyệt cần biết "có tin mới chưa?". Trước đây trang Hộp thư tải lại TOÀN BỘ
// danh sách hội thoại từ database mỗi 3 giây -> database Neon không bao giờ được ngủ và truyền
// ra hàng GB dữ liệu mỗi ngày, làm hết hạn mức gói miễn phí (lỗi SQLSTATE 53000).
//
// Giờ: mọi thay đổi ở bảng conversations / messages (webhook Facebook, đồng bộ, rep gửi tin,
// đổi trạng thái khách...) tự động tăng số phiên bản này (qua callback GORM). Trình duyệt chỉ hỏi
// số phiên bản — đọc từ RAM, KHÔNG chạm database — và chỉ tải danh sách khi số đó đổi.
package inboxrev

import (
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

var rev atomic.Int64

func init() {
	// Khởi động lại máy chủ -> số mới, lớn hơn mọi số cũ -> mọi tab tự tải lại một lần
	rev.Store(time.Now().UnixMilli())
}

// Current trả về số phiên bản hiện tại.
func Current() int64 { return rev.Load() }

// Bump báo "hộp thư vừa thay đổi".
func Bump() { rev.Add(1) }

func watched(table string) bool { return table == "conversations" || table == "messages" }

// Register gắn callback vào GORM: mọi INSERT / UPDATE / DELETE thành công trên bảng hộp thư
// (và thực sự thay đổi dòng nào đó) đều tăng số phiên bản.
func Register(db *gorm.DB) {
	hook := func(tx *gorm.DB) {
		if tx.Error != nil || tx.Statement == nil || tx.RowsAffected <= 0 || !watched(tx.Statement.Table) {
			return
		}
		Bump()
		// GORM bọc mỗi lệnh trong 1 transaction; callback chạy sau commit, nhưng nếu mã nguồn dùng
		// transaction lớn hơn thì dữ liệu có thể chưa hiện ngay -> tăng thêm lần nữa sau 0,4 giây.
		time.AfterFunc(400*time.Millisecond, Bump)
	}
	after := "gorm:commit_or_rollback_transaction"
	_ = db.Callback().Create().After(after).Register("inboxrev:create", hook)
	_ = db.Callback().Update().After(after).Register("inboxrev:update", hook)
	_ = db.Callback().Delete().After(after).Register("inboxrev:delete", hook)
}
