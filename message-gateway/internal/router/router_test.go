package router

import (
	"log/slog"
	"os"
	"strings"
	"testing"

	"message-gateway/internal/model"
)

func TestRouteHelp(t *testing.T) {
	r := New("", "", 0, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	got := r.Route(model.Envelope{EventID: "evt-1", Text: "/help", Kind: model.EnvelopeKindMessage})
	if !strings.Contains(got.Text, "可用命令") {
		t.Fatalf("expected help response, got %q", got.Text)
	}
}

func TestRouteEcho(t *testing.T) {
	r := New("", "", 0, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	got := r.Route(model.Envelope{EventID: "evt-2", Text: "hello", Kind: model.EnvelopeKindMessage})
	if !strings.Contains(got.Text, "hello") {
		t.Fatalf("expected echo response, got %q", got.Text)
	}
}
