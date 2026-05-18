package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"image2/backend/internal/dto"
	"image2/backend/internal/model"

	"gorm.io/gorm"
)

var (
	ErrPromptPlazaInvalid  = errors.New("提示词参数不合法")
	ErrPromptPlazaNotFound = errors.New("提示词不存在")
)

func (s *Service) ListPromptPlaza(ctx context.Context) (dto.ListPromptPlazaResponse, error) {
	return s.listPromptPlaza(ctx, 0)
}

func (s *Service) ListPromptPlazaForUser(ctx context.Context, userID uint64) (dto.ListPromptPlazaResponse, error) {
	return s.listPromptPlaza(ctx, userID)
}

func (s *Service) listPromptPlaza(ctx context.Context, userID uint64) (dto.ListPromptPlazaResponse, error) {
	var items []model.PromptPlazaItem
	if err := s.db.WithContext(ctx).
		Where("status = ?", model.PromptPlazaStatusApproved).
		Order("approved_at asc, id asc").
		Find(&items).Error; err != nil {
		return dto.ListPromptPlazaResponse{}, err
	}
	favoriteIDs, err := s.promptPlazaFavoriteIDs(ctx, userID, items)
	if err != nil {
		return dto.ListPromptPlazaResponse{}, err
	}
	return dto.ListPromptPlazaResponse{Items: s.promptPlazaItemsToDTO(ctx, items, favoriteIDs)}, nil
}

func (s *Service) SubmitPromptPlaza(ctx context.Context, userID uint64, req dto.SubmitPromptPlazaRequest) (dto.PromptPlazaItem, error) {
	var user model.User
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		return dto.PromptPlazaItem{}, err
	}
	previewURL, err := s.normalizePromptPlazaPreviewURL(userID, req.PreviewURL)
	if err != nil {
		return dto.PromptPlazaItem{}, err
	}
	item := model.PromptPlazaItem{
		Title:      strings.TrimSpace(req.Title),
		Prompt:     strings.TrimSpace(req.Prompt),
		PreviewURL: previewURL,
		Tags:       encodePromptPlazaTags(req.Tags),
		UserID:     userID,
		Author:     strings.TrimSpace(user.Username),
		Status:     model.PromptPlazaStatusPending,
	}
	if !validPromptPlazaItem(item) {
		return dto.PromptPlazaItem{}, ErrPromptPlazaInvalid
	}
	if item.Author == "" {
		item.Author = "用户" + strconv.FormatUint(userID, 10)
	}
	if err := s.db.WithContext(ctx).Create(&item).Error; err != nil {
		return dto.PromptPlazaItem{}, err
	}
	return s.promptPlazaItemToDTO(ctx, item, nil), nil
}

func (s *Service) ListMyPromptPlaza(ctx context.Context, userID uint64) (dto.ListPromptPlazaResponse, error) {
	var items []model.PromptPlazaItem
	if err := s.db.WithContext(ctx).Where("user_id = ? AND status <> ?", userID, model.PromptPlazaStatusRejected).Order("created_at desc, id desc").Find(&items).Error; err != nil {
		return dto.ListPromptPlazaResponse{}, err
	}
	favoriteIDs, err := s.promptPlazaFavoriteIDs(ctx, userID, items)
	if err != nil {
		return dto.ListPromptPlazaResponse{}, err
	}
	return dto.ListPromptPlazaResponse{Items: s.promptPlazaItemsToDTO(ctx, items, favoriteIDs)}, nil
}

func (s *Service) AdminListPromptPlaza(ctx context.Context, adminID uint64, status string, page int, pageSize int) (dto.AdminListPromptPlazaResponse, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AdminListPromptPlazaResponse{}, err
	}
	page, pageSize = normalizePage(page, pageSize)
	query := s.db.WithContext(ctx).Model(&model.PromptPlazaItem{})
	if normalized := normalizePromptPlazaStatus(status); normalized != "" {
		query = query.Where("status = ?", normalized)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return dto.AdminListPromptPlazaResponse{}, err
	}
	var pending int64
	if err := s.db.WithContext(ctx).Model(&model.PromptPlazaItem{}).Where("status = ?", model.PromptPlazaStatusPending).Count(&pending).Error; err != nil {
		return dto.AdminListPromptPlazaResponse{}, err
	}
	var items []model.PromptPlazaItem
	if err := query.Order("created_at desc, id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return dto.AdminListPromptPlazaResponse{}, err
	}
	return dto.AdminListPromptPlazaResponse{Items: s.promptPlazaItemsToDTO(ctx, items, nil), Page: page, PageSize: pageSize, Total: total, Pending: pending}, nil
}

func (s *Service) AdminCreatePromptPlaza(ctx context.Context, adminID uint64, req dto.AdminPromptPlazaPayload) (dto.PromptPlazaItem, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.PromptPlazaItem{}, err
	}
	now := s.now()
	status := normalizePromptPlazaStatus(req.Status)
	if status == "" {
		status = model.PromptPlazaStatusPending
	}
	item := model.PromptPlazaItem{
		Title:      strings.TrimSpace(req.Title),
		Prompt:     strings.TrimSpace(req.Prompt),
		PreviewURL: strings.TrimSpace(req.PreviewURL),
		Tags:       encodePromptPlazaTags(req.Tags),
		Author:     strings.TrimSpace(req.Author),
		Status:     status,
		ReviewedBy: adminID,
	}
	if item.Author == "" {
		item.Author = "管理员"
	}
	if status == model.PromptPlazaStatusApproved {
		item.ApprovedAt = &now
		item.ReviewedAt = &now
	}
	if !validPromptPlazaItem(item) {
		return dto.PromptPlazaItem{}, ErrPromptPlazaInvalid
	}
	if err := s.db.WithContext(ctx).Create(&item).Error; err != nil {
		return dto.PromptPlazaItem{}, err
	}
	return s.promptPlazaItemToDTO(ctx, item, nil), nil
}

func (s *Service) AdminUpdatePromptPlaza(ctx context.Context, adminID uint64, idText string, req dto.AdminPromptPlazaPayload) (dto.PromptPlazaItem, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.PromptPlazaItem{}, err
	}
	id, err := parsePromptPlazaID(idText)
	if err != nil {
		return dto.PromptPlazaItem{}, err
	}
	var updated model.PromptPlazaItem
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.PromptPlazaItem
		if err := tx.First(&item, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPromptPlazaNotFound
			}
			return err
		}
		before := item
		item.Title = strings.TrimSpace(req.Title)
		item.Prompt = strings.TrimSpace(req.Prompt)
		item.PreviewURL = strings.TrimSpace(req.PreviewURL)
		item.Tags = encodePromptPlazaTags(req.Tags)
		if author := strings.TrimSpace(req.Author); author != "" {
			item.Author = author
		}
		if status := normalizePromptPlazaStatus(req.Status); status != "" && status != item.Status {
			applyPromptPlazaReview(&item, adminID, status, "")
		}
		if !validPromptPlazaItem(item) {
			return ErrPromptPlazaInvalid
		}
		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		updated = item
		return recordAdminActionLog(tx, adminID, "prompt_plaza.update", "prompt_plaza", strconv.FormatUint(item.ID, 10), before, item, item.Title)
	})
	if err != nil {
		return dto.PromptPlazaItem{}, err
	}
	return s.promptPlazaItemToDTO(ctx, updated, nil), nil
}

func (s *Service) AdminReviewPromptPlaza(ctx context.Context, adminID uint64, idText string, req dto.AdminReviewPromptPlazaRequest) (dto.PromptPlazaItem, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.PromptPlazaItem{}, err
	}
	id, err := parsePromptPlazaID(idText)
	if err != nil {
		return dto.PromptPlazaItem{}, err
	}
	status := normalizePromptPlazaStatus(req.Status)
	if status != model.PromptPlazaStatusApproved && status != model.PromptPlazaStatusRejected {
		return dto.PromptPlazaItem{}, ErrPromptPlazaInvalid
	}
	var updated model.PromptPlazaItem
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.PromptPlazaItem
		if err := tx.First(&item, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPromptPlazaNotFound
			}
			return err
		}
		before := item
		reason := strings.TrimSpace(req.RejectNote)
		applyPromptPlazaReview(&item, adminID, status, reason)
		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		if item.UserID > 0 {
			title := "提示词审核结果"
			content := "你的提示词《" + item.Title + "》已通过审核。"
			if status == model.PromptPlazaStatusRejected {
				content = "您的提示词《" + item.Title + "》\n" + item.Prompt + "\n已被驳回"
				if reason == "" {
					content += "。"
				} else {
					content += "，理由：" + reason
				}
			}
			if err := createPersonalAnnouncement(tx, adminID, item.UserID, title, content); err != nil {
				return err
			}
		}
		updated = item
		if status == model.PromptPlazaStatusRejected {
			if err := tx.Unscoped().Delete(&item).Error; err != nil {
				return err
			}
		}
		return recordAdminActionLog(tx, adminID, "prompt_plaza.review", "prompt_plaza", strconv.FormatUint(item.ID, 10), before, item, item.Title)
	})
	if err != nil {
		return dto.PromptPlazaItem{}, err
	}
	return s.promptPlazaItemToDTO(ctx, updated, nil), nil
}

func (s *Service) TogglePromptPlazaFavorite(ctx context.Context, userID uint64, idText string) (dto.PromptPlazaItem, error) {
	id, err := parsePromptPlazaID(idText)
	if err != nil {
		return dto.PromptPlazaItem{}, err
	}

	var updated model.PromptPlazaItem
	isFavorited := false
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.PromptPlazaItem
		if err := tx.Where("id = ? AND status = ?", id, model.PromptPlazaStatusApproved).First(&item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPromptPlazaNotFound
			}
			return err
		}

		var favorite model.PromptPlazaFavorite
		err := tx.Where("user_id = ? AND prompt_plaza_item_id = ?", userID, id).First(&favorite).Error
		switch {
		case err == nil:
			if err := tx.Delete(&favorite).Error; err != nil {
				return err
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			if err := tx.Create(&model.PromptPlazaFavorite{UserID: userID, PromptPlazaItemID: id}).Error; err != nil {
				return err
			}
			isFavorited = true
		default:
			return err
		}

		var count int64
		if err := tx.Model(&model.PromptPlazaFavorite{}).Where("prompt_plaza_item_id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		item.Favorites = count
		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		updated = item
		return nil
	})
	if err != nil {
		return dto.PromptPlazaItem{}, err
	}
	favoriteIDs := map[uint64]struct{}{}
	if isFavorited {
		favoriteIDs[updated.ID] = struct{}{}
	}
	return s.promptPlazaItemToDTO(ctx, updated, favoriteIDs), nil
}

func (s *Service) AdminDeletePromptPlaza(ctx context.Context, adminID uint64, idText string, reason string) error {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return err
	}
	id, err := parsePromptPlazaID(idText)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.PromptPlazaItem
		if err := tx.First(&item, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPromptPlazaNotFound
			}
			return err
		}
		if err := tx.Delete(&item).Error; err != nil {
			return err
		}
		if item.UserID > 0 {
			content := "你的提示词《" + item.Title + "》已被删除。"
			if strings.TrimSpace(reason) != "" {
				content += "\n删除理由：" + strings.TrimSpace(reason)
			}
			if err := createPersonalAnnouncement(tx, adminID, item.UserID, "提示词删除通知", content); err != nil {
				return err
			}
		}
		return recordAdminActionLog(tx, adminID, "prompt_plaza.delete", "prompt_plaza", strconv.FormatUint(item.ID, 10), item, nil, item.Title)
	})
}

func createPersonalAnnouncement(tx *gorm.DB, adminID uint64, userID uint64, title string, content string) error {
	item := model.Announcement{
		Title: title, Content: content, Kind: model.AnnouncementKindPersonal, Enabled: true,
		SortOrder: 1, TargetUserID: userID, CreatedBy: adminID, UpdatedBy: adminID,
	}
	if err := tx.Create(&item).Error; err != nil {
		return err
	}
	return recordAdminActionLog(tx, adminID, "announcement.create", "announcement", strconv.FormatUint(item.ID, 10), nil, item, item.Title)
}

func applyPromptPlazaReview(item *model.PromptPlazaItem, adminID uint64, status string, rejectNote string) {
	now := time.Now()
	item.Status = status
	item.ReviewedBy = adminID
	item.ReviewedAt = &now
	item.RejectNote = rejectNote
	if status == model.PromptPlazaStatusApproved {
		if item.ApprovedAt == nil {
			item.ApprovedAt = &now
		}
		item.RejectNote = ""
	} else {
		item.ApprovedAt = nil
	}
}

func validPromptPlazaItem(item model.PromptPlazaItem) bool {
	return strings.TrimSpace(item.Title) != "" && strings.TrimSpace(item.Prompt) != "" && normalizePromptPlazaStatus(item.Status) != ""
}

func normalizePromptPlazaStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case model.PromptPlazaStatusPending, model.PromptPlazaStatusApproved, model.PromptPlazaStatusRejected:
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return ""
	}
}

func normalizePromptPlazaTags(tags []string) []string {
	result := make([]string, 0, 4)
	seen := map[string]struct{}{}
	for _, tag := range tags {
		normalized := strings.TrimSpace(strings.TrimPrefix(tag, "#"))
		normalized = strings.Join(strings.Fields(normalized), "")
		if normalized == "" {
			continue
		}
		if len([]rune(normalized)) > 24 {
			normalized = string([]rune(normalized)[:24])
		}
		key := strings.ToLower(normalized)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, normalized)
		if len(result) >= 4 {
			break
		}
	}
	return result
}

func encodePromptPlazaTags(tags []string) string {
	normalized := normalizePromptPlazaTags(tags)
	if len(normalized) == 0 {
		return ""
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return ""
	}
	return string(data)
}

func decodePromptPlazaTags(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []string{}
	}
	var tags []string
	if err := json.Unmarshal([]byte(trimmed), &tags); err != nil {
		return normalizePromptPlazaTags(strings.Split(trimmed, ","))
	}
	return normalizePromptPlazaTags(tags)
}

func parsePromptPlazaID(value string) (uint64, error) {
	id, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil || id == 0 {
		return 0, ErrPromptPlazaNotFound
	}
	return id, nil
}

func (s *Service) normalizePromptPlazaPreviewURL(userID uint64, rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", nil
	}
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		parsed, err := url.Parse(rawURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return "", ErrInvalidReferenceImageURL
		}
		base, err := url.Parse(strings.TrimRight(strings.TrimSpace(s.cfg.BackendPublicURL), "/"))
		if err == nil && base.Scheme != "" && base.Host != "" && strings.EqualFold(parsed.Scheme, base.Scheme) && strings.EqualFold(parsed.Host, base.Host) {
			if !s.isAllowedPromptPreviewPath(userID, parsed.Path) {
				return "", ErrInvalidReferenceImageURL
			}
		}
		return rawURL, nil
	}
	if strings.HasPrefix(rawURL, "/") {
		if !s.isAllowedPromptPreviewPath(userID, rawURL) {
			return "", ErrInvalidReferenceImageURL
		}
		base := strings.TrimRight(strings.TrimSpace(s.cfg.BackendPublicURL), "/")
		if base != "" {
			return base + rawURL, nil
		}
		return rawURL, nil
	}
	return "", ErrInvalidReferenceImageURL
}

func (s *Service) promptPlazaFavoriteIDs(ctx context.Context, userID uint64, items []model.PromptPlazaItem) (map[uint64]struct{}, error) {
	result := map[uint64]struct{}{}
	if userID == 0 || len(items) == 0 {
		return result, nil
	}
	ids := make([]uint64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	var favorites []model.PromptPlazaFavorite
	if err := s.db.WithContext(ctx).Where("user_id = ? AND prompt_plaza_item_id IN ?", userID, ids).Find(&favorites).Error; err != nil {
		return nil, err
	}
	for _, favorite := range favorites {
		result[favorite.PromptPlazaItemID] = struct{}{}
	}
	return result, nil
}

func (s *Service) promptPlazaItemsToDTO(ctx context.Context, items []model.PromptPlazaItem, favoriteIDs map[uint64]struct{}) []dto.PromptPlazaItem {
	result := make([]dto.PromptPlazaItem, 0, len(items))
	for _, item := range items {
		result = append(result, s.promptPlazaItemToDTO(ctx, item, favoriteIDs))
	}
	return result
}

func (s *Service) promptPlazaItemToDTO(ctx context.Context, item model.PromptPlazaItem, favoriteIDs map[uint64]struct{}) dto.PromptPlazaItem {
	_, isFavorited := favoriteIDs[item.ID]
	previewThumbURL := ""
	if strings.TrimSpace(item.PreviewURL) != "" {
		previewThumbURL = s.thumbnailURLForImage(ctx, item.PreviewURL)
	}
	return dto.PromptPlazaItem{
		ID:              strconv.FormatUint(item.ID, 10),
		Title:           item.Title,
		Prompt:          item.Prompt,
		PreviewURL:      item.PreviewURL,
		PreviewThumbURL: previewThumbURL,
		Tags:            decodePromptPlazaTags(item.Tags),
		Author:          item.Author,
		Status:          item.Status,
		Favorites:       item.Favorites,
		IsFavorited:     isFavorited,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
		ApprovedAt:      item.ApprovedAt,
		ReviewedAt:      item.ReviewedAt,
		RejectNote:      item.RejectNote,
	}
}
