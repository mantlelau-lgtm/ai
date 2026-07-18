package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"message-gateway/internal/config"
)

type RegisteredAgent struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Source      string            `json:"source"`
	Description string            `json:"description"`
	KeyName     string            `json:"key_name"`
	IsDefault   bool              `json:"is_default"`
	Tools       []string          `json:"tools"`
	RuntimeURL  string            `json:"runtime_url"`
	Tags        []string          `json:"tags"`
	Metadata    map[string]string `json:"metadata"`
	Enabled     bool              `json:"enabled"`
	Status      string            `json:"status"`
}

type AgentCenterClient struct {
	sourceURL string
	client    *http.Client
}

func NewAgentCenterClient(cfg config.Config) *AgentCenterClient {
	if strings.TrimSpace(cfg.AgentCenterBaseURL) == "" {
		return nil
	}
	return &AgentCenterClient{
		sourceURL: strings.TrimRight(cfg.AgentCenterBaseURL, "/") + cfg.AgentCenterAgentsPath,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *AgentCenterClient) ListAvailableAgents(ctx context.Context) ([]RegisteredAgent, error) {
	if c == nil {
		return nil, fmt.Errorf("agent center client not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.sourceURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("load agents failed: status=%d", resp.StatusCode)
	}

	var payload struct {
		Agents []RegisteredAgent `json:"agents"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	items := make([]RegisteredAgent, 0, len(payload.Agents))
	for _, item := range payload.Agents {
		item.Name = strings.ToLower(strings.TrimSpace(item.Name))
		item.RuntimeURL = strings.TrimSpace(item.RuntimeURL)
		item.Status = strings.ToLower(strings.TrimSpace(item.Status))
		if item.Name == "" || item.RuntimeURL == "" || !item.Enabled {
			continue
		}
		if item.Status == "offline" || item.Status == "unavailable" || item.Status == "disabled" {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}
