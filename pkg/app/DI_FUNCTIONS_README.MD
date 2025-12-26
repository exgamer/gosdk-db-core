##  Что доступно из DI после инициализации приложения

Db конфиг приложения:

```go
router, err := app.GetDbConfig(a *app.App) (*config.DbConfig, error)
```

