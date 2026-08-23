package summarizer

import (
	"net/url"
	"strings"
)

// sanitizeTextReplacer strips nothing by itself; text sanitization drops
// control characters and escapes square brackets so remote-controlled
// strings cannot forge Markdown structure (headings via newlines, link
// breakouts via brackets).
var bracketEscaper = strings.NewReplacer("[", "\\[", "]", "\\]")

// SanitizeMarkdownText makes a remote-controlled string safe to embed in
// generated Markdown: control characters (including newlines that could
// forge report structure) are removed and square brackets are escaped so
// the text cannot break out of link-text position.
func SanitizeMarkdownText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	return bracketEscaper.Replace(b.String())
}

// SafeLinkURL validates a remote-supplied URL for use as a Markdown link
// destination: only absolute http(s) URLs with a host are allowed. It
// returns "" for anything else (javascript:, data:, scheme-relative, ...),
// in which case callers should render the title as plain text.
// Parentheses and spaces are percent-escaped so the destination cannot
// terminate the Markdown link early.
func SafeLinkURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" || u.Host == "" {
		return ""
	}
	replay := strings.NewReplacer("(", "%28", ")", "%29", " ", "%20")
	return replay.Replace(u.String())
}

// MarkdownLink renders "[title](url)" when url is a safe http(s) link and
// plain sanitized title text otherwise.
func MarkdownLink(title, rawURL string) string {
	t := SanitizeMarkdownText(title)
	if link := SafeLinkURL(rawURL); link != "" {
		return "[" + t + "](" + link + ")"
	}
	return t
}
