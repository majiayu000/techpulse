package summarizer

import (
	"strings"
	"testing"
)

func TestSanitizeMarkdownText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain text untouched", "hello world", "hello world"},
		{"CJK preserved", "华为发布新一代芯片", "华为发布新一代芯片"},
		{
			"newlines cannot forge structure",
			"title\n## Sponsored by evil\nbody",
			"title## Sponsored by evilbody",
		},
		{"carriage return and tab stripped", "a\r\nb\tc", "abc"},
		// bracket escaping applies after control-char stripping
		{"other control chars stripped", "a\x00\x1b[31mb", "a\\[31mb"},
		{"brackets escaped", "see [this] link", "see \\[this\\] link"},
		{"backslash before bracket escaped first", `Read this\](https://attacker.example)`, `Read this\\\](https://attacker.example)`},
		{"DEL stripped", "a\x7fb", "ab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeMarkdownText(tt.in); got != tt.want {
				t.Errorf("SanitizeMarkdownText(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSafeLinkURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"https kept", "https://example.com/a?b=1", "https://example.com/a?b=1"},
		{"http kept", "http://example.com/x", "http://example.com/x"},
		{"javascript rejected", "javascript:alert(1)", ""},
		{"data rejected", "data:text/html;base64,AAAA", ""},
		{"scheme-relative rejected", "//evil.example/path", ""},
		{"bare text rejected", "not a url", ""},
		{"hostless rejected", "mailto:x@y.z", ""},
		{
			"parens escaped so link cannot terminate early",
			"https://en.wikipedia.org/wiki/Go_(programming_language)",
			"https://en.wikipedia.org/wiki/Go_%28programming_language%29",
		},
		{"spaces escaped", "https://example.com/a b", "https://example.com/a%20b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SafeLinkURL(tt.in); got != tt.want {
				t.Errorf("SafeLinkURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestMarkdownLink(t *testing.T) {
	if got := MarkdownLink("A [nice] title", "https://example.com/x"); got != "[A \\[nice\\] title](https://example.com/x)" {
		t.Errorf("MarkdownLink safe case = %q", got)
	}
	// A remote title that tries to close link text via a pre-escaped bracket
	// must not become the rendered destination.
	gotBreakout := MarkdownLink(`Read this\](https://attacker.example)`, "https://legitimate.example/a")
	if strings.Contains(gotBreakout, "](https://attacker.example)") && !strings.Contains(gotBreakout, `\\\]`) {
		t.Errorf("backslash breakout still active: %q", gotBreakout)
	}
	if !strings.HasPrefix(gotBreakout, "[") || !strings.Contains(gotBreakout, "](https://legitimate.example/a)") {
		t.Errorf("expected legitimate URL to remain the link destination, got %q", gotBreakout)
	}
	got := MarkdownLink("evil\n## injected", "javascript:alert(1)")
	if strings.Contains(got, "javascript:") || strings.Contains(got, "\n") {
		t.Errorf("MarkdownLink unsafe case leaked: %q", got)
	}
	if got != "evil## injected" {
		t.Errorf("MarkdownLink unsafe case = %q, want plain sanitized title", got)
	}
}

func TestGenerateMarkdownSanitizesRemoteContent(t *testing.T) {
	s := NewBasicSummarizer()
	// EnrichedArticle embeds filter.FilteredArticle; promoted fields must be
	// assigned, not set in the composite literal.
	var art EnrichedArticle
	art.Title = "broken ](https://evil.example) ![t](https://tracker.example/pixel.png)"
	art.URL = "javascript:alert(document.cookie)"
	art.Summary = "line one\n## forged heading"
	art.Importance = 5
	arts := []EnrichedArticle{art}
	md := s.generateMarkdown("2026-08-23", arts, Stats{TotalArticles: 1})

	for _, bad := range []string{"javascript:", "\n## forged", "!["} {
		if strings.Contains(md, bad) {
			t.Errorf("generateMarkdown output contains attack payload %q:\n%s", bad, md)
		}
	}
	// Brackets must be escaped, so the link breakout renders inert.
	if !strings.Contains(md, `\](https://evil.example`) {
		t.Errorf("expected escaped bracket in output:\n%s", md)
	}
}
