package extractor

import "testing"

func TestExtractTextFromHTML(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "simple paragraph",
			html: "<p>Hello World</p>",
			want: "Hello World",
		},
		{
			name: "removes script tags",
			html: "<p>Before</p><script>alert('hi')</script><p>After</p>",
			want: "Before After",
		},
		{
			name: "removes style tags",
			html: "<style>.foo{color:red}</style><p>Content</p>",
			want: "Content",
		},
		{
			name: "decodes entities",
			html: "<p>Tom &amp; Jerry &mdash; Best Friends</p>",
			want: "Tom & Jerry - Best Friends",
		},
		{
			name: "decodes escaped entities exactly once",
			html: "<p>Use &amp;lt;tag&amp;gt; carefully in prose</p>",
			want: "Use &lt;tag&gt; carefully in prose",
		},
		{
			name: "normalizes whitespace",
			html: "<p>  Multiple   spaces   here  </p>",
			want: "Multiple spaces here",
		},
		{
			name: "nested tags",
			html: "<div><p><strong>Bold</strong> and <em>italic</em></p></div>",
			want: "Bold and italic",
		},
		{
			name: "removes comments",
			html: "<p>Before</p><!-- comment --><p>After</p>",
			want: "Before After",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTextFromHTML(tt.html)
			if got != tt.want {
				t.Errorf("extractTextFromHTML() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDecodeHTMLEntities(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"&amp;", "&"},
		{"&lt;", "<"},
		{"&gt;", ">"},
		{"&quot;", "\""},
		{"&nbsp;", " "},
		{"&mdash;", "-"},
		{"&ndash;", "-"},
		{"mixed &amp; text", "mixed & text"},
		// Single ordered pass: escaped entities decode exactly one level.
		{"&amp;lt;", "&lt;"},
		{"&amp;amp;", "&amp;"},
		{"&hellip;", "..."},
		{"&#39;", "'"},
		// Unknown entities pass through instead of being silently deleted.
		{"&eacute;", "&eacute;"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := decodeHTMLEntities(tt.input)
			if got != tt.want {
				t.Errorf("decodeHTMLEntities(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCleanText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "normalizes whitespace",
			input: "  multiple   spaces  ",
			want:  "multiple spaces",
		},
		{
			name:  "keeps normal text",
			input: "This is normal content.",
			want:  "This is normal content.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanText(tt.input)
			if got != tt.want {
				t.Errorf("cleanText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMin(t *testing.T) {
	if min(5, 10) != 5 {
		t.Error("min(5, 10) should be 5")
	}
	if min(10, 5) != 5 {
		t.Error("min(10, 5) should be 5")
	}
	if min(5, 5) != 5 {
		t.Error("min(5, 5) should be 5")
	}
}
