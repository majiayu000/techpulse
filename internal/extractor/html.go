package extractor

import (
	"regexp"
	"strings"
)

// Pre-compiled regular expressions for HTML parsing.
var (
	// Remove script blocks
	scriptRe = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)

	// Remove style blocks
	styleRe = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)

	// Remove noscript blocks
	noscriptRe = regexp.MustCompile(`(?is)<noscript[^>]*>.*?</noscript>`)

	// Remove HTML comments
	commentRe = regexp.MustCompile(`(?s)<!--.*?-->`)

	// Remove remaining HTML tags
	tagRe = regexp.MustCompile(`<[^>]+>`)

	// Multiple whitespace
	whitespaceRe = regexp.MustCompile(`\s+`)
)

// extractTextFromHTML extracts plain text from HTML content.
func extractTextFromHTML(html string) string {
	// Remove script blocks
	text := scriptRe.ReplaceAllString(html, " ")

	// Remove style blocks
	text = styleRe.ReplaceAllString(text, " ")

	// Remove noscript blocks
	text = noscriptRe.ReplaceAllString(text, " ")

	// Remove HTML comments
	text = commentRe.ReplaceAllString(text, " ")

	// Remove boilerplate sections (header, nav, footer, etc.)
	text = removeBoilerplate(text)

	// Try to extract from main/article tags first
	if mainContent := extractMainContent(text); mainContent != "" {
		text = mainContent
	}

	// Extract paragraphs for better quality
	if paragraphs := extractParagraphs(text); len(paragraphs) > 0 {
		text = strings.Join(paragraphs, " ")
	} else {
		// Fallback: Remove all remaining HTML tags
		text = tagRe.ReplaceAllString(text, " ")
	}

	// Decode common HTML entities
	text = decodeHTMLEntities(text)

	// Normalize whitespace
	text = whitespaceRe.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

// htmlEntityPairs decodes common HTML entities. Order does not affect the
// result because strings.Replacer performs a single leftmost pass over the
// input and never rescans its own output, so "&amp;lt;" always decodes to
// "&lt;" — never to "<" — regardless of pair order.
var htmlEntityPairs = []string{
	"&nbsp;", " ",
	"&amp;", "&",
	"&lt;", "<",
	"&gt;", ">",
	"&quot;", "\"",
	"&apos;", "'",
	"&#39;", "'",
	"&mdash;", "-",
	"&ndash;", "-",
	"&rsquo;", "'",
	"&lsquo;", "'",
	"&rdquo;", "\"",
	"&ldquo;", "\"",
	"&hellip;", "...",
}

var entityReplacer = strings.NewReplacer(htmlEntityPairs...)

// decodeHTMLEntities converts common HTML entities to their characters in a
// single ordered pass. Unknown entities (e.g. "&eacute;") are left intact
// rather than being silently deleted.
func decodeHTMLEntities(s string) string {
	return entityReplacer.Replace(s)
}

// cleanText removes noise and normalizes text for display.
func cleanText(text string) string {
	// Normalize whitespace
	text = whitespaceRe.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	// Skip boilerplate at start of text
	text = findCleanStart(text)

	return strings.TrimSpace(text)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
