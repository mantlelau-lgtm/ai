package dispatcher

import (
	"context"
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
