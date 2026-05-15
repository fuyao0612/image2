package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"image2/backend/internal/dto"
	"image2/backend/internal/model"
	imagestorage "image2/backend/internal/storage"

	"gorm.io/gorm"
)

type cleanupFixture struct {
	svc              *Service
	db               *gorm.DB
	admin            model.User
	rangeStartDate   time.Time
	rangeMiddleDate  time.Time
	targetImagePath  string
	targetRefPath    string
	outsideImagePath string
}

func TestResolveDataCleanupRangeRejectsInvalidInput(t *testing.T) {
	svc, _ := newTestService(t)

	_, _, _, err := svc.resolveDataCleanupRange(dto.DataCleanupRequest{Mode: "date"})
	if !errors.Is(err, ErrDataCleanupInvalid) {
		t.Fatalf("expected invalid date error, got %v", err)
	}

	_, _, _, err = svc.resolveDataCleanupRange(dto.DataCleanupRequest{Mode: "date", StartDate: "2026-05-07", EndDate: "2026-05-06"})
	if !errors.Is(err, ErrDataCleanupInvalid) {
		t.Fatalf("expected invalid date range error, got %v", err)
	}

	_, _, _, err = svc.resolveDataCleanupRange(dto.DataCleanupRequest{Mode: "days_ago", DaysAgo: 0})
	if !errors.Is(err, ErrDataCleanupInvalid) {
		t.Fatalf("expected invalid days_ago error, got %v", err)
	}

	_, _, _, err = svc.resolveDataCleanupRange(dto.DataCleanupRequest{Mode: "unknown"})
	if !errors.Is(err, ErrDataCleanupInvalid) {
		t.Fatalf("expected invalid mode error, got %v", err)
	}
}

func TestAdminPreviewDataCleanupCountsOnlyTargetRange(t *testing.T) {
	fixture := setupCleanupFixture(t)
	ctx := context.Background()

	resp, err := fixture.svc.AdminPreviewDataCleanup(ctx, fixture.admin.ID, dto.DataCleanupRequest{
		Type:      "successful_image_tasks",
		Mode:      "date",
		StartDate: fixture.rangeStartDate.Format("2006-01-02"),
		EndDate:   fixture.rangeMiddleDate.Format("2006-01-02"),
	})
	if err != nil {
		t.Fatalf("preview cleanup: %v", err)
	}

	if !resp.DryRun {
		t.Fatalf("expected dry run preview")
	}
	if resp.TargetDate != "2026-05-06 ~ 2026-05-07" {
		t.Fatalf("unexpected target date: %s", resp.TargetDate)
	}
	if resp.RecordsRemoved != 2 {
		t.Fatalf("expected 2 records removed, got %d", resp.RecordsRemoved)
	}
	if resp.FilesRemoved != 4 {
		t.Fatalf("expected 4 files removed, got %d", resp.FilesRemoved)
	}

	expected := map[string]int64{
		"image_tasks": 2,
	}
	if len(resp.TableStats) != len(expected) {
		t.Fatalf("expected %d table stats, got %d", len(expected), len(resp.TableStats))
	}
	for _, stat := range resp.TableStats {
		if want, ok := expected[stat.Table]; !ok {
			t.Fatalf("unexpected table stat %s", stat.Table)
		} else if stat.Count != want {
			t.Fatalf("unexpected count for %s: want %d got %d", stat.Table, want, stat.Count)
		}
	}
}

func TestAdminExecuteDataCleanupDeletesTargetRangeRowsAndFiles(t *testing.T) {
	fixture := setupCleanupFixture(t)
	ctx := context.Background()

	resp, err := fixture.svc.AdminExecuteDataCleanup(ctx, fixture.admin.ID, dto.DataCleanupRequest{
		Type:      "successful_image_tasks",
		Mode:      "date",
		StartDate: fixture.rangeStartDate.Format("2006-01-02"),
		EndDate:   fixture.rangeMiddleDate.Format("2006-01-02"),
	})
	if err != nil {
		t.Fatalf("execute cleanup: %v", err)
	}

	if resp.DryRun {
		t.Fatalf("execute response should not be dry run")
	}
	if resp.FinishedAt == nil {
		t.Fatalf("expected finishedAt to be set after execute")
	}

	assertModelCount(t, fixture.db, &model.ImageTask{}, 1)
	assertModelCount(t, fixture.db, &model.PointsLog{}, 3)
	assertModelCount(t, fixture.db, &model.CreditLog{}, 3)
	assertModelCount(t, fixture.db, &model.ContentAuditLog{}, 3)
	assertModelCount(t, fixture.db, &model.AnnouncementRead{}, 3)

	assertFileMissing(t, fixture.targetImagePath)
	assertFileExists(t, fixture.targetRefPath)
	assertFileExists(t, fixture.outsideImagePath)

	var actionLogs int64
	if err := fixture.db.Model(&model.AdminActionLog{}).Where("action = ?", "data_cleanup.execute").Count(&actionLogs).Error; err != nil {
		t.Fatalf("count action logs: %v", err)
	}
	if actionLogs != 1 {
		t.Fatalf("expected 1 cleanup action log, got %d", actionLogs)
	}
}

func setupCleanupFixture(t *testing.T) cleanupFixture {
	t.Helper()

	svc, db := newTestService(t)
	if err := db.AutoMigrate(&model.AnnouncementRead{}); err != nil {
		t.Fatalf("migrate announcement reads: %v", err)
	}

	admin := createTestUserWithCredits(t, db, 100, 10, 10)
	if err := db.Model(&model.User{}).Where("id = ?", admin.ID).Update("role", model.UserRoleAdmin).Error; err != nil {
		t.Fatalf("promote admin: %v", err)
	}

	user := createTestUserWithCredits(t, db, 30, 3, 2)
	rangeStartDate := time.Date(2026, 5, 6, 0, 0, 0, 0, svc.location)
	rangeMiddleDate := rangeStartDate.Add(24 * time.Hour)
	outsideDate := rangeStartDate.Add(48 * time.Hour)

	tempDir := t.TempDir()
	store, err := imagestorage.New(imagestorage.Config{
		Driver:     "local",
		LocalDir:   tempDir,
		PublicPath: "/uploads",
	})
	if err != nil {
		t.Fatalf("create local image store: %v", err)
	}
	svc.imageStore = store
	svc.cfg.ImageStoragePublicPath = "/uploads"

	targetImageURL := "/uploads/cleanup/target.png"
	targetRefURL := "/uploads/cleanup/reference.png"
	secondRangeImageURL := "/uploads/cleanup/second.png"
	outsideImageURL := "/uploads/cleanup/outside.png"

	targetImagePath := filepath.Join(tempDir, filepath.FromSlash("cleanup/target.png"))
	targetRefPath := filepath.Join(tempDir, filepath.FromSlash("cleanup/reference.png"))
	secondRangeImagePath := filepath.Join(tempDir, filepath.FromSlash("cleanup/second.png"))
	outsideImagePath := filepath.Join(tempDir, filepath.FromSlash("cleanup/outside.png"))

	writeTestFile(t, targetImagePath)
	writeTestFile(t, targetRefPath)
	writeTestFile(t, secondRangeImagePath)
	writeTestFile(t, outsideImagePath)

	targetTask := model.ImageTask{
		UserID:         user.ID,
		Prompt:         "cleanup target",
		Status:         model.ImageTaskStatusSuccess,
		ImageURL:       targetImageURL,
		RequestParams:  `{"referenceImageUrl":"` + targetRefURL + `"}`,
		RequestedCount: 1,
	}
	if err := db.Create(&targetTask).Error; err != nil {
		t.Fatalf("create target task: %v", err)
	}
	setCreatedAt(t, db, &model.ImageTask{}, targetTask.ID, rangeStartDate.Add(2*time.Hour))

	secondRangeTask := model.ImageTask{
		UserID:         user.ID,
		Prompt:         "cleanup second day",
		Status:         model.ImageTaskStatusSuccess,
		ImageURL:       secondRangeImageURL,
		RequestedCount: 1,
	}
	if err := db.Create(&secondRangeTask).Error; err != nil {
		t.Fatalf("create second range task: %v", err)
	}
	setCreatedAt(t, db, &model.ImageTask{}, secondRangeTask.ID, rangeMiddleDate.Add(2*time.Hour))

	outsideTask := model.ImageTask{
		UserID:         user.ID,
		Prompt:         "cleanup outside",
		Status:         model.ImageTaskStatusSuccess,
		ImageURL:       outsideImageURL,
		RequestedCount: 1,
	}
	if err := db.Create(&outsideTask).Error; err != nil {
		t.Fatalf("create outside task: %v", err)
	}
	setCreatedAt(t, db, &model.ImageTask{}, outsideTask.ID, outsideDate.Add(2*time.Hour))

	targetPoints := model.PointsLog{UserID: user.ID, ChangeType: model.PointsChangeTypeAdjust, Delta: 2, BeforeBalance: 10, AfterBalance: 12}
	if err := db.Create(&targetPoints).Error; err != nil {
		t.Fatalf("create target points log: %v", err)
	}
	setCreatedAt(t, db, &model.PointsLog{}, targetPoints.ID, rangeStartDate.Add(3*time.Hour))

	secondRangePoints := model.PointsLog{UserID: user.ID, ChangeType: model.PointsChangeTypeAdjust, Delta: 4, BeforeBalance: 13, AfterBalance: 17}
	if err := db.Create(&secondRangePoints).Error; err != nil {
		t.Fatalf("create second range points log: %v", err)
	}
	setCreatedAt(t, db, &model.PointsLog{}, secondRangePoints.ID, rangeMiddleDate.Add(3*time.Hour))

	outsidePoints := model.PointsLog{UserID: user.ID, ChangeType: model.PointsChangeTypeAdjust, Delta: 1, BeforeBalance: 12, AfterBalance: 13}
	if err := db.Create(&outsidePoints).Error; err != nil {
		t.Fatalf("create outside points log: %v", err)
	}
	setCreatedAt(t, db, &model.PointsLog{}, outsidePoints.ID, outsideDate.Add(3*time.Hour))

	targetCredit := model.CreditLog{UserID: user.ID, ChangeType: model.CreditChangeTypeAdjust, DeltaFree: 1, BeforeFree: 2, AfterFree: 3}
	if err := db.Create(&targetCredit).Error; err != nil {
		t.Fatalf("create target credit log: %v", err)
	}
	setCreatedAt(t, db, &model.CreditLog{}, targetCredit.ID, rangeStartDate.Add(4*time.Hour))

	secondRangeCredit := model.CreditLog{UserID: user.ID, ChangeType: model.CreditChangeTypeAdjust, DeltaFree: 2, BeforeFree: 3, AfterFree: 5}
	if err := db.Create(&secondRangeCredit).Error; err != nil {
		t.Fatalf("create second range credit log: %v", err)
	}
	setCreatedAt(t, db, &model.CreditLog{}, secondRangeCredit.ID, rangeMiddleDate.Add(4*time.Hour))

	outsideCredit := model.CreditLog{UserID: user.ID, ChangeType: model.CreditChangeTypeAdjust, DeltaPaid: 1, BeforePaid: 1, AfterPaid: 2}
	if err := db.Create(&outsideCredit).Error; err != nil {
		t.Fatalf("create outside credit log: %v", err)
	}
	setCreatedAt(t, db, &model.CreditLog{}, outsideCredit.ID, outsideDate.Add(4*time.Hour))

	targetAudit := model.ContentAuditLog{UserID: user.ID, BizType: "image_task", ContentType: "prompt", Content: "cleanup", Status: model.ContentAuditStatusPass, Action: "allow"}
	if err := db.Create(&targetAudit).Error; err != nil {
		t.Fatalf("create target audit log: %v", err)
	}
	setCreatedAt(t, db, &model.ContentAuditLog{}, targetAudit.ID, rangeStartDate.Add(5*time.Hour))

	secondRangeAudit := model.ContentAuditLog{UserID: user.ID, BizType: "image_task", ContentType: "prompt", Content: "cleanup second", Status: model.ContentAuditStatusPass, Action: "allow"}
	if err := db.Create(&secondRangeAudit).Error; err != nil {
		t.Fatalf("create second range audit log: %v", err)
	}
	setCreatedAt(t, db, &model.ContentAuditLog{}, secondRangeAudit.ID, rangeMiddleDate.Add(5*time.Hour))

	outsideAudit := model.ContentAuditLog{UserID: user.ID, BizType: "image_task", ContentType: "prompt", Content: "keep", Status: model.ContentAuditStatusPass, Action: "allow"}
	if err := db.Create(&outsideAudit).Error; err != nil {
		t.Fatalf("create outside audit log: %v", err)
	}
	setCreatedAt(t, db, &model.ContentAuditLog{}, outsideAudit.ID, outsideDate.Add(5*time.Hour))

	targetRead := model.AnnouncementRead{UserID: user.ID, AnnouncementID: 101, AnnouncementUpdatedAtRead: rangeStartDate, ReadAt: rangeStartDate.Add(6 * time.Hour)}
	if err := db.Create(&targetRead).Error; err != nil {
		t.Fatalf("create target announcement read: %v", err)
	}
	setCreatedAt(t, db, &model.AnnouncementRead{}, targetRead.ID, rangeStartDate.Add(6*time.Hour))

	secondRangeRead := model.AnnouncementRead{UserID: user.ID, AnnouncementID: 102, AnnouncementUpdatedAtRead: rangeMiddleDate, ReadAt: rangeMiddleDate.Add(6 * time.Hour)}
	if err := db.Create(&secondRangeRead).Error; err != nil {
		t.Fatalf("create second range announcement read: %v", err)
	}
	setCreatedAt(t, db, &model.AnnouncementRead{}, secondRangeRead.ID, rangeMiddleDate.Add(6*time.Hour))

	outsideRead := model.AnnouncementRead{UserID: user.ID, AnnouncementID: 202, AnnouncementUpdatedAtRead: outsideDate, ReadAt: outsideDate.Add(6 * time.Hour)}
	if err := db.Create(&outsideRead).Error; err != nil {
		t.Fatalf("create outside announcement read: %v", err)
	}
	setCreatedAt(t, db, &model.AnnouncementRead{}, outsideRead.ID, outsideDate.Add(6*time.Hour))

	return cleanupFixture{
		svc:              svc,
		db:               db,
		admin:            admin,
		rangeStartDate:   rangeStartDate,
		rangeMiddleDate:  rangeMiddleDate,
		targetImagePath:  targetImagePath,
		targetRefPath:    targetRefPath,
		outsideImagePath: outsideImagePath,
	}
}

func setCreatedAt(t *testing.T, db *gorm.DB, modelValue any, id uint64, createdAt time.Time) {
	t.Helper()
	if err := db.Model(modelValue).Where("id = ?", id).Update("created_at", createdAt).Error; err != nil {
		t.Fatalf("update created_at for %T #%d: %v", modelValue, id, err)
	}
}

func assertModelCount(t *testing.T, db *gorm.DB, modelValue any, want int64) {
	t.Helper()
	var total int64
	if err := db.Model(modelValue).Count(&total).Error; err != nil {
		t.Fatalf("count %T: %v", modelValue, err)
	}
	if total != want {
		t.Fatalf("unexpected count for %T: want %d got %d", modelValue, want, total)
	}
}

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte("cleanup"), 0644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func assertFileMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected file %s to be removed, stat err=%v", path, err)
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file %s to exist: %v", path, err)
	}
}
