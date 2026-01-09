package database

import (
	"fmt"
	"github.com/exgamer/gosdk-db-core/pkg/middleware"
	"github.com/go-errors/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"time"
)

// GetGormConnection Возвращает клиент для работы с БД
func GetGormConnection(dbConfig *DbConfig) (*gorm.DB, error) {
	if dbConfig.Dialector == nil {
		return nil, errors.New("Unknown db dialector")
	}

	config := &gorm.Config{}

	if dbConfig.DbLogLevel == "info" {
		config.Logger = logger.Default.LogMode(logger.Info)
	} else if dbConfig.DbLogLevel == "errors" {
		config.Logger = logger.Default.LogMode(logger.Error)
	} else if dbConfig.DbLogLevel == "warnings" {
		config.Logger = logger.Default.LogMode(logger.Warn)
	} else {
		config.Logger = logger.Default.LogMode(logger.Error)
	}

	//if dbConfig.DisableAutomaticPing {
	//config.DisableAutomaticPing = true
	//}

	gormDb, err := gorm.Open(dbConfig.Dialector, config)

	if err != nil {
		return nil, err
	}

	db, err := gormDb.DB()

	if err != nil {
		return nil, err
	}

	db.SetConnMaxLifetime(time.Hour)

	if dbConfig.MaxOpenConnections > 0 {
		db.SetMaxOpenConns(dbConfig.MaxOpenConnections)
	}

	if dbConfig.MaxIdleConnections > 0 {
		db.SetMaxIdleConns(dbConfig.MaxIdleConnections)
	}

	var threshold time.Duration

	if dbConfig.Threshold == 0 {
		threshold = time.Second
	} else {
		threshold = time.Duration(dbConfig.Threshold) * time.Second
	}

	//мидлвар для мониторинга медленных запросов через сентри
	err = gormDb.Use(plugin.SlowSqlSentryMiddleware(threshold, dbConfig.ServiceName))

	if err != nil {
		fmt.Println("error init SlowSqlSentryMiddleware : ", err)
	}

	//мидлвар для вывода запросов в дебаг инфу
	err = gormDb.Use(plugin.NewGormDebugMiddleware())

	if err != nil {
		fmt.Println("error init GormDebugMiddleware : ", err)
	}

	return gormDb, nil
}
