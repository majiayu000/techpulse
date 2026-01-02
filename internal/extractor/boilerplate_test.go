package extractor

import (
	"strings"
	"testing"
)

func TestRemoveBoilerplate(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "removes header",
			html: "<header>Nav stuff</header><p>Content</p>",
			want: " <p>Content</p>",
		},
		{
			name: "removes nav",
			html: "<nav>Menu items</nav><article>Article</article>",
			want: " <article>Article</article>",
		},
		{
			name: "removes footer",
			html: "<p>Content</p><footer>Copyright 2024</footer>",
			want: "<p>Content</p> ",
		},
		{
			name: "removes aside",
			html: "<main>Main</main><aside>Sidebar</aside>",
			want: "<main>Main</main> ",
		},
		{
			name: "removes forms",
			html: "<form>Email signup</form><p>Article</p>",
			want: " <p>Article</p>",
		},
		{
			name: "keeps main content",
			html: "<article><p>Important content here</p></article>",
			want: "<article><p>Important content here</p></article>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeBoilerplate(tt.html)
			if got != tt.want {
				t.Errorf("removeBoilerplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractMainContent(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "extracts article content",
			html: "<header>nav</header><article>The article content</article><footer>f</footer>",
			want: "The article content",
		},
		{
			name: "extracts main content",
			html: "<nav>menu</nav><main>The main content</main>",
			want: "The main content",
		},
		{
			name: "no article or main returns empty",
			html: "<div>Just a div</div>",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractMainContent(tt.html)
			if got != tt.want {
				t.Errorf("extractMainContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractParagraphs(t *testing.T) {
	tests := []struct {
		name       string
		html       string
		wantCount  int
		wantFirst  string
	}{
		{
			name:       "extracts long paragraphs",
			html:       "<p>This is a paragraph with more than fifty characters of content here.</p>",
			wantCount:  1,
			wantFirst:  "This is a paragraph with more than fifty characters of content here.",
		},
		{
			name:       "ignores short paragraphs",
			html:       "<p>Short</p><p>This is a longer paragraph that has enough content.</p>",
			wantCount:  1,
			wantFirst:  "This is a longer paragraph that has enough content.",
		},
		{
			name:       "handles nested tags",
			html:       "<p>This paragraph has <strong>bold</strong> text and more content to make it long enough.</p>",
			wantCount:  1,
			wantFirst:  "This paragraph has bold text and more content to make it long enough.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractParagraphs(tt.html)
			if len(got) != tt.wantCount {
				t.Errorf("extractParagraphs() count = %d, want %d", len(got), tt.wantCount)
			}
			if tt.wantCount > 0 && got[0] != tt.wantFirst {
				t.Errorf("extractParagraphs()[0] = %q, want %q", got[0], tt.wantFirst)
			}
		})
	}
}

func TestIsBoilerplate(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"Cookie settings. More text here...", true},
		{"We use cookies to improve your experience.", true},
		{"Sign up for our newsletter!", true},
		{"Subscribe to get updates...", true},
		{"This is a regular article about technology.", false},
		{"AI research shows promising results.", false},
	}

	for _, tt := range tests {
		t.Run(tt.text[:min(30, len(tt.text))], func(t *testing.T) {
			got := isBoilerplate(tt.text)
			if got != tt.want {
				t.Errorf("isBoilerplate(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestFindCleanStart(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "skips cookie notice",
			input: "Cookie settings enabled. This is the real article content about AI.",
			want:  "This is the real article content about AI.",
		},
		{
			name:  "skips subscribe prompt",
			input: "Subscribe now for updates. Real content starts here about technology.",
			want:  "Real content starts here about technology.",
		},
		{
			name:  "keeps clean text",
			input: "This is a great article about machine learning techniques.",
			want:  "This is a great article about machine learning techniques.",
		},
		{
			name:  "handles multiple boilerplate sentences",
			input: "Cookie notice. Sign up now. Real article content about programming.",
			want:  "Real article content about programming.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findCleanStart(tt.input)
			if !strings.HasPrefix(got, tt.want) && got != tt.want {
				t.Errorf("findCleanStart() = %q, want prefix %q", got, tt.want)
			}
		})
	}
}
