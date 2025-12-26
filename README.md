# gosdk-db-core

SDK-пакет для работы с базами данных через **GORM v2**.  
Включает удобную инициализацию подключения, настройку пула соединений, логирование, а также плагины:

- **Slow SQL → Sentry** (отправка медленных запросов с указанием сервиса)
- **GORM Debug middleware** (сбор SQL/params/duration в debug-коллектор из `context.Context`)

## Возможности

- Инициализация `*gorm.DB` через общий `DbConfig`
- Настройка пула соединений: `MaxOpenConnections`, `MaxIdleConnections`, `ConnMaxLifetime`
- Опциональный логгер GORM (`logger.Info`)
- Подключение middleware через `db.Use(...)`
- Единый стиль хранения конфигурации для сервисов

## Установка

```bash
go get github.com/exgamer/gosdk-db-core
```

## Быстрый старт

### 1) Подготовка `DbConfig`

```go
import (
  "github.com/exgamer/gosdk-db-core/pkg/database"
  "gorm.io/driver/postgres"
)

cfg := &database.DbConfig{
  Dialector: postgres.Open(dsn),

  Logging: true,

  MaxOpenConnections: 50,
  MaxIdleConnections: 10,

  ServiceName: "catalog-service-go",
  Threshold:   1, // секунды
}
```

### 2) Получение подключения

```go
db, err := database.GetGormConnection(cfg)
if err != nil {
  panic(err)
}
```

## Конфигурация DbConfig

```go
type DbConfig struct {
  Dialector            gorm.Dialector

  Host                 string
  User                 string
  Password             string
  Db                   string
  Port                 string
  SslMode              bool

  MaxOpenConnections   int
  MaxIdleConnections   int

  Logging              bool
  DisableAutomaticPing bool

  ServiceName string
  Threshold   float64
}
```

## Middleware

### Slow SQL → Sentry

Отправляет события в Sentry для запросов, которые выполнялись дольше `Threshold`.

### Debug middleware

Собирает SQL-запросы, параметры и время выполнения в debug-коллектор из `context.Context`.

```go
type SqlStatement struct {
  Time      string        `json:"time,omitempty"`
  Operation string        `json:"operation,omitempty"`
  Sql       string        `json:"sql,omitempty"`
  Error     string        `json:"error,omitempty"`
  Params    []interface{} `json:"params,omitempty"`
  Duration  time.Duration `json:"duration,omitempty"`
}
```

## Рекомендации

- Всегда используйте `db.WithContext(ctx)`
- Настраивайте пул соединений под нагрузку
- Используйте `ServiceName` для удобной фильтрации slow SQL

## Пример

```go
cfg := &database.DbConfig{
  Dialector: postgres.Open(dsn),
  Logging: true,
  MaxOpenConnections: 50,
  MaxIdleConnections: 10,
  ServiceName: "my-service",
  Threshold: 1,
}

db, err := database.GetGormConnection(cfg)
if err != nil {
  panic(err)
}
```
