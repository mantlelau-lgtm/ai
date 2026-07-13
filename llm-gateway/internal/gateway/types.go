package gateway

import "time"

type ChatCompletionRequest struct {
	Model         string             `json:"model"`
	Messages      []ChatMessage      `json:"messages"`
	Stream        bool               `json:"stream,omitempty"`
	MaxTokens     int                `json:"max_tokens,omitempty"`
	Temperature   *float64           `json:"temperature,omitempty"`
	TopP          *float64           `json:"top_p,omitempty"`
	User          string             `json:"user,omitempty"`
	Metadata      map[string]string  `json:"metadata,omitempty"`
	Tools         []ChatTool         `json:"tools,omitempty"`
	ToolChoice    interface{}        `json:"tool_choice,omitempty"`
	StreamOptions *ChatStreamOptions `json:"stream_options,omitempty"`
}

type ChatStreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

type ChatMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	Name       string         `json:"name,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolCalls  []ChatToolCall `json:"tool_calls,omitempty"`
}

type ChatTool struct {
	Type     string                 `json:"type"`
	Function map[string]interface{} `json:"function"`
}

type ChatToolCall struct {
	ID       string               `json:"id,omitempty"`
	Type     string               `json:"type,omitempty"`
	Function ChatToolCallFunction `json:"function"`
}

type ChatToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ChatCompletionResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   Usage        `json:"usage"`
}

type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason,omitempty"`
}

type ChatCompletionChunk struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Created int64             `json:"created"`
	Model   string            `json:"model"`
	Choices []ChatChunkChoice `json:"choices"`
	Usage   *Usage            `json:"usage,omitempty"`
}

type ChatChunkChoice struct {
	Index        int              `json:"index"`
	Delta        ChatMessageDelta `json:"delta"`
	FinishReason *string          `json:"finish_reason,omitempty"`
}

type ChatMessageDelta struct {
	Role      string         `json:"role,omitempty"`
	Content   string         `json:"content,omitempty"`
	ToolCalls []ChatToolCall `json:"tool_calls,omitempty"`
}

type EmbeddingRequest struct {
	Model string      `json:"model"`
	Input interface{} `json:"input"`
	User  string      `json:"user,omitempty"`
}

type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  Usage           `json:"usage"`
}

type EmbeddingData struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

type ModelsResponse struct {
	Object string      `json:"object"`
	Data   []ModelInfo `json:"data"`
}

type ModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

type Usage struct {
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	Cost             float64   `json:"cost,omitempty"`
	LatencyMS        int64     `json:"latency_ms,omitempty"`
	StartedAt        time.Time `json:"started_at,omitempty"`
	FinishedAt       time.Time `json:"finished_at,omitempty"`
}

type ProviderConfig struct {
	Name          string            `json:"name"`
	Type          string            `json:"type"`
	BaseURL       string            `json:"base_url,omitempty"`
	APIKey        string            `json:"api_key,omitempty"`
	ModelPrefixes []string          `json:"model_prefixes,omitempty"`
	Enabled       bool              `json:"enabled"`
	IsDefault     bool              `json:"is_default"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	CreatedAt     time.Time         `json:"created_at,omitempty"`
	UpdatedAt     time.Time         `json:"updated_at,omitempty"`
}

type ModelRoute struct {
	Name                      string  `json:"name"`
	Provider                  string  `json:"provider"`
	UpstreamModel             string  `json:"upstream_model"`
	OwnedBy                   string  `json:"owned_by,omitempty"`
	PromptCostPer1KTokens     float64 `json:"prompt_cost_per_1k_tokens,omitempty"`
	CompletionCostPer1KTokens float64 `json:"completion_cost_per_1k_tokens,omitempty"`
	UnitPrice                 float64 `json:"unit_price,omitempty"`
	Enabled                   bool    `json:"enabled"`
}

type UsageRecord struct {
	ID               int64     `json:"id"`
	RequestID        string    `json:"request_id"`
	Provider         string    `json:"provider"`
	Endpoint         string    `json:"endpoint"`
	Model            string    `json:"model"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	Cost             float64   `json:"cost"`
	Success          bool      `json:"success"`
	LatencyMS        int64     `json:"latency_ms"`
	StartedAt        time.Time `json:"started_at"`
	FinishedAt       time.Time `json:"finished_at"`
	ErrorMessage     string    `json:"error_message,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type ErrorResponse struct {
	Error ErrorPayload `json:"error"`
}

type ErrorPayload struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}
