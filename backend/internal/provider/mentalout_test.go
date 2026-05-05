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
		Prompt:       "一只机械狐狸",
		Size:         "2048x1152",
		Style:        "油画",
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
		Prompt:            "一只站在城市屋顶上的机械狐狸",
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
	if got := receivedPayload["prompt"]; got != "一只站在城市屋顶上的机械狐狸" {
		t.Fatalf("unexpected prompt in payload: %#v", got)
	}
	if _, ok := receivedPayload["referenceImageUrl"]; ok {
		t.Fatalf("did not expect referenceImageUrl inside multipart payload: %#v", receivedPayload)
	}
}

func TestSubmitBatchRetriesEOFAndUsesBrowserHeaders(t *testing.T) {
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

		if atomic.AddInt32(&attempts, 1) == 1 {
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Fatalf("response writer does not support hijacking")
			}
			conn, _, err := hijacker.Hijack()
			if err != nil {
				t.Fatalf("hijack failed: %v", err)
			}
			_ = conn.Close()
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"batch-test","status":"running"}`))
	}))
	defer server.Close()

	provider := NewMentaloutProvider(MentaloutConfig{
		BatchServiceURL:    server.URL,
		ProviderAPIBaseURL: "https://chunfeng.mentalout.top",
		APIKey:             "test-key",
	})

	result, err := provider.submitBatch(context.Background(), GenerateRequest{Prompt: "一个人", Size: "1024x1024", Count: 1})
	if err != nil {
		t.Fatalf("submit batch failed: %v", err)
	}
	if atomic.LoadInt32(&attempts) != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
	if result.ProviderTaskID != "batch-test" {
		t.Fatalf("expected batch id, got %q", result.ProviderTaskID)
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
