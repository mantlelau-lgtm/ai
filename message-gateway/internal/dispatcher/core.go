package dispatcher

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"message-gateway/internal/config"
	"message-gateway/internal/model"
)

type CoreClient struct {
	baseURL    string
	streamPath string
	timeout    time.Duration
	client     *http.Client
}

func NewCoreClient(cfg config.Config) *CoreClient {
	if strings.TrimSpace(cfg.CoreBaseURL) == "" {
		return nil
	}
	return &CoreClient{
		baseURL:    strings.TrimRight(cfg.CoreBaseURL, "/"),
		streamPath: cfg.CoreStreamPath,
		timeout:    cfg.CoreTimeout,
		client: &http.Client{
			Timeout: cfg.CoreTimeout,
		},
	}
}

type CoreStreamResult struct {
	Text string
}

func (c *CoreClient) StreamReply(ctx context.Context, env model.Envelope, botID, sessionID string) (CoreStreamResult, error) {
	var b strings.Builder
	err := c.StreamReplyChunks(ctx, env, botID, sessionID, func(delta string) error {
		if delta != "" {
			b.WriteString(delta)
		}
		return nil
	})
	if err != nil {
		return CoreStreamResult{}, err
	}
	return CoreStreamResult{Text: b.String()}, nil
}

func (c *CoreClient) StreamReplyChunks(ctx context.Context, env model.Envelope, botID, sessionID string, onDelta func(text string) error) error {
	if c == nil {
		return fmt.Errorf("core client not configured")
	}

	body, err := json.Marshal(map[string]interface{}{
		"envelope": env,
	})
	if err != nil {
		return err
	}

	url := c.baseURL + c.streamPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream, application/x-ndjson, application/json")
	req.Header.Set("X-Bot-Id", botID)
	req.Header.Set("X-Session-Id", sessionID)
	if env.SenderUserID != "" {
		req.Header.Set("X-User-Id", env.SenderUserID)
	}
	if env.SenderOpenID != "" {
		req.Header.Set("X-Open-Id", env.SenderOpenID)
	}
	if env.ChatID != "" {
		req.Header.Set("X-Chat-Id", env.ChatID)
	}
	if env.MessageID != "" {
		req.Header.Set("X-Message-Id", env.MessageID)
	}
	if env.EventID != "" {
		req.Header.Set("X-Event-Id", env.EventID)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("core stream http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		return streamSSE(resp.Body, onDelta)
	}

	return streamNDJSONOrJSON(resp.Body, onDelta)
}

type coreStreamChunk struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
	Done bool   `json:"done,omitempty"`
}

func streamSSE(r io.Reader, onDelta func(text string) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			return nil
		}

		if strings.HasPrefix(data, "{") {
			var chunk coreStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err == nil {
				if chunk.Text != "" {
					if onDelta != nil {
						if err := onDelta(chunk.Text); err != nil {
							return err
						}
					}
				}
				if chunk.Done {
					return nil
				}
				continue
			}
		}

		if onDelta != nil {
			if err := onDelta(data); err != nil {
				return err
			}
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return nil
}

func streamNDJSONOrJSON(r io.Reader, onDelta func(text string) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	seenLine := false
	for sc.Scan() {
		seenLine = true
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "{") {
			var chunk coreStreamChunk
			if err := json.Unmarshal([]byte(line), &chunk); err == nil {
				if chunk.Text != "" {
					if onDelta != nil {
						if err := onDelta(chunk.Text); err != nil {
							return err
						}
					}
				}
				if chunk.Done {
					return nil
				}
				continue
			}
		}

		if onDelta != nil {
			if err := onDelta(line); err != nil {
				return err
			}
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if seenLine {
		return nil
	}

	raw, err := io.ReadAll(io.LimitReader(r, 1<<20))
	if err != nil {
		return err
	}
	if onDelta != nil {
		if err := onDelta(strings.TrimSpace(string(raw))); err != nil {
			return err
		}
	}
	return nil
}
