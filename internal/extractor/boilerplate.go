// Package extractor provides article content extraction and summarization.
package extractor

import (
	"regexp"
	"strings"
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

	// Remove common ad/tracking elements
	adClassRe = regexp.MustCompile(`(?is)<[^>]+(class|id)="[^"]*` +
		`(ad-|ads-|advertisement|banner|sidebar|social|share|cookie|consent|` +
		`newsletter|popup|modal|overlay|promo|signup|subscribe)` +
		`[^"]*"[^>]*>.*?</[^>]+>`)

	// Extract content from article tag
	articleRe = regexp.MustCompile(`(?is)<article[^>]*>(.*?)</article>`)
	// Extract content from main tag
	mainRe = regexp.MustCompile(`(?is)<main[^>]*>(.*?)</main>`)

	// Paragraph content
	paragraphRe = regexp.MustCompile(`(?is)<p[^>]*>(.*?)</p>`)
)

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
	html = adClassRe.ReplaceAllString(html, " ")

	return html
}

// extractMainContent attempts to extract text from article/main tags.
func extractMainContent(html string) string {
	// Try article tag first
	matches := articleRe.FindStringSubmatch(html)
	if len(matches) >= 2 {
		return matches[1]
	}
	// Try main tag
	matches = mainRe.FindStringSubmatch(html)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
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

// isBoilerplate checks if text starts with boilerplate phrases.
func isBoilerplate(text string) bool {
	lower := strings.ToLower(text)
	prefixLen := min(len(lower), 150)
	prefix := lower[:prefixLen]

	for _, phrase := range boilerplatePhrases {
		if strings.Contains(prefix, phrase) {
			return true
		}
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
