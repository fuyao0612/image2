package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"image2/backend/internal/model"
)

func TestExtractGenerateResultFromSubmitResponse(t *testing.T) {
	body := []byte(`{"data":{"batchId":"batch-abc123","status":"running"}}`)

	result := extractGenerateResult(body, "https://image.mentalout.top", "https://chunfeng.mentalout.top")

	if result.ProviderTaskID != "batch-abc123" {
		t.Fatalf("expected batch id, got %q", result.ProviderTaskID)
	}
	if result.Status != model.ImageTaskStatusGenerating {
		t.Fatalf("expected generating status, got %q", result.Status)
	}
}

func TestExtractGenerateResultFromSuccessResponse(t *testing.T) {
	body := []byte(`{"status":"success","items":[{"status":"success","imageUrl":"https://cdn.example.com/result.png"}]}`)

	result := extractGenerateResult(body, "https://image.mentalout.top", "https://chunfeng.mentalout.top")

	if result.Status != model.ImageTaskStatusSuccess {
		t.Fatalf("expected success status, got %q", result.Status)
	}
	if result.ImageURL != "https://cdn.example.com/result.png" {
		t.Fatalf("expected image url, got %q", result.ImageURL)
	}
}

func TestExtractGenerateResultSkipsAPIEndpointURLs(t *testing.T) {
	body := []byte(`{"status":"success","message":"http://chunfeng.mentalout.top/v1/images/edits","detail":"https://cdn.example.com/results/batch-abc123/final.png"}`)

	result := extractGenerateResult(body, "https://image.mentalout.top", "https://chunfeng.mentalout.top")

	if result.Status != model.ImageTaskStatusSuccess {
		t.Fatalf("expected success status, got %q", result.Status)
	}
	if result.ImageURL != "https://cdn.example.com/results/batch-abc123/final.png" {
		t.Fatalf("expected generated image url, got %q", result.ImageURL)
	}
	for _, candidate := range result.ImageURLs {
		if strings.Contains(candidate, "/v1/images/edits") {
			t.Fatalf("unexpected api endpoint in image urls: %#v", result.ImageURLs)
		}
	}
}

func TestExtractGenerateResultSkipsImagesEditsEndpoint(t *testing.T) {
	body := []byte(`{"status":"success","url":"https://image.mentalout.top/images/edits","message":"ok"}`)

	result := extractGenerateResult(body, "https://image.mentalout.top", "https://chunfeng.mentalout.top")

	if result.ImageURL != "" {
		t.Fatalf("expected images edits endpoint to be skipped, got %q", result.ImageURL)
	}
	if len(result.ImageURLs) != 0 {
		t.Fatalf("expected no image urls, got %#v", result.ImageURLs)
	}
}

func TestExtractGenerateResultFromSucceededResponseWithRelativeImageURL(t *testing.T) {
	body := []byte(`{"id":"batch-abc123","status":"succeeded","tasks":[{"status":"succeeded","imageUrl":"/results/batch-abc123/task-result.png"}]}`)

	result := extractGenerateResult(body, "https://image.mentalout.top", "https://chunfeng.mentalout.top")

	if result.Status != model.ImageTaskStatusSuccess {
		t.Fatalf("expected success status, got %q", result.Status)
	}
	if result.ImageURL != "https://image.mentalout.top/results/batch-abc123/task-result.png" {
		t.Fatalf("expected absolute image url, got %q", result.ImageURL)
	}
	if result.ErrorMessage != "" {
		t.Fatalf("expected empty error message, got %q", result.ErrorMessage)
	}
}

func TestExtractGenerateResultCollectsMultipleImageURLs(t *testing.T) {
	body := []byte(`{"id":"batch-abc123","status":"succeeded","tasks":[{"status":"succeeded","imageUrl":"/results/batch-abc123/one.png"},{"status":"succeeded","imageUrl":"/results/batch-abc123/two.png"}]}`)

	result := extractGenerateResult(body, "https://image.mentalout.top", "https://chunfeng.mentalout.top")

	if result.Status != model.ImageTaskStatusSuccess {
		t.Fatalf("expected success status, got %q", result.Status)
	}
	if len(result.ImageURLs) != 2 {
		t.Fatalf("expected 2 image urls, got %d: %#v", len(result.ImageURLs), result.ImageURLs)
	}
	if result.ImageURL != "https://image.mentalout.top/results/batch-abc123/one.png" {
		t.Fatalf("expected first image url, got %q", result.ImageURL)
	}
}

func TestBuildSubmitPayloadIncludesGenerationOptions(t *testing.T) {
	provider := NewMentaloutProvider(MentaloutConfig{
		BatchServiceURL:    "https://image.mentalout.top",
		ProviderAPIBaseURL: "https://chunfeng.mentalout.top",
		APIKey:             "test-key",
	})

	payload := provider.buildSubmitPayload(GenerateRequest{
		Prompt:       "test robot fox",
		Size:         "2048x1152",
		Style:        "oil",
		Count:        6,
		Quality:      "high",
		OutputFormat: "webp",
	})

	if payload["style"] != "oil painting" {
		t.Fatalf("expected normalized style, got %#v", payload["style"])
	}
	if payload["count"] != 6 {
		t.Fatalf("expected count 6, got %#v", payload["count"])
	}
	tuning, ok := payload["tuning"].(map[string]any)
	if !ok {
		t.Fatalf("expected tuning map, got %#v", payload["tuning"])
	}
	if tuning["quality"] != "high" || tuning["outputFormat"] != "webp" {
		t.Fatalf("unexpected tuning: %#v", tuning)
	}
	if _, ok := payload["referenceImageUrl"]; ok {
		t.Fatalf("did not expect referenceImageUrl in plain payload: %#v", payload)
	}
}

func TestConfiguredRejectsPlaceholderKeys(t *testing.T) {
	cases := []string{
		"",
		"change-me",
		"replace-with-your-api-key",
		"replace-with-real-provider-key",
	}

	for _, key := range cases {
		provider := NewMentaloutProvider(MentaloutConfig{
			BatchServiceURL:    "https://image.mentalout.top",
			ProviderAPIBaseURL: "https://chunfeng.mentalout.top",
			APIKey:             key,
		})
		if provider.Configured() {
			t.Fatalf("expected key %q to be treated as unconfigured", key)
		}
	}
}

func TestSubmitBatchWithReferenceImageUsesMultipart(t *testing.T) {
	var receivedPayload map[string]any
	var receivedFileName string
	var receivedFileBytes []byte
	var receivedContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/reference.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
		case "/api/batches":
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				t.Fatalf("expected multipart request, got %s", r.Header.Get("Content-Type"))
			}
			if err := r.ParseMultipartForm(8 << 20); err != nil {
				t.Fatalf("parse multipart form failed: %v", err)
			}
			if payloadRaw := r.FormValue("payload"); payloadRaw != "" {
				if err := json.Unmarshal([]byte(payloadRaw), &receivedPayload); err != nil {
					t.Fatalf("unmarshal payload failed: %v", err)
				}
			} else {
				t.Fatalf("expected payload form field")
			}
			file, header, err := r.FormFile("image")
			if err != nil {
				t.Fatalf("expected image form file: %v", err)
			}
			defer file.Close()
			receivedFileName = header.Filename
			receivedFileBytes, err = io.ReadAll(file)
			if err != nil {
				t.Fatalf("read uploaded image failed: %v", err)
			}
			receivedContentType = header.Header.Get("Content-Type")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"batch-ref","status":"running"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	provider := NewMentaloutProvider(MentaloutConfig{
		BatchServiceURL:    server.URL,
		ProviderAPIBaseURL: "https://chunfeng.mentalout.top",
		APIKey:             "test-key",
	})

	result, err := provider.submitBatch(context.Background(), GenerateRequest{
		Prompt:            "test robot fox on rooftop",
		Size:              "1024x1024",
		Count:             1,
		ReferenceImageURL: server.URL + "/reference.png",
	})
	if err != nil {
		t.Fatalf("submit batch failed: %v", err)
	}
	if result.ProviderTaskID != "batch-ref" {
		t.Fatalf("expected provider task id batch-ref, got %q", result.ProviderTaskID)
	}
	if receivedFileName == "" || !strings.Contains(receivedFileName, "reference") {
		t.Fatalf("unexpected uploaded file name: %q", receivedFileName)
	}
	if len(receivedFileBytes) == 0 {
		t.Fatalf("expected uploaded image bytes")
	}
	if !strings.HasPrefix(receivedContentType, "image/") {
		t.Fatalf("unexpected uploaded content type: %q", receivedContentType)
	}
	if got := receivedPayload["prompt"]; got != "test robot fox on rooftop" {
		t.Fatalf("unexpected prompt in payload: %#v", got)
	}
	if _, ok := receivedPayload["referenceImageUrl"]; ok {
		t.Fatalf("did not expect referenceImageUrl inside multipart payload: %#v", receivedPayload)
	}
}

func TestSubmitBatchWithProvidedReferenceImageSkipsDownload(t *testing.T) {
	var referenceDownloaded bool
	var receivedFileBytes []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/reference.png":
			referenceDownloaded = true
			w.WriteHeader(http.StatusInternalServerError)
		case "/api/batches":
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				t.Fatalf("expected multipart request, got %s", r.Header.Get("Content-Type"))
			}
			if err := r.ParseMultipartForm(8 << 20); err != nil {
				t.Fatalf("parse multipart form failed: %v", err)
			}
			file, _, err := r.FormFile("image")
			if err != nil {
				t.Fatalf("expected image form file: %v", err)
			}
			defer file.Close()
			receivedFileBytes, err = io.ReadAll(file)
			if err != nil {
				t.Fatalf("read uploaded image failed: %v", err)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"batch-ref","status":"running"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	provider := NewMentaloutProvider(MentaloutConfig{
		BatchServiceURL:    server.URL,
		ProviderAPIBaseURL: "https://chunfeng.mentalout.top",
		APIKey:             "test-key",
	})

	imageData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	result, err := provider.submitBatch(context.Background(), GenerateRequest{
		Prompt:            "test prompt",
		Size:              "1024x1024",
		Count:             1,
		ReferenceImageURL: server.URL + "/reference.png",
		ReferenceImage: &ReferenceImage{
			Name:        "reference.png",
			ContentType: "image/png",
			Data:        imageData,
		},
	})
	if err != nil {
		t.Fatalf("submit batch failed: %v", err)
	}
	if referenceDownloaded {
		t.Fatalf("expected provided reference image to skip URL download")
	}
	if string(receivedFileBytes) != string(imageData) {
		t.Fatalf("unexpected uploaded image bytes: %#v", receivedFileBytes)
	}
	if result.ProviderTaskID != "batch-ref" {
		t.Fatalf("expected provider task id batch-ref, got %q", result.ProviderTaskID)
	}
}

func TestSubmitBatchRejectsFakeReferenceImage(t *testing.T) {
	var batchCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/reference.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("<html>not an image</html>"))
		case "/api/batches":
			batchCalled = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"batch-ref","status":"running"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	provider := NewMentaloutProvider(MentaloutConfig{
		BatchServiceURL:    server.URL,
		ProviderAPIBaseURL: "https://chunfeng.mentalout.top",
		APIKey:             "test-key",
	})

	_, err := provider.submitBatch(context.Background(), GenerateRequest{
		Prompt:            "test prompt",
		Size:              "1024x1024",
		Count:             1,
		ReferenceImageURL: server.URL + "/reference.png",
	})
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("expected unsupported reference image error, got %v", err)
	}
	if batchCalled {
		t.Fatalf("batch endpoint should not be called after invalid reference image")
	}
}

func TestSubmitBatchDoesNotRetryEOFAndUsesBrowserHeaders(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/batches" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if strings.TrimSpace(r.Header.Get("User-Agent")) == "" {
			t.Fatalf("expected browser user agent header")
		}
		if strings.TrimSpace(r.Header.Get("Origin")) == "" {
			t.Fatalf("expected origin header")
		}

		atomic.AddInt32(&attempts, 1)
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Fatalf("response writer does not support hijacking")
		}
		conn, _, err := hijacker.Hijack()
		if err != nil {
			t.Fatalf("hijack failed: %v", err)
		}
		_ = conn.Close()
	}))
	defer server.Close()

	provider := NewMentaloutProvider(MentaloutConfig{
		BatchServiceURL:    server.URL,
		ProviderAPIBaseURL: "https://chunfeng.mentalout.top",
		APIKey:             "test-key",
	})

	_, err := provider.submitBatch(context.Background(), GenerateRequest{Prompt: "test prompt", Size: "1024x1024", Count: 1})
	if err == nil {
		t.Fatalf("expected submit batch to return the first network error")
	}
	if atomic.LoadInt32(&attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestFetchBatchReturnsTemporaryNetworkErrorWithoutImmediateRetry(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/batches/status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		atomic.AddInt32(&attempts, 1)
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Fatalf("response writer does not support hijacking")
		}
		conn, _, err := hijacker.Hijack()
		if err != nil {
			t.Fatalf("hijack failed: %v", err)
		}
		_ = conn.Close()
	}))
	defer server.Close()

	provider := NewMentaloutProvider(MentaloutConfig{
		BatchServiceURL:    server.URL,
		ProviderAPIBaseURL: "https://chunfeng.mentalout.top",
		APIKey:             "test-key",
	})

	_, err := provider.fetchBatch(context.Background(), "batch-test")
	if err == nil {
		t.Fatalf("expected fetch batch to return the first network error")
	}
	if atomic.LoadInt32(&attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestFetchBatchFallsBackToDetailEndpointWhenStatusEndpointFails(t *testing.T) {
	var statusAttempts int32
	var detailAttempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/batches/status":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected status method: %s", r.Method)
			}
			atomic.AddInt32(&statusAttempts, 1)
			http.Error(w, `{"error":"temporary unavailable"}`, http.StatusNotFound)
		case "/api/batches/batch-test":
			if r.Method != http.MethodGet {
				t.Fatalf("unexpected detail method: %s", r.Method)
			}
			atomic.AddInt32(&detailAttempts, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"batch-test","status":"success","imageUrl":"https://image.mentalout.top/results/demo.png"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	provider := NewMentaloutProvider(MentaloutConfig{
		BatchServiceURL:    server.URL,
		ProviderAPIBaseURL: "https://chunfeng.mentalout.top",
		APIKey:             "test-key",
	})

	result, err := provider.fetchBatch(context.Background(), "batch-test")
	if err != nil {
		t.Fatalf("expected fallback fetch to succeed, got %v", err)
	}
	if result.Status != model.ImageTaskStatusSuccess {
		t.Fatalf("expected success status, got %q", result.Status)
	}
	if result.ImageURL != "https://image.mentalout.top/results/demo.png" {
		t.Fatalf("unexpected image url: %q", result.ImageURL)
	}
	if atomic.LoadInt32(&statusAttempts) != 1 {
		t.Fatalf("expected 1 status attempt, got %d", statusAttempts)
	}
	if atomic.LoadInt32(&detailAttempts) != 1 {
		t.Fatalf("expected 1 detail attempt, got %d", detailAttempts)
	}
}

func TestExtractGenerateResultFromFailureResponse(t *testing.T) {
	body := []byte(`{"status":"failed","errorMessage":"provider refused prompt"}`)

	result := extractGenerateResult(body, "https://image.mentalout.top", "https://chunfeng.mentalout.top")

	if result.Status != model.ImageTaskStatusFailed {
		t.Fatalf("expected failed status, got %q", result.Status)
	}
	if result.ErrorMessage != "provider refused prompt" {
		t.Fatalf("expected error message, got %q", result.ErrorMessage)
	}
}
