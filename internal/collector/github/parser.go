package github

import (
	"regexp"
	"strconv"
	"strings"
)

// Parser parses GitHub trending HTML page.
type Parser struct{}

// NewParser creates a new HTML parser.
func NewParser() *Parser {
	return &Parser{}
}

// Parse extracts repositories from HTML content.
func (p *Parser) Parse(html string) []Repository {
	var repos []Repository

	// Find all article elements (each trending repo)
	articles := findAllArticles(html)

	for _, article := range articles {
		repo := p.parseArticle(article)
		if repo.Name != "" {
			repos = append(repos, repo)
		}
	}

	return repos
}

// findAllArticles extracts article HTML blocks from the page.
func findAllArticles(html string) []string {
	var articles []string
	re := regexp.MustCompile(`(?s)<article[^>]*class="[^"]*Box-row[^"]*"[^>]*>(.*?)</article>`)
	matches := re.FindAllStringSubmatch(html, -1)
	for _, m := range matches {
		if len(m) > 1 {
			articles = append(articles, m[1])
		}
	}
	return articles
}

// parseArticle parses a single article HTML block.
func (p *Parser) parseArticle(html string) Repository {
	return Repository{
		Name:        extractRepoName(html),
		URL:         extractRepoURL(html),
		Description: extractDescription(html),
		Language:    extractLanguage(html),
		Stars:       extractStars(html),
		StarsToday:  extractStarsToday(html),
		Forks:       extractForks(html),
	}
}

// extractRepoName extracts "owner/repo" from HTML.
func extractRepoName(html string) string {
	// Pattern for h2 heading with link to repo (2024+ structure)
	re := regexp.MustCompile(`<h2[^>]*>[\s\S]*?href="/([a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+)"`)
	if m := re.FindStringSubmatch(html); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	// Fallback: any link matching owner/repo pattern
	re = regexp.MustCompile(`href="/([a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+)"[^>]*class="[^"]*Link`)
	if m := re.FindStringSubmatch(html); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// extractRepoURL builds the full GitHub URL.
func extractRepoURL(html string) string {
	name := extractRepoName(html)
	if name == "" {
		return ""
	}
	return "https://github.com/" + name
}

// extractDescription extracts the repo description.
func extractDescription(html string) string {
	re := regexp.MustCompile(`(?s)<p[^>]*class="[^"]*col-9[^"]*"[^>]*>(.*?)</p>`)
	if m := re.FindStringSubmatch(html); len(m) > 1 {
		return cleanText(m[1])
	}
	return ""
}

// extractLanguage extracts the primary programming language.
func extractLanguage(html string) string {
	re := regexp.MustCompile(`<span[^>]*itemprop="programmingLanguage"[^>]*>([^<]+)</span>`)
	if m := re.FindStringSubmatch(html); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// extractStars extracts total star count.
func extractStars(html string) int {
	// Pattern 1: stargazers link with SVG, then number before </a> (2024+ structure)
	re := regexp.MustCompile(`(?s)href="/[^/]+/[^/]+/stargazers"[^>]*>.*?</svg>\s*([0-9,]+)\s*</a>`)
	if m := re.FindStringSubmatch(html); len(m) > 1 {
		return parseNumber(m[1])
	}
	// Pattern 2: simple stargazers link with direct number (legacy)
	re = regexp.MustCompile(`href="/[^/]+/[^/]+/stargazers"[^>]*>\s*([0-9,]+)`)
	if m := re.FindStringSubmatch(html); len(m) > 1 {
		return parseNumber(m[1])
	}
	return 0
}

// extractForks extracts fork count.
func extractForks(html string) int {
	// Pattern 1: forks link with SVG, then number before </a> (2024+ structure)
	re := regexp.MustCompile(`(?s)href="/[^/]+/[^/]+/forks"[^>]*>.*?</svg>\s*([0-9,]+)\s*</a>`)
	if m := re.FindStringSubmatch(html); len(m) > 1 {
		return parseNumber(m[1])
	}
	// Pattern 2: simple forks link with direct number (legacy)
	re = regexp.MustCompile(`href="/[^/]+/[^/]+/forks"[^>]*>\s*([0-9,]+)`)
	if m := re.FindStringSubmatch(html); len(m) > 1 {
		return parseNumber(m[1])
	}
	return 0
}

// extractStarsToday extracts stars gained today.
func extractStarsToday(html string) int {
	re := regexp.MustCompile(`([0-9,]+)\s*stars?\s*(today|this week|this month)`)
	if m := re.FindStringSubmatch(html); len(m) > 1 {
		return parseNumber(m[1])
	}
	return 0
}

// parseNumber converts string with commas to int.
func parseNumber(s string) int {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)
	n, _ := strconv.Atoi(s)
	return n
}

// cleanText removes HTML tags and extra whitespace.
func cleanText(s string) string {
	re := regexp.MustCompile(`<[^>]+>`)
	s = re.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	re = regexp.MustCompile(`\s+`)
	return re.ReplaceAllString(s, " ")
}
