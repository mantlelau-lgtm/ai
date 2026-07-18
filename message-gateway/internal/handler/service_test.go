package handler

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"message-gateway/internal/config"
	"message-gateway/internal/dispatcher"
	"message-gateway/internal/model"
)

func TestStreamingMessageUUIDIsShort(t *testing.T) {
	uuid := streamingMessageUUID("53ca512e414582ca531e910872b9cb3c")
	if len(uuid) > 32 {
		t.Fatalf("uuid too long: len=%d uuid=%s", len(uuid), uuid)
	}
	if uuid == "" {
		t.Fatalf("uuid should not be empty")
	}
}

func TestResolveTargetAgentFallsBackToDefaultWhenLLMReturnsEmpty(t *testing.T) {
	agentSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"agents": [
				{"name":"robot-d","runtime_url":"http://robot-d:7004","enabled":true,"status":"online","is_default":true},
				{"name":"atlas","runtime_url":"http://atlas:7001","enabled":true,"status":"online"}
			]
		}`))
	}))
	defer agentSrv.Close()

	selectorSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"choices": [
				{"message":{"content":"{\"agent_name\":\"\",\"reason\":\"no match\"}"}}
			]
		}`))
	}))
	defer selectorSrv.Close()

	svc := &Service{
		cfg: config.Config{LarkAppID: "cli_test"},
		agentCenter: dispatcher.NewAgentCenterClient(config.Config{
			AgentCenterBaseURL:    agentSrv.URL,
			AgentCenterAgentsPath: "",
		}),
		selector: dispatcher.NewLLMSelectorClient(config.Config{
			LLMGatewayBaseURL:    selectorSrv.URL,
			LLMGatewayChatPath:   "",
			AgentSelectorModel:   "test-model",
			AgentSelectorTimeout: 2 * time.Second,
		}),
		logger: slog.Default(),
	}

	resolution, err := svc.resolveTargetAgent(context.Background(), model.Envelope{EventID: "e1", BotID: "cli_test", Text: "hi"})
	if err != nil {
		t.Fatalf("resolveTargetAgent error: %v", err)
	}
	if resolution == nil {
		t.Fatalf("expected default agent fallback, got nil")
	}
	if resolution.Agent.Name != "robot-d" {
		t.Fatalf("expected robot-d, got %q", resolution.Agent.Name)
	}
	if resolution.ResolvedBy != "default_fallback" {
		t.Fatalf("expected default_fallback, got %q", resolution.ResolvedBy)
	}
}

func TestResolveTargetAgentFallsBackToDefaultAfterBotBindingMiss(t *testing.T) {
	agentSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"agents": [
				{"name":"robot-d","runtime_url":"http://robot-d:7004","enabled":true,"status":"online","is_default":true},
				{"name":"atlas","runtime_url":"http://atlas:7001","enabled":true,"status":"online"}
			]
		}`))
	}))
	defer agentSrv.Close()

	botSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"bots": [
				{"bot_id":"cli_test","agent_name":"missing-agent"}
			]
		}`))
	}))
	defer botSrv.Close()

	svc := &Service{
		cfg: config.Config{LarkAppID: "cli_test"},
		agentCenter: dispatcher.NewAgentCenterClient(config.Config{
			AgentCenterBaseURL:    agentSrv.URL,
			AgentCenterAgentsPath: "",
		}),
		botConfig: dispatcher.NewBotConfigClient(config.Config{
			AdminConfigBaseURL:   botSrv.URL,
			AdminMessageBotsPath: "",
		}),
		logger: slog.Default(),
	}

	resolution, err := svc.resolveTargetAgent(context.Background(), model.Envelope{EventID: "e2", BotID: "cli_test"})
	if err != nil {
		t.Fatalf("resolveTargetAgent error: %v", err)
	}
	if resolution == nil {
		t.Fatalf("expected default fallback, got nil")
	}
	if resolution.Agent.Name != "robot-d" {
		t.Fatalf("expected robot-d, got %q", resolution.Agent.Name)
	}
}

func TestFilterRoutableAgentsExcludesDefault(t *testing.T) {
	items := filterRoutableAgents([]dispatcher.RegisteredAgent{
		{Name: "robot-d", IsDefault: true},
		{Name: "atlas"},
		{Name: "calc"},
	})
	if len(items) != 2 {
		t.Fatalf("expected 2 routable agents, got %d", len(items))
	}
	if items[0].Name == "robot-d" || items[1].Name == "robot-d" {
		t.Fatalf("default agent should be excluded: %+v", items)
	}
}

func TestFindDefaultAgent(t *testing.T) {
	item, ok := findDefaultAgent([]dispatcher.RegisteredAgent{
		{Name: "atlas"},
		{Name: "robot-d", IsDefault: true},
	})
	if !ok {
		t.Fatalf("expected to find default agent")
	}
	if item.Name != "robot-d" {
		t.Fatalf("expected robot-d, got %q", item.Name)
	}
}

func TestResolveTargetAgentPrefersLLMSelectedNonDefaultAgent(t *testing.T) {
	agentSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"agents": [
				{"name":"robot-d","runtime_url":"http://robot-d:7004","enabled":true,"status":"online","is_default":true},
				{"name":"atlas","runtime_url":"http://atlas:7001","enabled":true,"status":"online"}
			]
		}`))
	}))
	defer agentSrv.Close()

	selectorSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"choices": [
				{"message":{"content":"{\"agent_name\":\"atlas\",\"reason\":\"matched\"}"}}
			]
		}`))
	}))
	defer selectorSrv.Close()

	svc := &Service{
		cfg: config.Config{LarkAppID: "cli_test"},
		agentCenter: dispatcher.NewAgentCenterClient(config.Config{
			AgentCenterBaseURL:    agentSrv.URL,
			AgentCenterAgentsPath: "",
		}),
		selector: dispatcher.NewLLMSelectorClient(config.Config{
			LLMGatewayBaseURL:    selectorSrv.URL,
			LLMGatewayChatPath:   "",
			AgentSelectorModel:   "test-model",
			AgentSelectorTimeout: 2 * time.Second,
		}),
		logger: slog.Default(),
	}

	resolution, err := svc.resolveTargetAgent(context.Background(), model.Envelope{EventID: "e3", BotID: "cli_test", Text: "need atlas"})
	if err != nil {
		t.Fatalf("resolveTargetAgent error: %v", err)
	}
	if resolution == nil {
		t.Fatalf("expected non-default agent, got nil")
	}
	if resolution.Agent.Name != "atlas" {
		t.Fatalf("expected atlas, got %q", resolution.Agent.Name)
	}
	if resolution.ResolvedBy != "llm" {
		t.Fatalf("expected llm, got %q", resolution.ResolvedBy)
	}
}
