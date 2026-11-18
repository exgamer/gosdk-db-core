package transaction

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/utils/tests"
)

// savepointDialector DummyDialector с точками сохранения, как у postgres-диалекта
type savepointDialector struct {
	tests.DummyDialector
}

func (savepointDialector) SavePoint(tx *gorm.DB, name string) error {
	return tx.Exec("SAVEPOINT " + name).Error
}

func (savepointDialector) RollbackTo(tx *gorm.DB, name string) error {
	return tx.Exec("ROLLBACK TO SAVEPOINT " + name).Error
}

// recordingPool Пул, который запоминает параметры BeginTx
type recordingPool struct {
	*sql.DB
	opts []*sql.TxOptions
}

func (p *recordingPool) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	p.opts = append(p.opts, opts)

	return p.DB.BeginTx(ctx, opts)
}

type repo struct {
	db *gorm.DB
}

func (r repo) insert(ctx context.Context, v string) error {
	return r.db.WithContext(ctx).Exec("INSERT INTO t VALUES (?)", v).Error
}

func newMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, *recordingPool) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}

	t.Cleanup(func() { _ = sqlDB.Close() })

	pool := &recordingPool{DB: sqlDB}

	db, err := gorm.Open(savepointDialector{}, &gorm.Config{ConnPool: pool, Logger: logger.Discard, SkipDefaultTransaction: true})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}

	return db, mock, pool
}

func newRepo(tx *gorm.DB) repo { return repo{db: tx} }

var errFail = errors.New("fail")

func TestExec_Commit(t *testing.T) {
	db, mock, _ := newMockDB(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO t").WithArgs("a").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := NewManager(db, newRepo).Exec(context.Background(), func(ctx context.Context, r repo) error {
		if !InTransaction(ctx) {
			t.Error("ctx without transaction")
		}

		return r.insert(ctx, "a")
	})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestExec_NestedUsesOuterTransaction(t *testing.T) {
	db, mock, _ := newMockDB(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO t").WithArgs("outer").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("SAVEPOINT sp").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO t").WithArgs("inner").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	outer := NewManager(db, newRepo)
	inner := NewManager2(db, newRepo, newRepo)

	err := outer.Exec(context.Background(), func(ctx context.Context, r repo) error {
		if err := r.insert(ctx, "outer"); err != nil {
			return err
		}

		return inner.Exec(ctx, func(ctx context.Context, r1, _ repo) error {
			return r1.insert(ctx, "inner")
		})
	})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestExec_NestedErrorRollsBackToSavepoint(t *testing.T) {
	db, mock, _ := newMockDB(t)

	mock.ExpectBegin()
	mock.ExpectExec("SAVEPOINT sp").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO t").WithArgs("inner").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("ROLLBACK TO SAVEPOINT sp").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO t").WithArgs("after").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	m := NewManager(db, newRepo)

	err := m.Exec(context.Background(), func(ctx context.Context, r repo) error {
		innerErr := m.Exec(ctx, func(ctx context.Context, r repo) error {
			if err := r.insert(ctx, "inner"); err != nil {
				return err
			}

			return errFail
		})
		if !errors.Is(innerErr, errFail) {
			t.Errorf("inner err = %v", innerErr)
		}

		return r.insert(ctx, "after")
	})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestExec_NestedOtherConnectionOpensOwnTransaction(t *testing.T) {
	db1, mock1, _ := newMockDB(t)
	db2, mock2, _ := newMockDB(t)

	mock1.ExpectBegin()
	mock2.ExpectBegin()
	mock2.ExpectExec("INSERT INTO t").WithArgs("db2").WillReturnResult(sqlmock.NewResult(1, 1))
	mock2.ExpectCommit()
	mock1.ExpectCommit()

	err := NewManager(db1, newRepo).Exec(context.Background(), func(ctx context.Context, _ repo) error {
		if FromContext(ctx, db2) != db2 {
			t.Error("FromContext returned a transaction of another connection")
		}

		return NewManager(db2, newRepo).Exec(ctx, func(ctx context.Context, r repo) error {
			return r.insert(ctx, "db2")
		})
	})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	for _, m := range []sqlmock.Sqlmock{mock1, mock2} {
		if err := m.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFromContext(t *testing.T) {
	db, mock, _ := newMockDB(t)

	if FromContext(context.Background(), db) != db {
		t.Fatal("FromContext without transaction must return client")
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO t").WithArgs("via ctx").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := NewManager(db, newRepo).Exec(context.Background(), func(ctx context.Context, _ repo) error {
		tx := FromContext(ctx, db)
		if tx == db {
			t.Error("FromContext returned pool inside Exec")
		}

		return tx.WithContext(ctx).Exec("INSERT INTO t VALUES (?)", "via ctx").Error
	})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestWithTimeout_NonPositiveIgnored(t *testing.T) {
	for _, d := range []time.Duration{0, -time.Second} {
		db, mock, _ := newMockDB(t)

		mock.ExpectBegin()
		mock.ExpectCommit()

		err := NewManager(db, newRepo).WithTimeout(d).Exec(context.Background(), func(ctx context.Context, _ repo) error {
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) < defaultTimeout-time.Second {
				t.Errorf("WithTimeout(%v): deadline %v, ok %v", d, deadline, ok)
			}

			return nil
		})
		if err != nil {
			t.Fatalf("WithTimeout(%v): Exec: %v", d, err)
		}
	}
}

func TestExecWithOptions_PassesTxOptions(t *testing.T) {
	db, mock, pool := newMockDB(t)

	mock.ExpectBegin()
	mock.ExpectCommit()

	opts := &sql.TxOptions{ReadOnly: true}

	err := NewManager3(db, newRepo, newRepo, newRepo).ExecWithOptions(context.Background(), opts,
		func(ctx context.Context, _, _, _ repo) error { return nil })
	if err != nil {
		t.Fatalf("ExecWithOptions: %v", err)
	}

	if len(pool.opts) != 1 || pool.opts[0] != opts {
		t.Fatalf("BeginTx opts = %v", pool.opts)
	}
}

type stateErr string

func (e stateErr) Error() string    { return "sqlstate " + string(e) }
func (e stateErr) SQLState() string { return string(e) }

func TestRetry(t *testing.T) {
	cases := []struct {
		name      string
		errs      []error
		attempts  int
		wantCalls int
		wantErr   bool
	}{
		{name: "40001 затем успех", errs: []error{stateErr("40001"), nil}, attempts: 3, wantCalls: 2},
		{name: "40P01 обёрнутая", errs: []error{errors.Join(errFail, stateErr("40P01")), nil}, attempts: 3, wantCalls: 2},
		{name: "другая ошибка — без повтора", errs: []error{stateErr("23505")}, attempts: 3, wantCalls: 1, wantErr: true},
		{name: "попытки кончились", errs: []error{stateErr("40001"), stateErr("40001")}, attempts: 2, wantCalls: 2, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0

			err := Retry(context.Background(), tc.attempts, func(ctx context.Context) error {
				calls++

				return tc.errs[calls-1]
			})

			if calls != tc.wantCalls || (err != nil) != tc.wantErr {
				t.Fatalf("calls %d, err %v", calls, err)
			}
		})
	}
}

func TestRetry_NotInsideTransaction(t *testing.T) {
	db, mock, _ := newMockDB(t)

	mock.ExpectBegin()
	mock.ExpectRollback()

	calls := 0

	_ = NewManager(db, newRepo).Exec(context.Background(), func(ctx context.Context, _ repo) error {
		return Retry(ctx, 5, func(ctx context.Context) error {
			calls++

			return stateErr("40001")
		})
	})

	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestWithoutTransaction_OpensIndependentTransaction(t *testing.T) {
	db, mock, _ := newMockDB(t)

	mock.ExpectBegin()
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO t").WithArgs("log").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectRollback()

	m := NewManager(db, newRepo)

	err := m.Exec(context.Background(), func(ctx context.Context, _ repo) error {
		detached := WithoutTransaction(ctx)
		if InTransaction(detached) || FromContext(detached, db) != db {
			t.Error("WithoutTransaction kept the transaction")
		}

		if err := m.Exec(detached, func(ctx context.Context, r repo) error { return r.insert(ctx, "log") }); err != nil {
			return err
		}

		return errFail
	})
	if !errors.Is(err, errFail) {
		t.Fatalf("Exec: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestExec_ParallelNestedRunSequentially(t *testing.T) {
	db, mock, _ := newMockDB(t)

	const workers = 8

	mock.ExpectBegin()

	for i := 0; i < workers; i++ {
		mock.ExpectExec("SAVEPOINT sp").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec("INSERT INTO t").WithArgs("g").WillReturnResult(sqlmock.NewResult(1, 1))

		if i%2 == 1 {
			mock.ExpectExec("ROLLBACK TO SAVEPOINT sp").WillReturnResult(sqlmock.NewResult(0, 0))
		}
	}

	mock.ExpectCommit()

	m := NewManager(db, newRepo)

	var (
		inside, order atomic.Int32
		parallel      atomic.Bool
	)

	err := m.Exec(context.Background(), func(ctx context.Context, _ repo) error {
		var wg sync.WaitGroup

		for i := 0; i < workers; i++ {
			wg.Add(1)

			go func() {
				defer wg.Done()

				_ = m.Exec(ctx, func(ctx context.Context, r repo) error {
					if inside.Add(1) > 1 {
						parallel.Store(true)
					}
					defer inside.Add(-1)

					seq := order.Add(1) - 1

					if err := r.insert(ctx, "g"); err != nil {
						return err
					}

					time.Sleep(5 * time.Millisecond)

					if seq%2 == 1 {
						return errFail
					}

					return nil
				})
			}()
		}

		wg.Wait()

		return nil
	})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if parallel.Load() {
		t.Fatal("nested Exec ran in parallel")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestExec_CommitWaitsForRunningNested(t *testing.T) {
	db, mock, _ := newMockDB(t)

	mock.ExpectBegin()
	mock.ExpectExec("SAVEPOINT sp").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO t").WithArgs("bg").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	m := NewManager(db, newRepo)
	started := make(chan struct{})
	finished := make(chan error, 1)

	err := m.Exec(context.Background(), func(ctx context.Context, _ repo) error {
		go func() {
			finished <- m.Exec(ctx, func(ctx context.Context, r repo) error {
				close(started)
				time.Sleep(30 * time.Millisecond)

				return r.insert(ctx, "bg")
			})
		}()

		<-started

		return nil
	})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if err := <-finished; err != nil {
		t.Fatalf("nested: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestExec_CtxAfterCommitOpensNewTransaction(t *testing.T) {
	db, mock, _ := newMockDB(t)

	mock.ExpectBegin()
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO t").WithArgs("late").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	m := NewManager(db, newRepo)

	var escaped context.Context

	if err := m.Exec(context.Background(), func(ctx context.Context, _ repo) error {
		escaped = context.WithoutCancel(ctx)

		return nil
	}); err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if InTransaction(escaped) || FromContext(escaped, db) != db {
		t.Fatal("finished transaction is still in ctx")
	}

	err := m.Exec(escaped, func(ctx context.Context, r repo) error {
		if !InTransaction(ctx) {
			t.Error("new Exec without transaction")
		}

		return r.insert(ctx, "late")
	})
	if err != nil {
		t.Fatalf("Exec after commit: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestExec_CtxAfterRollbackOpensNewTransaction(t *testing.T) {
	db, mock, _ := newMockDB(t)

	mock.ExpectBegin()
	mock.ExpectRollback()
	mock.ExpectBegin()
	mock.ExpectCommit()

	m := NewManager(db, newRepo)

	var escaped context.Context

	_ = m.Exec(context.Background(), func(ctx context.Context, _ repo) error {
		escaped = ctx

		return errFail
	})

	if InTransaction(escaped) {
		t.Fatal("rolled back transaction is still in ctx")
	}

	if err := m.Exec(context.WithoutCancel(escaped), func(context.Context, repo) error { return nil }); err != nil {
		t.Fatalf("Exec after rollback: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestExec_CtxAfterSavepointUsesOuterTransaction(t *testing.T) {
	db, mock, _ := newMockDB(t)

	mock.ExpectBegin()
	mock.ExpectExec("SAVEPOINT sp").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("SAVEPOINT sp").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO t").WithArgs("after").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	m := NewManager(db, newRepo)

	err := m.Exec(context.Background(), func(ctx context.Context, _ repo) error {
		outerTx := FromContext(ctx, db)

		var inner context.Context

		if err := m.Exec(ctx, func(ctx context.Context, _ repo) error {
			inner = ctx

			return nil
		}); err != nil {
			return err
		}

		if FromContext(inner, db) != outerTx || !InTransaction(inner) {
			t.Error("finished savepoint ctx does not lead to the outer transaction")
		}

		return m.Exec(inner, func(ctx context.Context, r repo) error { return r.insert(ctx, "after") })
	})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRetry_AfterCommitRetries(t *testing.T) {
	db, mock, _ := newMockDB(t)

	mock.ExpectBegin()
	mock.ExpectCommit()

	var escaped context.Context

	_ = NewManager(db, newRepo).Exec(context.Background(), func(ctx context.Context, _ repo) error {
		escaped = context.WithoutCancel(ctx)

		return nil
	})

	calls := 0

	_ = Retry(escaped, 3, func(context.Context) error {
		calls++

		return stateErr("40001")
	})

	if calls != 3 {
		t.Fatalf("calls = %d, want 3", calls)
	}
}
