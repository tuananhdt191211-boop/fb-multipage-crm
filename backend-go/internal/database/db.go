package database

import (
    "log"

    "fb-multipage-crm/internal/models"

    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

func Connect(dsn string) *gorm.DB {
    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
    if err != nil {
        log.Fatalf("Không thể kết nối PostgreSQL: %v", err)
    }

    if err := db.AutoMigrate(&models.User{}); err != nil {
        log.Fatalf("Lỗi đồng bộ bảng users: %v", err)
    }

    log.Println("Kết nối PostgreSQL và đồng bộ bảng thành công")
    return db
}