package storage

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
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

func TestStoreRemoteImagesDropsDuplicateImageContent(t *testing.T) {
	t.Parallel()

	imageData := testPNGBytes(t, color.RGBA{R: 32, G: 96, B: 180, A: 255})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageData)
	}))
	defer server.Close()

	dir := t.TempDir()
	client, err := New(Config{
		Driver:        "local",
		LocalDir:      dir,
		PublicBaseURL: "http://localhost:8080",
		PublicPath:    "/uploads",
		Timeout:       time.Second,
		MaxBytes:      1024 * 1024,
	})
	if err != nil {
		t.Fatalf("new storage client: %v", err)
	}

	urls, err := client.StoreRemoteImages(context.Background(), []string{
		server.URL + "/result-original.png",
		server.URL + "/result-compressed.png",
	}, "images/test/task-dup")
	if err != nil {
		t.Fatalf("store remote images: %v", err)
	}
	if len(urls) != 1 {
		t.Fatalf("expected duplicate image content to be stored once, got %#v", urls)
	}

	files, err := os.ReadDir(filepath.Join(dir, "images", "test"))
	if err != nil {
		t.Fatalf("read local dir: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected one stored file, got %d", len(files))
	}
}

func TestDownloadRemoteImageRejectsFakeImageBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("<html>not an image</html>"))
	}))
	defer server.Close()

	client, err := New(Config{
		Driver:   "local",
		LocalDir: t.TempDir(),
		Timeout:  time.Second,
		MaxBytes: 1024,
	})
	if err != nil {
		t.Fatalf("new storage client: %v", err)
	}

	if _, _, _, err := client.DownloadRemoteImage(context.Background(), server.URL+"/result.png"); err == nil || !strings.Contains(err.Error(), "not a supported image") {
		t.Fatalf("expected unsupported image error, got %v", err)
	}
}

func testPNGBytes(t *testing.T, c color.RGBA) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 24, 24))
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			img.SetRGBA(x, y, c)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
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

func TestStoreBytesUsesUniqueObjectKeys(t *testing.T) {
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

	dataA := []byte("\x89PNG\r\n\x1a\nfake-image-a")
	dataB := []byte("\x89PNG\r\n\x1a\nfake-image-b")

	urlA, err := client.StoreBytes(context.Background(), dataA, "reference-images/user-1/2026/05/13", "image/png", "a.png")
	if err != nil {
		t.Fatalf("store bytes A: %v", err)
	}
	urlB, err := client.StoreBytes(context.Background(), dataB, "reference-images/user-1/2026/05/13", "image/png", "b.png")
	if err != nil {
		t.Fatalf("store bytes B: %v", err)
	}

	if urlA == urlB {
		t.Fatalf("expected unique stored urls, got %q", urlA)
	}

	files, err := os.ReadDir(filepath.Join(dir, "reference-images", "user-1", "2026", "05"))
	if err != nil {
		t.Fatalf("read local dir: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected two stored files, got %d", len(files))
	}
}
