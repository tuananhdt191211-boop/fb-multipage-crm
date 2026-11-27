package database

import (
	"log"
	"time"

	"fb-multipage-crm/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(dsn string) *gorm.DB {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true, // Tắt prepared statement cache cho Neon PgBouncer
	}), &gorm.Config{
		PrepareStmt: false,
	})
	if err != nil {
		log.Fatalf("Không thể kết nối PostgreSQL: %v", err)
	}

	// Đóng kết nối rảnh sau 2 phút để Neon có thể tự ngủ khi không dùng (tiết kiệm giờ chạy gói miễn phí)
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxIdleConns(2)
		sqlDB.SetConnMaxIdleTime(2 * time.Minute)
		sqlDB.SetMaxOpenConns(10)
	}

	if err := db.AutoMigrate(&models.User{}); err != nil {
		log.Fatalf("Lỗi đồng bộ bảng users: %v", err)
	}

	log.Println("Kết nối PostgreSQL và đồng bộ bảng thành công")
	return db
}
