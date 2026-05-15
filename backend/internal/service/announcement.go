package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"image2/backend/internal/dto"
	"image2/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrAnnouncementInvalid  = errors.New("公告参数不合法")
	ErrAnnouncementNotFound = errors.New("公告不存在")
)

func (s *Service) ListAnnouncements(ctx context.Context, userID uint64) (dto.ListAnnouncementsResponse, error) {
	var announcements []model.Announcement
	if err := s.db.WithContext(ctx).
		Where("enabled = ? AND (target_user_id = 0 OR target_user_id = ?)", true, userID).
		Order("case when kind = 'pinned' then 0 else 1 end, sort_order asc, updated_at desc").
		Find(&announcements).Error; err != nil {
		return dto.ListAnnouncementsResponse{}, err
	}

	readMap, err := s.announcementReadMap(ctx, userID, announcements)
	if err != nil {
		return dto.ListAnnouncementsResponse{}, err
	}

	items := make([]dto.AnnouncementItem, 0, len(announcements))
	for _, announcement := range announcements {
		items = append(items, announcementToDTO(announcement, readMap[announcement.ID]))
	}
	return dto.ListAnnouncementsResponse{Items: items}, nil
}

func (s *Service) MarkAnnouncementRead(ctx context.Context, userID uint64, announcementID string) (dto.MarkAnnouncementReadResponse, error) {
	id, err := strconv.ParseUint(strings.TrimSpace(announcementID), 10, 64)
	if err != nil || id == 0 {
		return dto.MarkAnnouncementReadResponse{}, ErrAnnouncementNotFound
	}

	var announcement model.Announcement
	if err := s.db.WithContext(ctx).Where("id = ? AND enabled = ?", id, true).First(&announcement).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.MarkAnnouncementReadResponse{}, ErrAnnouncementNotFound
		}
		return dto.MarkAnnouncementReadResponse{}, err
	}

	now := s.now()
	read := model.AnnouncementRead{
		UserID:                    userID,
		AnnouncementID:            announcement.ID,
		AnnouncementUpdatedAtRead: announcement.UpdatedAt,
		ReadAt:                    now,
	}
	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "announcement_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"announcement_updated_at_read": announcement.UpdatedAt,
			"read_at":                      now,
			"updated_at":                   now,
		}),
	}).Create(&read).Error; err != nil {
		return dto.MarkAnnouncementReadResponse{}, err
	}

	return dto.MarkAnnouncementReadResponse{
		ID:     strconv.FormatUint(announcement.ID, 10),
		IsRead: true,
		ReadAt: now,
	}, nil
}

func (s *Service) AdminListAnnouncements(ctx context.Context, adminID uint64, page int, pageSize int) (dto.AdminListAnnouncementsResponse, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AdminListAnnouncementsResponse{}, err
	}
	page, pageSize = normalizePage(page, pageSize)

	query := s.db.WithContext(ctx).Model(&model.Announcement{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return dto.AdminListAnnouncementsResponse{}, err
	}

	var announcements []model.Announcement
	if err := query.Order("case when kind = 'pinned' then 0 else 1 end, sort_order asc, updated_at desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&announcements).Error; err != nil {
		return dto.AdminListAnnouncementsResponse{}, err
	}

	items := make([]dto.AnnouncementItem, 0, len(announcements))
	for _, announcement := range announcements {
		items = append(items, announcementToDTO(announcement, model.AnnouncementRead{}))
	}
	return dto.AdminListAnnouncementsResponse{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *Service) AdminCreateAnnouncement(ctx context.Context, adminID uint64, req dto.AdminCreateAnnouncementRequest) (dto.AnnouncementItem, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AnnouncementItem{}, err
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	announcement := model.Announcement{
		Title:        strings.TrimSpace(req.Title),
		Content:      strings.TrimSpace(req.Content),
		ImageURL:     strings.TrimSpace(req.ImageURL),
		Kind:         normalizeAnnouncementKind(req.Kind),
		TargetUserID: parseOptionalUint(req.TargetUserID),
		Enabled:      enabled,
		SortOrder:    req.SortOrder,
		CreatedBy:    adminID,
		UpdatedBy:    adminID,
	}
	if announcement.Kind != model.AnnouncementKindPersonal {
		announcement.TargetUserID = 0
	}
	if !validAnnouncement(announcement) {
		return dto.AnnouncementItem{}, ErrAnnouncementInvalid
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&announcement).Error; err != nil {
			return err
		}
		return recordAdminActionLog(tx, adminID, "announcement.create", "announcement", strconv.FormatUint(announcement.ID, 10), nil, announcement, announcement.Title)
	}); err != nil {
		return dto.AnnouncementItem{}, err
	}

	return announcementToDTO(announcement, model.AnnouncementRead{}), nil
}

func (s *Service) AdminUpdateAnnouncement(ctx context.Context, adminID uint64, announcementID string, req dto.AdminUpdateAnnouncementRequest) (dto.AnnouncementItem, error) {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return dto.AnnouncementItem{}, err
	}
	id, err := strconv.ParseUint(strings.TrimSpace(announcementID), 10, 64)
	if err != nil || id == 0 {
		return dto.AnnouncementItem{}, ErrAnnouncementNotFound
	}

	var updated model.Announcement
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var announcement model.Announcement
		if err := tx.First(&announcement, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAnnouncementNotFound
			}
			return err
		}
		before := announcement

		if title := strings.TrimSpace(req.Title); title != "" {
			announcement.Title = title
		}
		if content := strings.TrimSpace(req.Content); content != "" {
			announcement.Content = content
		}
		announcement.ImageURL = strings.TrimSpace(req.ImageURL)
		if kind := strings.TrimSpace(req.Kind); kind != "" {
			announcement.Kind = normalizeAnnouncementKind(kind)
		}
		if req.TargetUserID != "" {
			announcement.TargetUserID = parseOptionalUint(req.TargetUserID)
		} else if announcement.Kind != model.AnnouncementKindPersonal {
			announcement.TargetUserID = 0
		}
		if req.Enabled != nil {
			announcement.Enabled = *req.Enabled
		}
		if req.SortOrder != nil {
			announcement.SortOrder = *req.SortOrder
		}
		announcement.UpdatedBy = adminID
		if !validAnnouncement(announcement) {
			return ErrAnnouncementInvalid
		}
		if err := tx.Save(&announcement).Error; err != nil {
			return err
		}
		updated = announcement
		return recordAdminActionLog(tx, adminID, "announcement.update", "announcement", strconv.FormatUint(announcement.ID, 10), before, announcement, announcement.Title)
	})
	if err != nil {
		return dto.AnnouncementItem{}, err
	}

	return announcementToDTO(updated, model.AnnouncementRead{}), nil
}

func (s *Service) AdminDeleteAnnouncement(ctx context.Context, adminID uint64, announcementID string) error {
	if err := s.RequireAdmin(ctx, adminID); err != nil {
		return err
	}
	id, err := strconv.ParseUint(strings.TrimSpace(announcementID), 10, 64)
	if err != nil || id == 0 {
		return ErrAnnouncementNotFound
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var announcement model.Announcement
		if err := tx.First(&announcement, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAnnouncementNotFound
			}
			return err
		}
		before := announcement
		if err := tx.Delete(&announcement).Error; err != nil {
			return err
		}
		return recordAdminActionLog(tx, adminID, "announcement.delete", "announcement", strconv.FormatUint(announcement.ID, 10), before, nil, announcement.Title)
	})
}

func (s *Service) announcementReadMap(ctx context.Context, userID uint64, announcements []model.Announcement) (map[uint64]model.AnnouncementRead, error) {
	result := make(map[uint64]model.AnnouncementRead, len(announcements))
	if len(announcements) == 0 {
		return result, nil
	}
	ids := make([]uint64, 0, len(announcements))
	for _, announcement := range announcements {
		ids = append(ids, announcement.ID)
	}

	var reads []model.AnnouncementRead
	if err := s.db.WithContext(ctx).Where("user_id = ? AND announcement_id IN ?", userID, ids).Find(&reads).Error; err != nil {
		return nil, err
	}
	for _, read := range reads {
		result[read.AnnouncementID] = read
	}
	return result, nil
}

func announcementToDTO(announcement model.Announcement, read model.AnnouncementRead) dto.AnnouncementItem {
	isRead := read.ID > 0 && !read.AnnouncementUpdatedAtRead.Before(announcement.UpdatedAt)
	var readAt *time.Time
	if isRead {
		readAt = &read.ReadAt
	}
	return dto.AnnouncementItem{
		ID:           strconv.FormatUint(announcement.ID, 10),
		Title:        announcement.Title,
		Content:      announcement.Content,
		ImageURL:     strings.TrimSpace(announcement.ImageURL),
		Kind:         string(announcement.Kind),
		TargetUserID: formatOptionalUint(announcement.TargetUserID),
		Enabled:      announcement.Enabled,
		SortOrder:    announcement.SortOrder,
		IsRead:       isRead,
		ReadAt:       readAt,
		CreatedAt:    announcement.CreatedAt,
		UpdatedAt:    announcement.UpdatedAt,
	}
}

func normalizeAnnouncementKind(kind string) model.AnnouncementKind {
	switch strings.TrimSpace(kind) {
	case string(model.AnnouncementKindPinned):
		return model.AnnouncementKindPinned
	case string(model.AnnouncementKindPersonal):
		return model.AnnouncementKindPersonal
	default:
		return model.AnnouncementKindNormal
	}
}

func validAnnouncement(announcement model.Announcement) bool {
	return strings.TrimSpace(announcement.Title) != "" &&
		strings.TrimSpace(announcement.Content) != "" &&
		(announcement.Kind == model.AnnouncementKindPinned || announcement.Kind == model.AnnouncementKindNormal || announcement.Kind == model.AnnouncementKindPersonal) &&
		(announcement.Kind != model.AnnouncementKindPersonal || announcement.TargetUserID > 0)
}

func parseOptionalUint(value string) uint64 {
	id, _ := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	return id
}

func formatOptionalUint(value uint64) string {
	if value == 0 {
		return ""
	}
	return strconv.FormatUint(value, 10)
}
