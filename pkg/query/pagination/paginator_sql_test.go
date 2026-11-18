package pagination

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"

	"gorm.io/gorm"
)

type orderProbe struct {
	ID   uint
	Name string
}

type uuidProbe struct {
	Code string `gorm:"primaryKey"`
}

// captureSQL Пишет SQL каждого запроса; count-запрос получает countValue
func captureSQL(t *testing.T, db *gorm.DB, countValue int64) *[]string {
	t.Helper()

	var queries []string

	err := db.Callback().Query().After("gorm:query").Register("test:capture", func(db *gorm.DB) {
		queries = append(queries, db.Dialector.Explain(db.Statement.SQL.String(), db.Statement.Vars...))

		if c, ok := db.Statement.Dest.(*int64); ok {
			*c = countValue
			db.RowsAffected = 1
		}
	})
	if err != nil {
		t.Fatalf("register callback: %v", err)
	}

	return &queries
}

func lastSQL(t *testing.T, queries *[]string) string {
	t.Helper()

	if len(*queries) == 0 {
		t.Fatal("no queries")
	}

	return (*queries)[len(*queries)-1]
}

func TestPages_OrderByRejectsSQL(t *testing.T) {
	for _, o := range []string{
		"id; DROP TABLE users",
		"(CASE WHEN (SELECT 1)=1 THEN id END)",
		"id desc, name",
		"id desc nulls last",
		"random()",
		"name -- x",
		"a..b",
		"1",
		"",
	} {
		t.Run(o, func(t *testing.T) {
			db := openDummyDB(t)
			queries := captureSQL(t, db, 0)

			var rows []orderProbe

			_, err := Pages(&Param{DB: db.Model(&orderProbe{}), Paging: &Paging{OrderBy: []string{o}}}, &rows)
			if !errors.Is(err, ErrInvalidOrderBy) {
				t.Fatalf("err = %v, want ErrInvalidOrderBy", err)
			}

			if len(*queries) != 0 {
				t.Fatalf("queries ran: %q", *queries)
			}
		})
	}
}

func TestPages_OrderByColumnsQuoted(t *testing.T) {
	db := openDummyDB(t)
	queries := captureSQL(t, db, 0)

	var rows []orderProbe

	_, err := Pages(&Param{DB: db.Model(&orderProbe{}), Paging: &Paging{OrderBy: []string{" Name DESC ", "t.created_at asc", "id"}}}, &rows)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}

	if sql := lastSQL(t, queries); !strings.Contains(sql, "ORDER BY `name` DESC,`t`.`created_at`,`id`") {
		t.Fatalf("sql = %q", sql)
	}
}

func TestPages_DefaultOrderApplied(t *testing.T) {
	db := openDummyDB(t)
	queries := captureSQL(t, db, 0)

	var rows []orderProbe

	paging := &Paging{}

	if _, err := Pages(&Param{DB: db.Model(&orderProbe{}), Paging: paging}, &rows); err != nil {
		t.Fatalf("Pages: %v", err)
	}

	if sql := lastSQL(t, queries); !strings.Contains(sql, "ORDER BY `order_probes`.`id` DESC") {
		t.Fatalf("sql = %q", sql)
	}

	if len(paging.OrderBy) != 1 || paging.OrderBy[0] != "id desc" {
		t.Fatalf("OrderBy = %q", paging.OrderBy)
	}
}

func TestPages_DefaultOrderPrimaryKeyAndTable(t *testing.T) {
	db := openDummyDB(t)
	queries := captureSQL(t, db, 0)

	var rows []uuidProbe

	if _, err := Pages(&Param{DB: db.Table("codes")}, &rows); err != nil {
		t.Fatalf("Pages: %v", err)
	}

	if sql := lastSQL(t, queries); !strings.Contains(sql, "ORDER BY `codes`.`code` DESC") {
		t.Fatalf("sql = %q", sql)
	}
}

func TestPages_DefaultOrderSkipped(t *testing.T) {
	cases := map[string]func(db *gorm.DB) *gorm.DB{
		"ORDER BY запроса": func(db *gorm.DB) *gorm.DB { return db.Model(&orderProbe{}).Order("name") },
		"GROUP BY":         func(db *gorm.DB) *gorm.DB { return db.Model(&orderProbe{}).Select("name").Group("name") },
		"DISTINCT":         func(db *gorm.DB) *gorm.DB { return db.Model(&orderProbe{}).Distinct("name") },
		"без модели":       func(db *gorm.DB) *gorm.DB { return db.Table("raw") },
	}

	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			db := openDummyDB(t)
			queries := captureSQL(t, db, 0)

			var result any = &[]orderProbe{}
			if name == "без модели" {
				result = &[]map[string]any{}
			}

			if _, err := Pages(&Param{DB: build(db)}, result); err != nil {
				t.Fatalf("Pages: %v", err)
			}

			if sql := lastSQL(t, queries); strings.Contains(sql, "DESC") {
				t.Fatalf("default order applied: %q", sql)
			}
		})
	}
}

func TestPages_Limits(t *testing.T) {
	cases := []struct {
		name            string
		limit, maxLimit uint
		want            uint
	}{
		{name: "MaxLimit 0, Limit 0 — по умолчанию", want: DefaultLimit},
		{name: "MaxLimit 0, Limit 50", limit: 50, want: 50},
		{name: "MaxLimit 0, Limit больше потолка", limit: 5000, want: MaxLimitCeiling},
		{name: "MaxLimit больше потолка", limit: 5000, maxLimit: 100000, want: MaxLimitCeiling},
		{name: "MaxLimit 100", limit: 500, maxLimit: 100, want: 100},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := openDummyDB(t)
			queries := captureSQL(t, db, 0)

			var rows []orderProbe

			p, err := Pages(&Param{DB: db.Model(&orderProbe{}), Paging: &Paging{Limit: tc.limit, MaxLimit: tc.maxLimit}}, &rows)
			if err != nil {
				t.Fatalf("Pages: %v", err)
			}

			if p.Limit != tc.want {
				t.Fatalf("Limit = %d, want %d", p.Limit, tc.want)
			}

			if sql := lastSQL(t, queries); !strings.Contains(sql, "LIMIT "+itoa(tc.want)) {
				t.Fatalf("sql = %q", sql)
			}
		})
	}
}

func TestPages_TotalPage(t *testing.T) {
	db := openDummyDB(t)
	captureSQL(t, db, 45)

	var rows []orderProbe

	p, err := Pages(&Param{DB: db.Model(&orderProbe{}), Paging: &Paging{Page: 2, Limit: 10}}, &rows)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}

	if p.TotalRecords != 45 || p.TotalPage != 5 || p.PrevPage != 1 || p.NextPage != 3 || p.From != 10 {
		t.Fatalf("paginator = %+v", p)
	}
}

func TestPages_PageOverflowClamped(t *testing.T) {
	db := openDummyDB(t)
	queries := captureSQL(t, db, 0)

	var rows []orderProbe

	p, err := Pages(&Param{DB: db.Model(&orderProbe{}), Paging: &Paging{Page: math.MaxUint, Limit: 1000}}, &rows)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}

	if p.Offset > maxOffset || p.Offset == 0 {
		t.Fatalf("offset = %d", p.Offset)
	}

	if sql := lastSQL(t, queries); !strings.Contains(sql, "OFFSET "+itoa(p.Offset)) {
		t.Fatalf("sql = %q", sql)
	}
}

func TestPages_CountGroupedViaSubquery(t *testing.T) {
	db := openDummyDB(t)
	queries := captureSQL(t, db, 0)

	var rows []orderProbe

	q := db.Model(&orderProbe{}).Select("order_probes.*").Joins("JOIN tags ON tags.probe_id = order_probes.id").Group("order_probes.id")

	if _, err := Pages(&Param{DB: q}, &rows); err != nil {
		t.Fatalf("Pages: %v", err)
	}

	count := (*queries)[len(*queries)-2]
	if !strings.Contains(count, "count(*) FROM (SELECT order_probes.* FROM `order_probes` JOIN tags") ||
		!strings.Contains(count, ") AS pagination_count") {
		t.Fatalf("count sql = %q", count)
	}
}

func TestPaging_ServiceFieldsNotFromJSON(t *testing.T) {
	var p Paging

	if err := json.Unmarshal([]byte(`{"page":2,"limit":5,"MaxLimit":1000000,"ShowSQL":true,"max_limit":1000000,"show_sql":true}`), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if p.MaxLimit != 0 || p.ShowSQL || p.Page != 2 || p.Limit != 5 {
		t.Fatalf("paging = %+v", p)
	}
}

func itoa(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}

// countDTO Результат-DTO без своей таблицы: count должен идти по Model запроса
type countDTO struct {
	Name  string
	Total int64
}

func TestPages_CountUsesQueryModelNotResultType(t *testing.T) {
	cases := map[string]struct {
		build func(db *gorm.DB) *gorm.DB
		want  string
	}{
		"без GROUP BY": {
			build: func(db *gorm.DB) *gorm.DB { return db.Model(&orderProbe{}).Where("name <> ?", "x") },
			want:  "SELECT count(*) FROM `order_probes` WHERE name <> \"x\"",
		},
		"GROUP BY": {
			build: func(db *gorm.DB) *gorm.DB {
				return db.Model(&orderProbe{}).Select("name, count(*) AS total").Group("name")
			},
			want: "FROM (SELECT name, count(*) AS total FROM `order_probes` GROUP BY `name`) AS pagination_count",
		},
		"DISTINCT": {
			build: func(db *gorm.DB) *gorm.DB { return db.Model(&orderProbe{}).Distinct("name") },
			want:  "FROM (SELECT DISTINCT `name` FROM `order_probes`) AS pagination_count",
		},
		"Table без Model": {
			build: func(db *gorm.DB) *gorm.DB { return db.Table("codes") },
			want:  "SELECT count(*) FROM `codes`",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			db := openDummyDB(t)
			queries := captureSQL(t, db, 0)

			var rows []countDTO

			if _, err := Pages(&Param{DB: tc.build(db)}, &rows); err != nil {
				t.Fatalf("Pages: %v", err)
			}

			if len(*queries) < 2 {
				t.Fatalf("queries = %q", *queries)
			}

			count := (*queries)[len(*queries)-2]
			if !strings.Contains(count, tc.want) || strings.Contains(count, "count_dtos") {
				t.Fatalf("count sql = %q", count)
			}
		})
	}
}

func TestPages_CountUsesResultTypeWithoutModel(t *testing.T) {
	db := openDummyDB(t)
	queries := captureSQL(t, db, 0)

	var rows []orderProbe

	if _, err := Pages(&Param{DB: db.Where("name <> ?", "x")}, &rows); err != nil {
		t.Fatalf("Pages: %v", err)
	}

	if count := (*queries)[0]; !strings.Contains(count, "SELECT count(*) FROM `order_probes`") {
		t.Fatalf("count sql = %q", count)
	}
}
