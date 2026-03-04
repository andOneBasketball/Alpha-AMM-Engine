package database

import (
	"alpha-amm-engine/pkg/models"
	"database/sql"
	"errors"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

type ZapWriter struct {
}

func (l ZapWriter) Printf(s string, i ...interface{}) {
	log.Infof(s, i...)
}

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
			Logger: logger.New(ZapWriter{}, logger.Config{
				SlowThreshold:             200 * time.Millisecond,
				IgnoreRecordNotFoundError: true, // 忽略记录不存在的错误
				Colorful:                  true,
				LogLevel:                  logger.Error,
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

	/*
		if debug {
			dao.SetDefault(db.Debug())
		} else {
			dao.SetDefault(db)
		}
	*/
	return
}
