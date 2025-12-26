package pagination

import (
	"errors"
	"math"

	"gorm.io/gorm"
)

// Endpoint for pagination
func Pages(p *Param, result interface{}) (paginator *Pagination, err error) {

	var (
		db       = p.DB.Session(&gorm.Session{})
		defPage  = 1
		defLimit = 20
		count    int64
		offset   int
	)

	// get all counts
	getCounts(db, result, &count)

	// if not defined
	if p.Paging == nil {
		p.Paging = &Paging{}
	}

	// debug sql
	if p.Paging.ShowSQL {
		db = db.Debug()
	}
	// limit
	if p.Paging.Limit == 0 {
		p.Paging.Limit = defLimit
	}
	//Обработка ограничения максимального количества записей на странице
	if p.Paging.Limit > p.Paging.MaxLimit {
		p.Paging.Limit = p.Paging.MaxLimit
	}
	// page
	if p.Paging.Page < 1 {
		p.Paging.Page = defPage
	} else if p.Paging.Page > 1 {
		offset = (p.Paging.Page - 1) * p.Paging.Limit
	}
	// sort
	if len(p.Paging.OrderBy) > 0 {
		for _, o := range p.Paging.OrderBy {
			db = db.Order(o)
		}
	} else {
		str := "id desc"
		p.Paging.OrderBy = append(p.Paging.OrderBy, str)
	}

	// get
	if errGet := db.Limit(p.Paging.Limit).Offset(offset).Find(result).Error; errGet != nil && !errors.Is(errGet, gorm.ErrRecordNotFound) {
		return nil, errGet
	}

	// total pages
	total := int(math.Ceil(float64(count) / float64(p.Paging.Limit)))

	// construct pagination
	paginator = &Pagination{
		TotalRecords: count,
		Page:         p.Paging.Page,
		Offset:       offset,
		Limit:        p.Paging.Limit,
		TotalPage:    total,
		PrevPage:     p.Paging.Page,
		NextPage:     p.Paging.Page,
	}

	var pge = 0

	if p.Paging.Page > 0 {
		pge = p.Paging.Page - 1
	}

	paginator.From = p.Paging.Limit * pge

	// prev page
	if p.Paging.Page > 1 {
		paginator.PrevPage = p.Paging.Page - 1
	}

	if paginator.PrevPage < 0 {
		paginator.PrevPage = 1
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

func getCounts(db *gorm.DB, anyType interface{}, count *int64) {
	db.Select("*").Model(anyType).Count(count)
}

func (p Pagination) IsEmpty() bool {
	return p.TotalRecords <= 0
}
