package dispatcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAgentCenterClientListAvailableAgents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"agents": [
				{"name":"alpha","runtime_url":"http://alpha:8080","enabled":true,"status":"online","is_default":true},
				{"name":"beta","runtime_url":"","enabled":true,"status":"online"},
				{"name":"gamma","runtime_url":"http://gamma:8080","enabled":true,"status":"offline"}
			]
		}`))
	}))
	defer srv.Close()

	client := &AgentCenterClient{
		sourceURL: srv.URL,
		client:    &http.Client{Timeout: 2 * time.Second},
	}

	items, err := client.ListAvailableAgents(context.Background())
	if err != nil {
		t.Fatalf("ListAvailableAgents error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 available agent, got %d", len(items))
	}
	if items[0].Name != "alpha" {
		t.Fatalf("expected alpha, got %q", items[0].Name)
	}
	if !items[0].IsDefault {
		t.Fatalf("expected alpha to keep default flag")
	}
}

func TestBotConfigClientGetBotConfig(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"bots": [
				{"bot_id":"bot-a","app_id":"cli_bot_a","agent_name":"atlas"},
				{"bot_id":"bot-b","app_id":"cli_bot_b","agent_name":""}
			]
		}`))
	}))
	defer srv.Close()

	client := &BotConfigClient{
		sourceURL: srv.URL,
		client:    &http.Client{Timeout: 2 * time.Second},
	}

	item, err := client.GetBotConfig(context.Background(), "bot-a")
	if err != nil {
		t.Fatalf("GetBotConfig error: %v", err)
	}
	if item.AgentName != "atlas" {
		t.Fatalf("expected atlas, got %q", item.AgentName)
	}

	item, err = client.GetBotConfig(context.Background(), "cli_bot_a")
	if err != nil {
		t.Fatalf("GetBotConfig by app_id error: %v", err)
	}
	if item.BotID != "bot-a" {
		t.Fatalf("expected bot-a, got %q", item.BotID)
	}
}

func TestParseLLMSelection(t *testing.T) {
	name, reason, err := parseLLMSelection("```json\n{\"agent_name\":\"atlas\",\"reason\":\"最匹配\"}\n```")
	if err != nil {
		t.Fatalf("parseLLMSelection error: %v", err)
	}
	if name != "atlas" {
		t.Fatalf("expected atlas, got %q", name)
	}
	if reason != "最匹配" {
		t.Fatalf("expected reason 最匹配, got %q", reason)
	}
}
