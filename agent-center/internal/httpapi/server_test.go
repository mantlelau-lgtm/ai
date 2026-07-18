package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"agent-center/internal/agent"
	"agent-center/internal/config"
	"agent-center/internal/store"
)

func TestRegisterAndListAgents(t *testing.T) {
	st := store.NewMemoryStore()
	server := NewServer(config.Config{
		AdminToken: "test-token",
	}, st, slog.Default())

	body, err := json.Marshal(agent.RegisterAgentsRequest{
		Agents: []agent.RegisteredAgent{
			{
				Name:          "atlas",
				Type:          "research",
				Source:        "local",
				Description:   "research assistant",
				KeyName:       "deepseek-main",
				IsDefault:     true,
				Tools:         []string{"search.code", "run.command"},
				WorkspacePath: "/Users/rocky/CodingSpace/ai-coding/ai",
				RuntimeURL:    "http://127.0.0.1:50081",
				Enabled:       true,
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/agents/register", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("register status = %d, body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	req.Header.Set("X-Admin-Token", "test-token")
	rec = httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Agents []agent.RegisteredAgent `json:"agents"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode list payload: %v", err)
	}
	if len(payload.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(payload.Agents))
	}
	if payload.Agents[0].Name != "atlas" {
		t.Fatalf("expected atlas, got %q", payload.Agents[0].Name)
	}
	if !payload.Agents[0].IsDefault {
		t.Fatalf("expected atlas to be default agent")
	}
}

func TestRegisterSingleAgentAndRegisteredList(t *testing.T) {
	st := store.NewMemoryStore()
	server := NewServer(config.Config{
		AdminToken: "test-token",
	}, st, slog.Default())

	body := []byte(`{
          "agent": {
            "name": "atlas",
            "type": "research",
            "source": "local",
            "enabled": true
          }
        }`)

	req := httptest.NewRequest(http.MethodPost, "/api/agents/register", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("register single status = %d, body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/agents/registered", nil)
	req.Header.Set("X-Admin-Token", "test-token")
	rec = httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("registered list status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Total  int                     `json:"total"`
		Agents []agent.RegisteredAgent `json:"agents"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode registered list payload: %v", err)
	}
	if payload.Total != 1 {
		t.Fatalf("expected total 1, got %d", payload.Total)
	}
	if len(payload.Agents) != 1 || payload.Agents[0].Name != "atlas" {
		t.Fatalf("unexpected agents payload: %+v", payload.Agents)
	}
}

func TestHeartbeatUpdatesLastSeen(t *testing.T) {
	st := store.NewMemoryStore()
	_, _ = st.UpsertAgent(context.Background(), agent.RegisteredAgent{
		Name:    "echo",
		Enabled: true,
	})

	server := NewServer(config.Config{AdminToken: "test-token"}, st, slog.Default())
	req := httptest.NewRequest(http.MethodPost, "/api/agents/echo/heartbeat", bytes.NewReader([]byte(`{"status":"online"}`)))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("heartbeat status = %d, body = %s", rec.Code, rec.Body.String())
	}

	item, err := st.GetAgent(context.Background(), "echo")
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if item.LastSeenAt == nil {
		t.Fatalf("expected last_seen_at to be updated")
	}
	if item.Status != "online" {
		t.Fatalf("expected status online, got %q", item.Status)
	}
}

func TestHeartbeatDefaultsStatusToOnline(t *testing.T) {
	st := store.NewMemoryStore()
	_, _ = st.UpsertAgent(context.Background(), agent.RegisteredAgent{
		Name:    "atlas",
		Enabled: true,
	})

	server := NewServer(config.Config{AdminToken: "test-token"}, st, slog.Default())
	req := httptest.NewRequest(http.MethodPost, "/api/agents/atlas/heartbeat", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("heartbeat status = %d, body = %s", rec.Code, rec.Body.String())
	}

	item, err := st.GetAgent(context.Background(), "atlas")
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if item.Status != "online" {
		t.Fatalf("expected default heartbeat status online, got %q", item.Status)
	}
}

func TestOfflineMarksAgentUnavailableForRuntime(t *testing.T) {
	st := store.NewMemoryStore()
	now := time.Now().UTC()
	_, _ = st.UpsertAgent(context.Background(), agent.RegisteredAgent{
		Name:       "atlas",
		Enabled:    true,
		Status:     "online",
		RuntimeURL: "http://127.0.0.1:7001",
		LastSeenAt: &now,
	})

	server := NewServer(config.Config{
		AdminToken:          "test-token",
		AgentOfflineTimeout: 30 * time.Second,
	}, st, slog.Default())

	req := httptest.NewRequest(http.MethodPost, "/api/agents/atlas/offline", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("offline status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var offlinePayload struct {
		Agent agent.RegisteredAgent `json:"agent"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &offlinePayload); err != nil {
		t.Fatalf("decode offline payload: %v", err)
	}
	if offlinePayload.Agent.Status != "offline" {
		t.Fatalf("expected agent status offline, got %q", offlinePayload.Agent.Status)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/runtime/agents", nil)
	rec = httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("runtime list status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var runtimePayload struct {
		Agents []agent.RegisteredAgent `json:"agents"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &runtimePayload); err != nil {
		t.Fatalf("decode runtime payload: %v", err)
	}
	if len(runtimePayload.Agents) != 0 {
		t.Fatalf("expected no runtime agents after offline, got %+v", runtimePayload.Agents)
	}
}

func TestRuntimeAgentsFiltersOfflineByTimeout(t *testing.T) {
	st := store.NewMemoryStore()
	now := time.Now().UTC()
	staleSeenAt := now.Add(-2 * time.Minute)
	freshSeenAt := now.Add(-10 * time.Second)

	_, _ = st.UpsertAgent(context.Background(), agent.RegisteredAgent{
		Name:       "stale",
		Enabled:    true,
		Status:     "online",
		RuntimeURL: "http://127.0.0.1:7001",
		LastSeenAt: &staleSeenAt,
	})
	_, _ = st.UpsertAgent(context.Background(), agent.RegisteredAgent{
		Name:       "fresh",
		Enabled:    true,
		Status:     "online",
		RuntimeURL: "http://127.0.0.1:7002",
		LastSeenAt: &freshSeenAt,
	})
	_, _ = st.UpsertAgent(context.Background(), agent.RegisteredAgent{
		Name:       "registered-only",
		Enabled:    true,
		Status:     "registered",
		RuntimeURL: "http://127.0.0.1:7003",
	})

	server := NewServer(config.Config{
		AdminToken:          "test-token",
		AgentOfflineTimeout: 30 * time.Second,
	}, st, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/runtime/agents", nil)
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("runtime list status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Agents []agent.RegisteredAgent `json:"agents"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode runtime payload: %v", err)
	}
	if len(payload.Agents) != 1 {
		t.Fatalf("expected 1 runtime agent, got %d (%+v)", len(payload.Agents), payload.Agents)
	}
	if payload.Agents[0].Name != "fresh" {
		t.Fatalf("expected fresh agent, got %q", payload.Agents[0].Name)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/agents/registered", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec = httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("registered list status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var registeredPayload struct {
		Agents []agent.RegisteredAgent `json:"agents"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &registeredPayload); err != nil {
		t.Fatalf("decode registered payload: %v", err)
	}

	statusByName := map[string]string{}
	for _, item := range registeredPayload.Agents {
		statusByName[item.Name] = item.Status
	}
	if statusByName["stale"] != "offline" {
		t.Fatalf("expected stale agent to be offline, got %q", statusByName["stale"])
	}
	if statusByName["fresh"] != "online" {
		t.Fatalf("expected fresh agent to be online, got %q", statusByName["fresh"])
	}
	if statusByName["registered-only"] != "registered" {
		t.Fatalf("expected registered-only agent to stay registered, got %q", statusByName["registered-only"])
	}
}

func TestRegisterDefaultAgentKeepsSingleDefault(t *testing.T) {
	st := store.NewMemoryStore()
	server := NewServer(config.Config{AdminToken: "test-token"}, st, slog.Default())

	body := []byte(`{
		"agents": [
			{"name":"robot-a","enabled":true,"is_default":true},
			{"name":"robot-d","enabled":true,"is_default":true}
		]
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/agents/register", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("register status = %d, body = %s", rec.Code, rec.Body.String())
	}

	items, err := st.ListAgents(context.Background(), false)
	if err != nil {
		t.Fatalf("list agents: %v", err)
	}
	defaultCount := 0
	defaultName := ""
	for _, item := range items {
		if item.IsDefault {
			defaultCount++
			defaultName = item.Name
		}
	}
	if defaultCount != 1 {
		t.Fatalf("expected exactly one default agent, got %d (%+v)", defaultCount, items)
	}
	if defaultName != "robot-d" {
		t.Fatalf("expected last default agent robot-d, got %q", defaultName)
	}
}
