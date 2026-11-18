package pagination

// Paging Параметры страницы из запроса клиента. MaxLimit и ShowSQL задаёт только код сервиса: из JSON и
// form-параметров они не читаются
type Paging struct {
	Page uint `json:"page"`
	// OrderBy Сортировка: каждая строка — колонка (`name`, `table.name`) и необязательное `asc`/`desc`.
	// Другая строка — ошибка ErrInvalidOrderBy из Pages
	OrderBy []string `json:"order_by"`
	Limit   uint     `json:"limit"`
	// MaxLimit Потолок Limit; 0 или больше MaxLimitCeiling — MaxLimitCeiling
	MaxLimit uint `json:"-" form:"-"`
	ShowSQL  bool `json:"-" form:"-"`
}
