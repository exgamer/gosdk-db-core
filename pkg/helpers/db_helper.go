package database

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	logger2 "github.com/exgamer/gosdk-core/pkg/logger"
	"github.com/exgamer/gosdk-db-core/pkg/middleware"
	"github.com/go-errors/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// GetGormConnection Возвращает клиент для работы с БД
func GetGormConnection(dbConfig *DbConfig) (*gorm.DB, error) {
	if dbConfig.Dialector == nil {
		return nil, errors.New("Unknown db dialector")
	}

	config := &gorm.Config{
		Logger: newGormLogger(os.Stdout, logger2.IsDebugLevel()),
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

// envIgnoreRecordNotFound Переменная окружения: true — не писать «record not found» (First/Take/Last без строки)
// в лог GORM как ошибку запроса. По умолчанию пишется, как раньше
const envIgnoreRecordNotFound = "DB_IGNORE_RECORD_NOT_FOUND"

// newGormLogger Логгер GORM с настройками logger.Default: на debug — все запросы со значениями
// параметров, иначе — только ошибки, кроме context.Canceled (запрос оборвал клиент), SQL
// с плейсхолдерами без значений. «record not found» пишется как ошибка, DB_IGNORE_RECORD_NOT_FOUND=true — не пишется
func newGormLogger(writer io.Writer, debug bool) logger.Interface {
	level := logger.Error

	var out logger.Writer = log.New(writer, "\r\n", log.LstdFlags)

	if debug {
		level = logger.Info
	} else {
		out = skipCanceledWriter{Writer: out}
	}

	return logger.New(out, logger.Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  level,
		IgnoreRecordNotFoundError: ignoreRecordNotFound(),
		Colorful:                  true,
		ParameterizedQueries:      !debug,
	})
}

// ignoreRecordNotFound DB_IGNORE_RECORD_NOT_FOUND разобран как bool; пусто или неверно — false
func ignoreRecordNotFound() bool {
	v, err := strconv.ParseBool(os.Getenv(envIgnoreRecordNotFound))

	return err == nil && v
}

// skipCanceledWriter Не пишет строки лога GORM с ошибкой context.Canceled: запрос оборвал клиент
// (закрыл соединение, ушёл со страницы), это не сбой БД. context.DeadlineExceeded и прочие ошибки
// пишутся как раньше. Фильтр стоит на Writer, а не на Trace: обёртка Trace сдвинула бы файл:строку
// вызова в логе GORM на этот пакет
type skipCanceledWriter struct {
	logger.Writer
}

// Printf Пишет строку, если среди аргументов нет ошибки context.Canceled
func (w skipCanceledWriter) Printf(format string, args ...any) {
	for _, arg := range args {
		if err, ok := arg.(error); ok && errors.Is(err, context.Canceled) {
			return
		}
	}

	w.Writer.Printf(format, args...)
}
