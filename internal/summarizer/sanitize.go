package summarizer

import (
	"net/url"
	"strings"
)

// markdownTextEscaper escapes Markdown/HTML-sensitive characters after
// control stripping: backslashes then square brackets (link breakouts),
// then angle brackets (raw HTML injection into digests). Ampersands are
// left alone so already-escaped feed text (e.g. &lt;) is not double-escaped.
var markdownTextEscaper = strings.NewReplacer(
	"\\", "\\\\",
	"[", "\\[",
	"]", "\\]",
	"<", "&lt;",
	">", "&gt;",
)

// SanitizeMarkdownText makes a remote-controlled string safe to embed in
// generated Markdown: control characters (including newlines that could
// forge report structure) are removed; square brackets are escaped so the
// text cannot break out of link-text position; and angle brackets are
// escaped so classic RSS / remote titles cannot inject raw HTML into digests.
func SanitizeMarkdownText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	return markdownTextEscaper.Replace(b.String())
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
