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