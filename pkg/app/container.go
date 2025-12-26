package app

import (
	"github.com/exgamer/gosdk-core/pkg/app"
	"github.com/exgamer/gosdk-core/pkg/di"
	"github.com/exgamer/gosdk-db-core/pkg/config"
)

// GetDbConfig возвращает Db Config.
func GetDbConfig(a *app.App) (*config.DbConfig, error) {
	c, err := di.Resolve[*config.DbConfig](a.Container)

	if err != nil {
		return nil, err
	}

	return c, nil
}
