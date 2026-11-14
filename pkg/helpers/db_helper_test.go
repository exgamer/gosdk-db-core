package database

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/utils/tests"
)

const secretValue = "secret-hash-value"

type gormLoggerProbe struct {
	ID   uint
	Hash string
}

func TestNewGormLogger_SQLParams(t *testing.T) {
	cases := []struct {
		name       string
		debug      bool
		queryError bool
		wantLogged bool
		wantSecret bool
	}{
		{name: "не debug, ошибка запроса — SQL без значений", debug: false, queryError: true, wantLogged: true, wantSecret: false},
		{name: "не debug, без ошибки — запрос не пишется", debug: false, queryError: false, wantLogged: false},
		{name: "debug, ошибка запроса — SQL со значениями", debug: true, queryError: true, wantLogged: true, wantSecret: true},
		{name: "debug, без ошибки — SQL со значениями", debug: true, queryError: false, wantLogged: true, wantSecret: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer

			var queryErr error
			if tc.queryError {
				queryErr = errors.New("query failed")
			}

			db := openDummyDB(t, newGormLogger(&out, tc.debug), queryErr)

			var rows []gormLoggerProbe
			db.Where("hash = ?", secretValue).Find(&rows)

			logged := out.String()

			if tc.wantLogged != strings.Contains(logged, "gorm_logger_probes") {
				t.Fatalf("logged = %v, want %v; output: %q", !tc.wantLogged, tc.wantLogged, logged)
			}

			if !tc.wantLogged {
				return
			}

			if tc.wantSecret != strings.Contains(logged, secretValue) {
				t.Fatalf("secret in log = %v, want %v; output: %q", !tc.wantSecret, tc.wantSecret, logged)
			}

			if !tc.wantSecret && !strings.Contains(logged, "hash = ?") {
				t.Fatalf("placeholder not in log; output: %q", logged)
			}
		})
	}
}

func TestNewGormLogger_ContextCanceled(t *testing.T) {
	cases := []struct {
		name       string
		debug      bool
		queryErr   error
		wantLogged bool
	}{
		{name: "не debug, context.Canceled — не пишется", queryErr: context.Canceled, wantLogged: false},
		{name: "не debug, обёрнутый context.Canceled — не пишется", queryErr: fmt.Errorf("pgx: %w", context.Canceled), wantLogged: false},
		{name: "не debug, context.DeadlineExceeded — пишется", queryErr: context.DeadlineExceeded, wantLogged: true},
		{name: "не debug, другая ошибка — пишется", queryErr: errors.New("query failed"), wantLogged: true},
		{name: "debug, context.Canceled — пишется", debug: true, queryErr: context.Canceled, wantLogged: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer

			db := openDummyDB(t, newGormLogger(&out, tc.debug), tc.queryErr)

			var rows []gormLoggerProbe
			db.Where("hash = ?", secretValue).Find(&rows)

			logged := out.String()

			if tc.wantLogged != strings.Contains(logged, "gorm_logger_probes") {
				t.Fatalf("logged = %v, want %v; output: %q", !tc.wantLogged, tc.wantLogged, logged)
			}

			if !tc.wantLogged {
				return
			}

			if !strings.Contains(logged, tc.queryErr.Error()) {
				t.Fatalf("error %q not in log; output: %q", tc.queryErr, logged)
			}

			// место вызова в строке лога — код запроса, а не фильтр логгера
			if !strings.Contains(logged, "db_helper_test.go:") {
				t.Fatalf("caller file not in log; output: %q", logged)
			}
		})
	}
}

func TestNewGormLogger_RecordNotFound(t *testing.T) {
	cases := []struct {
		name       string
		env        string
		debug      bool
		wantLogged bool
	}{
		{name: "по умолчанию — пишется, как раньше", wantLogged: true},
		{name: "DB_IGNORE_RECORD_NOT_FOUND=true — не пишется", env: "true", wantLogged: false},
		{name: "DB_IGNORE_RECORD_NOT_FOUND=true, debug — не пишется", env: "true", debug: true, wantLogged: false},
		{name: "DB_IGNORE_RECORD_NOT_FOUND=abc — пишется", env: "abc", wantLogged: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(envIgnoreRecordNotFound, tc.env)

			var out bytes.Buffer

			db := openDummyDB(t, newGormLogger(&out, tc.debug), gorm.ErrRecordNotFound)

			var row gormLoggerProbe
			db.Where("hash = ?", secretValue).First(&row)

			logged := strings.Contains(out.String(), "record not found")
			if logged != tc.wantLogged {
				t.Fatalf("record not found logged = %v, want %v; output: %q", logged, tc.wantLogged, out.String())
			}
		})
	}
}

// openDummyDB Открывает GORM без БД; queryErr не nil — запрос завершается этой ошибкой
func openDummyDB(t *testing.T, gormLogger logger.Interface, queryErr error) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{Logger: gormLogger, DryRun: true})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}

	if queryErr == nil {
		return db
	}

	err = db.Callback().Query().After("gorm:query").Register("test:fail", func(db *gorm.DB) {
		db.Error = queryErr
	})
	if err != nil {
		t.Fatalf("register callback: %v", err)
	}

	return db
}
