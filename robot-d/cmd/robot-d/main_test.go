package main

import (
	"strings"
	"testing"
)

func TestCurrentQuestionUsesCurrentTextOnly(t *testing.T) {
	got := currentQuestion(envelope{
		Text:         "  你好，介绍一下自己  ",
		MessageType:  "text",
		ActionName:   "ignored",
		ActionTag:    "ignored",
		SenderOpenID: "ou_xxx",
	})
	if got != "你好，介绍一下自己" {
		t.Fatalf("unexpected current question: %q", got)
	}
}

func TestCurrentQuestionFallsBackToMetadata(t *testing.T) {
	got := currentQuestion(envelope{
		MessageType: "image",
		ActionName:  "preview",
		ActionTag:   "detail",
	})
	if got == "" {
		t.Fatalf("expected fallback question to be non-empty")
	}
	if want := "message_type=image"; !contains(got, want) {
		t.Fatalf("expected fallback question to contain %q, got %q", want, got)
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
