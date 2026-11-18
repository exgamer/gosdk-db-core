package database

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestApplyPoolLimits(t *testing.T) {
	cases := []struct {
		name               string
		maxOpen, maxIdle   int
		wantOpen, wantIdle int
	}{
		{name: "не заданы — по умолчанию", wantOpen: defaultMaxOpenConnections, wantIdle: defaultMaxIdleConnections},
		{name: "заданы — как есть", maxOpen: 40, maxIdle: 20, wantOpen: 40, wantIdle: 20},
		{name: "задан только MaxOpen меньше idle по умолчанию", maxOpen: 4, wantOpen: 4, wantIdle: 4},
		{name: "задан только MaxIdle", maxIdle: 3, wantOpen: defaultMaxOpenConnections, wantIdle: 3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, _, err := sqlmock.New()
			if err != nil {
				t.Fatalf("sqlmock: %v", err)
			}
			defer db.Close()

			applyPoolLimits(db, &DbConfig{MaxOpenConnections: tc.maxOpen, MaxIdleConnections: tc.maxIdle})

			if got := db.Stats().MaxOpenConnections; got != tc.wantOpen {
				t.Fatalf("MaxOpenConnections = %d, want %d", got, tc.wantOpen)
			}

			if got := idleAfterBurst(t, db, tc.wantOpen); got != tc.wantIdle {
				t.Fatalf("idle = %d, want %d", got, tc.wantIdle)
			}
		})
	}
}

// idleAfterBurst Открывает n соединений, возвращает их в пул и считает простаивающие
func idleAfterBurst(t *testing.T, db *sql.DB, n int) int {
	t.Helper()

	conns := make([]*sql.Conn, 0, n)

	for range n {
		c, err := db.Conn(context.Background())
		if err != nil {
			t.Fatalf("conn: %v", err)
		}

		conns = append(conns, c)
	}

	for _, c := range conns {
		_ = c.Close()
	}

	return db.Stats().Idle
}
