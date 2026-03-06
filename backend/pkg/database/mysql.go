package database

import (
	"alpha-amm-engine/internal/dao"
	"alpha-amm-engine/pkg/logger"
	"alpha-amm-engine/pkg/models"
	"database/sql"
	"errors"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

func InitDatabase(cfg *models.MySQLConfig, debug bool) (db *gorm.DB, err error) {
	if cfg.Uri == "" {
		err = errors.New("mysql uri is empty")
		return
	}

	db, err = gorm.Open(mysql.Open(cfg.Uri),
		&gorm.Config{
			NamingStrategy: schema.NamingStrategy{
				SingularTable: true,
			},
			Logger: gormLogger.New(logger.Log, gormLogger.Config{
				SlowThreshold:             200 * time.Millisecond,
				IgnoreRecordNotFoundError: true, // 忽略记录不存在的错误
				Colorful:                  true,
				LogLevel:                  gormLogger.Error,
			}),
		})
	if err != nil {
		return
	}
	var sqlDB *sql.DB
	sqlDB, err = db.DB()
	if err != nil || sqlDB == nil {
		return
	}

	sqlDB.SetMaxIdleConns(cfg.IdlePoolSize)
	sqlDB.SetMaxOpenConns(cfg.MaxPoolSize)
	sqlDB.SetConnMaxIdleTime(time.Duration(cfg.IdleTimeout) * time.Millisecond)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.MaxLifetime) * time.Millisecond)

	if debug {
		dao.SetDefault(db.Debug())
	} else {
		dao.SetDefault(db)
	}

	return
}
