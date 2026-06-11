package dispatcher

import (
	"encoding/json"
	"regexp"
	"strings"
)

var (
	headerRe = regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)
	quoteRe  = regexp.MustCompile(`(?m)^>\s?(.*)$`)
	hrRe     = regexp.MustCompile(`(?m)^---+$`)
	imageRe  = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`)
	titleRe  = regexp.MustCompile(`(?m)^#{1,3}\s+(.+)$`)
)

func toLarkMarkdown(s string) string {
	s = headerRe.ReplaceAllString(s, "**$2**")
	s = quoteRe.ReplaceAllString(s, "*$1*")
	s = hrRe.ReplaceAllString(s, "---")
	s = imageRe.ReplaceAllString(s, "$1")
	return s
}

func extractTitle(content string) string {
	if m := titleRe.FindStringSubmatch(content); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	if i := strings.IndexByte(content, '\n'); i > 0 && i <= 60 {
		return strings.TrimSpace(content[:i])
	}
	if len(content) > 60 {
		return content[:60]
	}
	if content != "" {
		return content
	}
	return "Knowledge Agent"
}

func stripTitle(content string) string {
	loc := titleRe.FindStringIndex(content)
	if loc == nil || loc[0] != 0 {
		return content
	}
	return strings.TrimLeft(content[loc[1]:], "\n")
}

func buildCardHeader(title string) map[string]any {
	return map[string]any{
		"title": map[string]any{
			"tag":     "plain_text",
			"content": title,
		},
		"template": "blue",
		"icon": map[string]any{
			"tag":   "standard_icon",
			"token": "ai-colorful",
		},
	}
}

func buildCardConfig(streaming bool, summary string) map[string]any {
	cfg := map[string]any{
		"width_mode":   "fill",
		"update_multi": true,
		"style": map[string]any{
			"text_size": map[string]any{
				"usage-size": map[string]any{
					"default": "small",
					"pc":      "small",
					"mobile":  "small",
				},
			},
		},
	}
	if streaming {
		cfg["streaming_mode"] = true
		cfg["streaming_config"] = map[string]any{
			"print_frequency_ms": map[string]any{"default": 30, "pc": 50},
			"print_step":         map[string]any{"default": 2},
			"print_strategy":     "fast",
		}
		if strings.TrimSpace(summary) != "" {
			cfg["summary"] = map[string]any{"content": summary}
		}
	}
	return cfg
}

const maxLarkTables = 5

func limitTables(s string) string {
	lines := strings.Split(s, "\n")
	tableCount := 0
	inTable := false
	overflow := false

	for i, line := range lines {
		isTableLine := strings.HasPrefix(strings.TrimSpace(line), "|")
		if isTableLine && !inTable {
			tableCount++
			inTable = true
			overflow = tableCount > maxLarkTables
		} else if !isTableLine {
			inTable = false
		}
		if overflow && isTableLine {
			lines[i] = strings.ReplaceAll(line, "|", "│")
		}
	}
	return strings.Join(lines, "\n")
}

func BuildStreamingCard(content string) (string, error) {
	return buildCard(content, true, "生成中...")
}

func BuildFinalCard(content string) (string, error) {
	return buildCard(content, false, "")
}

func buildCard(content string, streaming bool, summary string) (string, error) {
	card := map[string]any{
		"schema": "2.0",
		"config": buildCardConfig(streaming, summary),
		"header": buildCardHeader(extractTitle(content)),
		"body": map[string]any{
			"vertical_spacing": "8px",
			"elements": []any{
				map[string]any{
					"tag":        "markdown",
					"content":    toLarkMarkdown(limitTables(stripTitle(content))),
					"element_id": "main_content",
				},
			},
		},
	}
	b, err := json.Marshal(card)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
