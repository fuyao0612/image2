package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"image2/backend/internal/config"
	"image2/backend/internal/database"
	"image2/backend/internal/model"
	"image2/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func TestDownloadImageTaskImageReturnsAttachmentResponse(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	workDir := t.TempDir()
	localDir := filepath.Join(workDir, "uploads")
	dbPath := filepath.Join(workDir, "app.db")

	cfg := config.Config{
		AppEnv:                    "dev",
		SQLitePath:                dbPath,
		ImageStorageDriver:        "local",
		ImageStorageLocalDir:      localDir,
		ImageStoragePublicPath:    "/uploads",
		BackendPublicURL:          "http://localhost:8080",
		ImageStorageMaxBytes:      1024 * 1024,
		ImageStorageTimeout:       5 * time.Second,
		CheckinPoints:             3,
		ImageCostPoints:           1,
		ImageProviderTimeout:      5 * time.Second,
		ImageProviderPollInterval: time.Second,
	}

	db, err := database.Open(cfg)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(localDir, "images", "test"), 0o755); err != nil {
		t.Fatalf("create local dir: %v", err)
	}

	fileBytes := []byte("fake-image-bytes")
	fileName := "task-1-1.png"
	imagePath := filepath.Join(localDir, "images", "test", fileName)
	if err := os.WriteFile(imagePath, fileBytes, 0o644); err != nil {
		t.Fatalf("write image file: %v", err)
	}

	user := model.User{
		Username:     "tester",
		Email:        "tester@example.com",
		PasswordHash: "hash",
		Status:       model.UserStatusNormal,
		Role:         model.UserRoleUser,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	imageURL := "/uploads/images/test/" + fileName
	imageURLsJSON, err := json.Marshal([]string{imageURL})
	if err != nil {
		t.Fatalf("marshal image urls: %v", err)
	}

	task := model.ImageTask{
		UserID:      user.ID,
		Prompt:      "test prompt",
		ImageURL:    imageURL,
		ImageURLs:   string(imageURLsJSON),
		Status:      model.ImageTaskStatusSuccess,
		CostCredits: 1,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc := service.New(cfg, db, nil)
	h := New(svc)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/image-tasks/"+strconv.FormatUint(task.ID, 10)+"/images/download?index=0", nil)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(task.ID, 10)}}
	ctx.Set("auth_user_id", user.ID)

	h.DownloadImageTaskImage(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d, body=%s", recorder.Code, recorder.Body.String())
	}

	if got := recorder.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("unexpected content type: %q", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store, max-age=0" {
		t.Fatalf("unexpected cache control: %q", got)
	}

	contentDisposition := recorder.Header().Get("Content-Disposition")
	if !strings.Contains(contentDisposition, "attachment") {
		t.Fatalf("missing attachment disposition: %q", contentDisposition)
	}
	if !strings.Contains(contentDisposition, "filename*=UTF-8''") {
		t.Fatalf("missing encoded filename: %q", contentDisposition)
	}

	if !bytes.Equal(recorder.Body.Bytes(), fileBytes) {
		t.Fatalf("unexpected body: %q", recorder.Body.Bytes())
	}
}
