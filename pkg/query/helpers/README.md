## Постраничная выборка: правила

- `pagination.Paging.OrderBy` (`json:"order_by"`) — только колонки: `name`, `table.name`, с необязательным `asc`/`desc`
  (без учёта регистра). Другая строка (выражение, функция, `;`, `nulls last`) — ошибка `pagination.ErrInvalidOrderBy`
  из `Pages`: отвечать клиенту 400/422. Колонка уходит в SQL в кавычках, имя приводится к нижнему регистру.
  Сортировку выражением задавать в колбэке запроса (`client.Order(...)`), не из данных клиента.
- `MaxLimit` и `ShowSQL` из JSON и form-параметров не читаются. `MaxLimit` 0 или больше 1000 — 1000
  (`pagination.MaxLimitCeiling`); `Limit` 0 — 20 (`pagination.DefaultLimit`); страница ограничена так, чтобы
  OFFSET не превышал 2^31-1.
- Без `OrderBy` — сортировка по первичному ключу по убыванию (`"table"."id" DESC`), если у запроса нет своего
  ORDER BY, GROUP BY или DISTINCT.
- Запрос с GROUP BY или DISTINCT считается подзапросом: `SELECT count(*) FROM (<запрос>) AS pagination_count`.
- `GormPaginatedHelper.SetTimeout(5 * time.Second)` — `time.Duration`; значение меньше миллисекунды — секунды
  (старый вызов `SetTimeout(10)` — 10 с); 0 и меньше — таймаут не меняется (по умолчанию 10 с).
- Поиск подстрокой: `Where("name ILIKE ?", "%"+helpers.EscapeLike(q)+"%")` — `%`, `_`, `\` из ввода не работают
  как шаблон.

Пример запроса постраничного списка

```go
package app

// Paginated Постраничный список
func (repo *LoyaltyBannerRepository) Paginated(ctx context.Context, page int, perPage int) (*pagination.Paginated[models.LoyaltyBanner], error) {
	helper := helpers.NewGormPaginatedHelper[models.LoyaltyBanner](ctx, repo.client).SetPerPage(perPage)
	result, err := helper.Paginated(page, func(client *gorm.DB) *gorm.DB {

		var query []string
		var args []any

		if bannerSearchDto.Id > 0 {
			query = append(query, "banners.id=?")
			args = append(args, bannerSearchDto.Id)
		}

		if bannerSearchDto.LocationId > 0 {
			query = append(query, "banners.location_id=?")
			args = append(args, bannerSearchDto.LocationId)
		}

		if bannerSearchDto.LocationSectionId > 0 {
			query = append(query, "banners.location_section_id=?")
			args = append(args, bannerSearchDto.LocationSectionId)
		}

		return client.WithContext(ctx).
			Select("banners.*").
			Joins("INNER JOIN banner_descriptions bd ON banners.id = bd.banner_id").
			Joins("INNER JOIN locations loc ON banners.location_id = loc.id").
			Joins("INNER JOIN location_sections sec ON banners.location_section_id = sec.id").
			Joins("LEFT JOIN banner_categories bcat ON banners.id = bcat.banner_id").
			Joins("LEFT JOIN banner_companies bcom ON banners.id = bcom.banner_id").
			Joins("LEFT JOIN banner_cities bcit ON banners.id = bcit.banner_id").
			Where(strings.Join(query, " AND "), args...).
			Order(orderBy).
			Group("banners.id")
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

```

Все остальные запросы делаются согласно документации gorm

```go
type CityRepository struct {
	client *gorm.DB
}

func (r *CityRepository) GetCityById(ctx context.Context, id uint) (*models.City, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var model models.City
	result := r.client.WithContext(ctx).
		Where("id = ?", id).
		First(&model)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	if result.Error != nil {
		return nil, result.Error
	}

	return &model, nil
}



```