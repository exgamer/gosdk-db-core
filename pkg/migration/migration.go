// Package migration — тонкая обёртка над gormigrate для прогона
// версионированных миграций схемы через *gorm.DB. Полностью dialect-agnostic
// (как и остальной db-core) — ничего не знает про конкретную СУБД. В
// частности, здесь намеренно нет блокировки от параллельного запуска
// (advisory lock и подобное) — это СУБД-специфичный механизм (у Postgres
// свой синтаксис, у MySQL свой, у SQLite такого нет вовсе), поэтому он
// не может жить в dialect-agnostic коде; вызывающая сторона, знающая
// конкретную СУБД (см. gosdk-postgres-core: PostgresKernel.WithMigrations),
// оборачивает Run такой блокировкой сама.
//
// Это не Kernel и не подключается через RegisterAndInitKernels напрямую:
// gosdk-core.KernelManager инициализирует kernels в произвольном порядке
// (обход map, не списка), поэтому нет гарантии, что миграции применятся
// раньше, чем остальные kernels начнут работать с БД. Run вызывается внутри
// Init() того kernel'а, который открывает соединение — это гарантирует
// порядок, потому что шаги внутри одного Init() выполняются последовательно
// обычным Go-кодом, а не через map.
package migration

import (
	"log"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// Migration — ре-экспорт gormigrate.Migration, чтобы вызывающему коду не
// нужно было импортировать gormigrate напрямую.
type Migration = gormigrate.Migration

// Run применяет все ещё не применённые migrations к db, по одной, в порядке
// объявления. Пустой список — не ошибка: пишется лог, Migrate не
// вызывается. Никакой защиты от параллельного вызова из нескольких
// инстансов сервиса здесь нет — это ответственность вызывающей стороны
// (СУБД-специфичная блокировка, если нужна).
//
// migrations обычно — результат <project>/internal/migrations.All(),
// сгенерированной командой `codegen migration add`, но можно передать и
// явный список напрямую.
func Run(db *gorm.DB, migrations []*Migration) error {
	if len(migrations) == 0 {
		log.Println("migration: nothing to apply")

		return nil
	}

	log.Printf("migration: applying up to %d migration(s)", len(migrations))

	return gormigrate.New(db, gormigrate.DefaultOptions, migrations).Migrate()
}

// RollbackLast откатывает последнюю применённую миграцию из migrations,
// вызывая её Rollback. Пустой список — не ошибка: пишется лог, ничего не
// откатывается. Как и Run, не защищена от параллельного вызова из нескольких
// инстансов — это ответственность вызывающей стороны (СУБД-специфичная
// блокировка, если нужна).
//
// Это ручная/oncall-операция — в отличие от Run, её не следует вызывать
// автоматически при старте сервиса.
func RollbackLast(db *gorm.DB, migrations []*Migration) error {
	if len(migrations) == 0 {
		log.Println("migration: nothing to rollback")

		return nil
	}

	log.Println("migration: rolling back last migration")

	return gormigrate.New(db, gormigrate.DefaultOptions, migrations).RollbackLast()
}

// RollbackTo откатывает все применённые миграции из migrations, идущие после
// migrationID, в обратном порядке — саму migrationID не откатывает
// (семантика gormigrate.RollbackTo). Как и RollbackLast — ручная операция,
// без защиты от параллельного вызова.
func RollbackTo(db *gorm.DB, migrations []*Migration, migrationID string) error {
	if len(migrations) == 0 {
		log.Println("migration: nothing to rollback")

		return nil
	}

	log.Printf("migration: rolling back to %q (exclusive)", migrationID)

	return gormigrate.New(db, gormigrate.DefaultOptions, migrations).RollbackTo(migrationID)
}
