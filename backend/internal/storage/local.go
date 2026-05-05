package storage

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type localStore struct {
	dir           string
	publicBaseURL string
	publicPath    string
}

func newLocalStore(cfg Config) (*localStore, error) {
	dir := strings.TrimSpace(cfg.LocalDir)
	if dir == "" {
		dir = "storage/uploads"
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create local image storage dir: %w", err)
	}

	publicPath := strings.TrimSpace(cfg.PublicPath)
	if publicPath == "" {
		publicPath = "/uploads"
	}
	publicPath = "/" + strings.Trim(publicPath, "/")

	return &localStore{
		dir:           dir,
		publicBaseURL: strings.TrimRight(strings.TrimSpace(cfg.PublicBaseURL), "/"),
		publicPath:    publicPath,
	}, nil
}

func (s *localStore) Save(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	_ = ctx
	key = strings.Trim(strings.TrimSpace(key), "/")
	if key == "" {
		return "", fmt.Errorf("local image key is empty")
	}

	target := filepath.Join(s.dir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return "", fmt.Errorf("create local image dir: %w", err)
	}
	if err := os.WriteFile(target, data, 0644); err != nil {
		return "", fmt.Errorf("write local image: %w", err)
	}

	publicPath := path.Join(s.publicPath, key)
	if s.publicBaseURL == "" {
		return publicPath, nil
	}
	return s.publicBaseURL + "/" + strings.TrimLeft(publicPath, "/"), nil
}

func (s *localStore) ReadByURL(rawURL string) ([]byte, string, string, bool, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil, "", "", false, nil
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, "", "", false, fmt.Errorf("parse local image url: %w", err)
	}

	publicPath := s.publicPath
	if publicPath == "" {
		publicPath = "/uploads"
	}
	if parsed.Path == "" {
		return nil, "", "", false, nil
	}

	normalizedPath := path.Clean("/" + strings.TrimLeft(parsed.Path, "/"))
	if normalizedPath == "." || normalizedPath == "/" {
		return nil, "", "", false, nil
	}

	prefix := strings.TrimRight(publicPath, "/") + "/"
	if normalizedPath != strings.TrimRight(publicPath, "/") && !strings.HasPrefix(normalizedPath, prefix) {
		return nil, "", "", false, nil
	}

	relativePath := strings.TrimPrefix(normalizedPath, strings.TrimRight(publicPath, "/"))
	relativePath = strings.TrimPrefix(relativePath, "/")
	relativePath = filepath.Clean(filepath.FromSlash(relativePath))
	if relativePath == "." || strings.HasPrefix(relativePath, "..") {
		return nil, "", "", true, fmt.Errorf("invalid local image url path: %s", rawURL)
	}

	target := filepath.Join(s.dir, relativePath)
	data, err := os.ReadFile(target)
	if err != nil {
		return nil, "", "", true, fmt.Errorf("read local image: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(target))
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	if ext == "" {
		ext = extensionFromContentType(contentType)
	}

	return data, contentType, ext, true, nil
}
