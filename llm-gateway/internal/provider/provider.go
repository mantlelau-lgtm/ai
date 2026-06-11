package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"llm-gateway/internal/gateway"
	"llm-gateway/internal/store"
)

type Provider interface {
	ListModels(ctx context.Context, cfg gateway.ProviderConfig) ([]gateway.ModelInfo, error)
	Chat(ctx context.Context, cfg gateway.ProviderConfig, req gateway.ChatCompletionRequest) (gateway.ChatCompletionResponse, error)
	StreamChat(ctx context.Context, cfg gateway.ProviderConfig, req gateway.ChatCompletionRequest, write func([]byte) error) (gateway.Usage, error)
	Embeddings(ctx context.Context, cfg gateway.ProviderConfig, req gateway.EmbeddingRequest) (gateway.EmbeddingResponse, error)
}

type Manager struct {
	store     store.Store
	providers map[string]Provider
	mu        sync.RWMutex
	models    map[string]gateway.ModelRoute
}

func NewManager(store store.Store) *Manager {
	return &Manager{
		store: store,
		providers: map[string]Provider{
			"mock":   MockProvider{},
			"openai": OpenAIProvider{HTTPClient: &http.Client{Timeout: 2 * time.Minute}},
		},
		models: map[string]gateway.ModelRoute{},
	}
}

func (m *Manager) SetModelRoutes(items []gateway.ModelRoute) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.models = map[string]gateway.ModelRoute{}
	for _, item := range items {
		if item.Enabled {
			m.models[item.Name] = item
		}
	}
}

func (m *Manager) ResolveProvider(ctx context.Context, model string) (gateway.ProviderConfig, Provider, string, error) {
	providers, err := m.store.ListProviders(ctx)
	if err != nil {
		return gateway.ProviderConfig{}, nil, "", err
	}

	m.mu.RLock()
	if route, ok := m.models[model]; ok {
		for _, candidate := range providers {
			if candidate.Enabled && candidate.Name == route.Provider {
				handler, ok := m.providers[candidate.Type]
				if !ok {
					m.mu.RUnlock()
					return gateway.ProviderConfig{}, nil, "", fmt.Errorf("unsupported provider type %q", candidate.Type)
				}
				m.mu.RUnlock()
				return candidate, handler, route.UpstreamModel, nil
			}
		}
		m.mu.RUnlock()
		return gateway.ProviderConfig{}, nil, "", fmt.Errorf("provider %q for model %q not found", route.Provider, model)
	}
	m.mu.RUnlock()

	var defaultProvider *gateway.ProviderConfig
	for _, candidate := range providers {
		if !candidate.Enabled {
			continue
		}
		if candidate.IsDefault {
			c := candidate
			defaultProvider = &c
		}
		for _, prefix := range candidate.ModelPrefixes {
			if prefix != "" && strings.HasPrefix(model, prefix) {
				handler, ok := m.providers[candidate.Type]
				if !ok {
					return gateway.ProviderConfig{}, nil, "", fmt.Errorf("unsupported provider type %q", candidate.Type)
				}
				return candidate, handler, model, nil
			}
		}
	}

	if defaultProvider == nil {
		return gateway.ProviderConfig{}, nil, "", fmt.Errorf("no enabled provider matched model %q and no default provider configured", model)
	}
	handler, ok := m.providers[defaultProvider.Type]
	if !ok {
		return gateway.ProviderConfig{}, nil, "", fmt.Errorf("unsupported provider type %q", defaultProvider.Type)
	}
	return *defaultProvider, handler, model, nil
}

func (m *Manager) AggregateModels(ctx context.Context) ([]gateway.ModelInfo, error) {
	m.mu.RLock()
	if len(m.models) > 0 {
		models := make([]gateway.ModelInfo, 0, len(m.models))
		for _, item := range m.models {
			models = append(models, gateway.ModelInfo{
				ID:      item.Name,
				Object:  "model",
				OwnedBy: item.OwnedBy,
			})
		}
		m.mu.RUnlock()
		return models, nil
	}
	m.mu.RUnlock()

	providers, err := m.store.ListProviders(ctx)
	if err != nil {
		return nil, err
	}

	var models []gateway.ModelInfo
	seen := map[string]struct{}{}
	for _, candidate := range providers {
		if !candidate.Enabled {
			continue
		}
		handler, ok := m.providers[candidate.Type]
		if !ok {
			continue
		}
		items, err := handler.ListModels(ctx, candidate)
		if err != nil {
			return nil, fmt.Errorf("list models for %s: %w", candidate.Name, err)
		}
		for _, model := range items {
			if _, ok := seen[model.ID]; ok {
				continue
			}
			seen[model.ID] = struct{}{}
			models = append(models, model)
		}
	}
	return models, nil
}

type MockProvider struct{}

func (MockProvider) ListModels(context.Context, gateway.ProviderConfig) ([]gateway.ModelInfo, error) {
	return []gateway.ModelInfo{
		{ID: "mock-chat", Object: "model", OwnedBy: "mock"},
		{ID: "mock-embedding", Object: "model", OwnedBy: "mock"},
	}, nil
}

func (MockProvider) Chat(_ context.Context, cfg gateway.ProviderConfig, req gateway.ChatCompletionRequest) (gateway.ChatCompletionResponse, error) {
	content := buildMockReply(cfg.Name, req.Messages)
	usage := estimateUsage(req.Messages, content)
	return gateway.ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-mock-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []gateway.ChatChoice{{
			Index: 0,
			Message: gateway.ChatMessage{
				Role:    "assistant",
				Content: content,
			},
			FinishReason: "stop",
		}},
		Usage: usage,
	}, nil
}

func (MockProvider) StreamChat(_ context.Context, cfg gateway.ProviderConfig, req gateway.ChatCompletionRequest, write func([]byte) error) (gateway.Usage, error) {
	content := buildMockReply(cfg.Name, req.Messages)
	chunkID := fmt.Sprintf("chatcmpl-mock-%d", time.Now().UnixNano())
	createdAt := time.Now().Unix()

	firstChunk := gateway.ChatCompletionChunk{
		ID:      chunkID,
		Object:  "chat.completion.chunk",
		Created: createdAt,
		Model:   req.Model,
		Choices: []gateway.ChatChunkChoice{{
			Index: 0,
			Delta: gateway.ChatMessageDelta{Role: "assistant"},
		}},
	}
	if err := writeSSEChunk(firstChunk, write); err != nil {
		return gateway.Usage{}, err
	}

	for _, token := range strings.Fields(content) {
		chunk := gateway.ChatCompletionChunk{
			ID:      chunkID,
			Object:  "chat.completion.chunk",
			Created: createdAt,
			Model:   req.Model,
			Choices: []gateway.ChatChunkChoice{{
				Index: 0,
				Delta: gateway.ChatMessageDelta{Content: token + " "},
			}},
		}
		if err := writeSSEChunk(chunk, write); err != nil {
			return gateway.Usage{}, err
		}
	}

	stop := "stop"
	finalChunk := gateway.ChatCompletionChunk{
		ID:      chunkID,
		Object:  "chat.completion.chunk",
		Created: createdAt,
		Model:   req.Model,
		Choices: []gateway.ChatChunkChoice{{
			Index:        0,
			Delta:        gateway.ChatMessageDelta{},
			FinishReason: &stop,
		}},
	}
	usage := estimateUsage(req.Messages, content)
	if req.StreamOptions != nil && req.StreamOptions.IncludeUsage {
		finalChunk.Usage = &usage
	}
	if err := writeSSEChunk(finalChunk, write); err != nil {
		return gateway.Usage{}, err
	}
	if err := write([]byte("data: [DONE]\n\n")); err != nil {
		return gateway.Usage{}, err
	}
	return usage, nil
}

func (MockProvider) Embeddings(_ context.Context, _ gateway.ProviderConfig, req gateway.EmbeddingRequest) (gateway.EmbeddingResponse, error) {
	inputs := normalizeEmbeddingInput(req.Input)
	data := make([]gateway.EmbeddingData, 0, len(inputs))
	totalChars := 0
	for idx, item := range inputs {
		totalChars += len(item)
		data = append(data, gateway.EmbeddingData{
			Object:    "embedding",
			Index:     idx,
			Embedding: deterministicEmbedding(item),
		})
	}
	usage := gateway.Usage{PromptTokens: max(1, totalChars/4), TotalTokens: max(1, totalChars/4)}
	return gateway.EmbeddingResponse{
		Object: "list",
		Data:   data,
		Model:  req.Model,
		Usage:  usage,
	}, nil
}

type OpenAIProvider struct {
	HTTPClient *http.Client
}

func (p OpenAIProvider) ListModels(ctx context.Context, cfg gateway.ProviderConfig) ([]gateway.ModelInfo, error) {
	body, err := p.doJSON(ctx, cfg, http.MethodGet, "/models", nil)
	if err != nil {
		return nil, err
	}
	var response gateway.ModelsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decode models response: %w", err)
	}
	return response.Data, nil
}

func (p OpenAIProvider) Chat(ctx context.Context, cfg gateway.ProviderConfig, req gateway.ChatCompletionRequest) (gateway.ChatCompletionResponse, error) {
	req.Stream = false
	body, err := json.Marshal(req)
	if err != nil {
		return gateway.ChatCompletionResponse{}, fmt.Errorf("marshal chat request: %w", err)
	}
	responseBody, err := p.doJSON(ctx, cfg, http.MethodPost, "/chat/completions", bytes.NewReader(body))
	if err != nil {
		return gateway.ChatCompletionResponse{}, err
	}
	var response gateway.ChatCompletionResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return gateway.ChatCompletionResponse{}, fmt.Errorf("decode chat response: %w", err)
	}
	return response, nil
}

func (p OpenAIProvider) StreamChat(ctx context.Context, cfg gateway.ProviderConfig, req gateway.ChatCompletionRequest, write func([]byte) error) (gateway.Usage, error) {
	req.Stream = true
	if req.StreamOptions == nil {
		req.StreamOptions = &gateway.ChatStreamOptions{IncludeUsage: true}
	} else {
		req.StreamOptions.IncludeUsage = true
	}

	body, err := json.Marshal(req)
	if err != nil {
		return gateway.Usage{}, fmt.Errorf("marshal stream chat request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, joinURL(cfg.BaseURL, "/chat/completions"), bytes.NewReader(body))
	if err != nil {
		return gateway.Usage{}, fmt.Errorf("create stream chat request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	resp, err := p.client().Do(httpReq)
	if err != nil {
		return gateway.Usage{}, fmt.Errorf("call upstream stream chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		payload, _ := io.ReadAll(resp.Body)
		return gateway.Usage{}, fmt.Errorf("upstream stream chat failed: %s", strings.TrimSpace(string(payload)))
	}

	var usage gateway.Usage
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := write([]byte("\n")); err != nil {
				return gateway.Usage{}, err
			}
			continue
		}
		if strings.HasPrefix(line, "data: ") {
			payload := strings.TrimPrefix(line, "data: ")
			if payload == "[DONE]" {
				if err := write([]byte("data: [DONE]\n\n")); err != nil {
					return gateway.Usage{}, err
				}
				break
			}
			var chunk gateway.ChatCompletionChunk
			if err := json.Unmarshal([]byte(payload), &chunk); err == nil && chunk.Usage != nil {
				usage = *chunk.Usage
			}
			if err := write([]byte(line + "\n")); err != nil {
				return gateway.Usage{}, err
			}
			continue
		}
		if err := write([]byte(line + "\n")); err != nil {
			return gateway.Usage{}, err
		}
	}
	if err := scanner.Err(); err != nil {
		return gateway.Usage{}, fmt.Errorf("read upstream stream: %w", err)
	}
	return usage, nil
}

func (p OpenAIProvider) Embeddings(ctx context.Context, cfg gateway.ProviderConfig, req gateway.EmbeddingRequest) (gateway.EmbeddingResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return gateway.EmbeddingResponse{}, fmt.Errorf("marshal embeddings request: %w", err)
	}
	responseBody, err := p.doJSON(ctx, cfg, http.MethodPost, "/embeddings", bytes.NewReader(body))
	if err != nil {
		return gateway.EmbeddingResponse{}, err
	}
	var response gateway.EmbeddingResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return gateway.EmbeddingResponse{}, fmt.Errorf("decode embeddings response: %w", err)
	}
	return response, nil
}

func (p OpenAIProvider) doJSON(ctx context.Context, cfg gateway.ProviderConfig, method, path string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, joinURL(cfg.BaseURL, path), body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	resp, err := p.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("call upstream: %w", err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read upstream response: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("upstream returned %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	return payload, nil
}

func (p OpenAIProvider) client() *http.Client {
	if p.HTTPClient != nil {
		return p.HTTPClient
	}
	return &http.Client{Timeout: 2 * time.Minute}
}

func buildMockReply(providerName string, messages []gateway.ChatMessage) string {
	last := ""
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && messages[i].Content != "" {
			last = messages[i].Content
			break
		}
	}
	if last == "" {
		last = "empty prompt"
	}
	return fmt.Sprintf("mock provider %s handled: %s", providerName, last)
}

func estimateUsage(messages []gateway.ChatMessage, completion string) gateway.Usage {
	promptChars := 0
	for _, message := range messages {
		promptChars += len(message.Content)
	}
	promptTokens := max(1, promptChars/4)
	completionTokens := max(1, len(completion)/4)
	return gateway.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
	}
}

func normalizeEmbeddingInput(input interface{}) []string {
	switch typed := input.(type) {
	case string:
		return []string{typed}
	case []string:
		return typed
	case []interface{}:
		items := make([]string, 0, len(typed))
		for _, value := range typed {
			items = append(items, fmt.Sprint(value))
		}
		return items
	default:
		return []string{fmt.Sprint(input)}
	}
}

func deterministicEmbedding(text string) []float64 {
	if text == "" {
		text = " "
	}
	vector := make([]float64, 8)
	for idx, r := range text {
		slot := idx % len(vector)
		vector[slot] += float64((int(r)%97)+1) / 100
	}
	for idx := range vector {
		vector[idx] = math.Round(vector[idx]*1000) / 1000
	}
	return vector
}

func writeSSEChunk(chunk gateway.ChatCompletionChunk, write func([]byte) error) error {
	payload, err := json.Marshal(chunk)
	if err != nil {
		return fmt.Errorf("marshal stream chunk: %w", err)
	}
	return write([]byte("data: " + string(payload) + "\n\n"))
}

func joinURL(baseURL, path string) string {
	trimmed := strings.TrimRight(baseURL, "/")
	if trimmed == "" {
		trimmed = "https://api.openai.com/v1"
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return trimmed + path
	}
	u.Path = strings.TrimRight(u.Path, "/") + path
	return u.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
