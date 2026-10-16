# migration

Обёртка над [gormigrate](https://github.com/go-gormigrate/gormigrate) для версионированных миграций схемы.
Блокировки от параллельного запуска здесь нет — её ставит вызывающая сторона (`gosdk-postgres-core`: `Migrator`).

## Функции

| Функция | Что делает |
|---|---|
| `Run(db, migrations)` | применяет ещё не применённые миграции |
| `RollbackLast(db, migrations)` | откатывает последнюю применённую миграцию |
| `RollbackTo(db, migrations, migrationID)` | откатывает миграции после `migrationID` (саму её — нет) |
| `RunWithOptions`, `RollbackLastWithOptions`, `RollbackToWithOptions` | то же с настройками `Options` |

Функции без `Options` работают как раньше — с `gormigrate.DefaultOptions` (таблица `migrations`, без транзакции).

## Options

```go
type Options struct {
	UseTransaction bool
}
```

`UseTransaction: true` — все миграции прогона выполняются в одной транзакции: если миграция из нескольких DDL
упала посередине, схема откатывается целиком, а не остаётся наполовину изменённой.
Подходит для СУБД с транзакционным DDL (PostgreSQL); MySQL DDL неявно коммитит, там опция не защищает.

```go
err := migration.RunWithOptions(db, migrations.All(), migration.Options{UseTransaction: true})
```

Нулевое значение `Options{}` — то же, что `Run`/`RollbackLast`/`RollbackTo`.
