package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Config struct {
	HTTPAddr           string
	AgentName          string
	AgentDescription   string
	RuntimeURL         string
	AdminToken         string
	AgentCenterBaseURL string
	RegisterPath       string
	HeartbeatInterval  time.Duration
	RequestTimeout     time.Duration
	LLMGatewayBaseURL  string
	LLMChatPath        string
	LLMModelsPath      string
	LLMModel           string
	LLMKeyName         string
	SystemPrompt       string
}

type envelope struct {
	EventID      string `json:"event_id"`
	MessageID    string `json:"message_id"`
	BotID        string `json:"bot_id"`
	ChatID       string `json:"chat_id"`
	MessageType  string `json:"message_type"`
	Text         string `json:"text"`
	SenderOpenID string `json:"sender_open_id"`
	ActionName   string `json:"action_name"`
	ActionTag    string `json:"action_tag"`
}

type streamRequest struct {
	Envelope envelope `json:"envelope"`
}

type llmClient struct {
	baseURL     string
	chatPath    string
	modelsPath  string
	model       string
	keyName     string
	prompt      string
	httpClient  *http.Client
	mu          sync.RWMutex
	cachedModel string
}

type registrar struct {
	cfg        Config
	httpClient *http.Client
}

func main() {
	cfg := loadConfig()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	httpClient := &http.Client{Timeout: cfg.RequestTimeout}
	llm := &llmClient{
		baseURL:    strings.TrimRight(cfg.LLMGatewayBaseURL, "/"),
		chatPath:   cfg.LLMChatPath,
		modelsPath: cfg.LLMModelsPath,
		model:      strings.TrimSpace(cfg.LLMModel),
		keyName:    strings.TrimSpace(cfg.LLMKeyName),
		prompt:     cfg.SystemPrompt,
		httpClient: httpClient,
	}
	reg := &registrar{cfg: cfg, httpClient: httpClient}

	go reg.run(ctx, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/v1/messages:stream", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		var req streamRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid message payload")
			return
		}
		reply, err := llm.reply(r.Context(), req.Envelope)
		if err != nil {
			logger.Error("robot-d reply failed", "event_id", req.Envelope.EventID, "error", err)
			writeError(w, http.StatusBadGateway, "llm_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"text": reply,
			"done": true,
		})
	})

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("robot-d listening", "addr", cfg.HTTPAddr, "runtime_url", cfg.RuntimeURL, "agent_name", cfg.AgentName)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("robot-d server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = reg.offline(shutdownCtx)
	_ = server.Shutdown(shutdownCtx)
}

func loadConfig() Config {
	return Config{
		HTTPAddr:           getenv("HTTP_ADDR", ":7004"),
		AgentName:          getenv("ROBOT_D_AGENT_NAME", "robot-D"),
		AgentDescription:   getenv("ROBOT_D_AGENT_DESCRIPTION", "robot-D 默认回退 agent，单轮问答，不记录上下文。"),
		RuntimeURL:         getenv("ROBOT_D_RUNTIME_URL", "http://127.0.0.1:7004"),
		AdminToken:         strings.TrimSpace(os.Getenv("ADMIN_TOKEN")),
		AgentCenterBaseURL: strings.TrimRight(getenv("AGENT_CENTER_BASE_URL", "http://127.0.0.1:9999"), "/"),
		RegisterPath:       getenv("AGENT_CENTER_REGISTER_PATH", "/api/agents/register"),
		HeartbeatInterval:  getDuration("ROBOT_D_HEARTBEAT_INTERVAL", 20*time.Second),
		RequestTimeout:     getDuration("ROBOT_D_REQUEST_TIMEOUT", 30*time.Second),
		LLMGatewayBaseURL:  strings.TrimRight(getenv("LLM_GATEWAY_BASE_URL", "http://127.0.0.1:50080"), "/"),
		LLMChatPath:        getenv("LLM_GATEWAY_CHAT_PATH", "/v1/chat/completions"),
		LLMModelsPath:      getenv("LLM_GATEWAY_MODELS_PATH", "/v1/models"),
		LLMModel:           strings.TrimSpace(os.Getenv("ROBOT_D_LLM_MODEL")),
		LLMKeyName:         strings.TrimSpace(os.Getenv("ROBOT_D_LLM_KEY_NAME")),
		SystemPrompt: getenv(
			"ROBOT_D_SYSTEM_PROMPT",
			"你是 robot-D，一个默认回退 agent。你只根据当前这一次输入回答，不保留上下文，不引用历史对话，也不假装记得之前聊过什么。请用简洁、直接、可靠的中文回答；如果信息不足，就明确说明。",
		),
	}
}

func (c *llmClient) reply(ctx context.Context, env envelope) (string, error) {
	modelName, err := c.resolveModel(ctx)
	if err != nil {
		return "", err
	}

	body, err := json.Marshal(map[string]any{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "system", "content": c.prompt},
			{"role": "user", "content": currentQuestion(env)},
		},
		"stream": false,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+c.chatPath, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.keyName != "" {
		req.Header.Set("X-LLM-Key", c.keyName)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusMultipleChoices {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return "", fmt.Errorf("llm chat http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if len(payload.Choices) == 0 {
		return "", fmt.Errorf("llm returned no choices")
	}
	return strings.TrimSpace(payload.Choices[0].Message.Content), nil
}

func (c *llmClient) resolveModel(ctx context.Context) (string, error) {
	if c.model != "" {
		return c.model, nil
	}
	c.mu.RLock()
	if c.cachedModel != "" {
		modelName := c.cachedModel
		c.mu.RUnlock()
		return modelName, nil
	}
	c.mu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+c.modelsPath, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusMultipleChoices {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return "", fmt.Errorf("load llm models http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if len(payload.Data) == 0 || strings.TrimSpace(payload.Data[0].ID) == "" {
		return "", fmt.Errorf("llm gateway returned no models")
	}

	modelName := strings.TrimSpace(payload.Data[0].ID)
	c.mu.Lock()
	c.cachedModel = modelName
	c.mu.Unlock()
	return modelName, nil
}

func (r *registrar) run(ctx context.Context, logger *slog.Logger) {
	r.syncOnce(ctx, logger)
	ticker := time.NewTicker(r.cfg.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.syncOnce(ctx, logger)
		}
	}
}

func (r *registrar) syncOnce(ctx context.Context, logger *slog.Logger) {
	if err := r.register(ctx); err != nil {
		logger.Warn("robot-d register failed", "error", err)
		return
	}
	if err := r.heartbeat(ctx); err != nil {
		logger.Warn("robot-d heartbeat failed", "error", err)
		return
	}
	logger.Info("robot-d registration synced", "agent_name", r.cfg.AgentName, "runtime_url", r.cfg.RuntimeURL)
}

func (r *registrar) register(ctx context.Context) error {
	payload := map[string]any{
		"agent": map[string]any{
			"name":        r.cfg.AgentName,
			"type":        "default",
			"source":      "robot-d",
			"description": r.cfg.AgentDescription,
			"runtime_url": r.cfg.RuntimeURL,
			"enabled":     true,
			"is_default":  true,
			"tags":        []string{"default", "fallback", "stateless"},
			"metadata": map[string]string{
				"display_name": "robot-D",
				"context_mode": "stateless",
				"memory":       "disabled",
			},
		},
	}
	return r.postJSON(ctx, r.cfg.RegisterPath, payload)
}

func (r *registrar) heartbeat(ctx context.Context) error {
	path := fmt.Sprintf("/api/agents/%s/heartbeat", r.cfg.AgentName)
	return r.postJSON(ctx, path, map[string]string{"status": "online"})
}

func (r *registrar) offline(ctx context.Context) error {
	path := fmt.Sprintf("/api/agents/%s/offline", r.cfg.AgentName)
	return r.postJSON(ctx, path, map[string]string{})
}

func (r *registrar) postJSON(ctx context.Context, path string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(r.cfg.AgentCenterBaseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if r.cfg.AdminToken != "" {
		req.Header.Set("Authorization", "Bearer "+r.cfg.AdminToken)
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusMultipleChoices {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("agent center http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}

func currentQuestion(env envelope) string {
	text := strings.TrimSpace(env.Text)
	if text != "" {
		return text
	}
	return fmt.Sprintf("收到一条非纯文本消息，请基于以下元信息给出帮助：message_type=%s action_name=%s action_tag=%s", strings.TrimSpace(env.MessageType), strings.TrimSpace(env.ActionName), strings.TrimSpace(env.ActionTag))
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, errType, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"type":    errType,
			"message": message,
		},
	})
}

func getenv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func getDuration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return value
}
