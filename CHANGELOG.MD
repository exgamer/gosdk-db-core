# Changelog

## [1.4.0]

Зависимость `gosdk-core` поднята до v1.3.0.

Постраничная выборка без SQL-инъекции через сортировку и с лимитами, транзакции через ctx, лимиты пула по умолчанию.
Сигнатуры не изменились, новых переменных окружения нет.

### Исправлено

- `pagination.Pages`: строка `Paging.OrderBy` (`json:"order_by"`) уходила в `ORDER BY` как есть — SQL-инъекция из
  запроса клиента. Теперь — только колонка (`name`, `table.name`) с необязательным `asc`/`desc`, в кавычках диалекта,
  имя в нижнем регистре; другая строка — ошибка `pagination.ErrInvalidOrderBy`, запросы не выполняются.
- `Paging.MaxLimit` и `Paging.ShowSQL` читались из JSON/form-параметров клиента — теперь `json:"-" form:"-"`.
- `MaxLimit` 0 давал `LIMIT 0` (пустая страница); теперь 0 или больше 1000 — потолок 1000
  (`pagination.MaxLimitCeiling`). `TotalPage` считается целочисленно, без деления на ноль.
- Сортировка по умолчанию `id desc` записывалась в `Paging.OrderBy`, но не применялась к запросу — теперь применяется:
  первичный ключ модели по убыванию с именем таблицы (`"banners"."id" DESC`), если у запроса нет своего ORDER BY,
  GROUP BY или DISTINCT.
- Номер страницы ограничен: `(page-1)*limit` больше не переполняется (OFFSET не больше 2^31-1).
- Count запроса с GROUP BY или DISTINCT — `SELECT count(*) FROM (<запрос>) AS pagination_count` вместо выборки строки
  на каждую группу.
- `GormPaginatedHelper.SetTimeout(time.Duration)` умножал значение на секунду (`SetTimeout(5*time.Second)` —
  переполнение и мгновенный таймаут). Теперь — настоящий `Duration`; значение меньше миллисекунды — секунды
  (`SetTimeout(10)` — 10 с, как раньше); 0 и меньше — не меняет таймаут.
- Транзакции: вложенный `Exec` открывал вторую независимую транзакцию (своё соединение пула, взаимоблокировка с
  внешней, фиксация отдельно). Теперь `Exec` кладёт транзакцию в `ctx`, вложенный `Exec` на том же подключении идёт
  точкой сохранения внешней транзакции.
- Count в `Pages` брал таблицу из типа результата (`Model(result)`), затирая `Model` запроса: с результатом-DTO count
  с GROUP BY/DISTINCT падал (`relation "..." does not exist`), без них — считал не ту таблицу. Теперь тип результата —
  только если у запроса нет `Model` и `Table`.
- `WithTimeout(d)` с `d ≤ 0` давал мгновенный таймаут каждой транзакции — теперь игнорируется.
- `GetGormConnection`: пул без пределов — теперь `MaxOpenConnections` по умолчанию 25, `MaxIdleConnections` 10 (не
  больше открытых), `ConnMaxIdleTime` 5 мин; заданные в конфиге значения не меняются.

### Добавлено

- `transaction.FromContext(ctx, client)` — транзакция `Exec` из ctx (на том же подключении) или `client`;
  `transaction.InTransaction(ctx)`, `transaction.WithoutTransaction(ctx)` — ctx для независимой транзакции.
- `ExecWithOptions(ctx, *sql.TxOptions, fn)` у `Manager`…`Manager5` — уровень изоляции, read-only.
- `transaction.Retry(ctx, attempts, fn)` и `transaction.IsRetryable(err)` — повтор транзакции при SQLSTATE `40001` и
  `40P01`, только явно и не внутри открытой транзакции.
- `helpers.EscapeLike(s)` (`pkg/query/helpers`) — экранирует `%`, `_`, `\` для LIKE/ILIKE.
- Константы `pagination.DefaultLimit`, `pagination.MaxLimitCeiling`, ошибка `pagination.ErrInvalidOrderBy`.

### ⚠️ Изменения поведения

- `Pages` возвращает ошибку `ErrInvalidOrderBy` на `order_by` с выражением, функцией, `nulls last`, несколькими
  колонками в одной строке. Сервис отвечает клиенту 400/422 (`errors.Is(err, pagination.ErrInvalidOrderBy)`), а не
  500. Сортировку выражением задавать в колбэке запроса, а не через `Paging.OrderBy`. Имя колонки идёт в кавычках в
  нижнем регистре: колонку со смешанным регистром в схеме (`"createdAt"`) так не отсортировать.
- Страница не больше 1000 записей, даже если `MaxLimit` больше; `MaxLimit` из JSON запроса больше не действует.
- Без `order_by` страница отсортирована по первичному ключу по убыванию (раньше — порядок БД).
- Вложенный `Exec` с ctx внешнего `fn` больше не фиксируется независимо: откат внешней транзакции откатывает и его.
  Если нужна независимая фиксация (например, журнал при откате) — `Exec(transaction.WithoutTransaction(ctx), ...)`,
  понимая риск взаимоблокировки с внешней транзакцией.
- Вложенные `Exec` с одним ctx из разных горутин выполняются по очереди, каждый целиком: соединение у транзакции одно,
  и `ROLLBACK TO` одного откатывал записи другого. Внешний `fn` перед фиксацией ждёт уже идущий вложенный `Exec`.
  Параллельная независимая работа — `Exec(transaction.WithoutTransaction(ctx), ...)`.
- ctx, переживший `Exec` (отложенный `go m.Exec(context.WithoutCancel(ctx), ...)`), после фиксации или отката не несёт
  транзакцию: `Exec` с ним открывает новую (раньше — ошибка `transaction has already been committed`), `FromContext` —
  `client`, `InTransaction` — `false`.
- Без `MaxOpenConnections` пул ограничен 25 соединениями: запросы сверх предела ждут свободного соединения (до
  таймаута ctx). Под высокой нагрузкой задать `POSTGRES_DB_MAX_OPEN_CONNECTIONS` явно.
- Тестовая зависимость `github.com/DATA-DOG/go-sqlmock` v1.5.2 в `go.mod`.

## [1.3.1]

Зависимость `gosdk-core` поднята до v1.2.0. API пакета не изменился.

## [1.3.0]

Логгер GORM можно настроить не писать «record not found». По умолчанию поведение не изменилось.

### Добавлено

- `DB_IGNORE_RECORD_NOT_FOUND=true` — `First`/`Take`/`Last` без строки (`gorm.ErrRecordNotFound`) не пишется в
  лог GORM как ошибка запроса: это обычный результат поиска, а не сбой БД. По умолчанию пишется, как раньше.

## [1.2.0]

Логгер GORM не пишет запросы, прерванные отменой контекста. API пакета не изменился.

### Исправлено

- `GetGormConnection`: на уровнях `LOG_LEVEL` info и выше запрос с ошибкой `context.Canceled`
  (клиент закрыл соединение, контекст запроса отменён) не пишется в лог GORM как ошибка запроса.
  `context.DeadlineExceeded` и остальные ошибки пишутся как раньше.

### ⚠️ Изменения поведения

- На уровнях `LOG_LEVEL` info/warn/error строки `context canceled` из лога GORM пропадают. На
  debug/trace - как раньше. Затрагивает все сервисы на `gosdk-postgres-core` (`PostgresKernel`
  подключается через `GetGormConnection`). Действий от сервиса не требуется; алерты по строке
  `context canceled` в логе GORM перестанут срабатывать.

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
