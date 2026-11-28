package database

import (
	"fmt"
	"log"
	"strings"
	"time"

	"fb-multipage-crm/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TryConnect mở kết nối PostgreSQL và đồng bộ bảng users. Trả lỗi thay vì làm sập tiến trình.
func TryConnect(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true, // Tắt prepared statement cache cho Neon PgBouncer
	}), &gorm.Config{
		PrepareStmt: false,
		// Khi mở lỗi, GORM tự in cả chục dòng địa chỉ IP; ta tự giải thích bằng ExplainError nên tắt log lúc mở
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	db.Logger = logger.Default // bật lại log bình thường (cảnh báo truy vấn chậm, lỗi SQL)

	// Đóng kết nối rảnh sau 2 phút để Neon có thể tự ngủ khi không dùng (tiết kiệm giờ chạy gói miễn phí)
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxIdleConns(2)
		sqlDB.SetConnMaxIdleTime(2 * time.Minute)
		sqlDB.SetMaxOpenConns(10)
	}

	if err := db.AutoMigrate(&models.User{}); err != nil {
		return nil, fmt.Errorf("lỗi đồng bộ bảng users: %w", err)
	}
	return db, nil
}

// Connect giữ cho mã cũ: lỗi là thoát luôn. Máy chủ chính dùng ConnectWithRetry.
func Connect(dsn string) *gorm.DB {
	db, err := TryConnect(dsn)
	if err != nil {
		log.Fatalf("Không thể kết nối PostgreSQL.\n%s", ExplainError(err))
	}
	log.Println("Kết nối PostgreSQL và đồng bộ bảng thành công")
	return db
}

// ConnectWithRetry thử kết nối cho tới khi được (chờ 5s, 10s, 20s... tối đa 60s giữa các lần).
// onFail được gọi sau mỗi lần thất bại để tầng trên báo cho người dùng biết lý do.
func ConnectWithRetry(dsn string, onFail func(err error, attempt int)) *gorm.DB {
	delay := 5 * time.Second
	for attempt := 1; ; attempt++ {
		db, err := TryConnect(dsn)
		if err == nil {
			log.Println("Kết nối PostgreSQL và đồng bộ bảng thành công")
			return db
		}
		if onFail != nil {
			onFail(err, attempt)
		}
		// Hết hạn mức (53000) hay sai mật khẩu thì thử nhanh cũng vô ích -> chờ lâu hơn
		if IsQuotaError(err) || strings.Contains(err.Error(), "SQLSTATE 28P01") {
			time.Sleep(time.Minute)
			continue
		}
		time.Sleep(delay)
		if delay < time.Minute {
			delay *= 2
			if delay > time.Minute {
				delay = time.Minute
			}
		}
	}
}

// IsQuotaError: Neon báo dự án hết hạn mức (compute-hours / băng thông) -> SQLSTATE 53000.
func IsQuotaError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "SQLSTATE 53000") || strings.Contains(s, "has exceeded the")
}

// ExplainError diễn giải lỗi kết nối bằng tiếng Việt, kèm cách xử lý.
func ExplainError(err error) string {
	if err == nil {
		return ""
	}
	raw := err.Error()
	// Chỉ giữ dòng lỗi đầu tiên cho gọn (pgx liệt kê lỗi cho từng địa chỉ IP)
	short := raw
	if i := strings.Index(short, "\n"); i > 0 {
		short = short[:i]
	}
	if len(short) > 300 {
		short = short[:300] + "…"
	}
	var b strings.Builder
	switch {
	case IsQuotaError(err):
		b.WriteString("→ Neon đã TẠM KHOÁ database vì dự án hết hạn mức gói miễn phí (SQLSTATE 53000). Đây KHÔNG phải lỗi code.\n")
		b.WriteString("  1) Vào https://console.neon.tech → chọn dự án → Billing / Usage: xem hạn mức nào đã hết\n")
		b.WriteString("     (giờ chạy compute 100 CU-giờ/tháng hay băng thông 5 GB/tháng) và ngày được reset.\n")
		b.WriteString("  2) Cách mở lại: đợi kỳ thanh toán mới, HOẶC nâng cấp gói Launch (trả theo mức dùng, không có mức tối thiểu).\n")
		b.WriteString("  3) Trong lúc chờ, chạy database ngay trên máy: `docker compose up -d postgres` (ở thư mục gốc dự án),\n")
		b.WriteString("     rồi tạo file backend-go/.env.local gồm 1 dòng:\n")
		b.WriteString("     DATABASE_URL=postgres://fbcrm:fbcrm_secret_password@localhost:5432/fb_multipage_crm?sslmode=disable\n")
	case strings.Contains(raw, "Tenant or user not found"):
		// Lỗi của pooler Supabase (Supavisor)
		b.WriteString("→ Pooler Supabase không nhận ra tài khoản. Hai nguyên nhân thường gặp:\n")
		b.WriteString("  1) Với pooler, tên đăng nhập phải có dạng postgres.<mã-dự-án> (không phải chỉ \"postgres\").\n")
		b.WriteString("     Hãy copy lại nguyên chuỗi \"Session pooler\" trong Supabase Dashboard → nút Connect.\n")
		b.WriteString("  2) Dự án gói miễn phí tự TẠM DỪNG sau 1 tuần không hoạt động → vào Dashboard bấm Restore project.\n")
	case strings.Contains(raw, "network is unreachable") || strings.Contains(raw, "no route to host"):
		b.WriteString("→ Máy chủ này không có đường mạng tới địa chỉ database (thường do địa chỉ chỉ có IPv6).\n")
		b.WriteString("  Supabase: chuỗi \"Direct connection\" (db.<mã>.supabase.co) chỉ có IPv6, còn Render không có IPv6 ra ngoài.\n")
		b.WriteString("  Hãy dùng chuỗi \"Session pooler\" (host *.pooler.supabase.com, cổng 5432) — chạy được trên IPv4.\n")
	case strings.Contains(raw, "SQLSTATE 28P01"):
		b.WriteString("→ Sai tên đăng nhập / mật khẩu database. Kiểm tra lại DATABASE_URL.\n")
		b.WriteString("  (Supabase: mật khẩu đặt khi tạo dự án; quên thì đặt lại ở Dashboard → Project Settings → Database.)\n")
	case strings.Contains(raw, "refused") || strings.Contains(raw, "actively refused"):
		b.WriteString("→ Không có PostgreSQL nào đang nghe ở địa chỉ trong DATABASE_URL.\n")
		b.WriteString("  Chạy database cục bộ: `docker compose up -d postgres` (cần mở Docker Desktop trước).\n")
	case strings.Contains(raw, "no such host") || strings.Contains(raw, "timeout") || strings.Contains(raw, "i/o timeout"):
		b.WriteString("→ Không tới được máy chủ database (mạng / DNS). Kiểm tra internet và địa chỉ trong DATABASE_URL.\n")
	default:
		b.WriteString("→ Lỗi kết nối database. Kiểm tra DATABASE_URL.\n")
	}
	b.WriteString("  Chi tiết: " + short)
	return b.String()
}
