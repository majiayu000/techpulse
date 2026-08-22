package github

import (
	"fmt"
	"strings"
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
	repos, err := p.Parse(html)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

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
	repos, err := p.Parse(html)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(repos) != 3 {
		t.Fatalf("Parse() got %d repos, want 3", len(repos))
	}
}

func TestParser_ParseEmpty(t *testing.T) {
	p := NewParser()
	repos, err := p.Parse("")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(repos) != 0 {
		t.Errorf("Parse() got %d repos, want 0", len(repos))
	}
}

func TestParseNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		wantErr  bool
	}{
		{"123", 123, false},
		{"1,234", 1234, false},
		{"12,345,678", 12345678, false},
		{" 456 ", 456, false},
		{"", 0, true},                      // unparsable: must error, not silently yield 0
		{"abc", 0, true},                   // unparsable: must error, not silently yield 0
		{strings.Repeat("9", 25), 0, true}, // overflows int64: must error, not clamp
	}

	for _, tt := range tests {
		got, err := parseNumber(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseNumber(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.expected {
			t.Errorf("parseNumber(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestParser_ParseInvalidNumber(t *testing.T) {
	// A star count too large for int64 must fail parsing instead of being
	// silently clamped or zeroed.
	html := fmt.Sprintf(`
	<article class="Box-row">
		<h2><a href="/owner/repo"></a></h2>
		<a href="/owner/repo/stargazers">%s</a>
	</article>
	`, strings.Repeat("9", 25))

	p := NewParser()
	repos, err := p.Parse(html)
	if err == nil {
		t.Fatalf("Parse() expected error for unparsable star count, got repos=%v", repos)
	}
	if !strings.Contains(err.Error(), "invalid number") {
		t.Errorf("Parse() error = %v, want it to mention the invalid number", err)
	}
	if repos != nil {
		t.Errorf("Parse() repos = %v, want nil on error", repos)
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
		wantErr  bool
	}{
		{"123 stars today", 123, false},
		{"1,234 stars today", 1234, false},
		{"456 stars this week", 456, false},
		{"789 stars this month", 789, false},
		{"no stars", 0, false}, // no match is not an error: field is simply absent
	}

	for _, tt := range tests {
		got, err := extractStarsToday(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("extractStarsToday(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.expected {
			t.Errorf("extractStarsToday(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}
