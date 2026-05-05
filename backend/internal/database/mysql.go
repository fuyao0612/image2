package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"image2/backend/internal/config"
	"image2/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Open 连接数据库。
// 生产环境使用 MySQL；本地开发未配置 MYSQL_DSN 时，自动使用 SQLite 兜底，方便先跑通完整闭环。
func Open(cfg config.Config) (*gorm.DB, error) {
	if strings.TrimSpace(cfg.MySQLDSN) == "" {
		return openSQLite(cfg.SQLitePath)
	}

	db, err := gorm.Open(mysql.Open(cfg.MySQLDSN), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("打开 MySQL 失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取数据库句柄失败: %w", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetConnMaxLifetime(time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("MySQL Ping 失败: %w", err)
	}

	return db, nil
}

func openSQLite(path string) (*gorm.DB, error) {
	if strings.TrimSpace(path) == "" {
		path = "data/app.db"
	}

	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("创建 SQLite 目录失败: %w", err)
		}
	}

	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite 失败: %w", err)
	}

	return db, nil
}

// AutoMigrate 统一迁移核心表结构。
func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	return db.AutoMigrate(
		&model.User{},
		&model.PointsLog{},
		&model.CreditLog{},
		&model.ImageTask{},
		&model.BillingPackage{},
		&model.RechargeOrder{},
		&model.ContentAuditLog{},
		&model.AdminActionLog{},
		&model.SystemSetting{},
	)
}
