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
		{
			name: "keeps download-link class (substring of ad keyword)",
			html: `<div class="download-link">Get the release notes.</div>`,
			want: `<div class="download-link">Get the release notes.</div>`,
		},
		{
			name: "keeps read-more-btn class",
			html: `<span class="read-more-btn">Continue the tour</span>`,
			want: `<span class="read-more-btn">Continue the tour</span>`,
		},
		{
			name: "keeps upload-form class",
			html: `<div class="upload-form">Drop your files here.</div>`,
			want: `<div class="upload-form">Drop your files here.</div>`,
		},
		{
			name: "removes element with ad class through its own close tag",
			html: `<div class="ad-banner">Buy stuff now</div><p>Story</p>`,
			want: ` <p>Story</p>`,
		},
		{
			name: "removes element with ad id attribute",
			html: `<div id="cookie-consent">Accept cookies?</div><p>Story</p>`,
			want: ` <p>Story</p>`,
		},
		{
			name: "closes at matching tag, not first closing tag of any element",
			html: `<div class="share"><p>Share us</p></div><p>Keep me</p>`,
			want: ` <p>Keep me</p>`,
		},
		{
			name: "removes ad element nested in innocent parent",
			html: `<div><div class="promo-box">Deal</div><p>Keep</p></div>`,
			want: `<div> <p>Keep</p></div>`,
		},
		{
			name: "checks later attributes too when first class attr is innocent",
			html: `<div class="story" id="ad-slot">Ad body</div><p>Story</p>`,
			want: ` <p>Story</p>`,
		},
		{
			name: "keeps unclosed ad element instead of truncating document",
			html: `<p>Before</p><div class="ad-banner">Unclosed ad and more text`,
			want: `<p>Before</p><div class="ad-banner">Unclosed ad and more text`,
		},
		{
			name: "removes self-closing ad tag only",
			html: `<div class="ads"/><p>Story</p>`,
			want: ` <p>Story</p>`,
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

func TestAdTokenHasKeyword(t *testing.T) {
	tests := []struct {
		tok  string
		kw   string
		want bool
	}{
		{"ad-banner", "ad", true},
		{"ads-sidebar", "ads", true},
		{"data-ad-slot", "ad", true},
		{"top-ad", "ad", true},
		{"top_ads", "ads", true},
		{"download-link", "ad", false},
		{"upload-form", "ad", false},
		{"read-more-btn", "ad", false},
		{"breadcrumb", "ad", false},
		{"head", "ad", false},
		{"banner-image", "banner", true},
		{"banners", "banner", false},
		{"social-links", "social", true},
		{"share-promo", "share", true},
		{"share-promo", "promo", true},
	}

	for _, tt := range tests {
		t.Run(tt.tok+"~"+tt.kw, func(t *testing.T) {
			got := adTokenHasKeyword(tt.tok, tt.kw)
			if got != tt.want {
				t.Errorf("adTokenHasKeyword(%q, %q) = %v, want %v", tt.tok, tt.kw, got, tt.want)
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

func TestExtractMainContentNestedArticle(t *testing.T) {
	tests := []struct {
		name       string
		html       string
		wantSubstr []string // all must appear in extracted content
	}{
		{
			name: "nested article teaser does not truncate body",
			html: `<article><h1>Listing</h1>` +
				`<article class="teaser"><p>Teaser summary text</p></article>` +
				`<p>Main story body continues here</p></article>`,
			wantSubstr: []string{"Teaser summary text", "Main story body continues here"},
		},
		{
			name:       "sibling articles return first",
			html:       `<article>First story</article><article>Second story</article>`,
			wantSubstr: []string{"First story"},
		},
		{
			name:       "unclosed article falls through to main",
			html:       `<article>Broken <main>Fallback content</main>`,
			wantSubstr: []string{"Fallback content"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractMainContent(tt.html)
			for _, sub := range tt.wantSubstr {
				if !strings.Contains(got, sub) {
					t.Errorf("extractMainContent() = %q, want to contain %q", got, sub)
				}
			}
		})
	}
}

// TestExtractArticleContentPreservesUTF8Offsets ensures length-changing
// Unicode case folds (İ→i) do not corrupt slices taken from the original HTML.
func TestExtractArticleContentPreservesUTF8Offsets(t *testing.T) {
	const body = "İstanbul tech briefing with length-changing case folds"
	html := "<ARTICLE>" + body + "</ARTICLE>"
	got := extractArticleContent(html)
	if got != body {
		t.Fatalf("extractArticleContent() = %q, want %q", got, body)
	}
}

// TestRemoveAdElementsPreservesUTF8Offsets ensures ad removal still finds
// ASCII tags when preceding text contains length-changing case folds.
func TestRemoveAdElementsPreservesUTF8Offsets(t *testing.T) {
	html := `<p>İstanbul</p><div class="ad-banner">Buy</div><p>Keep</p>`
	got := removeBoilerplate(html)
	if !strings.Contains(got, "İstanbul") {
		t.Errorf("removeBoilerplate dropped UTF-8 content: %q", got)
	}
	if !strings.Contains(got, "Keep") {
		t.Errorf("removeBoilerplate dropped trailing content: %q", got)
	}
	if strings.Contains(got, "Buy") {
		t.Errorf("removeBoilerplate left ad body: %q", got)
	}
}

func TestExtractParagraphs(t *testing.T) {
	tests := []struct {
		name      string
		html      string
		wantCount int
		wantFirst string
	}{
		{
			name:      "extracts long paragraphs",
			html:      "<p>This is a paragraph with more than fifty characters of content here.</p>",
			wantCount: 1,
			wantFirst: "This is a paragraph with more than fifty characters of content here.",
		},
		{
			name:      "ignores short paragraphs",
			html:      "<p>Short</p><p>This is a longer paragraph that has enough content.</p>",
			wantCount: 1,
			wantFirst: "This is a longer paragraph that has enough content.",
		},
		{
			name:      "handles nested tags",
			html:      "<p>This paragraph has <strong>bold</strong> text and more content to make it long enough.</p>",
			wantCount: 1,
			wantFirst: "This paragraph has bold text and more content to make it long enough.",
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
		// Phrases must match at the start at a word boundary, not anywhere.
		{"Developers can now sign in with passkeys.", false},
		{"The menu offers three viewing modes for readers.", false},
		{"Registration is open for new contributors.", false},
		{"Readers share on social media every day.", false},
		{"Menu navigation help is available.", true},
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
		{
			name:  "keeps sentences that merely contain boilerplate phrases",
			input: "Developers can now sign in with passkeys. The rest of this story explains how.",
			want:  "Developers can now sign in with passkeys. The rest of this story explains how.",
		},
		{
			name:  "keeps sentence mentioning menu mid-sentence",
			input: "The menu of supported formats keeps growing every release.",
			want:  "The menu of supported formats keeps growing every release.",
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
