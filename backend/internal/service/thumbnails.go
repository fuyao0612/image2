package service

import (
	"bytes"
	"context"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"image2/backend/internal/model"

	_ "golang.org/x/image/webp"
)

const thumbnailMaxEdge = 480

func (s *Service) thumbnailURLsFromTask(ctx context.Context, task model.ImageTask) []string {
	originals := imageURLsFromTask(task)
	if len(originals) == 0 {
		return nil
	}
	thumbs := make([]string, 0, len(originals))
	for _, original := range originals {
		thumbs = append(thumbs, s.thumbnailURLForImage(ctx, original))
	}
	return thumbs
}

func (s *Service) thumbnailURLForImage(ctx context.Context, rawURL string) string {
	thumbURL, err := s.ensureThumbnail(ctx, rawURL)
	if err != nil {
		return rawURL
	}
	if strings.TrimSpace(thumbURL) == "" {
		return rawURL
	}
	return thumbURL
}

func (s *Service) ensureThumbnail(ctx context.Context, rawURL string) (string, error) {
	_ = ctx

	if s == nil || s.imageStore == nil || !s.imageStore.Enabled() {
		return "", nil
	}
	localDir := strings.TrimSpace(s.imageStore.LocalDir())
	if localDir == "" {
		return "", nil
	}

	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Path == "" {
		return "", nil
	}

	publicPath := strings.TrimRight(strings.TrimSpace(s.cfg.ImageStoragePublicPath), "/")
	if publicPath == "" {
		publicPath = "/uploads"
	}
	normalizedPath := path.Clean("/" + strings.TrimLeft(parsed.Path, "/"))
	prefix := strings.TrimRight(publicPath, "/") + "/"
	if normalizedPath != strings.TrimRight(publicPath, "/") && !strings.HasPrefix(normalizedPath, prefix) {
		return "", nil
	}

	relativePath := strings.TrimPrefix(normalizedPath, strings.TrimRight(publicPath, "/"))
	relativePath = strings.TrimPrefix(relativePath, "/")
	relativePath = filepath.Clean(filepath.FromSlash(relativePath))
	if relativePath == "." || strings.HasPrefix(relativePath, "..") {
		return "", nil
	}

	thumbRelative := thumbnailRelativePath(relativePath)
	thumbTarget := filepath.Join(localDir, thumbRelative)
	if _, statErr := os.Stat(thumbTarget); statErr == nil {
		return buildThumbnailURL(rawURL, publicPath, thumbRelative), nil
	}

	originalTarget := filepath.Join(localDir, relativePath)
	data, err := os.ReadFile(originalTarget)
	if err != nil {
		return "", nil
	}

	src, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", nil
	}
	if format != "jpeg" && format != "png" && format != "gif" && format != "webp" {
		return "", nil
	}

	thumb := resizeImage(src, thumbnailMaxEdge, thumbnailMaxEdge)
	if thumb == nil {
		return "", nil
	}

	if err := os.MkdirAll(filepath.Dir(thumbTarget), 0755); err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 76}); err != nil {
		return "", err
	}

	if err := os.WriteFile(thumbTarget, buf.Bytes(), 0644); err != nil {
		return "", err
	}

	return buildThumbnailURL(rawURL, publicPath, thumbRelative), nil
}

func thumbnailRelativePath(relativePath string) string {
	cleaned := filepath.Clean(relativePath)
	ext := filepath.Ext(cleaned)
	if ext != "" {
		cleaned = strings.TrimSuffix(cleaned, ext)
	}
	cleaned += ".jpg"
	return filepath.Join("thumbs", cleaned)
}

func buildThumbnailURL(rawURL string, publicPath string, thumbRelative string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	thumbPath := path.Join(strings.TrimRight(publicPath, "/"), filepath.ToSlash(thumbRelative))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return thumbPath
	}
	parsed.Path = thumbPath
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func resizeImage(src image.Image, maxWidth, maxHeight int) image.Image {
	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil
	}

	scale := minFloat(float64(maxWidth)/float64(width), float64(maxHeight)/float64(height))
	if scale <= 0 {
		return nil
	}
	if scale >= 1 {
		return src
	}

	newWidth := int(float64(width) * scale)
	newHeight := int(float64(height) * scale)
	if newWidth < 1 {
		newWidth = 1
	}
	if newHeight < 1 {
		newHeight = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	// Nearest-neighbor resize keeps the dependency surface small.
	for y := 0; y < newHeight; y++ {
		srcY := bounds.Min.Y + int(float64(y)/scale)
		if srcY >= bounds.Max.Y {
			srcY = bounds.Max.Y - 1
		}
		for x := 0; x < newWidth; x++ {
			srcX := bounds.Min.X + int(float64(x)/scale)
			if srcX >= bounds.Max.X {
				srcX = bounds.Max.X - 1
			}
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}

	return dst
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
