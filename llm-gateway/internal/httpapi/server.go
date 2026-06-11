package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"llm-gateway/internal/config"
	"llm-gateway/internal/gateway"
	"llm-gateway/internal/provider"
	"llm-gateway/internal/store"
)

type Server struct {
	logger  *slog.Logger
	config  config.Config
	store   store.Store
	manager *provider.Manager
	reload  func(context.Context) error
}

func NewServer(cfg config.Config, stores store.Store, manager *provider.Manager, reload func(context.Context) error, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		logger:  logger,
		config:  cfg,
		store:   stores,
		manager: manager,
		reload:  reload,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/v1/models", s.handleModels)
	mux.HandleFunc("/v1/chat/completions", s.handleChatCompletions)
	mux.HandleFunc("/v1/embeddings", s.handleEmbeddings)
	mux.HandleFunc("/admin/providers", s.handleProviders)
	mux.HandleFunc("/admin/providers/", s.handleProviderByName)
	mux.HandleFunc("/admin/usages", s.handleUsage)
	mux.HandleFunc("/admin/reload", s.handleReload)
	return s.loggingMiddleware(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.config.DefaultTimeout)
	defer cancel()

	models, err := s.manager.AggregateModels(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, "provider_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, gateway.ModelsResponse{Object: "list", Data: models})
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req gateway.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid chat request JSON")
		return
	}
	if req.Model == "" || len(req.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "model and messages are required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.config.DefaultTimeout)
	defer cancel()
	providerCfg, handler, upstreamModel, err := s.manager.ResolveProvider(ctx, req.Model)
	if err != nil {
		writeError(w, http.StatusBadGateway, "provider_error", err.Error())
		return
	}
	downstreamReq := req
	downstreamReq.Model = upstreamModel

	startedAt := time.Now()
	requestID := requestIDFromTime(startedAt)
	if req.Stream {
		s.handleStreamingChat(w, ctx, requestID, providerCfg, handler, req.Model, downstreamReq, startedAt)
		return
	}

	response, callErr := handler.Chat(ctx, providerCfg, downstreamReq)
	s.recordUsage(ctx, gateway.UsageRecord{
		RequestID:        requestID,
		Provider:         providerCfg.Name,
		Endpoint:         "chat.completions",
		Model:            req.Model,
		PromptTokens:     response.Usage.PromptTokens,
		CompletionTokens: response.Usage.CompletionTokens,
		TotalTokens:      response.Usage.TotalTokens,
		Success:          callErr == nil,
		LatencyMS:        time.Since(startedAt).Milliseconds(),
		ErrorMessage:     errorMessage(callErr),
	})
	if callErr != nil {
		writeError(w, http.StatusBadGateway, "provider_error", callErr.Error())
		return
	}
	w.Header().Set("X-Request-ID", requestID)
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleStreamingChat(w http.ResponseWriter, ctx context.Context, requestID string, providerCfg gateway.ProviderConfig, handler provider.Provider, requestedModel string, req gateway.ChatCompletionRequest, startedAt time.Time) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Request-ID", requestID)

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "stream_error", "streaming not supported")
		return
	}

	usage, err := handler.StreamChat(ctx, providerCfg, req, func(chunk []byte) error {
		if _, writeErr := w.Write(chunk); writeErr != nil {
			return writeErr
		}
		flusher.Flush()
		return nil
	})

	s.recordUsage(ctx, gateway.UsageRecord{
		RequestID:        requestID,
		Provider:         providerCfg.Name,
		Endpoint:         "chat.completions.stream",
		Model:            requestedModel,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
		Success:          err == nil,
		LatencyMS:        time.Since(startedAt).Milliseconds(),
		ErrorMessage:     errorMessage(err),
	})

	if err != nil {
		s.logger.Error("stream chat failed", "request_id", requestID, "provider", providerCfg.Name, "error", err)
	}
}

func (s *Server) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req gateway.EmbeddingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid embeddings request JSON")
		return
	}
	if req.Model == "" || req.Input == nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "model and input are required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.config.DefaultTimeout)
	defer cancel()
	providerCfg, handler, upstreamModel, err := s.manager.ResolveProvider(ctx, req.Model)
	if err != nil {
		writeError(w, http.StatusBadGateway, "provider_error", err.Error())
		return
	}
	downstreamReq := req
	downstreamReq.Model = upstreamModel

	startedAt := time.Now()
	requestID := requestIDFromTime(startedAt)
	response, callErr := handler.Embeddings(ctx, providerCfg, downstreamReq)
	s.recordUsage(ctx, gateway.UsageRecord{
		RequestID:        requestID,
		Provider:         providerCfg.Name,
		Endpoint:         "embeddings",
		Model:            req.Model,
		PromptTokens:     response.Usage.PromptTokens,
		CompletionTokens: response.Usage.CompletionTokens,
		TotalTokens:      response.Usage.TotalTokens,
		Success:          callErr == nil,
		LatencyMS:        time.Since(startedAt).Milliseconds(),
		ErrorMessage:     errorMessage(callErr),
	})
	if callErr != nil {
		writeError(w, http.StatusBadGateway, "provider_error", callErr.Error())
		return
	}
	w.Header().Set("X-Request-ID", requestID)
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		ctx, cancel := context.WithTimeout(r.Context(), s.config.DefaultTimeout)
		defer cancel()
		providers, err := s.store.ListProviders(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string][]gateway.ProviderConfig{"data": providers})
	case http.MethodPost:
		var providerConfig gateway.ProviderConfig
		if err := json.NewDecoder(r.Body).Decode(&providerConfig); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid provider JSON")
			return
		}
		if err := validateProvider(providerConfig); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), s.config.DefaultTimeout)
		defer cancel()
		if err := s.store.UpsertProvider(ctx, providerConfig); err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, providerConfig)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *Server) handleProviderByName(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/admin/providers/")
	if name == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "provider name is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.config.DefaultTimeout)
	defer cancel()

	switch r.Method {
	case http.MethodGet:
		providerConfig, err := s.store.GetProvider(ctx, name)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, providerConfig)
	case http.MethodPut:
		var providerConfig gateway.ProviderConfig
		if err := json.NewDecoder(r.Body).Decode(&providerConfig); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid provider JSON")
			return
		}
		providerConfig.Name = name
		if err := validateProvider(providerConfig); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		if err := s.store.UpsertProvider(ctx, providerConfig); err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, providerConfig)
	case http.MethodDelete:
		if err := s.store.DeleteProvider(ctx, name); err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 || value > 500 {
			writeError(w, http.StatusBadRequest, "invalid_request", "limit must be between 1 and 500")
			return
		}
		limit = value
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.config.DefaultTimeout)
	defer cancel()
	records, err := s.store.ListUsage(ctx, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string][]gateway.UsageRecord{"data": records})
}

func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if s.reload == nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "catalog reload is not configured")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.config.DefaultTimeout)
	defer cancel()
	if err := s.reload(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "reload_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	expected := "Bearer " + s.config.AdminToken
	if auth != expected {
		writeError(w, http.StatusUnauthorized, "unauthorized", "admin token required")
		return false
	}
	return true
}

func (s *Server) recordUsage(ctx context.Context, record gateway.UsageRecord) {
	if err := s.store.RecordUsage(ctx, record); err != nil {
		s.logger.Error("record usage failed", "request_id", record.RequestID, "error", err)
	}
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Info("request completed", "method", r.Method, "path", r.URL.Path, "latency_ms", time.Since(startedAt).Milliseconds())
	})
}

func validateProvider(providerConfig gateway.ProviderConfig) error {
	if providerConfig.Name == "" {
		return fmt.Errorf("provider name is required")
	}
	if providerConfig.Type != "mock" && providerConfig.Type != "openai" {
		return fmt.Errorf("provider type must be one of: mock, openai")
	}
	if providerConfig.Type == "openai" && providerConfig.BaseURL == "" {
		return fmt.Errorf("base_url is required for openai providers")
	}
	return nil
}

func requestIDFromTime(now time.Time) string {
	return fmt.Sprintf("req_%d", now.UnixNano())
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, kind, message string) {
	writeJSON(w, status, gateway.ErrorResponse{Error: gateway.ErrorPayload{Message: message, Type: kind}})
}
