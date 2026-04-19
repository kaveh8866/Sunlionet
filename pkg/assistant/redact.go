package assistant

import (
	"regexp"
	"strings"
)

var (
	reEmail = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
	reURL   = regexp.MustCompile(`(?i)\bhttps?://[^\s]+\b`)
	reDigit = regexp.MustCompile(`\d{3,}`)
)

func RedactText(s string, profile RedactionProfile) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	switch profile {
	case RedactionOff:
		return s
	case RedactionModerate:
		s = reEmail.ReplaceAllString(s, "[redacted_email]")
		s = reURL.ReplaceAllString(s, "[redacted_url]")
		return s
	default:
		s = reEmail.ReplaceAllString(s, "[redacted_email]")
		s = reURL.ReplaceAllString(s, "[redacted_url]")
		s = reDigit.ReplaceAllString(s, "[redacted_digits]")
		return s
	}
}

func BuildPrompt(items []Item, profile RedactionProfile, maxItems int, maxBytes int) (string, int) {
	if maxItems <= 0 {
		maxItems = 24
	}
	if maxBytes <= 0 {
		maxBytes = 24 * 1024
	}
	start := 0
	if len(items) > maxItems {
		start = len(items) - maxItems
	}
	var b strings.Builder
	for i := start; i < len(items); i++ {
		role := string(items[i].Role)
		txt := RedactText(items[i].Text, profile)
		line := role + ": " + txt + "\n"
		if b.Len()+len(line) > maxBytes {
			break
		}
		b.WriteString(line)
	}
	return strings.TrimSpace(b.String()), b.Len()
}
