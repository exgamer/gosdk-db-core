package database

import (
	"bytes"
	"errors"
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

			db := openDummyDB(t, newGormLogger(&out, tc.debug), tc.queryError)

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

func openDummyDB(t *testing.T, gormLogger logger.Interface, queryError bool) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{Logger: gormLogger, DryRun: true})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}

	if !queryError {
		return db
	}

	err = db.Callback().Query().After("gorm:query").Register("test:fail", func(db *gorm.DB) {
		db.Error = errors.New("query failed")
	})
	if err != nil {
		t.Fatalf("register callback: %v", err)
	}

	return db
}
