package dispatcher

import (
	"encoding/json"
	"testing"
)

func TestBuildStreamingCardUsesFallbackBodyWhenContentOnlyHasTitle(t *testing.T) {
	card, err := BuildStreamingCard("# 标题\n\n")
	if err != nil {
		t.Fatalf("BuildStreamingCard returned error: %v", err)
	}

	var payload struct {
		Elements []struct {
			Text struct {
				Content string `json:"content"`
			} `json:"text"`
		} `json:"elements"`
	}
	if err := json.Unmarshal([]byte(card), &payload); err != nil {
		t.Fatalf("unmarshal card: %v", err)
	}
	if len(payload.Elements) == 0 {
		t.Fatalf("expected card elements")
	}
	if payload.Elements[0].Text.Content == "" {
		t.Fatalf("expected non-empty lark_md content")
	}
}
