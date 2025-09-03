package database

import (
	"fmt"
	"time"

	"github.com/weiwangfds/scinote/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Init 初始化数据库连接
func Init(cfg config.DatabaseConfig) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch cfg.Driver {
	case "sqlite":
		// 启用WAL模式和其他SQLite优化选项
		dsn := cfg.DSN + "?_journal_mode=WAL&_synchronous=NORMAL&_cache_size=1000&_timeout=5000&_busy_timeout=5000"
		dialector = sqlite.Open(dsn)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// 对于SQLite，限制并发连接数以避免锁定问题
	if cfg.Driver == "sqlite" {
		sqlDB.SetMaxIdleConns(1)
		sqlDB.SetMaxOpenConns(1)
	} else {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	// 自动迁移表结构
	if err := autoMigrate(db); err != nil {
		return nil, fmt.Errorf("failed to auto migrate: %w", err)
	}

	return db, nil
}

// autoMigrate 自动迁移数据库表结构
func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&FileMetadata{},
		&OSSConfig{},
		&SyncLog{},
		&Note{},
		&Tag{},
		&NoteTag{},
		&NoteProperty{},
	)
}
