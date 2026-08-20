// Package transactionmanager — Manager/Manager2/Manager3/Manager4/Manager5:
// по типу на число репозиториев в одной Postgres-транзакции. В Go нет
// variadic generics, поэтому один Manager[R...] невозможен — тот же case,
// что у errgroup и lo.Zip2/Zip3/Zip4. Такой набор типов под конкретную
// арность — идиоматичный для Go подход
//
// Строители — функции, не готовые репозитории: каждый Exec открывает свою
// транзакцию и должен получить репозиторий именно на неё, а не на пул.
package transaction

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// defaultTimeout — верхняя граница на всю транзакцию (Exec), тот же
// magic number, что уже используется на каждый отдельный DB-запрос в
// репозиториях этого проекта (context.WithTimeout(ctx, 10*time.Second)).
// Без него транзакция ограничена только входящим ctx: для HTTP это
// HANDLER_TIMEOUT (30s), а для любого другого вызова (consumer, cron) —
// вообще ничем, транзакция может держать locks неограниченно долго.
const defaultTimeout = 10 * time.Second

// Manager runs fn inside a single Postgres transaction. build rebuilds a
// repository bound to that transaction's *gorm.DB, using the exact same
// constructor the repository already has — no changes are required in any
// repository to support this.
type Manager[R any] struct {
	client  *gorm.DB
	build   func(tx *gorm.DB) R
	timeout time.Duration
}

func NewManager[R any](client *gorm.DB, build func(tx *gorm.DB) R) *Manager[R] {
	return &Manager[R]{
		client:  client,
		build:   build,
		timeout: defaultTimeout,
	}
}

// WithTimeout переопределяет дефолтный таймаут транзакции (10s). Вызывать
// сразу после NewManager, до начала обработки запросов.
func (m *Manager[R]) WithTimeout(d time.Duration) *Manager[R] {
	m.timeout = d
	return m
}

// Default returns a plain, non-transactional instance bound to the regular
// connection pool — the same thing build(client) would give you directly.
func (m *Manager[R]) Default() R {
	return m.build(m.client)
}

// Exec commits everything fn does through repo if fn returns nil, and rolls
// back everything if fn returns an error (or panics). The transaction is
// bounded by m.timeout regardless of ctx's own deadline (the earlier of the
// two wins, per context.WithTimeout semantics).
func (m *Manager[R]) Exec(ctx context.Context, fn func(ctx context.Context, repo R) error) error {
	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	return m.client.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(ctx, m.build(tx))
	})
}

// Manager2 is Manager for the case where one atomic operation needs two
// different repositories at once. There is no bundle/struct type involved —
// both repositories are passed to fn as separate, explicit parameters, so a
// domain still only ever depends on the exact repositories it needs.
type Manager2[R1, R2 any] struct {
	client  *gorm.DB
	build1  func(tx *gorm.DB) R1
	build2  func(tx *gorm.DB) R2
	timeout time.Duration
}

func NewManager2[R1, R2 any](client *gorm.DB, build1 func(tx *gorm.DB) R1, build2 func(tx *gorm.DB) R2) *Manager2[R1, R2] {
	return &Manager2[R1, R2]{
		client:  client,
		build1:  build1,
		build2:  build2,
		timeout: defaultTimeout,
	}
}

// WithTimeout переопределяет дефолтный таймаут транзакции (10s). Вызывать
// сразу после NewManager2, до начала обработки запросов.
func (m *Manager2[R1, R2]) WithTimeout(d time.Duration) *Manager2[R1, R2] {
	m.timeout = d
	return m
}

// Default returns plain, non-transactional instances of both repositories.
func (m *Manager2[R1, R2]) Default() (R1, R2) {
	return m.build1(m.client), m.build2(m.client)
}

// Exec commits everything fn does through repo1/repo2 if fn returns nil, and
// rolls back everything (across both repositories) if fn returns an error.
// The transaction is bounded by m.timeout regardless of ctx's own deadline.
func (m *Manager2[R1, R2]) Exec(ctx context.Context, fn func(ctx context.Context, repo1 R1, repo2 R2) error) error {
	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	return m.client.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(ctx, m.build1(tx), m.build2(tx))
	})
}

// Manager3 is Manager for the case where one atomic operation needs three
// different repositories at once. Same shape as Manager2 — no bundle/struct
// type, all three repositories passed to fn as separate, explicit params.
type Manager3[R1, R2, R3 any] struct {
	client  *gorm.DB
	build1  func(tx *gorm.DB) R1
	build2  func(tx *gorm.DB) R2
	build3  func(tx *gorm.DB) R3
	timeout time.Duration
}

func NewManager3[R1, R2, R3 any](
	client *gorm.DB,
	build1 func(tx *gorm.DB) R1,
	build2 func(tx *gorm.DB) R2,
	build3 func(tx *gorm.DB) R3,
) *Manager3[R1, R2, R3] {
	return &Manager3[R1, R2, R3]{
		client:  client,
		build1:  build1,
		build2:  build2,
		build3:  build3,
		timeout: defaultTimeout,
	}
}

// WithTimeout переопределяет дефолтный таймаут транзакции (10s). Вызывать
// сразу после NewManager3, до начала обработки запросов.
func (m *Manager3[R1, R2, R3]) WithTimeout(d time.Duration) *Manager3[R1, R2, R3] {
	m.timeout = d
	return m
}

// Default returns plain, non-transactional instances of all three repositories.
func (m *Manager3[R1, R2, R3]) Default() (R1, R2, R3) {
	return m.build1(m.client), m.build2(m.client), m.build3(m.client)
}

// Exec commits everything fn does through repo1/repo2/repo3 if fn returns
// nil, and rolls back everything (across all three repositories) if fn
// returns an error. The transaction is bounded by m.timeout regardless of
// ctx's own deadline.
func (m *Manager3[R1, R2, R3]) Exec(ctx context.Context, fn func(ctx context.Context, repo1 R1, repo2 R2, repo3 R3) error) error {
	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	return m.client.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(ctx, m.build1(tx), m.build2(tx), m.build3(tx))
	})
}

// Manager4 is Manager for the case where one atomic operation needs four
// different repositories at once. Same shape as Manager2/Manager3.
type Manager4[R1, R2, R3, R4 any] struct {
	client  *gorm.DB
	build1  func(tx *gorm.DB) R1
	build2  func(tx *gorm.DB) R2
	build3  func(tx *gorm.DB) R3
	build4  func(tx *gorm.DB) R4
	timeout time.Duration
}

func NewManager4[R1, R2, R3, R4 any](
	client *gorm.DB,
	build1 func(tx *gorm.DB) R1,
	build2 func(tx *gorm.DB) R2,
	build3 func(tx *gorm.DB) R3,
	build4 func(tx *gorm.DB) R4,
) *Manager4[R1, R2, R3, R4] {
	return &Manager4[R1, R2, R3, R4]{
		client:  client,
		build1:  build1,
		build2:  build2,
		build3:  build3,
		build4:  build4,
		timeout: defaultTimeout,
	}
}

// WithTimeout переопределяет дефолтный таймаут транзакции (10s). Вызывать
// сразу после NewManager4, до начала обработки запросов.
func (m *Manager4[R1, R2, R3, R4]) WithTimeout(d time.Duration) *Manager4[R1, R2, R3, R4] {
	m.timeout = d
	return m
}

// Default returns plain, non-transactional instances of all four repositories.
func (m *Manager4[R1, R2, R3, R4]) Default() (R1, R2, R3, R4) {
	return m.build1(m.client), m.build2(m.client), m.build3(m.client), m.build4(m.client)
}

// Exec commits everything fn does through repo1..repo4 if fn returns nil,
// and rolls back everything (across all four repositories) if fn returns an
// error. The transaction is bounded by m.timeout regardless of ctx's own
// deadline.
func (m *Manager4[R1, R2, R3, R4]) Exec(ctx context.Context, fn func(ctx context.Context, repo1 R1, repo2 R2, repo3 R3, repo4 R4) error) error {
	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	return m.client.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(ctx, m.build1(tx), m.build2(tx), m.build3(tx), m.build4(tx))
	})
}

// Manager5 is Manager for the case where one atomic operation needs five
// different repositories at once. Same shape as Manager2/Manager3/Manager4.
type Manager5[R1, R2, R3, R4, R5 any] struct {
	client  *gorm.DB
	build1  func(tx *gorm.DB) R1
	build2  func(tx *gorm.DB) R2
	build3  func(tx *gorm.DB) R3
	build4  func(tx *gorm.DB) R4
	build5  func(tx *gorm.DB) R5
	timeout time.Duration
}

func NewManager5[R1, R2, R3, R4, R5 any](
	client *gorm.DB,
	build1 func(tx *gorm.DB) R1,
	build2 func(tx *gorm.DB) R2,
	build3 func(tx *gorm.DB) R3,
	build4 func(tx *gorm.DB) R4,
	build5 func(tx *gorm.DB) R5,
) *Manager5[R1, R2, R3, R4, R5] {
	return &Manager5[R1, R2, R3, R4, R5]{
		client:  client,
		build1:  build1,
		build2:  build2,
		build3:  build3,
		build4:  build4,
		build5:  build5,
		timeout: defaultTimeout,
	}
}

// WithTimeout переопределяет дефолтный таймаут транзакции (10s). Вызывать
// сразу после NewManager5, до начала обработки запросов.
func (m *Manager5[R1, R2, R3, R4, R5]) WithTimeout(d time.Duration) *Manager5[R1, R2, R3, R4, R5] {
	m.timeout = d
	return m
}

// Default returns plain, non-transactional instances of all five repositories.
func (m *Manager5[R1, R2, R3, R4, R5]) Default() (R1, R2, R3, R4, R5) {
	return m.build1(m.client), m.build2(m.client), m.build3(m.client), m.build4(m.client), m.build5(m.client)
}

// Exec commits everything fn does through repo1..repo5 if fn returns nil,
// and rolls back everything (across all five repositories) if fn returns an
// error. The transaction is bounded by m.timeout regardless of ctx's own
// deadline.
func (m *Manager5[R1, R2, R3, R4, R5]) Exec(ctx context.Context, fn func(ctx context.Context, repo1 R1, repo2 R2, repo3 R3, repo4 R4, repo5 R5) error) error {
	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	return m.client.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(ctx, m.build1(tx), m.build2(tx), m.build3(tx), m.build4(tx), m.build5(tx))
	})
}
