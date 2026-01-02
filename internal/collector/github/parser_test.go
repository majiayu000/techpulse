package github

import (
	"testing"
)

func TestParser_Parse(t *testing.T) {
	html := `
	<article class="Box-row">
		<h2>
			<a href="/owner/repo" data-view-component="true">
				<span>owner</span> / <span>repo</span>
			</a>
		</h2>
		<p class="col-9 color-fg-muted my-1 pr-4">A great repository</p>
		<span itemprop="programmingLanguage">Python</span>
		<a href="/owner/repo/stargazers">1,234</a>
		<a href="/owner/repo/forks">567</a>
		<span>89 stars today</span>
	</article>
	`

	p := NewParser()
	repos := p.Parse(html)

	if len(repos) != 1 {
		t.Fatalf("Parse() got %d repos, want 1", len(repos))
	}

	repo := repos[0]
	if repo.Name != "owner/repo" {
		t.Errorf("Name = %q, want %q", repo.Name, "owner/repo")
	}
	if repo.URL != "https://github.com/owner/repo" {
		t.Errorf("URL = %q, want %q", repo.URL, "https://github.com/owner/repo")
	}
	if repo.Description != "A great repository" {
		t.Errorf("Description = %q, want %q", repo.Description, "A great repository")
	}
	if repo.Language != "Python" {
		t.Errorf("Language = %q, want %q", repo.Language, "Python")
	}
	if repo.Stars != 1234 {
		t.Errorf("Stars = %d, want %d", repo.Stars, 1234)
	}
	if repo.Forks != 567 {
		t.Errorf("Forks = %d, want %d", repo.Forks, 567)
	}
	if repo.StarsToday != 89 {
		t.Errorf("StarsToday = %d, want %d", repo.StarsToday, 89)
	}
}

func TestParser_ParseMultiple(t *testing.T) {
	html := `
	<article class="Box-row">
		<h2><a href="/first/repo"></a></h2>
	</article>
	<article class="Box-row">
		<h2><a href="/second/repo"></a></h2>
	</article>
	<article class="Box-row">
		<h2><a href="/third/repo"></a></h2>
	</article>
	`

	p := NewParser()
	repos := p.Parse(html)

	if len(repos) != 3 {
		t.Fatalf("Parse() got %d repos, want 3", len(repos))
	}
}

func TestParser_ParseEmpty(t *testing.T) {
	p := NewParser()
	repos := p.Parse("")

	if len(repos) != 0 {
		t.Errorf("Parse() got %d repos, want 0", len(repos))
	}
}

func TestParseNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"123", 123},
		{"1,234", 1234},
		{"12,345,678", 12345678},
		{" 456 ", 456},
		{"", 0},
		{"abc", 0},
	}

	for _, tt := range tests {
		got := parseNumber(tt.input)
		if got != tt.expected {
			t.Errorf("parseNumber(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestCleanText(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"<p>Hello</p>", "Hello"},
		{"  Multiple   spaces  ", "Multiple spaces"},
		{"Line1\nLine2", "Line1 Line2"},
		{"<a href='#'>Link</a> text", "Link text"},
	}

	for _, tt := range tests {
		got := cleanText(tt.input)
		if got != tt.expected {
			t.Errorf("cleanText(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestExtractStarsToday(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"123 stars today", 123},
		{"1,234 stars today", 1234},
		{"456 stars this week", 456},
		{"789 stars this month", 789},
		{"no stars", 0},
	}

	for _, tt := range tests {
		got := extractStarsToday(tt.input)
		if got != tt.expected {
			t.Errorf("extractStarsToday(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}
