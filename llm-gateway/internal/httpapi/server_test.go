package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"llm-gateway/internal/config"
	"llm-gateway/internal/gateway"
	"llm-gateway/internal/provider"
	"llm-gateway/internal/store"
)

func TestChatCompletionAndUsage(t *testing.T) {
	svc, memoryStore := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"mock-chat",
		"messages":[{"role":"user","content":"hello"}]
	}`))
	rec := httptest.NewRecorder()

	svc.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp gateway.ChatCompletionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Object != "chat.completion" {
		t.Fatalf("unexpected object: %s", resp.Object)
	}
	if len(resp.Choices) != 1 || !strings.Contains(resp.Choices[0].Message.Content, "hello") {
		t.Fatalf("unexpected completion: %+v", resp.Choices)
	}

	records, err := memoryStore.ListUsage(context.Background(), 10)
	if err != nil {
		t.Fatalf("list usage: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 usage record, got %d", len(records))
	}
	if !records[0].Success || records[0].Provider != "default-mock" {
		t.Fatalf("unexpected usage record: %+v", records[0])
	}
}

func TestStreamingCompletionAndAdminProviderCRUD(t *testing.T) {
	svc, memoryStore := newTestServer(t)

	streamReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"mock-chat",
		"stream":true,
		"messages":[{"role":"user","content":"hello stream"}]
	}`))
	streamRec := httptest.NewRecorder()
	svc.Routes().ServeHTTP(streamRec, streamReq)

	if streamRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for stream, got %d: %s", streamRec.Code, streamRec.Body.String())
	}
	body := streamRec.Body.String()
	if !strings.Contains(body, "data: [DONE]") {
		t.Fatalf("expected DONE marker, got: %s", body)
	}

	adminPayload := gateway.ProviderConfig{
		Type:          "mock",
		Enabled:       true,
		IsDefault:     false,
		ModelPrefixes: []string{"tenant-"},
	}
	payloadBytes, _ := json.Marshal(adminPayload)

	putReq := httptest.NewRequest(http.MethodPut, "/admin/providers/tenant-mock", bytes.NewReader(payloadBytes))
	putReq.Header.Set("Authorization", "Bearer admin-token")
	putRec := httptest.NewRecorder()
	svc.Routes().ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for provider upsert, got %d: %s", putRec.Code, putRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/admin/providers", nil)
	getReq.Header.Set("Authorization", "Bearer admin-token")
	getRec := httptest.NewRecorder()
	svc.Routes().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for provider list, got %d: %s", getRec.Code, getRec.Body.String())
	}
	if !strings.Contains(getRec.Body.String(), "tenant-mock") {
		t.Fatalf("expected provider list to include tenant-mock: %s", getRec.Body.String())
	}

	usageReq := httptest.NewRequest(http.MethodGet, "/admin/usages?limit=5", nil)
	usageReq.Header.Set("Authorization", "Bearer admin-token")
	usageRec := httptest.NewRecorder()
	svc.Routes().ServeHTTP(usageRec, usageReq)
	if usageRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for usage list, got %d: %s", usageRec.Code, usageRec.Body.String())
	}

	usageRecords, err := memoryStore.ListUsage(context.Background(), 10)
	if err != nil {
		t.Fatalf("list usage: %v", err)
	}
	if len(usageRecords) == 0 {
		t.Fatalf("expected usage records")
	}
}

func TestEmbeddingsEndpoint(t *testing.T) {
	svc, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", strings.NewReader(`{
		"model":"mock-embedding",
		"input":["alpha","beta"]
	}`))
	rec := httptest.NewRecorder()

	svc.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "\"embedding\"") {
		t.Fatalf("expected embedding response, got %s", string(body))
	}
}

func newTestServer(t *testing.T) (*Server, *store.MemoryStore) {
	t.Helper()

	memoryStore := store.NewMemoryStore()
	if err := memoryStore.UpsertProvider(context.Background(), gateway.ProviderConfig{
		Name:          "default-mock",
		Type:          "mock",
		Enabled:       true,
		IsDefault:     true,
		ModelPrefixes: []string{"mock-"},
	}); err != nil {
		t.Fatalf("seed provider: %v", err)
	}

	cfg := config.Config{
		AdminToken:     "admin-token",
		DefaultTimeout: time.Second,
	}
	manager := provider.NewManager(memoryStore)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewServer(cfg, memoryStore, manager, nil, logger), memoryStore
}
