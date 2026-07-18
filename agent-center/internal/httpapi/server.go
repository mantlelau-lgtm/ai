package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"agent-center/internal/agent"
	"agent-center/internal/config"
	"agent-center/internal/store"
)

type Server struct {
	logger *slog.Logger
	cfg    config.Config
	store  store.Store
}

func NewServer(cfg config.Config, st store.Store, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		logger: logger,
		cfg:    cfg,
		store:  st,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/api/agents", s.handleAgents)
	mux.HandleFunc("/api/agents/registered", s.handleRegisteredAgents)
	mux.HandleFunc("/api/agents/register", s.handleRegisterAgents)
	mux.HandleFunc("/api/runtime/agents", s.handleRuntimeAgents)
	mux.HandleFunc("/api/agents/", s.handleAgentByName)
	return s.loggingMiddleware(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.DefaultTimeout)
	defer cancel()
	items, err := s.store.ListAgents(ctx, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string][]agent.RegisteredAgent{"agents": s.projectAgents(items)})
}

func (s *Server) handleRegisteredAgents(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.DefaultTimeout)
	defer cancel()
	items, err := s.store.ListAgents(ctx, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	projected := s.projectAgents(items)
	writeJSON(w, http.StatusOK, map[string]any{
		"total":  len(projected),
		"agents": projected,
	})
}

func (s *Server) handleRuntimeAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.DefaultTimeout)
	defer cancel()
	items, err := s.store.ListAgents(ctx, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	projected := s.projectAgents(items)
	runtimeAgents := make([]agent.RegisteredAgent, 0, len(projected))
	for _, item := range projected {
		if !store.IsRuntimeAvailable(item) {
			continue
		}
		runtimeAgents = append(runtimeAgents, item)
	}
	writeJSON(w, http.StatusOK, map[string][]agent.RegisteredAgent{"agents": runtimeAgents})
}

func (s *Server) handleRegisterAgents(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid register request body")
		return
	}
	items, err := decodeRegisterAgentsPayload(payload)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid register request JSON")
		return
	}
	if len(items) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "agents is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.DefaultTimeout)
	defer cancel()
	saved := make([]agent.RegisteredAgent, 0, len(items))
	for _, item := range items {
		normalized := store.NormalizeAgent(item)
		if normalized.Name == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "agent name is required")
			return
		}
		stored, err := s.store.UpsertAgent(ctx, normalized)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		saved = append(saved, stored)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"registered": len(saved),
		"agents":     s.projectAgents(saved),
	})
}

func (s *Server) handleAgentByName(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/agents/")
	path = strings.Trim(path, "/")
	if path == "" {
		writeError(w, http.StatusNotFound, "not_found", "agent not found")
		return
	}
	if strings.HasSuffix(path, "/offline") {
		name := strings.TrimSuffix(path, "/offline")
		s.handleOffline(w, r, strings.Trim(name, "/"))
		return
	}
	if strings.HasSuffix(path, "/heartbeat") {
		name := strings.TrimSuffix(path, "/heartbeat")
		s.handleHeartbeat(w, r, strings.Trim(name, "/"))
		return
	}
	switch r.Method {
	case http.MethodGet:
		if !s.requireAdmin(w, r) {
			return
		}
		s.handleGetAgent(w, r, path)
	case http.MethodPut:
		if !s.requireAdmin(w, r) {
			return
		}
		s.handleUpsertAgent(w, r, path)
	case http.MethodDelete:
		if !s.requireAdmin(w, r) {
			return
		}
		s.handleDeleteAgent(w, r, path)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request, name string) {
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.DefaultTimeout)
	defer cancel()
	item, err := s.store.GetAgent(ctx, name)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "agent not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]agent.RegisteredAgent{"agent": s.projectAgent(item)})
}

func (s *Server) handleUpsertAgent(w http.ResponseWriter, r *http.Request, name string) {
	var item agent.RegisteredAgent
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid agent JSON")
		return
	}
	item.Name = name
	normalized := store.NormalizeAgent(item)
	if normalized.Name == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "agent name is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.DefaultTimeout)
	defer cancel()
	saved, err := s.store.UpsertAgent(ctx, normalized)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]agent.RegisteredAgent{"agent": s.projectAgent(saved)})
}

func (s *Server) handleDeleteAgent(w http.ResponseWriter, r *http.Request, name string) {
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.DefaultTimeout)
	defer cancel()
	deleted, err := s.store.DeleteAgent(ctx, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	if !deleted {
		writeError(w, http.StatusNotFound, "not_found", "agent not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "name": strings.ToLower(strings.TrimSpace(name))})
}

func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request, name string) {
	if !s.requireAdmin(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var req agent.HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid heartbeat JSON")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.DefaultTimeout)
	defer cancel()
	item, err := s.store.TouchHeartbeat(ctx, name, time.Now(), req.Status)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "agent not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]agent.RegisteredAgent{"agent": s.projectAgent(item)})
}

func (s *Server) handleOffline(w http.ResponseWriter, r *http.Request, name string) {
	if !s.requireAdmin(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.DefaultTimeout)
	defer cancel()
	item, err := s.store.TouchHeartbeat(ctx, name, time.Now(), "offline")
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "agent not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]agent.RegisteredAgent{"agent": s.projectAgent(item)})
}

func (s *Server) projectAgent(item agent.RegisteredAgent) agent.RegisteredAgent {
	return store.ProjectAgent(item, time.Now(), s.cfg.AgentOfflineTimeout)
}

func (s *Server) projectAgents(items []agent.RegisteredAgent) []agent.RegisteredAgent {
	return store.ProjectAgents(items, time.Now(), s.cfg.AgentOfflineTimeout)
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	token := strings.TrimSpace(s.cfg.AdminToken)
	if token == "" {
		return true
	}
	if strings.TrimSpace(r.Header.Get("X-Admin-Token")) == token {
		return true
	}
	if strings.TrimSpace(r.Header.Get("Authorization")) == "Bearer "+token {
		return true
	}
	writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
	return false
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Info("agent center request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"duration_ms", time.Since(startedAt).Milliseconds(),
		)
	})
}

type errorResponse struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, errType, message string) {
	payload := errorResponse{}
	payload.Error.Type = errType
	payload.Error.Message = message
	writeJSON(w, status, payload)
}

func decodeRegisterAgentsPayload(payload []byte) ([]agent.RegisteredAgent, error) {
	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" {
		return nil, io.EOF
	}

	var batchReq agent.RegisterAgentsRequest
	if err := json.Unmarshal(payload, &batchReq); err == nil && len(batchReq.Agents) > 0 {
		return batchReq.Agents, nil
	}

	var singleReq agent.RegisterAgentRequest
	if err := json.Unmarshal(payload, &singleReq); err == nil && strings.TrimSpace(singleReq.Agent.Name) != "" {
		return []agent.RegisteredAgent{singleReq.Agent}, nil
	}

	var singleAgent agent.RegisteredAgent
	if err := json.Unmarshal(payload, &singleAgent); err == nil && strings.TrimSpace(singleAgent.Name) != "" {
		return []agent.RegisteredAgent{singleAgent}, nil
	}

	return nil, io.ErrUnexpectedEOF
}
