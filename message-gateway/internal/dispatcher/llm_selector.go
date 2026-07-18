package dispatcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"message-gateway/internal/config"
	"message-gateway/internal/model"
)

type LLMSelectorClient struct {
	baseURL  string
	chatPath string
	model    string
	keyName  string
	client   *http.Client
}

func NewLLMSelectorClient(cfg config.Config) *LLMSelectorClient {
	if strings.TrimSpace(cfg.LLMGatewayBaseURL) == "" || strings.TrimSpace(cfg.AgentSelectorModel) == "" {
		return nil
	}
	return &LLMSelectorClient{
		baseURL:  strings.TrimRight(cfg.LLMGatewayBaseURL, "/"),
		chatPath: cfg.LLMGatewayChatPath,
		model:    strings.TrimSpace(cfg.AgentSelectorModel),
		keyName:  strings.TrimSpace(cfg.AgentSelectorKeyName),
		client: &http.Client{
			Timeout: cfg.AgentSelectorTimeout,
		},
	}
}

func (c *LLMSelectorClient) SelectAgent(ctx context.Context, env model.Envelope, agents []RegisteredAgent) (string, string, error) {
	if c == nil {
		return "", "", fmt.Errorf("llm selector client not configured")
	}
	if len(agents) == 0 {
		return "", "", nil
	}

	candidates := make([]map[string]any, 0, len(agents))
	for _, item := range agents {
		candidates = append(candidates, map[string]any{
			"name":        item.Name,
			"type":        item.Type,
			"description": item.Description,
			"tools":       item.Tools,
			"tags":        item.Tags,
		})
	}
	candidateJSON, _ := json.Marshal(candidates)

	requestBody := map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "你是一个 agent 路由器。请根据用户消息，在候选 agent 中选择最合适的一个。你必须只返回 JSON，格式为 {\"agent_name\":\"候选中的名称或空字符串\",\"reason\":\"简短原因\"}。如果没有合适的 agent，返回空字符串。",
			},
			{
				"role": "user",
				"content": fmt.Sprintf("bot_id: %s\nkind: %s\nevent_type: %s\nmessage_type: %s\ntext: %s\naction_name: %s\naction_tag: %s\n候选 agents: %s",
					strings.TrimSpace(env.BotID),
					strings.TrimSpace(env.Kind),
					strings.TrimSpace(env.EventType),
					strings.TrimSpace(env.MessageType),
					strings.TrimSpace(env.Text),
					strings.TrimSpace(env.ActionName),
					strings.TrimSpace(env.ActionTag),
					string(candidateJSON),
				),
			},
		},
		"stream": false,
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+c.chatPath, bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.keyName != "" {
		req.Header.Set("X-LLM-Key", c.keyName)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return "", "", fmt.Errorf("llm selector http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", "", err
	}
	if len(payload.Choices) == 0 {
		return "", "", fmt.Errorf("llm selector returned no choices")
	}

	name, reason, err := parseLLMSelection(payload.Choices[0].Message.Content)
	if err != nil {
		return "", "", err
	}
	return strings.ToLower(strings.TrimSpace(name)), strings.TrimSpace(reason), nil
}

func parseLLMSelection(raw string) (string, string, error) {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		trimmed = trimmed[start : end+1]
	}

	var payload struct {
		AgentName string `json:"agent_name"`
		Reason    string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return "", "", err
	}
	return payload.AgentName, payload.Reason, nil
}
