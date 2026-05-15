package service

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"

	"image2/backend/internal/dto"
	"image2/backend/internal/model"

	"gorm.io/gorm"
)

func (s *Service) ListProductCategories(ctx context.Context) (dto.ListProductCategoriesResponse, error) {
	var items []model.ProductCategory
	if err := s.db.WithContext(ctx).
		Where("enabled = ?", true).
		Order("sort_order asc, created_at asc").
		Find(&items).Error; err != nil {
		return dto.ListProductCategoriesResponse{}, err
	}
	return dto.ListProductCategoriesResponse{Items: s.productCategoriesToDTO(items)}, nil
}

func (s *Service) ListProducts(ctx context.Context, keyword string, categoryID string) (dto.ListProductsResponse, error) {
	query := s.db.WithContext(ctx).
		Model(&model.ProductItem{}).
		Preload("Categories", "enabled = ?", true).
		Where("product_items.enabled = ?", true)

	if normalizedCategoryID, err := parseOptionalUint64(categoryID); err != nil {
		return dto.ListProductsResponse{}, ErrProductCategoryInvalid
	} else if normalizedCategoryID > 0 {
		query = query.Joins("JOIN product_item_categories pic ON pic.product_item_id = product_items.id").
			Where("pic.product_category_id = ?", normalizedCategoryID)
	}

	if kw := strings.TrimSpace(keyword); kw != "" {
		like := "%" + kw + "%"
		query = query.Joins("LEFT JOIN product_item_categories pic_search ON pic_search.product_item_id = product_items.id").
			Joins("LEFT JOIN product_categories pc_search ON pc_search.id = pic_search.product_category_id").
			Where(
				"product_items.name LIKE ? OR product_items.short_description LIKE ? OR product_items.description LIKE ? OR pc_search.name LIKE ?",
				like, like, like, like,
			)
	}

	var items []model.ProductItem
	if err := query.Distinct("product_items.*").Order("product_items.sort_order asc, product_items.created_at desc").Find(&items).Error; err != nil {
		return dto.ListProductsResponse{}, err
	}
	return dto.ListProductsResponse{Items: s.productsToDTO(ctx, items)}, nil
}

func (s *Service) AdminListProductCategories(ctx context.Context, adminID uint64, page int, pageSize int) (dto.AdminListProductCategoriesResponse, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AdminListProductCategoriesResponse{}, err
	}
	page, pageSize = normalizePage(page, pageSize)
	query := s.db.WithContext(ctx).Model(&model.ProductCategory{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return dto.AdminListProductCategoriesResponse{}, err
	}
	var items []model.ProductCategory
	if err := query.Order("sort_order asc, created_at asc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return dto.AdminListProductCategoriesResponse{}, err
	}
	return dto.AdminListProductCategoriesResponse{
		Items:    s.productCategoriesToDTO(items),
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

func (s *Service) AdminCreateProductCategory(ctx context.Context, adminID uint64, req dto.AdminProductCategoryPayload) (dto.ProductCategoryItem, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.ProductCategoryItem{}, err
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	item := model.ProductCategory{
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		SortOrder:   req.SortOrder,
		Enabled:     enabled,
	}
	if strings.TrimSpace(item.Name) == "" {
		return dto.ProductCategoryItem{}, ErrProductCategoryInvalid
	}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		return recordAdminActionLog(tx, adminID, "product_category.create", "product_category", strconv.FormatUint(item.ID, 10), nil, item, item.Name)
	}); err != nil {
		return dto.ProductCategoryItem{}, err
	}
	return s.productCategoryToDTO(item), nil
}

func (s *Service) AdminUpdateProductCategory(ctx context.Context, adminID uint64, idText string, req dto.AdminProductCategoryPayload) (dto.ProductCategoryItem, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.ProductCategoryItem{}, err
	}
	id, err := parseRequiredUint64(idText)
	if err != nil {
		return dto.ProductCategoryItem{}, ErrProductCategoryInvalid
	}
	var updated model.ProductCategory
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.ProductCategory
		if err := tx.First(&item, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrProductCategoryInvalid
			}
			return err
		}
		before := item
		item.Name = strings.TrimSpace(req.Name)
		item.Description = strings.TrimSpace(req.Description)
		item.SortOrder = req.SortOrder
		if req.Enabled != nil {
			item.Enabled = *req.Enabled
		}
		if strings.TrimSpace(item.Name) == "" {
			return ErrProductCategoryInvalid
		}
		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		updated = item
		return recordAdminActionLog(tx, adminID, "product_category.update", "product_category", strconv.FormatUint(item.ID, 10), before, item, item.Name)
	}); err != nil {
		return dto.ProductCategoryItem{}, err
	}
	return s.productCategoryToDTO(updated), nil
}

func (s *Service) AdminDeleteProductCategory(ctx context.Context, adminID uint64, idText string) error {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return err
	}
	id, err := parseRequiredUint64(idText)
	if err != nil {
		return ErrProductCategoryInvalid
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.ProductCategory
		if err := tx.First(&item, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrProductCategoryInvalid
			}
			return err
		}
		var bindingCount int64
		if err := tx.Model(&model.ProductItemCategory{}).Where("product_category_id = ?", id).Count(&bindingCount).Error; err != nil {
			return err
		}
		if bindingCount > 0 {
			return ErrProductCategoryInUse
		}
		before := item
		if err := tx.Delete(&item).Error; err != nil {
			return err
		}
		return recordAdminActionLog(tx, adminID, "product_category.delete", "product_category", strconv.FormatUint(item.ID, 10), before, nil, item.Name)
	})
}

func (s *Service) AdminListProducts(ctx context.Context, adminID uint64, keyword string, categoryID string, enabled string, page int, pageSize int) (dto.AdminListProductsResponse, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AdminListProductsResponse{}, err
	}
	page, pageSize = normalizePage(page, pageSize)
	query := s.db.WithContext(ctx).Model(&model.ProductItem{}).Preload("Categories")

	if kw := strings.TrimSpace(keyword); kw != "" {
		like := "%" + kw + "%"
		query = query.Joins("LEFT JOIN product_item_categories pic_search ON pic_search.product_item_id = product_items.id").
			Joins("LEFT JOIN product_categories pc_search ON pc_search.id = pic_search.product_category_id").
			Where(
				"product_items.name LIKE ? OR product_items.short_description LIKE ? OR product_items.description LIKE ? OR pc_search.name LIKE ?",
				like, like, like, like,
			)
	}
	if normalizedCategoryID, err := parseOptionalUint64(categoryID); err != nil {
		return dto.AdminListProductsResponse{}, ErrProductCategoryInvalid
	} else if normalizedCategoryID > 0 {
		query = query.Joins("JOIN product_item_categories pic ON pic.product_item_id = product_items.id").
			Where("pic.product_category_id = ?", normalizedCategoryID)
	}
	switch strings.ToLower(strings.TrimSpace(enabled)) {
	case "true":
		query = query.Where("product_items.enabled = ?", true)
	case "false":
		query = query.Where("product_items.enabled = ?", false)
	}

	var total int64
	if err := query.Distinct("product_items.id").Count(&total).Error; err != nil {
		return dto.AdminListProductsResponse{}, err
	}
	var items []model.ProductItem
	if err := query.Distinct("product_items.*").Order("product_items.sort_order asc, product_items.created_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return dto.AdminListProductsResponse{}, err
	}

	return dto.AdminListProductsResponse{
		Items:    s.productsToDTO(ctx, items),
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

func (s *Service) AdminCreateProduct(ctx context.Context, adminID uint64, req dto.AdminProductPayload) (dto.ProductItem, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.ProductItem{}, err
	}
	item, categoryIDs, err := s.productPayloadToModel(req)
	if err != nil {
		return dto.ProductItem{}, err
	}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		categories, err := loadProductCategoriesByIDs(tx, categoryIDs)
		if err != nil {
			return err
		}
		item.Categories = categories
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		return recordAdminActionLog(tx, adminID, "product.create", "product", strconv.FormatUint(item.ID, 10), nil, item, item.Name)
	}); err != nil {
		return dto.ProductItem{}, err
	}
	return s.productToDTO(ctx, item), nil
}

func (s *Service) AdminUpdateProduct(ctx context.Context, adminID uint64, idText string, req dto.AdminProductPayload) (dto.ProductItem, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.ProductItem{}, err
	}
	id, err := parseRequiredUint64(idText)
	if err != nil {
		return dto.ProductItem{}, ErrProductInvalid
	}
	newItem, categoryIDs, err := s.productPayloadToModel(req)
	if err != nil {
		return dto.ProductItem{}, err
	}
	var updated model.ProductItem
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.ProductItem
		if err := tx.Preload("Categories").First(&item, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrProductInvalid
			}
			return err
		}
		before := item
		item.Name = newItem.Name
		item.ShortDescription = newItem.ShortDescription
		item.Description = newItem.Description
		item.PreviewURL = newItem.PreviewURL
		item.AmountCents = newItem.AmountCents
		item.ProductURL = newItem.ProductURL
		item.SortOrder = newItem.SortOrder
		item.Enabled = newItem.Enabled
		categories, err := loadProductCategoriesByIDs(tx, categoryIDs)
		if err != nil {
			return err
		}
		if err := tx.Model(&item).Association("Categories").Replace(categories); err != nil {
			return err
		}
		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		if err := tx.Preload("Categories").First(&item, id).Error; err != nil {
			return err
		}
		updated = item
		return recordAdminActionLog(tx, adminID, "product.update", "product", strconv.FormatUint(item.ID, 10), before, item, item.Name)
	}); err != nil {
		return dto.ProductItem{}, err
	}
	return s.productToDTO(ctx, updated), nil
}

func (s *Service) AdminDeleteProduct(ctx context.Context, adminID uint64, idText string) error {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return err
	}
	id, err := parseRequiredUint64(idText)
	if err != nil {
		return ErrProductInvalid
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.ProductItem
		if err := tx.First(&item, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrProductInvalid
			}
			return err
		}
		before := item
		if err := tx.Delete(&item).Error; err != nil {
			return err
		}
		return recordAdminActionLog(tx, adminID, "product.delete", "product", strconv.FormatUint(item.ID, 10), before, nil, item.Name)
	})
}

func (s *Service) productPayloadToModel(req dto.AdminProductPayload) (model.ProductItem, []uint64, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	item := model.ProductItem{
		Name:             strings.TrimSpace(req.Name),
		ShortDescription: strings.TrimSpace(req.ShortDescription),
		Description:      strings.TrimSpace(req.Description),
		PreviewURL:       strings.TrimSpace(req.PreviewURL),
		AmountCents:      req.AmountCents,
		ProductURL:       strings.TrimSpace(req.ProductURL),
		SortOrder:        req.SortOrder,
		Enabled:          enabled,
	}
	if err := validateProductItem(item); err != nil {
		return model.ProductItem{}, nil, err
	}
	categoryIDs, err := normalizeProductCategoryIDs(req.CategoryIDs)
	if err != nil {
		return model.ProductItem{}, nil, err
	}
	return item, categoryIDs, nil
}

func validateProductItem(item model.ProductItem) error {
	if strings.TrimSpace(item.Name) == "" {
		return ErrProductInvalid
	}
	if item.AmountCents <= 0 {
		return ErrProductInvalid
	}
	if item.ProductURL != "" && !strings.HasPrefix(item.ProductURL, "http://") && !strings.HasPrefix(item.ProductURL, "https://") {
		return ErrProductInvalid
	}
	if item.PreviewURL != "" {
		parsed, err := url.Parse(item.PreviewURL)
		if err != nil || (parsed.Scheme != "" && parsed.Host == "") {
			return ErrProductInvalid
		}
	}
	return nil
}

func normalizeProductCategoryIDs(values []string) ([]uint64, error) {
	result := make([]uint64, 0, len(values))
	seen := map[uint64]struct{}{}
	for _, value := range values {
		id, err := parseOptionalUint64(value)
		if err != nil {
			return nil, ErrProductCategoryInvalid
		}
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
		if len(result) > 6 {
			return nil, ErrProductCategoryInvalid
		}
	}
	return result, nil
}

func loadProductCategoriesByIDs(tx *gorm.DB, ids []uint64) ([]model.ProductCategory, error) {
	if len(ids) == 0 {
		return []model.ProductCategory{}, nil
	}
	var categories []model.ProductCategory
	if err := tx.Where("id IN ?", ids).Order("sort_order asc, created_at asc").Find(&categories).Error; err != nil {
		return nil, err
	}
	if len(categories) != len(ids) {
		return nil, ErrProductCategoryInvalid
	}
	return categories, nil
}

func (s *Service) productCategoriesToDTO(items []model.ProductCategory) []dto.ProductCategoryItem {
	result := make([]dto.ProductCategoryItem, 0, len(items))
	for _, item := range items {
		result = append(result, s.productCategoryToDTO(item))
	}
	return result
}

func (s *Service) productCategoryToDTO(item model.ProductCategory) dto.ProductCategoryItem {
	return dto.ProductCategoryItem{
		ID:          strconv.FormatUint(item.ID, 10),
		Name:        item.Name,
		Description: item.Description,
		SortOrder:   item.SortOrder,
		Enabled:     item.Enabled,
	}
}

func (s *Service) productsToDTO(ctx context.Context, items []model.ProductItem) []dto.ProductItem {
	result := make([]dto.ProductItem, 0, len(items))
	for _, item := range items {
		result = append(result, s.productToDTO(ctx, item))
	}
	return result
}

func (s *Service) productToDTO(ctx context.Context, item model.ProductItem) dto.ProductItem {
	previewThumbURL := ""
	if strings.TrimSpace(item.PreviewURL) != "" {
		previewThumbURL = s.thumbnailURLForImage(ctx, item.PreviewURL)
	}
	return dto.ProductItem{
		ID:               strconv.FormatUint(item.ID, 10),
		Name:             item.Name,
		ShortDescription: item.ShortDescription,
		Description:      item.Description,
		PreviewURL:       item.PreviewURL,
		PreviewThumbURL:  previewThumbURL,
		AmountCents:      item.AmountCents,
		ProductURL:       item.ProductURL,
		SortOrder:        item.SortOrder,
		Enabled:          item.Enabled,
		Categories:       s.productCategoriesToDTO(item.Categories),
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
}

func parseRequiredUint64(value string) (uint64, error) {
	id, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil || id == 0 {
		return 0, ErrProductInvalid
	}
	return id, nil
}

func parseOptionalUint64(value string) (uint64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, nil
	}
	id, err := strconv.ParseUint(trimmed, 10, 64)
	if err != nil {
		return 0, err
	}
	return id, nil
}
