package database

import "gorm.io/gorm"

// DbConfig Модель данных для описания соединения с БД
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
	// ServiceName -  данное поле нужно для записи в сентри медленных запросов с отоборажением какой именно сервис вызывает это
	ServiceName string
	// Threshold - максимальный порог в сек, выше которого в сентри будут записыватся данные. Время в секундах,если не указано то дефолт 1 сек
	Threshold float64
}
