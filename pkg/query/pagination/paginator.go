package pagination

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// DefaultLimit Limit, если он не задан
	DefaultLimit = 20
	// MaxLimitCeiling Жёсткий потолок записей на странице: MaxLimit 0 или больше — этот потолок
	MaxLimitCeiling = 1000
	// maxOffset Наибольший OFFSET: страница дальше ограничивается, чтобы (page-1)*limit не переполнялся
	maxOffset = math.MaxInt32
)

// ErrInvalidOrderBy Строка Paging.OrderBy — не колонка с необязательным asc/desc
var ErrInvalidOrderBy = errors.New("pagination: invalid order_by")

// orderByPattern Колонка (`name`, `table.name`) и необязательное направление
var orderByPattern = regexp.MustCompile(`(?i)^([a-z_][a-z0-9_]*(?:\.[a-z_][a-z0-9_]*)*)(?:\s+(asc|desc))?$`)

// Pages Постраничная выборка: count, затем страница с LIMIT/OFFSET. Ошибка — при неверном Paging.OrderBy
// (ErrInvalidOrderBy) и при ошибке запросов
func Pages(p *Param, result interface{}) (paginator *Pagination, err error) {
	var (
		db     = p.DB.Session(&gorm.Session{})
		count  int64
		offset uint
	)

	// if not defined
	if p.Paging == nil {
		p.Paging = &Paging{}
	}

	order, err := parseOrderBy(p.Paging.OrderBy)
	if err != nil {
		return nil, err
	}

	// get all counts
	if errCount := getCounts(db, result, &count); errCount != nil {
		return nil, fmt.Errorf("pagination: count: %w", errCount)
	}

	// debug sql
	if p.Paging.ShowSQL {
		db = db.Debug()
	}

	normalize(p.Paging)

	if p.Paging.Page > 1 {
		offset = (p.Paging.Page - 1) * p.Paging.Limit
	}

	// sort
	if len(order) > 0 {
		for _, o := range order {
			db = db.Order(o)
		}
	} else if def, ok := defaultOrder(db, result); ok {
		db = db.Order(def)
		p.Paging.OrderBy = append(p.Paging.OrderBy, def.Column.Name+" desc")
	}

	// get
	if errGet := db.Limit(int(p.Paging.Limit)).Offset(int(offset)).Find(result).Error; errGet != nil && !errors.Is(errGet, gorm.ErrRecordNotFound) {
		return nil, errGet
	}

	// total pages
	total := uint((uint64(count) + uint64(p.Paging.Limit) - 1) / uint64(p.Paging.Limit))

	// construct pagination
	paginator = &Pagination{
		TotalRecords: uint64(count),
		Page:         p.Paging.Page,
		Offset:       offset,
		Limit:        p.Paging.Limit,
		TotalPage:    total,
		PrevPage:     p.Paging.Page,
		NextPage:     p.Paging.Page,
	}

	paginator.From = p.Paging.Limit * (p.Paging.Page - 1)

	// prev page
	if p.Paging.Page > 1 {
		paginator.PrevPage = p.Paging.Page - 1
	}

	// next page
	paginator.NextPage = p.Paging.Page + 1

	if p.Paging.Page > paginator.TotalPage {
		paginator.NextPage = paginator.TotalPage
		paginator.PrevPage = paginator.NextPage - 1
	}

	if paginator.NextPage > paginator.TotalPage {
		paginator.NextPage = paginator.TotalPage
	}

	if paginator.TotalPage == 0 {
		paginator.From = 0
		paginator.To = 0
		paginator.PrevPage = 0
	}

	return paginator, nil
}

// normalize Limit: 0 — DefaultLimit, не больше MaxLimit (0 или больше MaxLimitCeiling — MaxLimitCeiling);
// Page: 0 — 1, не дальше страницы с OFFSET maxOffset
func normalize(paging *Paging) {
	if paging.MaxLimit == 0 || paging.MaxLimit > MaxLimitCeiling {
		paging.MaxLimit = MaxLimitCeiling
	}

	if paging.Limit == 0 {
		paging.Limit = DefaultLimit
	}

	//Обработка ограничения максимального количества записей на странице
	if paging.Limit > paging.MaxLimit {
		paging.Limit = paging.MaxLimit
	}

	if paging.Page < 1 {
		paging.Page = 1
	}

	if lastPage := uint(maxOffset)/paging.Limit + 1; paging.Page > lastPage {
		paging.Page = lastPage
	}
}

// parseOrderBy Строки OrderBy в колонки сортировки; имя колонки — в нижнем регистре (как без кавычек в
// Postgres), в SQL — в кавычках диалекта. Неверная строка — ErrInvalidOrderBy
func parseOrderBy(orderBy []string) ([]clause.OrderByColumn, error) {
	columns := make([]clause.OrderByColumn, 0, len(orderBy))

	for _, o := range orderBy {
		m := orderByPattern.FindStringSubmatch(strings.TrimSpace(o))
		if m == nil {
			return nil, fmt.Errorf("%w: %q", ErrInvalidOrderBy, o)
		}

		columns = append(columns, clause.OrderByColumn{
			Column: clause.Column{Name: strings.ToLower(m[1])},
			Desc:   strings.EqualFold(m[2], "desc"),
		})
	}

	return columns, nil
}

// defaultOrder Сортировка по умолчанию — первичный ключ модели по убыванию, с именем таблицы запроса. Не
// применяется, если у запроса уже есть ORDER BY, GROUP BY или DISTINCT, и если первичный ключ не найден
func defaultOrder(db *gorm.DB, result interface{}) (clause.OrderByColumn, bool) {
	stmt := db.Statement

	for _, name := range []string{"ORDER BY", "GROUP BY"} {
		if _, ok := stmt.Clauses[name]; ok {
			return clause.OrderByColumn{}, false
		}
	}

	if stmt.Distinct {
		return clause.OrderByColumn{}, false
	}

	model := stmt.Model
	if model == nil {
		model = result
	}

	if model == nil {
		return clause.OrderByColumn{}, false
	}

	parsed := &gorm.Statement{DB: db}
	if err := parsed.Parse(model); err != nil {
		return clause.OrderByColumn{}, false
	}

	pk := parsed.Schema.PrioritizedPrimaryField
	if pk == nil || pk.DBName == "" {
		return clause.OrderByColumn{}, false
	}

	return clause.OrderByColumn{
		Column: clause.Column{Table: clause.CurrentTable, Name: pk.DBName},
		Desc:   true,
	}, true
}

// getCounts Число строк выборки. С GROUP BY или DISTINCT — count(*) по подзапросу с теми же SELECT и условиями:
// число групп или различных строк, без выборки строк в приложение. Таблица — из Model или Table запроса; тип
// результата (anyType) — только если запрос их не задал: результат-DTO не подменяет таблицу выборки
func getCounts(db *gorm.DB, anyType interface{}, count *int64) error {
	if db.Statement.Model == nil && db.Statement.Table == "" {
		db = db.Model(anyType)
	}

	_, grouped := db.Statement.Clauses["GROUP BY"]
	if !grouped && !db.Statement.Distinct {
		return db.Select("*").Count(count).Error
	}

	return db.Session(&gorm.Session{NewDB: true}).Table("(?) AS pagination_count", db).Count(count).Error
}

func (p Pagination) IsEmpty() bool {
	return p.TotalRecords <= 0
}
