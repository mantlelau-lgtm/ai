package dispatcher

import (
	"context"
	"fmt"

	"message-gateway/internal/config"
	"message-gateway/internal/model"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type LarkClient struct {
	client *lark.Client
}

func NewLarkClient(cfg config.Config) *LarkClient {
	return &LarkClient{
		client: lark.NewClient(
			cfg.LarkAppID,
			cfg.LarkAppSecret,
			lark.WithOpenBaseUrl(cfg.LarkOpenBaseURL),
			lark.WithEnableTokenCache(true),
			lark.WithLogLevel(larkcore.LogLevelInfo),
		),
	}
}

func (c *LarkClient) SendMessage(ctx context.Context, payload model.SendMessagePayload) error {
	_, err := c.CreateMessage(ctx, payload)
	return err
}

func (c *LarkClient) CreateMessage(ctx context.Context, payload model.SendMessagePayload) (string, error) {
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(payload.ReceiveIDType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(payload.ReceiveID).
			MsgType(payload.MsgType).
			Content(payload.Content).
			Uuid(payload.UUID).
			Build()).
		Build()

	resp, err := c.client.Im.Message.Create(ctx, req)
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

func (c *LarkClient) PatchMessage(ctx context.Context, messageID string, content string) error {
	req := larkim.NewPatchMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewPatchMessageReqBodyBuilder().
			Content(content).
			Build()).
		Build()

	resp, err := c.client.Im.Message.Patch(ctx, req)
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("lark patch message failed: code=%d msg=%s request_id=%s", resp.Code, resp.Msg, resp.RequestId())
	}
	return nil
}
