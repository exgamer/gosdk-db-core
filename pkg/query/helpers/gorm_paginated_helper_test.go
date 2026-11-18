package helpers

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/utils/tests"
)

type helperProbe struct {
	ID uint
}

func TestEscapeLike(t *testing.T) {
	cases := map[string]string{
		"plain":     "plain",
		"50%":       `50\%`,
		"a_b":       `a\_b`,
		`c:\dir`:    `c:\\dir`,
		`%_\`:       `\%\_\\`,
		"":          "",
		"кириллица": "кириллица",
	}

	for in, want := range cases {
		if got := EscapeLike(in); got != want {
			t.Errorf("EscapeLike(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGormPaginatedHelper_Timeout(t *testing.T) {
	cases := []struct {
		name string
		set  *time.Duration
		want time.Duration
	}{
		{name: "по умолчанию", want: 10 * time.Second},
		{name: "старый вызов SetTimeout(3) — секунды", set: ptr(time.Duration(3)), want: 3 * time.Second},
		{name: "Duration 2s", set: ptr(2 * time.Second), want: 2 * time.Second},
		{name: "Duration 1h — без переполнения", set: ptr(time.Hour), want: time.Hour},
		{name: "0 — не меняет", set: ptr(time.Duration(0)), want: 10 * time.Second},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{Logger: logger.Discard, DryRun: true})
			if err != nil {
				t.Fatalf("gorm.Open: %v", err)
			}

			var remaining time.Duration

			err = db.Callback().Query().After("gorm:query").Register("test:deadline", func(db *gorm.DB) {
				if d, ok := db.Statement.Context.Deadline(); ok {
					remaining = time.Until(d)
				}
			})
			if err != nil {
				t.Fatalf("register: %v", err)
			}

			h := NewGormPaginatedHelper[helperProbe](context.Background(), db)
			if tc.set != nil {
				h.SetTimeout(*tc.set)
			}

			if _, err := h.Paginated(1, func(client *gorm.DB) *gorm.DB { return client.Model(&helperProbe{}) }); err != nil {
				t.Fatalf("Paginated: %v", err)
			}

			if remaining <= tc.want-time.Second || remaining > tc.want {
				t.Fatalf("remaining = %v, want about %v", remaining, tc.want)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }
