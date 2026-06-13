package dispatcher

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"message-gateway/internal/model"
)

func TestCoreClientStreamReplyChunksSSE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages:stream" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		f, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("response writer does not support flush")
		}

		_, _ = w.Write([]byte("data: {\"text\":\"he\",\"done\":false}\n\n"))
		f.Flush()
		_, _ = w.Write([]byte("data: {\"text\":\"llo\",\"done\":true}\n\n"))
		f.Flush()
	}))
	defer srv.Close()

	c := &CoreClient{
		baseURL:    srv.URL,
		streamPath: "/v1/messages:stream",
		timeout:    2 * time.Second,
		client: &http.Client{
			Timeout: 2 * time.Second,
		},
	}

	var got strings.Builder
	err := c.StreamReplyChunks(context.Background(), model.Envelope{EventID: "e1"}, "bot1", "s1", func(text string) error {
		got.WriteString(text)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamReplyChunks error: %v", err)
	}
	if got.String() != "hello" {
		t.Fatalf("unexpected text: %q", got.String())
	}
}

func TestCoreClientStreamReplyChunksNDJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages:stream" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{\"text\":\"a\",\"done\":false}\n"))
		_, _ = w.Write([]byte("{\"text\":\"b\",\"done\":true}\n"))
	}))
	defer srv.Close()

	c := &CoreClient{
		baseURL:    srv.URL,
		streamPath: "/v1/messages:stream",
		timeout:    2 * time.Second,
		client: &http.Client{
			Timeout: 2 * time.Second,
		},
	}

	var got strings.Builder
	err := c.StreamReplyChunks(context.Background(), model.Envelope{EventID: "e1"}, "bot1", "s1", func(text string) error {
		got.WriteString(text)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamReplyChunks error: %v", err)
	}
	if got.String() != "ab" {
		t.Fatalf("unexpected text: %q", got.String())
	}
}

func TestCoreClientStreamReplyChunksSendsSnakeCaseEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages:stream" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		raw, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var payload map[string]any
		if err := json.Unmarshal(raw, &payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		env, ok := payload["envelope"].(map[string]any)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if env["event_id"] != "e1" {
			t.Fatalf("missing or wrong event_id, got=%v", env["event_id"])
		}
		if _, ok := env["EventID"]; ok {
			t.Fatalf("unexpected EventID field in envelope JSON")
		}
		if env["text"] != "hi" {
			t.Fatalf("missing or wrong text, got=%v", env["text"])
		}
		if _, ok := env["Text"]; ok {
			t.Fatalf("unexpected Text field in envelope JSON")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"text\":\"ok\",\"done\":true}\n\n"))
	}))
	defer srv.Close()

	c := &CoreClient{
		baseURL:    srv.URL,
		streamPath: "/v1/messages:stream",
		timeout:    2 * time.Second,
		client: &http.Client{
			Timeout: 2 * time.Second,
		},
	}

	var got strings.Builder
	err := c.StreamReplyChunks(context.Background(), model.Envelope{EventID: "e1", Text: "hi"}, "bot1", "s1", func(text string) error {
		got.WriteString(text)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamReplyChunks error: %v", err)
	}
	if got.String() != "ok" {
		t.Fatalf("unexpected text: %q", got.String())
	}
}
