package model

import (
	"encoding/json"
	"time"
)

type Envelope struct {
	EventID       string
	EventType     string
	Kind          string
	ChatID        string
	ChatType      string
	MessageID     string
	MessageType   string
	SenderOpenID  string
	SenderUserID  string
	SenderUnionID string
	TenantKey     string
	Text          string
	ActionName    string
	ActionTag     string
	ActionToken   string
	ActionValue   json.RawMessage
	FormValue     json.RawMessage
	InputValue    string
	TraceID       string
	Raw           json.RawMessage
}

type RouteResult struct {
	Text      string
	ToastText string
	DedupKey  string
}

type SendMessagePayload struct {
	ReceiveID     string `json:"receive_id"`
	ReceiveIDType string `json:"receive_id_type"`
	MsgType       string `json:"msg_type"`
	Content       string `json:"content"`
	UUID          string `json:"uuid,omitempty"`
}

type ForwardToCorePayload struct {
	Envelope Envelope `json:"envelope"`
}

type Job struct {
	ID          string
	JobType     string
	Status      string
	Attempts    int
	MaxAttempts int
	Payload     []byte
}

type JobSummary struct {
	JobID       string          `json:"job_id"`
	JobType     string          `json:"job_type"`
	Status      string          `json:"status"`
	Attempts    int             `json:"attempts"`
	MaxAttempts int             `json:"max_attempts"`
	LastError   string          `json:"last_error"`
	UpdatedAt   time.Time       `json:"updated_at"`
	Payload     json.RawMessage `json:"payload"`
}

const (
	JobStatusPending   = "pending"
	JobStatusRunning   = "running"
	JobStatusSucceeded = "succeeded"
	JobStatusFailed    = "failed"
	JobStatusDead      = "dead"
)

const (
	EnvelopeKindMessage    = "message"
	EnvelopeKindCardAction = "card_action"
)

type LarkURLVerification struct {
	Type      string `json:"type"`
	Token     string `json:"token"`
	Challenge string `json:"challenge"`
}

type LarkEventRequest struct {
	Schema string `json:"schema"`
	Header struct {
		EventID    string `json:"event_id"`
		EventType  string `json:"event_type"`
		AppID      string `json:"app_id"`
		TenantKey  string `json:"tenant_key"`
		CreateTime string `json:"create_time"`
	} `json:"header"`
	Event struct {
		Message struct {
			MessageID   string `json:"message_id"`
			ChatID      string `json:"chat_id"`
			ChatType    string `json:"chat_type"`
			MessageType string `json:"message_type"`
			Content     string `json:"content"`
		} `json:"message"`
		Sender struct {
			SenderID struct {
				OpenID string `json:"open_id"`
			} `json:"sender_id"`
			TenantKey string `json:"tenant_key"`
		} `json:"sender"`
	} `json:"event"`
}

type LarkMessageContent struct {
	Text string `json:"text"`
}

type RetryDecision struct {
	NextRunAt time.Time
	MarkDead  bool
}
