package app

import (
	"encoding/json"
	"fmt"

	"message-gateway/internal/model"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func envelopeFromP2MessageReceiveV1(event *larkim.P2MessageReceiveV1) (model.Envelope, error) {
	if event == nil || event.EventV2Base == nil || event.EventV2Base.Header == nil {
		return model.Envelope{}, fmt.Errorf("missing event header")
	}

	var msg *larkim.EventMessage
	var sender *larkim.EventSender
	if event.Event != nil {
		msg = event.Event.Message
		sender = event.Event.Sender
	}

	text := ""
	messageType := ""
	messageID := ""
	chatID := ""
	chatType := ""
	if msg != nil {
		messageType = strPtr(msg.MessageType)
		messageID = strPtr(msg.MessageId)
		chatID = strPtr(msg.ChatId)
		chatType = strPtr(msg.ChatType)
		if raw := strPtr(msg.Content); raw != "" {
			var content model.LarkMessageContent
			_ = json.Unmarshal([]byte(raw), &content)
			text = content.Text
		}
	}

	senderOpenID := ""
	senderUserID := ""
	senderUnionID := ""
	tenantKey := ""
	if sender != nil {
		tenantKey = strPtr(sender.TenantKey)
		if sender.SenderId != nil {
			senderOpenID = strPtr(sender.SenderId.OpenId)
			senderUserID = strPtr(sender.SenderId.UserId)
			senderUnionID = strPtr(sender.SenderId.UnionId)
		}
	}

	raw, _ := json.Marshal(event)
	return model.Envelope{
		BotID:         event.EventV2Base.Header.AppID,
		EventID:       event.EventV2Base.Header.EventID,
		EventType:     event.EventV2Base.Header.EventType,
		Kind:          model.EnvelopeKindMessage,
		ChatID:        chatID,
		ChatType:      chatType,
		MessageID:     messageID,
		MessageType:   messageType,
		SenderOpenID:  senderOpenID,
		SenderUserID:  senderUserID,
		SenderUnionID: senderUnionID,
		TenantKey:     tenantKey,
		Text:          text,
		TraceID:       event.EventV2Base.Header.EventID,
		Raw:           raw,
	}, nil
}

func envelopeFromP1MessageReceiveV1(event *larkim.P1MessageReceiveV1) (model.Envelope, error) {
	if event == nil || event.Event == nil {
		return model.Envelope{}, fmt.Errorf("missing event body")
	}

	raw, _ := json.Marshal(event)
	return model.Envelope{
		BotID:         "",
		EventID:       event.Event.OpenMessageID,
		EventType:     event.Event.Type,
		Kind:          model.EnvelopeKindMessage,
		ChatID:        event.Event.OpenChatID,
		ChatType:      event.Event.ChatType,
		MessageID:     event.Event.OpenMessageID,
		MessageType:   event.Event.MsgType,
		SenderOpenID:  event.Event.OpenID,
		SenderUserID:  event.Event.EmployeeID,
		SenderUnionID: event.Event.UnionID,
		TenantKey:     event.Event.TenantKey,
		Text:          event.Event.TextWithoutAtBot,
		TraceID:       event.Event.OpenMessageID,
		Raw:           raw,
	}, nil
}

func strPtr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
