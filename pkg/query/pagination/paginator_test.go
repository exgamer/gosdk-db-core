package pagination

import (
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/utils/tests"
)

var errCountFailed = errors.New("count failed")

type pagesProbe struct {
	ID uint
}

func TestPages_CountErrorReturned(t *testing.T) {
	db := openDummyDB(t)

	err := db.Callback().Query().After("gorm:query").Register("test:fail_count", func(db *gorm.DB) {
		if strings.Contains(strings.ToLower(db.Statement.SQL.String()), "count(") {
			db.Error = errCountFailed
		}
	})
	if err != nil {
		t.Fatalf("register callback: %v", err)
	}

	var rows []pagesProbe

	paginator, err := Pages(&Param{DB: db.Model(&pagesProbe{}), Paging: &Paging{Page: 1, Limit: 10, MaxLimit: 100}}, &rows)

	if !errors.Is(err, errCountFailed) {
		t.Fatalf("err = %v, want wrapped %v", err, errCountFailed)
	}

	if !strings.HasPrefix(err.Error(), "pagination: count: ") {
		t.Fatalf("err = %q, want prefix %q", err.Error(), "pagination: count: ")
	}

	if paginator != nil {
		t.Fatalf("paginator = %+v, want nil", paginator)
	}
}

func TestPages_WithoutErrors_ReturnsPagination(t *testing.T) {
	db := openDummyDB(t)

	var rows []pagesProbe

	paginator, err := Pages(&Param{DB: db.Model(&pagesProbe{}), Paging: &Paging{Page: 2, Limit: 10, MaxLimit: 100}}, &rows)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}

	if paginator.Page != 2 || paginator.Limit != 10 || paginator.Offset != 10 || paginator.TotalRecords != 0 {
		t.Fatalf("paginator = %+v", paginator)
	}
}

func openDummyDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{Logger: logger.Discard, DryRun: true})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}

	return db
}
