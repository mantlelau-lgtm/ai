package app

import (
	"encoding/json"
	"fmt"

	"message-gateway/internal/model"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
)

func envelopeFromP2CardActionTrigger(event *callback.CardActionTriggerEvent) (model.Envelope, error) {
	if event == nil || event.EventV2Base == nil || event.EventV2Base.Header == nil {
		return model.Envelope{}, fmt.Errorf("missing event header")
	}

	operatorOpenID := ""
	operatorUserID := ""
	tenantKey := ""
	actionToken := ""
	actionName := ""
	actionTag := ""
	inputValue := ""
	var actionValue json.RawMessage
	var formValue json.RawMessage

	chatID := ""
	messageID := ""

	if event.Event != nil {
		actionToken = event.Event.Token
		if event.Event.Operator != nil {
			operatorOpenID = event.Event.Operator.OpenID
			if event.Event.Operator.UserID != nil {
				operatorUserID = *event.Event.Operator.UserID
			}
			if event.Event.Operator.TenantKey != nil {
				tenantKey = *event.Event.Operator.TenantKey
			}
		}

		if event.Event.Context != nil {
			chatID = event.Event.Context.OpenChatID
			messageID = event.Event.Context.OpenMessageID
		}

		if event.Event.Action != nil {
			actionTag = event.Event.Action.Tag
			actionName = event.Event.Action.Name
			inputValue = event.Event.Action.InputValue
			if b, err := json.Marshal(event.Event.Action.Value); err == nil {
				actionValue = b
			}
			if b, err := json.Marshal(event.Event.Action.FormValue); err == nil {
				formValue = b
			}
		}
	}

	raw, _ := json.Marshal(event)
	return model.Envelope{
		BotID:        event.EventV2Base.Header.AppID,
		EventID:      event.EventV2Base.Header.EventID,
		EventType:    event.EventV2Base.Header.EventType,
		Kind:         model.EnvelopeKindCardAction,
		ChatID:       chatID,
		MessageID:    messageID,
		MessageType:  "card_action",
		SenderOpenID: operatorOpenID,
		SenderUserID: operatorUserID,
		TenantKey:    tenantKey,
		ActionName:   actionName,
		ActionTag:    actionTag,
		ActionToken:  actionToken,
		ActionValue:  actionValue,
		FormValue:    formValue,
		InputValue:   inputValue,
		TraceID:      event.EventV2Base.Header.EventID,
		Raw:          raw,
	}, nil
}
