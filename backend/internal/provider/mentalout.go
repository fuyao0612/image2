package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"image2/backend/internal/model"
)

var ErrNotConfigured = errors.New("image provider not configured")

const maxReferenceImageBytes = 25 * 1024 * 1024

var (
	batchIDPattern       = regexp.MustCompile(`batch-[A-Za-z0-9_-]+`)
	relativeImagePattern = regexp.MustCompile(`/(?:results|images|uploads|outputs)/[^\s"'<>]+`)
	urlPattern           = regexp.MustCompile(`https?://[^\s"'<>]+`)
)

// GenerateRequest 是图片生成请求参数。
type GenerateRequest struct {
	Prompt             string
	Size               string
	Style              string
	Count              int
	Quality            string
	OutputFormat       string
	ReferenceImageURL  string
	ReferenceImageURLs []string
	ReferenceImage     *ReferenceImage
	ReferenceImages    []ReferenceImage
}

type ReferenceImage struct {
	Name        string
	ContentType string
	Data        []byte
}

// GenerateResult 是第三方图片服务返回的统一结果。
type GenerateResult struct {
	ProviderTaskID      string
	Status              model.ImageTaskStatus
	ImageURL            string
	ImageURLs           []string
	ImageElapsedSeconds []int64
	ErrorMessage        string
	RawResponse         string
}

// MentaloutConfig 保存第三方图片服务配置。
type MentaloutConfig struct {
	BatchServiceURL    string
	ProviderAPIBaseURL string
	APIKey             string
	PollInterval       time.Duration
	Timeout            time.Duration
}

// MentaloutProvider 负责对接 image.mentalout.top 批次接口。
type MentaloutProvider struct {
	cfg    MentaloutConfig
	client *http.Client
}

// NewMentaloutProvider 创建第三方图片服务适配器。
func NewMentaloutProvider(cfg MentaloutConfig) *MentaloutProvider {
	batchURL := normalizeBaseURL(cfg.BatchServiceURL, "https://image.mentalout.top")
	apiBaseURL := normalizeProviderAPIBaseURL(cfg.ProviderAPIBaseURL)
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	pollInterval := cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Resolver: &net.Resolver{
			PreferGo: true,
		},
	}

	return &MentaloutProvider{
		cfg: MentaloutConfig{
			BatchServiceURL:    batchURL,
			ProviderAPIBaseURL: apiBaseURL,
			APIKey:             strings.TrimSpace(cfg.APIKey),
			PollInterval:       pollInterval,
			Timeout:            timeout,
		},
		client: &http.Client{
			Timeout: 0,
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				DialContext:           dialer.DialContext,
				DisableKeepAlives:     true,
				ForceAttemptHTTP2:     false,
				MaxIdleConns:          10,
				IdleConnTimeout:       30 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: timeout,
				ExpectContinueTimeout: time.Second,
			},
		},
	}
}

// Name 返回适配器名称。
func (p *MentaloutProvider) Name() string {
	return "mentalout"
}

// Configured 表示是否已经配置了可用的 API Key。
func (p *MentaloutProvider) Configured() bool {
	key := strings.ToLower(strings.TrimSpace(p.cfg.APIKey))
	switch key {
	case "", "change-me", "your-api-key", "replace-with-your-api-key", "replace-with-real-provider-key":
		return false
	default:
		return true
	}
}

// Generate 提交批次并轮询直到图片生成完成或失败。
func (p *MentaloutProvider) Generate(ctx context.Context, req GenerateRequest) (GenerateResult, error) {
	if !p.Configured() {
		return GenerateResult{}, ErrNotConfigured
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return GenerateResult{}, fmt.Errorf("prompt cannot be empty")
	}

	submitted, err := p.submitBatch(ctx, req)
	if err != nil {
		return submitted, err
	}

	if submitted.Status == model.ImageTaskStatusSuccess && strings.TrimSpace(submitted.ImageURL) != "" {
		return submitted, nil
	}
	if submitted.Status == model.ImageTaskStatusFailed {
		return submitted, nil
	}

	if strings.TrimSpace(submitted.ProviderTaskID) == "" {
		return submitted, fmt.Errorf("provider batch id not found in submit response")
	}

	result := submitted
	ticker := time.NewTicker(p.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if result.ErrorMessage == "" {
				result.ErrorMessage = ctx.Err().Error()
			}
			if result.Status == "" {
				result.Status = model.ImageTaskStatusGenerating
			}
			return result, ctx.Err()
		case <-ticker.C:
		}

		fetched, err := p.fetchBatch(ctx, result.ProviderTaskID)
		if err != nil {
			return fetched, err
		}

		if fetched.ProviderTaskID == "" {
			fetched.ProviderTaskID = result.ProviderTaskID
		}
		result = mergeResult(result, fetched)

		switch result.Status {
		case model.ImageTaskStatusSuccess:
			if strings.TrimSpace(result.ImageURL) != "" {
				return result, nil
			}
		case model.ImageTaskStatusFailed:
			return result, nil
		}
	}
}

func (p *MentaloutProvider) submitBatch(ctx context.Context, req GenerateRequest) (GenerateResult, error) {
	var (
		body   []byte
		status int
		err    error
	)
	if len(req.ReferenceImages) == 0 && req.ReferenceImage == nil && len(req.ReferenceImageURLs) == 0 && strings.TrimSpace(req.ReferenceImageURL) == "" {
		body, status, err = p.doJSONRequest(ctx, http.MethodPost, p.submitURL(), p.buildSubmitPayload(req))
	} else {
		body, status, err = p.doMultipartBatchRequest(ctx, p.submitURL(), p.buildSubmitPayload(req), req.ReferenceImageURL, req.ReferenceImage, req.ReferenceImageURLs, req.ReferenceImages)
	}
	if err != nil {
		return GenerateResult{}, err
	}
	if status < 200 || status >= 300 {
		return GenerateResult{}, fmt.Errorf("submit batch failed: http %d: %s", status, truncateText(string(body), 500))
	}

	return extractGenerateResult(body, p.cfg.BatchServiceURL, p.cfg.ProviderAPIBaseURL), nil
}

func (p *MentaloutProvider) buildSubmitPayload(req GenerateRequest) map[string]any {
	payload := map[string]any{
		"apiBaseUrl": p.cfg.ProviderAPIBaseURL,
		"apiKey":     p.cfg.APIKey,
		"prompt":     strings.TrimSpace(req.Prompt),
		"style":      normalizeStyle(req.Style),
		"size":       normalizeSize(req.Size),
		"count":      normalizeCount(req.Count),
		"tuning": map[string]any{
			"quality":           normalizeQuality(req.Quality),
			"outputFormat":      normalizeOutputFormat(req.OutputFormat),
			"outputCompression": nil,
		},
	}
	return payload
}

func (p *MentaloutProvider) doMultipartBatchRequest(ctx context.Context, targetURL string, payload map[string]any, referenceImageURL string, providedImage *ReferenceImage, referenceImageURLs []string, providedImages []ReferenceImage) ([]byte, int, error) {
	var images []referenceImageFile
	if providedImage != nil && len(providedImage.Data) > 0 {
		image := referenceImageFile{
			Name:        providedImage.Name,
			ContentType: providedImage.ContentType,
			Data:        providedImage.Data,
		}
		if strings.TrimSpace(image.Name) == "" {
			image.Name = referenceImageNameFromURL(referenceImageURL, image.ContentType)
		}
		images = append(images, image)
	}
	for _, provided := range providedImages {
		if len(provided.Data) == 0 {
			continue
		}
		image := referenceImageFile{Name: provided.Name, ContentType: provided.ContentType, Data: provided.Data}
		if strings.TrimSpace(image.Name) == "" {
			image.Name = referenceImageNameFromURL("", image.ContentType)
		}
		images = append(images, image)
	}
	if len(images) == 0 && strings.TrimSpace(referenceImageURL) != "" {
		image, err := p.downloadReferenceImage(ctx, referenceImageURL)
		if err != nil {
			return nil, 0, err
		}
		images = append(images, image)
	} else {
		for _, url := range referenceImageURLs {
			if strings.TrimSpace(url) == "" {
				continue
			}
			image, err := p.downloadReferenceImage(ctx, url)
			if err != nil {
				return nil, 0, err
			}
			images = append(images, image)
		}
	}
	if len(images) == 0 {
		image, err := p.downloadReferenceImage(ctx, referenceImageURL)
		if err != nil {
			return nil, 0, err
		}
		images = append(images, image)
	}

	raw, contentType, err := buildMultipartBatchBody(payload, images)
	if err != nil {
		return nil, 0, err
	}

	return p.doRequestOnce(ctx, http.MethodPost, targetURL, contentType, raw)
}

type referenceImageFile struct {
	Name        string
	ContentType string
	Data        []byte
}

func buildMultipartBatchBody(payload map[string]any, images []referenceImageFile) ([]byte, string, error) {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}
	if err := writer.WriteField("payload", string(payloadJSON)); err != nil {
		return nil, "", err
	}

	for _, image := range images {
		fileName := sanitizeReferenceImageFileName(image.Name, image.ContentType)
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, "image", fileName))
		header.Set("Content-Type", image.ContentType)
		part, err := writer.CreatePart(header)
		if err != nil {
			return nil, "", err
		}
		if _, err := part.Write(image.Data); err != nil {
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}

	return buffer.Bytes(), writer.FormDataContentType(), nil
}

func (p *MentaloutProvider) downloadReferenceImage(ctx context.Context, rawURL string) (referenceImageFile, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return referenceImageFile{}, fmt.Errorf("reference image url is empty")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, trimmed, nil)
	if err != nil {
		return referenceImageFile{}, fmt.Errorf("create reference image request: %w", err)
	}
	req.Header.Set("Accept", "image/webp,image/png,image/jpeg,image/gif,*/*;q=0.8")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36 Edg/147.0.0.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return referenceImageFile{}, fmt.Errorf("download reference image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return referenceImageFile{}, fmt.Errorf("download reference image: http %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxReferenceImageBytes+1))
	if err != nil {
		return referenceImageFile{}, fmt.Errorf("read reference image: %w", err)
	}
	if len(data) == 0 {
		return referenceImageFile{}, fmt.Errorf("reference image is empty")
	}
	if len(data) > maxReferenceImageBytes {
		return referenceImageFile{}, fmt.Errorf("reference image is too large")
	}

	contentType, err := normalizeReferenceImageContentType(resp.Header.Get("Content-Type"), data)
	if err != nil {
		return referenceImageFile{}, err
	}

	return referenceImageFile{
		Name:        referenceImageNameFromURL(trimmed, contentType),
		ContentType: contentType,
		Data:        data,
	}, nil
}

func referenceImageNameFromURL(rawURL string, contentType string) string {
	parsed, err := url.Parse(rawURL)
	if err == nil {
		name := path.Base(parsed.Path)
		if name != "." && name != "/" && name != "" {
			return name
		}
	}

	ext := ".png"
	if extensions, err := mime.ExtensionsByType(contentType); err == nil && len(extensions) > 0 {
		ext = extensions[0]
	}
	return "reference-image" + ext
}

func sanitizeReferenceImageFileName(fileName string, contentType string) string {
	name := filepath.Base(strings.TrimSpace(fileName))
	name = strings.Trim(name, ". ")
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = referenceImageNameFromURL("", contentType)
	}
	if filepath.Ext(name) == "" {
		if extensions, err := mime.ExtensionsByType(contentType); err == nil && len(extensions) > 0 {
			name += extensions[0]
		}
	}
	return name
}

func normalizeReferenceImageContentType(headerContentType string, data []byte) (string, error) {
	detected := cleanContentType(http.DetectContentType(data))
	if isSupportedReferenceImageContentType(detected) {
		return detected, nil
	}

	headerType := cleanContentType(headerContentType)
	if isSupportedReferenceImageContentType(headerType) && detected == "application/octet-stream" {
		return headerType, nil
	}

	if detected == "" {
		detected = headerType
	}
	return "", fmt.Errorf("reference image content type is not supported: %s", detected)
}

func cleanContentType(contentType string) string {
	return strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
}

func isSupportedReferenceImageContentType(contentType string) bool {
	switch cleanContentType(contentType) {
	case "image/jpeg", "image/jpg", "image/png", "image/webp", "image/gif":
		return true
	default:
		return false
	}
}

func (p *MentaloutProvider) fetchBatch(ctx context.Context, batchID string) (GenerateResult, error) {
	body, status, err := p.doJSONRequest(ctx, http.MethodGet, p.fetchURL(batchID), nil)
	if err != nil {
		return GenerateResult{}, err
	}
	if status < 200 || status >= 300 {
		return GenerateResult{}, fmt.Errorf("fetch batch failed: http %d: %s", status, truncateText(string(body), 500))
	}

	result := extractGenerateResult(body, p.cfg.BatchServiceURL, p.cfg.ProviderAPIBaseURL)
	if result.ProviderTaskID == "" {
		result.ProviderTaskID = batchID
	}
	return result, nil
}

func (p *MentaloutProvider) doJSONRequest(ctx context.Context, method string, targetURL string, payload any) ([]byte, int, error) {
	var raw []byte
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, err
		}
		raw = encoded
	}

	attempts := 1
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		body, status, err := p.doJSONRequestOnce(ctx, method, targetURL, raw)
		if err == nil {
			return body, status, nil
		}
		lastErr = err
		if !isRetriableNetworkError(err) || attempt == attempts {
			return nil, status, err
		}
		if err := sleepWithContext(ctx, time.Duration(attempt)*300*time.Millisecond); err != nil {
			return nil, 0, err
		}
	}

	return nil, 0, lastErr
}

func (p *MentaloutProvider) doJSONRequestOnce(ctx context.Context, method string, targetURL string, raw []byte) ([]byte, int, error) {
	return p.doRequestOnce(ctx, method, targetURL, "application/json", raw)
}

func (p *MentaloutProvider) doRequestOnce(ctx context.Context, method string, targetURL string, contentType string, raw []byte) ([]byte, int, error) {
	var body io.Reader
	if raw != nil {
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, targetURL, body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6")
	req.Header.Set("Cache-Control", "no-cache")
	if strings.TrimSpace(contentType) != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Origin", p.cfg.BatchServiceURL)
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Referer", p.cfg.BatchServiceURL+"/")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36 Edg/147.0.0.0")
	req.Header.Set("sec-ch-ua", `"Microsoft Edge";v="147", "Not.A/Brand";v="8", "Chromium";v="147"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"Windows"`)
	req.Close = true

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return responseBody, resp.StatusCode, nil
}

func isRetriableNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "eof") ||
		strings.Contains(message, "connection reset") ||
		strings.Contains(message, "connection refused") ||
		strings.Contains(message, "unexpected end of file")
}

func IsTemporaryFetchError(err error) bool {
	if err == nil {
		return false
	}
	if isRetriableNetworkError(err) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "fetch batch failed: http 500") ||
		strings.Contains(message, "fetch batch failed: http 502") ||
		strings.Contains(message, "fetch batch failed: http 503") ||
		strings.Contains(message, "fetch batch failed: http 504") ||
		strings.Contains(message, "stream disconnected") ||
		strings.Contains(message, "stream error") ||
		strings.Contains(message, "internal_error")
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (p *MentaloutProvider) submitURL() string {
	return joinURL(p.cfg.BatchServiceURL, "/api/batches")
}

func (p *MentaloutProvider) fetchURL(batchID string) string {
	return joinURL(p.cfg.BatchServiceURL, "/api/batches/"+url.PathEscape(strings.TrimSpace(batchID)))
}

func mergeResult(base GenerateResult, next GenerateResult) GenerateResult {
	if strings.TrimSpace(next.ProviderTaskID) != "" {
		base.ProviderTaskID = next.ProviderTaskID
	}
	if strings.TrimSpace(next.ImageURL) != "" {
		base.ImageURL = next.ImageURL
	}
	base.ImageURLs = mergeImageURLs(base.ImageURLs, next.ImageURLs)
	if base.ImageURL == "" && len(base.ImageURLs) > 0 {
		base.ImageURL = base.ImageURLs[0]
	}
	if strings.TrimSpace(next.ErrorMessage) != "" {
		base.ErrorMessage = next.ErrorMessage
	}
	if next.Status != "" {
		base.Status = next.Status
	}
	if strings.TrimSpace(next.RawResponse) != "" {
		base.RawResponse = next.RawResponse
	}
	return base
}

func extractGenerateResult(body []byte, batchServiceURL string, providerAPIBaseURL string) GenerateResult {
	rawText := strings.TrimSpace(string(body))
	payload, _ := decodePayload(body)

	hints := collectHints(payload, rawText)
	result := GenerateResult{
		ProviderTaskID: hints.BatchID,
		ImageURL:       hints.ImageURL,
		ImageURLs:      absolutizeImageURLs(hints.ImageURLs, batchServiceURL),
		ErrorMessage:   hints.ErrorMessage,
		RawResponse:    rawText,
	}
	if result.ImageURL == "" && len(result.ImageURLs) > 0 {
		result.ImageURL = result.ImageURLs[0]
	}

	switch {
	case hints.HasFailed:
		result.Status = model.ImageTaskStatusFailed
	case hints.HasRunning:
		result.Status = model.ImageTaskStatusGenerating
	case hints.HasSuccess || strings.TrimSpace(result.ImageURL) != "":
		result.Status = model.ImageTaskStatusSuccess
	case strings.Contains(strings.ToLower(rawText), "failed") || strings.Contains(strings.ToLower(rawText), "error"):
		result.Status = model.ImageTaskStatusFailed
	default:
		result.Status = model.ImageTaskStatusGenerating
	}

	if result.ProviderTaskID == "" {
		if matched := batchIDPattern.FindString(rawText); matched != "" {
			result.ProviderTaskID = matched
		}
	}
	if result.ImageURL == "" {
		result.ImageURL = findAnyURL(rawText, batchServiceURL, providerAPIBaseURL)
		if result.Status == model.ImageTaskStatusSuccess && result.ImageURL == "" {
			result.ImageURL = findURLCandidate(rawText)
		}
	}
	if result.ImageURL == "" {
		result.ImageURL = findRelativeImageURL(rawText, batchServiceURL)
	}
	result.ImageURL = absolutizeImageURL(result.ImageURL, batchServiceURL)
	result.ImageURLs = mergeImageURLs(result.ImageURLs, []string{result.ImageURL})
	if result.ErrorMessage == "" && result.Status == model.ImageTaskStatusFailed {
		result.ErrorMessage = inferErrorFromText(rawText)
	}

	if result.Status == model.ImageTaskStatusSuccess && result.ImageURL == "" && strings.Contains(strings.ToLower(rawText), "success") {
		result.ImageURL = findAnyURL(rawText, batchServiceURL, providerAPIBaseURL)
		result.ImageURLs = mergeImageURLs(result.ImageURLs, []string{result.ImageURL})
	}

	return result
}

type resultHints struct {
	BatchID      string
	ImageURL     string
	ImageURLs    []string
	ErrorMessage string
	HasSuccess   bool
	HasRunning   bool
	HasFailed    bool
}

func collectHints(value any, rawText string) resultHints {
	hints := resultHints{}
	collectHintsInto(value, &hints, batchIDPattern, rawText)
	return hints
}

func collectHintsInto(value any, hints *resultHints, batchPattern *regexp.Regexp, rawText string) {
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			child := v[key]
			lowerKey := strings.ToLower(strings.TrimSpace(key))

			switch typed := child.(type) {
			case string:
				trimmed := strings.TrimSpace(typed)
				if trimmed == "" {
					continue
				}
				if isStatusKey(lowerKey) {
					switch normalizeStatus(trimmed) {
					case model.ImageTaskStatusSuccess:
						hints.HasSuccess = true
					case model.ImageTaskStatusFailed:
						hints.HasFailed = true
					default:
						hints.HasRunning = true
					}
				}
				if isErrorKey(lowerKey) && hints.ErrorMessage == "" && !isTrivialMessage(trimmed) {
					hints.ErrorMessage = trimmed
				}
				if isBatchKey(lowerKey) && hints.BatchID == "" {
					if matched := batchPattern.FindString(trimmed); matched != "" {
						hints.BatchID = matched
					} else if strings.HasPrefix(strings.ToLower(trimmed), "batch-") {
						hints.BatchID = trimmed
					}
				}
				if isThumbnailURLKey(lowerKey) {
					continue
				}
				if isURLKey(lowerKey) && isLikelyGeneratedImageURL(trimmed) {
					hints.addImageURL(trimmed)
				}
				if isURLKey(lowerKey) && isRelativeImageURL(trimmed) {
					hints.addImageURL(trimmed)
				}
				if hints.BatchID == "" && batchPattern.MatchString(trimmed) {
					hints.BatchID = batchPattern.FindString(trimmed)
				}
				if isLooseURL(trimmed) && shouldTreatAsImageURL(lowerKey, trimmed) {
					hints.addImageURL(trimmed)
				}
			default:
				collectHintsInto(child, hints, batchPattern, rawText)
			}
		}
	case []any:
		for _, item := range v {
			collectHintsInto(item, hints, batchPattern, rawText)
		}
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return
		}
		if hints.BatchID == "" && batchPattern.MatchString(trimmed) {
			hints.BatchID = batchPattern.FindString(trimmed)
		}
		if isLikelyGeneratedImageURL(trimmed) {
			hints.addImageURL(trimmed)
		}
		if isRelativeImageURL(trimmed) {
			hints.addImageURL(trimmed)
		}
		if hints.ErrorMessage == "" && strings.Contains(strings.ToLower(trimmed), "error") {
			hints.ErrorMessage = trimmed
		}
		switch normalizeStatus(trimmed) {
		case model.ImageTaskStatusSuccess:
			hints.HasSuccess = true
		case model.ImageTaskStatusFailed:
			hints.HasFailed = true
		case model.ImageTaskStatusGenerating:
			hints.HasRunning = true
		}
	}
}

func (h *resultHints) addImageURL(value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	if h.ImageURL == "" {
		h.ImageURL = trimmed
	}
	h.ImageURLs = mergeImageURLs(h.ImageURLs, []string{trimmed})
}

func decodePayload(body []byte) (any, error) {
	var payload any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func normalizeCount(count int) int {
	if count <= 0 {
		return 1
	}
	if count > 6 {
		return 6
	}
	return count
}

func normalizeSize(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "1024x1024"
	}
	return trimmed
}

func normalizeStyle(value string) string {
	trimmed := strings.TrimSpace(value)
	switch strings.ToLower(trimmed) {
	case "", "none", "无":
		return ""
	case "realistic", "写实":
		return "realistic"
	case "anime", "动漫":
		return "anime"
	case "oil", "oil-painting", "oil painting", "油画":
		return "oil painting"
	case "watercolor", "水彩":
		return "watercolor"
	case "pixel", "pixel-art", "pixel art", "像素":
		return "pixel art"
	case "3d", "3d render", "三维":
		return "3d"
	case "cinematic", "电影感":
		return "cinematic"
	default:
		return trimmed
	}
}

func normalizeQuality(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "auto", "default":
		return ""
	case "low", "medium", "high":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeOutputFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "default", "auto":
		return ""
	case "jpeg", "jpg":
		return "jpeg"
	case "webp":
		return "webp"
	case "png":
		return "png"
	default:
		return ""
	}
}

func normalizeStatus(value string) model.ImageTaskStatus {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "success", "succeeded", "done", "completed", "complete", "finished", "finish":
		return model.ImageTaskStatusSuccess
	case "failed", "failure", "error", "canceled", "cancelled", "aborted":
		return model.ImageTaskStatusFailed
	case "running", "generating", "processing", "pending", "queued", "in_progress", "working", "active":
		return model.ImageTaskStatusGenerating
	default:
		return ""
	}
}

func inferErrorFromText(rawText string) string {
	trimmed := strings.TrimSpace(rawText)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) > 240 {
		trimmed = trimmed[:240]
	}
	return trimmed
}

func isStatusKey(key string) bool {
	switch key {
	case "status", "state", "phase", "taskstatus", "batchstatus":
		return true
	default:
		return false
	}
}

func isErrorKey(key string) bool {
	switch key {
	case "error", "errormessage", "reason", "detail", "details", "exception":
		return true
	default:
		return false
	}
}

func isBatchKey(key string) bool {
	switch key {
	case "batchid", "batch_id", "id", "taskid", "task_id", "jobid", "job_id":
		return true
	default:
		return false
	}
}

func isURLKey(key string) bool {
	switch key {
	case "url", "imageurl", "image_url", "resulturl", "result_url", "downloadurl", "download_url", "previewurl", "preview_url", "src", "image":
		return true
	default:
		return false
	}
}

func isThumbnailURLKey(key string) bool {
	return strings.Contains(key, "thumb") ||
		strings.Contains(key, "thumbnail") ||
		strings.Contains(key, "preview")
}

func shouldTreatAsImageURL(key string, value string) bool {
	if isThumbnailURLKey(key) {
		return false
	}
	if strings.Contains(key, "image") || strings.Contains(key, "result") || strings.Contains(key, "download") || strings.Contains(key, "preview") || key == "url" || key == "src" {
		return isLikelyGeneratedImageURL(value)
	}
	return false
}

func isImageURL(value string, rawText string) bool {
	_ = rawText
	return isLikelyGeneratedImageURL(value)
}

func isLooseURL(value string) bool {
	return urlPattern.MatchString(strings.TrimSpace(value))
}

func isKnownServiceURL(value string) bool {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return false
	}
	known := []string{
		"https://image.mentalout.top",
		"https://image.mentalout.top/",
		"http://chunfeng.mentalout.top",
		"http://chunfeng.mentalout.top/",
		"https://chunfeng.mentalout.top",
		"https://chunfeng.mentalout.top/",
	}
	for _, item := range known {
		if normalized == item {
			return true
		}
	}
	return false
}

func findAnyURL(rawText string, batchServiceURL string, providerAPIBaseURL string) string {
	candidates := urlPattern.FindAllString(rawText, -1)
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if candidate == strings.TrimSpace(batchServiceURL) || candidate == strings.TrimSpace(batchServiceURL)+"/" {
			continue
		}
		if candidate == strings.TrimSpace(providerAPIBaseURL) || candidate == strings.TrimSpace(providerAPIBaseURL)+"/" {
			continue
		}
		if strings.Contains(candidate, "/api/batches") {
			continue
		}
		if !isLikelyGeneratedImageURL(candidate) {
			continue
		}
		return candidate
	}
	return ""
}

func findRelativeImageURL(rawText string, batchServiceURL string) string {
	candidates := relativeImagePattern.FindAllString(rawText, -1)
	for _, candidate := range candidates {
		if isRelativeImageURL(candidate) {
			return absolutizeImageURL(candidate, batchServiceURL)
		}
	}
	return ""
}

func findURLCandidate(rawText string) string {
	candidates := urlPattern.FindAllString(rawText, -1)
	if len(candidates) == 0 {
		return ""
	}
	for _, candidate := range candidates {
		if isLikelyGeneratedImageURL(candidate) {
			return candidate
		}
	}
	return ""
}

func isRelativeImageURL(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	return relativeImagePattern.MatchString(trimmed) && !isKnownImageAPIEndpoint(trimmed)
}

func absolutizeImageURL(value string, batchServiceURL string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	if strings.HasPrefix(trimmed, "/") {
		return joinURL(batchServiceURL, trimmed)
	}
	return trimmed
}

func absolutizeImageURLs(values []string, batchServiceURL string) []string {
	urls := make([]string, 0, len(values))
	for _, value := range values {
		urls = mergeImageURLs(urls, []string{absolutizeImageURL(value, batchServiceURL)})
	}
	return urls
}

func mergeImageURLs(base []string, next []string) []string {
	seen := make(map[string]struct{}, len(base)+len(next))
	urls := make([]string, 0, len(base)+len(next))
	for _, value := range append(base, next...) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := canonicalImageURLKey(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		urls = append(urls, trimmed)
	}
	return urls
}

func canonicalImageURLKey(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	candidate := trimmed
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Path != "" {
		candidate = parsed.Path
	}
	candidate = strings.ReplaceAll(candidate, "\\", "/")
	candidate = strings.ReplaceAll(candidate, "/thumbs/", "/")
	candidate = path.Clean(candidate)
	if candidate == "." {
		return ""
	}
	return candidate
}

func normalizeBaseURL(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		trimmed = fallback
	}
	return strings.TrimRight(trimmed, "/")
}

func normalizeProviderAPIBaseURL(value string) string {
	return normalizeBaseURL(value, "https://chunfeng.mentalout.top")
}

func joinURL(base string, path string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

func truncateText(text string, max int) string {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) <= max {
		return trimmed
	}
	return trimmed[:max] + "..."
}

func isTrivialMessage(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "ok", "success", "done", "completed":
		return true
	default:
		return false
	}
}

func isLikelyGeneratedImageURL(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || isKnownServiceURL(trimmed) || isKnownImageAPIEndpoint(trimmed) || isLikelyGeneratedThumbnailURL(trimmed) {
		return false
	}

	candidate := trimmed
	if parsed, err := url.Parse(trimmed); err == nil {
		if parsed.Path == "" {
			return false
		}
		candidate = parsed.Path
	}
	candidate = strings.ReplaceAll(candidate, "\\", "/")
	candidate = path.Clean(candidate)
	if candidate == "." {
		return false
	}

	if isImageFilePath(candidate) {
		return true
	}

	return strings.HasPrefix(candidate, "/results/") ||
		strings.HasPrefix(candidate, "/images/") ||
		strings.HasPrefix(candidate, "/uploads/") ||
		strings.HasPrefix(candidate, "/outputs/")
}

func isKnownImageAPIEndpoint(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	candidate := trimmed
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Path != "" {
		candidate = parsed.Path
	}
	candidate = path.Clean("/" + strings.TrimLeft(strings.ToLower(strings.ReplaceAll(candidate, "\\", "/")), "/"))
	switch candidate {
	case "/images/edits", "/images/generations", "/images/variations",
		"/v1/images/edits", "/v1/images/generations", "/v1/images/variations":
		return true
	default:
		return false
	}
}

func isLikelyGeneratedThumbnailURL(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	candidate := trimmed
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Path != "" {
		candidate = parsed.Path
	}
	candidate = strings.ToLower(strings.ReplaceAll(candidate, "\\", "/"))
	return strings.Contains(candidate, "/thumbs/") ||
		strings.Contains(candidate, "/thumbnail/") ||
		strings.Contains(candidate, "/thumbnails/")
}

func isImageFilePath(value string) bool {
	ext := strings.ToLower(path.Ext(strings.TrimSpace(value)))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp", ".tif", ".tiff", ".avif":
		return true
	default:
		return false
	}
}
