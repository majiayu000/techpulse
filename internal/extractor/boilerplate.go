// Package extractor provides article content extraction and summarization.
package extractor

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// Boilerplate detection regexes for common non-content elements.
var (
	// Remove header/nav/footer sections
	headerRe = regexp.MustCompile(`(?is)<header[^>]*>.*?</header>`)
	navRe    = regexp.MustCompile(`(?is)<nav[^>]*>.*?</nav>`)
	footerRe = regexp.MustCompile(`(?is)<footer[^>]*>.*?</footer>`)
	asideRe  = regexp.MustCompile(`(?is)<aside[^>]*>.*?</aside>`)

	// Remove form elements (login, subscribe)
	formRe = regexp.MustCompile(`(?is)<form[^>]*>.*?</form>`)

	// adOpenTagRe locates the next opening tag that carries a class or id
	// attribute. Group 1 captures the tag name and group 3 the attribute
	// value; the match ends at the value's closing quote, so callers must
	// locate the tag terminator ('>') themselves.
	adOpenTagRe = regexp.MustCompile(`(?is)<([a-z][a-z0-9-]*)[^>]*?[\s"'](class|id)="([^"]*)"`)

	// adAttrRe finds every class/id attribute within a single opening tag's
	// text, so an element with several attributes is judged on all of them.
	adAttrRe = regexp.MustCompile(`(?i)[\s"'](class|id)="([^"]*)"`)

	// Extract content from main tag
	mainRe = regexp.MustCompile(`(?is)<main[^>]*>(.*?)</main>`)

	// Paragraph content
	paragraphRe = regexp.MustCompile(`(?is)<p[^>]*>(.*?)</p>`)
)

// adKeywords are class/id name components that mark an element as an ad or
// tracking widget when they appear at word boundaries inside one token of the
// space-separated attribute value. "ad"/"ads" therefore require separator
// boundaries, so ordinary words containing "ad" (download, upload, read,
// breadcrumb...) are never flagged, while "ad-banner", "top-ad" or
// "data-ad-slot" still are.
var adKeywords = []string{
	"ad", "ads", "advertisement", "banner", "sidebar", "social", "share",
	"cookie", "consent", "newsletter", "popup", "modal", "overlay",
	"promo", "signup", "subscribe",
}

// boilerplatePhrases that indicate non-content text.
var boilerplatePhrases = []string{
	// Cookie/Privacy consent
	"cookie", "cookies", "accept all", "privacy policy", "privacy settings",
	"we use cookies", "this site uses", "consent",
	// Account actions
	"sign up", "sign in", "log in", "login", "register", "subscribe",
	"create account", "forgot password",
	// Navigation
	"skip to content", "skip to main", "menu", "navigation",
	"home page", "back to top",
	// Ads/Promos
	"advertisement", "sponsored", "promoted", "ad content",
	// Social
	"share on", "follow us", "like us on", "tweet this",
	// Misc boilerplate
	"read more", "continue reading", "show more", "load more",
	"comments", "leave a comment", "related articles",
	"newsletter", "join our mailing", "enter your email",
}

// removeBoilerplate strips common non-content HTML sections.
func removeBoilerplate(html string) string {
	// Remove structural boilerplate
	html = headerRe.ReplaceAllString(html, " ")
	html = navRe.ReplaceAllString(html, " ")
	html = footerRe.ReplaceAllString(html, " ")
	html = asideRe.ReplaceAllString(html, " ")
	html = formRe.ReplaceAllString(html, " ")

	// Remove ad/tracking elements
	return removeAdElements(html)
}

// toLowerASCII folds ASCII A-Z to a-z without changing UTF-8 byte length.
// strings.ToLower can shrink or grow runes (e.g. "İ" → "i"), so offsets found
// in a Unicode-lowered copy must not be used to slice the original HTML.
func toLowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

// removeAdElements removes elements whose class or id marks them as ads or
// tracking widgets. A keyword only counts when it appears at word boundaries
// within one token of the space-separated class/id list, so real content such
// as class="download-link" is never treated as an ad. Each element is removed
// through its own matching close tag (nesting aware); an element with no
// matching close tag (malformed HTML or a void element) is kept so that the
// rest of the document is never truncated.
func removeAdElements(html string) string {
	lower := toLowerASCII(html)
	var out strings.Builder
	pos := 0
	for {
		m := adOpenTagRe.FindStringSubmatchIndex(lower[pos:])
		if m == nil {
			out.WriteString(html[pos:])
			return out.String()
		}
		openStart := pos + m[0]
		quoteEnd := pos + m[1]

		// Locate the real end of the opening tag; the regex match stops at
		// the attribute value's closing quote, which may precede further
		// attributes and the tag's '>'.
		gtRel := strings.IndexByte(lower[quoteEnd:], '>')
		if gtRel < 0 {
			// Unterminated tag: leave it untouched and keep scanning after it.
			out.WriteString(html[pos:quoteEnd])
			pos = quoteEnd
			continue
		}
		openEnd := quoteEnd + gtRel + 1

		out.WriteString(html[pos:openStart])
		if !tagIsAd(html[openStart:openEnd]) {
			// Not an ad: keep the opening tag and resume scanning after it.
			out.WriteString(html[openStart:openEnd])
			pos = openEnd
			continue
		}

		name := lower[pos+m[2] : pos+m[3]]
		if strings.HasSuffix(strings.TrimSpace(html[openStart:openEnd]), "/>") {
			// Self-closing element: drop just the tag itself.
			out.WriteString(" ")
			pos = openEnd
			continue
		}

		_, closeEnd, found := findCloseTag(lower, name, openEnd)
		if !found {
			// Unclosed element: keep everything from this tag onward rather
			// than deleting the remainder of the document, and keep scanning.
			out.WriteString(html[openStart:])
			return out.String()
		}
		out.WriteString(" ")
		pos = closeEnd
	}
}

// tagIsAd reports whether any class/id attribute of the opening tag text
// carries an ad-keyword token.
func tagIsAd(openTag string) bool {
	for _, m := range adAttrRe.FindAllStringSubmatch(openTag, -1) {
		if elementIsAd(m[2]) {
			return true
		}
	}
	return false
}

// elementIsAd reports whether any whitespace-separated token of the class/id
// value contains an ad keyword at word boundaries.
func elementIsAd(value string) bool {
	value = strings.ToLower(value)
	for _, tok := range strings.Fields(value) {
		for _, kw := range adKeywords {
			if adTokenHasKeyword(tok, kw) {
				return true
			}
		}
	}
	return false
}

// adTokenHasKeyword reports whether kw occurs in tok delimited by the token
// edges or by separator characters ('-' and '_') on both sides — i.e. at word
// boundaries inside the token. "download-link" contains no boundary-delimited
// "ad", while "ad-banner", "top-ad" and "-ad-" compounds do.
func adTokenHasKeyword(tok, kw string) bool {
	for i := 0; i+len(kw) <= len(tok); {
		idx := strings.Index(tok[i:], kw)
		if idx < 0 {
			return false
		}
		i += idx
		end := i + len(kw)
		if (i == 0 || tok[i-1] == '-' || tok[i-1] == '_') &&
			(end == len(tok) || tok[end] == '-' || tok[end] == '_') {
			return true
		}
		i++
	}
	return false
}

// findOpenTag locates the next opening tag of the given (lower-case) name
// from index from, verifying that a tag-name boundary follows the name. It
// returns the half-open span of the opening tag including its '>'.
func findOpenTag(lower, name string, from int) (start, end int, found bool) {
	openTag := "<" + name
	for i := from; i < len(lower); {
		j := strings.IndexByte(lower[i:], '<')
		if j < 0 {
			return 0, 0, false
		}
		i += j
		if hasTagAt(lower, openTag, i) {
			gt := strings.IndexByte(lower[i:], '>')
			if gt < 0 {
				return 0, 0, false
			}
			return i, i + gt + 1, true
		}
		i++
	}
	return 0, 0, false
}

// findCloseTag scans lower from from for the close tag balancing an element
// opened just before from, counting same-name nesting depth and skipping
// self-closing same-name tags. It returns the half-open span of the matching
// close tag, or found=false when the document ends first.
func findCloseTag(lower, name string, from int) (start, end int, found bool) {
	openTag := "<" + name
	closeTag := "</" + name
	depth := 1
	for i := from; i < len(lower); {
		j := strings.IndexByte(lower[i:], '<')
		if j < 0 {
			return 0, 0, false
		}
		i += j
		if hasTagAt(lower, closeTag, i) {
			gt := strings.IndexByte(lower[i:], '>')
			if gt < 0 {
				return 0, 0, false
			}
			end := i + gt + 1
			depth--
			if depth == 0 {
				return i, end, true
			}
			i = end
			continue
		}
		if hasTagAt(lower, openTag, i) {
			gt := strings.IndexByte(lower[i:], '>')
			if gt < 0 {
				return 0, 0, false
			}
			tagEnd := i + gt + 1
			if !strings.HasSuffix(strings.TrimSpace(lower[i:tagEnd]), "/>") {
				depth++
			}
			i = tagEnd
			continue
		}
		i++
	}
	return 0, 0, false
}

// hasTagAt reports whether the tag token tag starts at i in lower and is
// followed by a tag-name boundary character.
func hasTagAt(lower, tag string, i int) bool {
	if !strings.HasPrefix(lower[i:], tag) {
		return false
	}
	j := i + len(tag)
	return j >= len(lower) || isTagBoundary(lower[j])
}

// isTagBoundary reports whether b may follow a tag name inside a tag.
func isTagBoundary(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\f', '\r', '/', '>':
		return true
	}
	return false
}

// extractMainContent attempts to extract text from article/main tags.
func extractMainContent(html string) string {
	// Try the article tag first, honoring nested articles.
	if content := extractArticleContent(html); content != "" {
		return content
	}
	// Try main tag
	matches := mainRe.FindStringSubmatch(html)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// extractArticleContent returns the inner HTML of the first well-formed
// <article> element. Nested <article> teasers are counted toward nesting
// depth instead of truncating extraction at the first inner close tag;
// listing pages commonly nest article cards inside the main article.
func extractArticleContent(html string) string {
	// ASCII fold only: Unicode ToLower can change byte length (İ→i), and
	// offsets from that copy must not slice the original document.
	lower := toLowerASCII(html)
	from := 0
	for {
		_, openEnd, found := findOpenTag(lower, "article", from)
		if !found {
			return ""
		}
		closeStart, _, ok := findCloseTag(lower, "article", openEnd)
		if !ok {
			// Unclosed article: try the next one rather than truncating here.
			from = openEnd
			continue
		}
		return html[openEnd:closeStart]
	}
}

// extractParagraphs extracts text from all <p> tags.
func extractParagraphs(html string) []string {
	matches := paragraphRe.FindAllStringSubmatch(html, -1)
	var paragraphs []string
	for _, m := range matches {
		if len(m) >= 2 {
			text := strings.TrimSpace(tagRe.ReplaceAllString(m[1], " "))
			text = whitespaceRe.ReplaceAllString(text, " ")
			if len(text) > 50 { // Only substantial paragraphs
				paragraphs = append(paragraphs, text)
			}
		}
	}
	return paragraphs
}

// isBoilerplate reports whether text begins with a known boilerplate phrase
// ending at a word boundary. Matching at the start avoids flagging real
// sentences that merely contain a phrase, such as "Developers can now sign in
// with passkeys".
func isBoilerplate(text string) bool {
	lower := strings.ToLower(text)
	for _, phrase := range boilerplatePhrases {
		if phraseEndsAtWordBoundary(lower, phrase) {
			return true
		}
	}
	return false
}

// phraseEndsAtWordBoundary reports whether lower starts with phrase and the
// phrase ends at a word boundary (next character is not part of a word).
func phraseEndsAtWordBoundary(lower, phrase string) bool {
	n := len(phrase)
	if !strings.HasPrefix(lower, phrase) {
		return false
	}
	if n >= len(lower) {
		return true
	}
	return !isWordChar(lower[n])
}

// isWordChar reports whether b continues a word (letter, digit, or part of a
// multi-byte rune).
func isWordChar(b byte) bool {
	switch {
	case 'a' <= b && b <= 'z', 'A' <= b && b <= 'Z', '0' <= b && b <= '9':
		return true
	case b >= utf8.RuneSelf:
		return true
	}
	return false
}

// findCleanStart skips boilerplate at the start of text.
func findCleanStart(text string) string {
	// Try to skip past boilerplate by finding sentence boundaries
	sentences := strings.Split(text, ". ")
	var cleanParts []string

	skipped := 0
	for i, sent := range sentences {
		sent = strings.TrimSpace(sent)
		if sent == "" {
			continue
		}

		// Skip first few sentences if they look like boilerplate
		if i < 5 && isBoilerplate(sent) {
			skipped++
			continue
		}

		// Stop skipping once we find good content
		if skipped > 0 && len(sent) > 30 && !isBoilerplate(sent) {
			cleanParts = append(cleanParts, sentences[i:]...)
			break
		}

		cleanParts = append(cleanParts, sent)
	}

	if len(cleanParts) == 0 {
		return text // Fall back to original
	}

	return strings.Join(cleanParts, ". ")
}
