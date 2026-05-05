package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLocalStoreRemoteImages(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\nfake-image"))
	}))
	defer server.Close()

	dir := t.TempDir()
	client, err := New(Config{
		Driver:        "local",
		LocalDir:      dir,
		PublicBaseURL: "http://localhost:8080",
		PublicPath:    "/uploads",
		Timeout:       time.Second,
		MaxBytes:      1024,
	})
	if err != nil {
		t.Fatalf("new storage client: %v", err)
	}

	urls, err := client.StoreRemoteImages(context.Background(), []string{server.URL + "/result.png"}, "images/test/task-1")
	if err != nil {
		t.Fatalf("store remote images: %v", err)
	}
	if len(urls) != 1 {
		t.Fatalf("expected one stored url, got %#v", urls)
	}
	if !strings.HasPrefix(urls[0], "http://localhost:8080/uploads/images/test/task-1-1") {
		t.Fatalf("unexpected stored url: %s", urls[0])
	}

	files, err := os.ReadDir(filepath.Join(dir, "images", "test"))
	if err != nil {
		t.Fatalf("read local dir: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected one stored file, got %d", len(files))
	}
}

func TestStoreRemoteImagesKeepsOriginalURLOnDownloadFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "failed", http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := New(Config{
		Driver:        "local",
		LocalDir:      t.TempDir(),
		PublicBaseURL: "http://localhost:8080",
		Timeout:       time.Second,
		MaxBytes:      1024,
	})
	if err != nil {
		t.Fatalf("new storage client: %v", err)
	}

	original := server.URL + "/result.png"
	urls, err := client.StoreRemoteImages(context.Background(), []string{original}, "images/test/task-2")
	if err == nil {
		t.Fatalf("expected a download error")
	}
	if len(urls) != 1 || urls[0] != original {
		t.Fatalf("expected original url fallback, got %#v", urls)
	}
}

func TestDownloadStoredImageReadsLocalFileDirectly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	client, err := New(Config{
		Driver:        "local",
		LocalDir:      dir,
		PublicBaseURL: "http://localhost:8080",
		PublicPath:    "/uploads",
		Timeout:       time.Second,
		MaxBytes:      1024,
	})
	if err != nil {
		t.Fatalf("new storage client: %v", err)
	}

	targetDir := filepath.Join(dir, "images", "2026", "05", "04", "user-1")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("create target dir: %v", err)
	}

	original := []byte("\x89PNG\r\n\x1a\nfake-image")
	fileName := "task-1-1.png"
	if err := os.WriteFile(filepath.Join(targetDir, fileName), original, 0644); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	data, contentType, ext, err := client.DownloadStoredImage(context.Background(), "http://localhost:8080/uploads/images/2026/05/04/user-1/"+fileName)
	if err != nil {
		t.Fatalf("download stored image: %v", err)
	}
	if string(data) != string(original) {
		t.Fatalf("unexpected image data: %q", string(data))
	}
	if contentType != "image/png" {
		t.Fatalf("expected image/png content type, got %q", contentType)
	}
	if ext != ".png" {
		t.Fatalf("expected .png extension, got %q", ext)
	}
}
