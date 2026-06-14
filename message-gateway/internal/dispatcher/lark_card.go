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
	}
}

func buildCardConfig(streaming bool, summary string) map[string]any {
	cfg := map[string]any{
		"update_multi": true,
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
	body := strings.TrimSpace(toLarkMarkdown(limitTables(stripTitle(content))))
	if body == "" {
		body = "生成中..."
	}
	card := map[string]any{
		"config": buildCardConfig(streaming, summary),
		"header": buildCardHeader(extractTitle(content)),
		"elements": []any{
			map[string]any{
				"tag": "div",
				"text": map[string]any{
					"tag":     "lark_md",
					"content": body,
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
