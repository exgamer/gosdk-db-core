# transactionmanager

`Manager`/`Manager2`/`Manager3`/`Manager4`/`Manager5` — менеджеры
Postgres-транзакций для доменных сервисов. Repository-конструкторы не
меняются вообще — `Manager` пересобирает тот же самый репозиторий на
`*gorm.DB` транзакции вместо обычного клиента.

## Пример (реальный, `internal/domains/handbook/city`)

**1. Домен объявляет свой интерфейс** — `internal/domains/handbook/city/tx_manager.go`. `LogRepository` в конструктор `Service` не попадает — он нигде не нужен вне `Exec`-колбэка, только `Repository` (используется и внутри транзакции, и напрямую в обычных методах):

```go
type TxManager interface {
	Exec(ctx context.Context, fn func(ctx context.Context, repo Repository, logRepo LogRepository) error) error
}

func NewService(repository Repository, txManager TxManager, hub *ws.Hub) *Service
```

**2. Bootstrap заводит два раздельных поля — обычный репозиторий и менеджер под конкретную операцию.** Брать "обычный" репозиторий через `CityCreateTxManager.Default()` было бы нелогично: имя поля говорит "это для Create", а `GetById`/`Update`/... к созданию не относятся:

```go
// app/bootstrap/city/repositories_factory.go
type repositoriesFactory struct {
	PostgresRepository  *citypostgres.PostgresRepository                      // для GetById/Update/Activate/Deactivate/Delete/Paginated
	CityCreateTxManager *transactionmanager.Manager2[city.Repository, city.LogRepository]    // только для Create/CreateBatch
	HttpRepository      *cityhttp.HttpRepository
}

PostgresRepository: citypostgres.NewPostgresRepository(client),
CityCreateTxManager: transactionmanager.NewManager2[city.Repository, city.LogRepository](
	client,
	func(db *gorm.DB) city.Repository    { return citypostgres.NewPostgresRepository(db) },
	func(db *gorm.DB) city.LogRepository { return citypostgres.NewLogPostgresRepository(db) },
),
```

**3. Сервис вызывает `Exec` там, где нужна атомарность:**

```go
func (s *Service) Create(ctx context.Context, model *City) (*City, error) {
	var created *City
	err := s.txManager.Exec(ctx, func(ctx context.Context, repo Repository, logRepo LogRepository) error {
		var err error
		created, err = repo.Create(ctx, model)
		if err != nil {
			return err
		}
		_, err = logRepo.Create(ctx, &CreationLog{CityID: created.ID, Message: "city created: " + created.Name})
		return err
	})
	...
}
```

## Важно: менеджер — один на операцию, не один на всё приложение

Каждой атомарной (transactional) операции соответствует **свой** `Manager`/`Manager2`/`Manager3`/`Manager4`/`Manager5` — той арности, которая нужна именно ей, а не единый универсальный менеджер на всё приложение или даже на весь модуль. `city` использует только один `CityCreateTxManager` — под связку `Repository`+`LogRepository`, нужную `Create`/`CreateBatch`; если бы появилась другая операция, которой атомарно нужны, скажем, `StatsRepository`+`AuditRepository`, — под неё завёлся бы отдельный, независимый `Manager2[StatsRepository, AuditRepository]` со своим именем (например, `RecalculateStatsTxManager`), никак не связанный с первым.

Отсюда и правило именования: поле называть не по типу-реализации ("PostgresManager", "TxManager2"), а по тому, какую атомарную операцию оно обслуживает.

И правило для репозитория: если он нужен и вне транзакции (обычные CRUD-методы) — у него всегда своё обычное поле (`PostgresRepository`), собранное напрямую через конструктор, а не через `.Default()` менеджера, у которого своё, более узкое назначение.

## Таймаут транзакции

`Exec` сам ограничивает транзакцию `10 * time.Second` (тот же magic number, что уже используется в каждом repository-методе на один запрос) — независимо от того, есть ли дедлайн у входящего `ctx`. Без этого транзакция была бы ограничена только вызывающей стороной: для HTTP это `HANDLER_TIMEOUT` (30s), а для остальных путей (consumer, cron) — вообще ничем, и зависший `fn` держал бы Postgres-locks неограниченно долго.

Переопределяется сразу после конструктора, тем же паттерном, что уже используется в `app.go` для `RabbitKernel`:

```go
CityCreateTxManager: transactionmanager.NewManager2[city.Repository, city.LogRepository](
	client, buildCity, buildLog,
).WithTimeout(30 * time.Second),
```

Если входящий `ctx` уже содержит более короткий дедлайн (например, из `HANDLER_TIMEOUT`) — сработает более ранний из двух, `context.WithTimeout` берёт минимум автоматически.

## Почему несколько типов, а не один универсальный — это идиоматично для Go

В Go нет variadic generics — `Manager[R...]` на любое число репозиториев одним типом невозможен. Альтернатива — один нетипизированный менеджер поверх `map[reflect.Type]any`/`map[string]any` с `type assertion` в рантайме — рассматривалась и отклонена: она меняет ошибки компиляции на панику/ошибку в рантайме, что противоречит стилю остального проекта (явная инъекция зависимостей в bootstrap, никакого service locator).

Закрытый набор типов под конкретную арность — стандартная практика в Go-экосистеме, а не костыль: `errgroup`, `iter.Seq`/`iter.Seq2` в стандартной библиотеке, `samber/lo` (`Zip2`/`Zip3`/`Zip4`) устроены точно так же. `Manager6` и далее добавляются по тому же образцу, только когда реально понадобится — раньше не нужно.
