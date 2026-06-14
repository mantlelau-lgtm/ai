package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"message-gateway/internal/config"
	"message-gateway/internal/model"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type LarkClient struct {
	clients       map[string]*lark.Client
	defaultBotID  string
	defaultClient *lark.Client
}

type LarkBotCredential struct {
	BotID       string
	AppID       string
	AppSecret   string
	OpenBaseURL string
}

type larkBotsFile struct {
	Bots []struct {
		BotID       string `json:"bot_id"`
		AppID       string `json:"app_id"`
		AppSecret   string `json:"app_secret"`
		OpenBaseURL string `json:"open_base_url"`
	} `json:"bots"`
}

func NewLarkClient(cfg config.Config) *LarkClient {
	bots := LoadLarkBotCredentials(cfg)
	clients := map[string]*lark.Client{}
	defaultBotID := strings.TrimSpace(cfg.LarkAppID)
	for _, b := range bots {
		botID := strings.TrimSpace(b.BotID)
		if botID == "" {
			botID = strings.TrimSpace(b.AppID)
		}
		if botID == "" || strings.TrimSpace(b.AppSecret) == "" {
			continue
		}
		openBaseURL := strings.TrimSpace(b.OpenBaseURL)
		if openBaseURL == "" {
			openBaseURL = cfg.LarkOpenBaseURL
		}
		clients[botID] = lark.NewClient(
			botID,
			b.AppSecret,
			lark.WithOpenBaseUrl(openBaseURL),
			lark.WithEnableTokenCache(true),
			lark.WithLogLevel(larkcore.LogLevelInfo),
		)
	}

	defaultClient := clients[defaultBotID]
	if defaultClient == nil && len(clients) > 0 {
		for id, c := range clients {
			defaultBotID = id
			defaultClient = c
			break
		}
	}
	if defaultClient == nil && strings.TrimSpace(cfg.LarkAppID) != "" && strings.TrimSpace(cfg.LarkAppSecret) != "" {
		defaultClient = lark.NewClient(
			cfg.LarkAppID,
			cfg.LarkAppSecret,
			lark.WithOpenBaseUrl(cfg.LarkOpenBaseURL),
			lark.WithEnableTokenCache(true),
			lark.WithLogLevel(larkcore.LogLevelInfo),
		)
		clients[strings.TrimSpace(cfg.LarkAppID)] = defaultClient
		defaultBotID = strings.TrimSpace(cfg.LarkAppID)
	}

	return &LarkClient{clients: clients, defaultBotID: defaultBotID, defaultClient: defaultClient}
}

func LoadLarkBotCredentials(cfg config.Config) []LarkBotCredential {
	if baseURL := strings.TrimSpace(cfg.AdminConfigBaseURL); baseURL != "" {
		if bots, err := loadLarkBotCredentialsFromURL(baseURL + cfg.AdminMessageBotsPath); err == nil {
			return bots
		}
	}
	path := strings.TrimSpace(cfg.LarkBotsPath)
	if path == "" {
		if strings.TrimSpace(cfg.LarkAppID) == "" {
			return nil
		}
		return []LarkBotCredential{
			{
				BotID:       cfg.LarkAppID,
				AppID:       cfg.LarkAppID,
				AppSecret:   cfg.LarkAppSecret,
				OpenBaseURL: cfg.LarkOpenBaseURL,
			},
		}
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var file larkBotsFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil
	}
	var out []LarkBotCredential
	for _, it := range file.Bots {
		botID := strings.TrimSpace(it.BotID)
		appID := strings.TrimSpace(it.AppID)
		if botID == "" {
			botID = appID
		}
		if botID == "" || strings.TrimSpace(it.AppSecret) == "" {
			continue
		}
		out = append(out, LarkBotCredential{
			BotID:       botID,
			AppID:       botID,
			AppSecret:   strings.TrimSpace(it.AppSecret),
			OpenBaseURL: strings.TrimSpace(it.OpenBaseURL),
		})
	}
	return out
}

func loadLarkBotCredentialsFromURL(url string) ([]LarkBotCredential, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("load lark bots failed: status=%d", resp.StatusCode)
	}
	var file larkBotsFile
	if err := json.NewDecoder(resp.Body).Decode(&file); err != nil {
		return nil, err
	}
	var out []LarkBotCredential
	for _, it := range file.Bots {
		botID := strings.TrimSpace(it.BotID)
		appID := strings.TrimSpace(it.AppID)
		if botID == "" {
			botID = appID
		}
		if botID == "" || strings.TrimSpace(it.AppSecret) == "" {
			continue
		}
		out = append(out, LarkBotCredential{
			BotID:       botID,
			AppID:       appID,
			AppSecret:   strings.TrimSpace(it.AppSecret),
			OpenBaseURL: strings.TrimSpace(it.OpenBaseURL),
		})
	}
	return out, nil
}

func (c *LarkClient) clientFor(botID string) (*lark.Client, error) {
	key := strings.TrimSpace(botID)
	if key != "" {
		if v := c.clients[key]; v != nil {
			return v, nil
		}
	}
	if c.defaultClient != nil {
		return c.defaultClient, nil
	}
	return nil, fmt.Errorf("lark bot not configured")
}

func (c *LarkClient) SendMessage(ctx context.Context, botID string, payload model.SendMessagePayload) error {
	_, err := c.CreateMessage(ctx, botID, payload)
	return err
}

func (c *LarkClient) CreateMessage(ctx context.Context, botID string, payload model.SendMessagePayload) (string, error) {
	cli, err := c.clientFor(botID)
	if err != nil {
		return "", err
	}
	uuid := normalizeLarkUUID(payload.UUID)
	slog.Default().Info("lark create message request", "bot_id", botID, "receive_id_type", payload.ReceiveIDType, "receive_id", payload.ReceiveID, "msg_type", payload.MsgType, "uuid", uuid, "content_len", len(payload.Content), "content_preview", truncateLogValue(payload.Content, 500))
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(payload.ReceiveIDType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(payload.ReceiveID).
			MsgType(payload.MsgType).
			Content(payload.Content).
			Uuid(uuid).
			Build()).
		Build()

	resp, err := cli.Im.Message.Create(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("lark create message failed: code=%d msg=%s request_id=%s", resp.Code, resp.Msg, resp.RequestId())
	}
	if resp.Data == nil || resp.Data.MessageId == nil || *resp.Data.MessageId == "" {
		return "", fmt.Errorf("lark create message missing message_id: request_id=%s", resp.RequestId())
	}
	return *resp.Data.MessageId, nil
}

func truncateLogValue(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}

func normalizeLarkUUID(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
		if b.Len() >= 32 {
			break
		}
	}
	return strings.Trim(b.String(), "-")
}

func (c *LarkClient) PatchMessage(ctx context.Context, botID string, messageID string, content string) error {
	cli, err := c.clientFor(botID)
	if err != nil {
		return err
	}
	req := larkim.NewPatchMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewPatchMessageReqBodyBuilder().
			Content(content).
			Build()).
		Build()

	resp, err := cli.Im.Message.Patch(ctx, req)
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("lark patch message failed: code=%d msg=%s request_id=%s", resp.Code, resp.Msg, resp.RequestId())
	}
	return nil
}
