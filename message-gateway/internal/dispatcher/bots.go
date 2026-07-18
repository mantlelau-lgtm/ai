package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"message-gateway/internal/config"
)

type BotRuntimeConfig struct {
	BotID     string `json:"bot_id"`
	AppID     string `json:"app_id"`
	AgentName string `json:"agent_name"`
}

type BotConfigClient struct {
	sourceURL string
	client    *http.Client
}

func NewBotConfigClient(cfg config.Config) *BotConfigClient {
	if strings.TrimSpace(cfg.AdminConfigBaseURL) == "" {
		return nil
	}
	return &BotConfigClient{
		sourceURL: strings.TrimRight(cfg.AdminConfigBaseURL, "/") + cfg.AdminMessageBotsPath,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *BotConfigClient) GetBotConfig(ctx context.Context, botID string) (BotRuntimeConfig, error) {
	if c == nil {
		return BotRuntimeConfig{}, fmt.Errorf("bot config client not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.sourceURL, nil)
	if err != nil {
		return BotRuntimeConfig{}, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return BotRuntimeConfig{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return BotRuntimeConfig{}, fmt.Errorf("load bot config failed: status=%d", resp.StatusCode)
	}

	var payload struct {
		Bots []BotRuntimeConfig `json:"bots"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return BotRuntimeConfig{}, err
	}

	normalizedBotID := strings.TrimSpace(botID)
	for _, item := range payload.Bots {
		itemBotID := strings.TrimSpace(item.BotID)
		itemAppID := strings.TrimSpace(item.AppID)
		if itemBotID == normalizedBotID || itemAppID == normalizedBotID {
			item.AgentName = strings.ToLower(strings.TrimSpace(item.AgentName))
			return item, nil
		}
	}
	candidates := make([]map[string]string, 0, len(payload.Bots))
	for _, item := range payload.Bots {
		candidates = append(candidates, map[string]string{
			"bot_id":     strings.TrimSpace(item.BotID),
			"app_id":     strings.TrimSpace(item.AppID),
			"agent_name": strings.TrimSpace(item.AgentName),
		})
	}
	slog.Warn("bot config not matched", "lookup_bot_id", normalizedBotID, "candidates", candidates)
	return BotRuntimeConfig{}, fmt.Errorf("bot config not found: %s", botID)
}
