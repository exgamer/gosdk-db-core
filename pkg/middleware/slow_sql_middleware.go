package plugin

import (
	"fmt"
	"github.com/getsentry/sentry-go"
	"gorm.io/gorm"
	"strings"
	"time"
)

func SlowSqlSentryMiddleware(threshold time.Duration, serviceName string) gorm.Plugin {
	return &SlowSqlSentry{threshold, serviceName}
}

type SlowSqlSentry struct {
	threshold   time.Duration // Порог для определения медленных запросов
	serviceName string
}

func (g *SlowSqlSentry) Name() string {
	return "slow_sql_plugin_sentry"
}

func (p *SlowSqlSentry) Initialize(db *gorm.DB) error {
	// Before any operation callback
	db.Callback().Create().Before("gorm:before_create").Register("gormsentry:before_create", p.before())
	db.Callback().Query().Before("gorm:before_query").Register("gormsentry:before_query", p.before())
	db.Callback().Delete().Before("gorm:before_delete").Register("gormsentry:before_delete", p.before())
	db.Callback().Update().Before("gorm:before_update").Register("gormsentry:before_update", p.before())
	db.Callback().Row().Before("gorm:before_row").Register("gormsentry:before_row", p.before())
	db.Callback().Raw().Before("gorm:before_raw").Register("gormsentry:before_raw", p.before())

	// After any operation callback
	db.Callback().Create().After("gorm:after_create").Register("gormsentry:after_create", p.after)
	db.Callback().Query().After("gorm:after_query").Register("gormsentry:after_query", p.after)
	db.Callback().Delete().After("gorm:after_delete").Register("gormsentry:after_delete", p.after)
	db.Callback().Update().After("gorm:after_update").Register("gormsentry:after_update", p.after)
	db.Callback().Row().After("gorm:after_row").Register("gormsentry:after_row", p.after)
	db.Callback().Raw().After("gorm:after_raw").Register("gormsentry:after_raw", p.after)

	return nil
}

func (p *SlowSqlSentry) before() func(*gorm.DB) {
	return func(db *gorm.DB) {
		startTime := time.Now()
		db.InstanceSet("start_time", startTime)
	}
}

func (p *SlowSqlSentry) after(db *gorm.DB) {
	startTime, ok := db.InstanceGet("start_time")

	if !ok {
		return
	}

	// Рассчитываем продолжительность выполнения SQL-запроса
	duration := time.Since(startTime.(time.Time))

	// Проверяем, превысил ли запрос установленный порог
	if duration > p.threshold {
		sentry.WithScope(func(scope *sentry.Scope) {
			mapData := make(map[string]interface{})
			mapData["Table"] = db.Statement.Table
			mapData["Sql "] = db.Statement.SQL.String()
			mapData["Duration"] = fmt.Sprintf("%.00f sec", duration.Seconds())
			mapData["Rows affected"] = db.RowsAffected

			var operation string
			if db.Statement.ReflectValue.IsValid() {
				operation = db.Statement.ReflectValue.Type().Name()
			} else {
				operation = p.detectOperationType(db.Statement.SQL.String())
			}
			mapData["Operation"] = operation
			mapData["Sql Params"] = db.Statement.Vars

			if db.Error != nil {
				mapData["Sql Error"] = db.Error.Error()
			}

			scope.SetContext("Sql data", mapData)
			scope.SetLevel(sentry.LevelWarning)

			sentry.CaptureMessage("Slow SQL Request " + fmt.Sprintf("%.00f", duration.Seconds()) + " sec" + ". Service Name - " + p.serviceName)
		})
	}
}

func (p *SlowSqlSentry) detectOperationType(sql string) string {
	if len(sql) == 0 {
		return "unknown"
	}

	sql = strings.ToUpper(strings.TrimSpace(sql))
	switch {
	case strings.HasPrefix(sql, "SELECT"):
		return "query"
	case strings.HasPrefix(sql, "INSERT"):
		return "create"
	case strings.HasPrefix(sql, "UPDATE"):
		return "update"
	case strings.HasPrefix(sql, "DELETE"):
		return "delete"
	default:
		return "raw"
	}
}
