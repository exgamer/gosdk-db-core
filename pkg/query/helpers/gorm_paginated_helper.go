package helpers

import (
	"context"
	"github.com/exgamer/gosdk-db-core/pkg/query/pagination"
	"gorm.io/gorm"
	"time"
)

func NewGormPaginatedHelper[E interface{}](ctx context.Context, client *gorm.DB) *GormPaginatedHelper[E] {
	return &GormPaginatedHelper[E]{
		client:     client,
		ctx:        ctx,
		perPage:    30,
		maxPerPage: 1000,
		timeout:    10,
	}
}

// GormPaginatedHelper - Вспомогательный хелпер для постраничного чтения данных
type GormPaginatedHelper[E interface{}] struct {
	client     *gorm.DB
	perPage    uint
	maxPerPage uint
	timeout    time.Duration
	model      E
	ctx        context.Context
}

func (h *GormPaginatedHelper[E]) SetContext(ctx context.Context) *GormPaginatedHelper[E] {
	h.ctx = ctx

	return h
}

func (h *GormPaginatedHelper[E]) SetTimeout(timeout time.Duration) *GormPaginatedHelper[E] {
	h.timeout = timeout

	return h
}

func (h *GormPaginatedHelper[E]) SetPerPage(perPage uint) *GormPaginatedHelper[E] {
	h.perPage = perPage

	return h
}

func (h *GormPaginatedHelper[E]) Paginated(page uint, callback func(client *gorm.DB) *gorm.DB) (*pagination.Paginated[E], error) {
	var structure pagination.Paginated[E]
	var err error
	paging := pagination.Paging{}
	paging.Page = page
	paging.Limit = h.perPage
	paging.MaxLimit = h.maxPerPage
	var ctx context.Context

	if h.ctx != nil {
		ctx = h.ctx
	} else {
		ctx = context.Background()
	}

	ctx, cancel := context.WithTimeout(ctx, h.timeout*time.Second)
	defer cancel()

	structure.Pagination, err = pagination.Pages(&pagination.Param{
		DB:     callback(h.client).WithContext(ctx),
		Paging: &paging,
	}, &structure.Items)

	if err != nil {
		return nil, err
	}

	structure.Pagination.To = structure.Pagination.From + uint(len(structure.Items))

	if len(structure.Items) == 0 {
		structure.Pagination.From = 0
	}

	structure.Pagination.From += 1

	return &structure, nil
}
