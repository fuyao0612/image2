package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	_ "golang.org/x/image/webp"
)

// Config 保存图片转存配置。
type Config struct {
	Driver            string
	LocalDir          string
	PublicBaseURL     string
	PublicPath        string
	Timeout           time.Duration
	MaxBytes          int64
	S3Endpoint        string
	S3Region          string
	S3Bucket          string
	S3AccessKeyID     string
	S3SecretAccessKey string
	S3PublicBaseURL   string
	S3PathPrefix      string
	S3ForcePathStyle  bool
	S3DisableSSL      bool
}

// Client 负责下载第三方图片并保存到自有存储。
type Client struct {
	cfg        Config
	httpClient *http.Client
	local      *localStore
	s3         *s3Store
}

// New 创建图片存储客户端。
func New(cfg Config) (*Client, error) {
	cfg.Driver = strings.ToLower(strings.TrimSpace(cfg.Driver))
	if cfg.Driver == "" {
		cfg.Driver = "none"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = 25 * 1024 * 1024
	}
	if cfg.PublicPath == "" {
		cfg.PublicPath = "/uploads"
	}

	client := &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}

	switch cfg.Driver {
	case "none", "off", "disabled":
		return client, nil
	case "local":
		local, err := newLocalStore(cfg)
		if err != nil {
			return nil, err
		}
		client.local = local
	case "s3":
		s3Client, err := newS3Store(cfg)
		if err != nil {
			return nil, err
		}
		client.s3 = s3Client
	default:
		return nil, fmt.Errorf("unsupported image storage driver: %s", cfg.Driver)
	}

	return client, nil
}

// Enabled 表示是否启用了图片转存。
func (c *Client) Enabled() bool {
	if c == nil {
		return false
	}
	switch c.cfg.Driver {
	case "", "none", "off", "disabled":
		return false
	default:
		return true
	}
}

// DeleteStoredURLs deletes the given stored image URLs when the active backend supports it.
func (c *Client) DeleteStoredURLs(ctx context.Context, urls []string) error {
	if c == nil || len(urls) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var firstErr error
	for _, rawURL := range urls {
		trimmed := strings.TrimSpace(rawURL)
		if trimmed == "" {
			continue
		}
		var err error
		switch c.cfg.Driver {
		case "local":
			err = c.local.DeleteByURL(trimmed)
		case "s3":
			err = c.s3.DeleteByURL(ctx, trimmed)
		default:
			err = nil
		}
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// TotalStoredURLSize returns the total size in bytes for URLs the active backend can resolve.
func (c *Client) TotalStoredURLSize(ctx context.Context, urls []string) (int64, error) {
	if c == nil || len(urls) == 0 {
		return 0, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var total int64
	for _, rawURL := range urls {
		trimmed := strings.TrimSpace(rawURL)
		if trimmed == "" {
			continue
		}
		var size int64
		var err error
		switch c.cfg.Driver {
		case "local":
			size, err = c.local.SizeByURL(trimmed)
		case "s3":
			size, err = c.s3.SizeByURL(ctx, trimmed)
		default:
			size = 0
		}
		if err != nil {
			return 0, err
		}
		total += size
	}
	return total, nil
}

// LocalDir 返回本地存储根目录，供 HTTP 静态资源服务使用。
func (c *Client) LocalDir() string {
	if c == nil || c.local == nil {
		return ""
	}
	return c.local.dir
}

// StoreRemoteImages 下载并转存一组远程图片。单张失败时保留原 URL，避免用户丢失已生成结果。
func (c *Client) StoreRemoteImages(ctx context.Context, imageURLs []string, keyPrefix string) ([]string, error) {
	if !c.Enabled() {
		return imageURLs, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	storedURLs := make([]string, 0, len(imageURLs))
	seenFingerprints := make(map[string]struct{}, len(imageURLs))
	var firstErr error
	for index, imageURL := range imageURLs {
		trimmed := strings.TrimSpace(imageURL)
		if trimmed == "" {
			continue
		}

		data, contentType, ext, err := c.download(ctx, trimmed)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			storedURLs = mergeURLs(storedURLs, []string{trimmed})
			continue
		}
		if fingerprint, ok := generatedImageFingerprint(data); ok {
			if _, exists := seenFingerprints[fingerprint]; exists {
				continue
			}
			seenFingerprints[fingerprint] = struct{}{}
		}

		key := buildObjectKey(keyPrefix, index, ext)
		storedURL, err := c.store(ctx, key, data, contentType)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			storedURLs = mergeURLs(storedURLs, []string{trimmed})
			continue
		}
		storedURLs = mergeURLs(storedURLs, []string{storedURL})
	}
	return storedURLs, firstErr
}

func generatedImageFingerprint(data []byte) (string, bool) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", false
	}
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return "", false
	}

	const grid = 16
	var luminance [grid * grid]uint32
	var totalLum uint64
	var totalR uint64
	var totalG uint64
	var totalB uint64
	for y := 0; y < grid; y++ {
		srcY := bounds.Min.Y + (y*height+height/2)/grid
		if srcY >= bounds.Max.Y {
			srcY = bounds.Max.Y - 1
		}
		for x := 0; x < grid; x++ {
			srcX := bounds.Min.X + (x*width+width/2)/grid
			if srcX >= bounds.Max.X {
				srcX = bounds.Max.X - 1
			}
			r, g, b, _ := img.At(srcX, srcY).RGBA()
			r8 := r >> 8
			g8 := g >> 8
			b8 := b >> 8
			lum := uint32((299*r8 + 587*g8 + 114*b8) / 1000)
			idx := y*grid + x
			luminance[idx] = lum
			totalLum += uint64(lum)
			totalR += uint64(r8)
			totalG += uint64(g8)
			totalB += uint64(b8)
		}
	}

	avgLum := uint32(totalLum / uint64(len(luminance)))
	var hashParts [4]uint64
	for i, lum := range luminance {
		if lum >= avgLum {
			part := i / 64
			offset := i % 64
			hashParts[part] |= 1 << uint(offset)
		}
	}

	samples := uint64(len(luminance))
	avgR := (totalR / samples) >> 4
	avgG := (totalG / samples) >> 4
	avgB := (totalB / samples) >> 4
	return fmt.Sprintf(
		"%dx%d:%016x%016x%016x%016x:%02x%02x%02x",
		width,
		height,
		hashParts[0],
		hashParts[1],
		hashParts[2],
		hashParts[3],
		avgR,
		avgG,
		avgB,
	), true
}

// StoreBytes 将原始字节写入图片存储，并返回可访问的公开地址。
func (c *Client) StoreBytes(ctx context.Context, data []byte, keyPrefix string, contentType string, fileName string) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("image storage disabled")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	ext := extensionFromContentType(contentType)
	if ext == "" {
		ext = extensionFromFilename(fileName)
	}
	if ext == "" {
		ext = ".png"
	}

	key := buildUniqueObjectKey(keyPrefix, ext)
	return c.store(ctx, key, data, contentType)
}

// DownloadRemoteImage downloads an image URL and returns its bytes, content type, and file extension.
func (c *Client) DownloadRemoteImage(ctx context.Context, rawURL string) ([]byte, string, string, error) {
	if c == nil {
		return nil, "", "", fmt.Errorf("image storage client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return c.download(ctx, rawURL)
}

// DownloadStoredImage 优先直接读取本地缓存文件，只有本地读取失败或不是本地存储地址时才回退到 HTTP 下载。
func (c *Client) DownloadStoredImage(ctx context.Context, rawURL string) ([]byte, string, string, error) {
	if c == nil {
		return nil, "", "", fmt.Errorf("image storage client is nil")
	}
	if c.local != nil {
		if data, contentType, ext, ok, err := c.local.ReadByURL(rawURL); ok {
			if err != nil {
				return nil, "", "", err
			}
			return data, contentType, ext, nil
		} else if err != nil {
			return nil, "", "", err
		}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return c.download(ctx, rawURL)
}

func (c *Client) download(ctx context.Context, rawURL string) ([]byte, string, string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, "", "", fmt.Errorf("invalid image url: %s", rawURL)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, "", "", fmt.Errorf("unsupported image url scheme: %s", parsed.Scheme)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", "", err
	}
	req.Header.Set("User-Agent", "ai-image-saas/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", "", fmt.Errorf("download image failed: http %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, c.cfg.MaxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", "", err
	}
	if int64(len(data)) > c.cfg.MaxBytes {
		return nil, "", "", fmt.Errorf("image is larger than %d bytes", c.cfg.MaxBytes)
	}

	contentType, err := normalizeDownloadedImageContentType(resp.Header.Get("Content-Type"), data)
	if err != nil {
		return nil, "", "", err
	}
	ext := extensionFromContentType(contentType)
	if ext == "" {
		ext = extensionFromURL(parsed.Path)
	}
	if ext == "" {
		ext = ".png"
	}

	return data, contentType, ext, nil
}

func (c *Client) store(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	switch c.cfg.Driver {
	case "local":
		return c.local.Save(ctx, key, data, contentType)
	case "s3":
		return c.s3.Save(ctx, key, data, contentType)
	default:
		return "", fmt.Errorf("unsupported image storage driver: %s", c.cfg.Driver)
	}
}

func buildObjectKey(prefix string, index int, ext string) string {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		prefix = time.Now().Format("2006/01/02/image")
	}
	ext = strings.TrimSpace(ext)
	if ext == "" {
		ext = ".png"
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return path.Clean(fmt.Sprintf("%s-%d%s", prefix, index+1, ext))
}

func buildUniqueObjectKey(prefix string, ext string) string {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		prefix = time.Now().Format("2006/01/02/image")
	}
	ext = strings.TrimSpace(ext)
	if ext == "" {
		ext = ".png"
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	return path.Clean(fmt.Sprintf("%s-%d-%s%s", prefix, time.Now().UnixNano(), randomObjectKeyToken(), ext))
}

func randomObjectKeyToken() string {
	var data [12]byte
	if _, err := rand.Read(data[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(data[:])
}

func extensionFromContentType(contentType string) string {
	contentType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	switch contentType {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	}
	extensions, err := mime.ExtensionsByType(contentType)
	if err == nil && len(extensions) > 0 {
		return extensions[0]
	}
	return ""
}

func normalizeDownloadedImageContentType(headerContentType string, data []byte) (string, error) {
	detected := cleanContentType(http.DetectContentType(data))
	if isSupportedImageContentType(detected) {
		return detected, nil
	}

	headerType := cleanContentType(headerContentType)
	if isSupportedImageContentType(headerType) && detected == "application/octet-stream" {
		return headerType, nil
	}

	if detected == "" {
		detected = headerType
	}
	return "", fmt.Errorf("downloaded file is not a supported image: %s", detected)
}

func cleanContentType(contentType string) string {
	return strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
}

func isSupportedImageContentType(contentType string) bool {
	switch cleanContentType(contentType) {
	case "image/jpeg", "image/jpg", "image/png", "image/webp", "image/gif":
		return true
	default:
		return false
	}
}

func extensionFromURL(rawPath string) string {
	ext := strings.ToLower(filepath.Ext(rawPath))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		return ext
	default:
		return ""
	}
}

func extensionFromFilename(fileName string) string {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(fileName)))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		return ext
	default:
		return ""
	}
}

func mergeURLs(base []string, next []string) []string {
	seen := make(map[string]struct{}, len(base)+len(next))
	result := make([]string, 0, len(base)+len(next))
	for _, item := range append(base, next...) {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func readSeekable(data []byte) io.ReadSeeker {
	return bytes.NewReader(data)
}
