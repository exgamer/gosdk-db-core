# Changelog

## [1.1.0]

Зависимость `gosdk-core` поднята до v1.1.0. API пакета не изменился.

### Изменено

- `gosdk-core` v1.0.29 → v1.1.0.

### ⚠️ Изменения поведения

- Требует `gosdk-core` v1.1.0: поднимать вместе с `gosdk-http-core` ≥ v1.2.0, иначе сервис теряет
  AppInfo (имя сервиса) в логах запросов и событиях Sentry. AppInfo читать через
  `context.GetAppInfoFromContext`.
- Логгер экранирует управляющие символы в сообщениях (многострочное сообщение - одна строка с `\n`),
  `logger.Dump` пишет значение одной строкой.

## [1.0.22]

Минимальная версия Go поднята до 1.27.3. API и поведение пакета не изменились.

### Изменено

- `go.mod`: `go 1.26.0` → `go 1.27.3`. Причина - уязвимости стандартной библиотеки go1.26.0
  (`govulncheck`), исправлены в новых версиях Go. На go1.27.3 `govulncheck` уязвимостей стандартной библиотеки не находит.
- `gosdk-core` v1.0.28 → v1.0.29 (тоже `go 1.27.3`).

### ⚠️ Изменения поведения

- Потребителю нужен Go ≥ 1.27.3: после `go get` этой версии в его `go.mod` станет `go 1.27.3`.
  С `GOTOOLCHAIN=auto` toolchain скачается сам; с `GOTOOLCHAIN=local` на Go < 1.27.3 или в образе
  без доступа к `proxy.golang.org` сборка упадёт - образ сборки `golang:1.27.3`.

## [1.0.21]

Параметры SQL не попадают в лог GORM и в Sentry на рабочих уровнях логирования, `pagination.Pages`
возвращает ошибку `COUNT`, миграции можно прогнать в транзакции. Экспортированные типы, функции
и сигнатуры не изменились, новое только добавлено.

### Добавлено

- `migration.Options{UseTransaction bool}` и функции `RunWithOptions`, `RollbackLastWithOptions`,
  `RollbackToWithOptions`. `UseTransaction: true` - все миграции прогона в одной транзакции: упавшая
  миграция из нескольких DDL не оставляет схему наполовину изменённой. Подробности:
  [pkg/migration/README.md](pkg/migration/README.md).

### Исправлено

- `GetGormConnection`: на уровнях `LOG_LEVEL` info и выше логгер GORM пишет SQL с плейсхолдерами
  (`$1`) без значений параметров (`ParameterizedQueries`) - пароли, хеши и персональные данные
  не уходят в лог.
- `SlowSqlSentryMiddleware`: значения параметров запроса не отправляются в Sentry.
- `pagination.Pages`: ошибка запроса `COUNT` больше не теряется - возвращается как
  `pagination: count: <ошибка>`.

### ⚠️ Изменения поведения

- На уровнях `LOG_LEVEL` info/warn/error в логе GORM SQL с плейсхолдерами `$1` вместо значений
  (раньше - со значениями). На debug/trace - как раньше, со значениями. Затрагивает все сервисы
  на `gosdk-postgres-core` (`PostgresKernel` подключается через `GetGormConnection`).
- `GetGormConnection` строит свой логгер с настройками `logger.Default` gorm v1.31.2 (stdout,
  `SlowThreshold` 200 мс, `Colorful`, уровень Info на debug, иначе Error), а не берёт
  переменную `gorm.io/gorm/logger.Default`: подмена `logger.Default` в сервисе больше не влияет
  на соединение из `GetGormConnection`.
- В контексте «Sql data» события Sentry «Slow SQL Request» больше нет ключа `Sql Params`.
  `Table`, SQL, `Duration`, `Rows affected`, `Operation`, `Sql Error` - как раньше.
- Ошибка `COUNT` в `pagination.Pages` (и `GormPaginatedHelper.Paginated`) теперь ошибка
  (раньше - `total = 0` без ошибки, список при этом мог вернуться непустым). Затрагивает
  репозитории на `NewGormPaginatedHelper`: `go-sdk-rest-template` (`handbook/city`) и
  репозитории, сгенерированные `gosdk-generator` (`templates/infra/postgres/repository.go.tmpl`).
- Без изменений: `Run`, `RollbackLast`, `RollbackTo` - как раньше, с `gormigrate.DefaultOptions`
  (без транзакции), в том числе `Migrator` из `gosdk-postgres-core`; `GormDebugMiddleware`
  по-прежнему собирает параметры в debug-коллектор (он есть только на debug-уровне).
