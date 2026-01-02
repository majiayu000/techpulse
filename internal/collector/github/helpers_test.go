package github

import (
	"reflect"
	"testing"
)

func TestSanitizeID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"owner/repo", "owner_repo"},
		{"some-org/some-repo", "some-org_some-repo"},
		{"simple", "simple"},
		{"a/b/c", "a_b_c"},
	}

	for _, tt := range tests {
		got := sanitizeID(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizeID(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestExtractOwner(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"owner/repo", "owner"},
		{"anthropic/claude", "anthropic"},
		{"just-owner", "just-owner"},
		{"", ""},
	}

	for _, tt := range tests {
		got := extractOwner(tt.input)
		if got != tt.expected {
			t.Errorf("extractOwner(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestBuildTags(t *testing.T) {
	tests := []struct {
		repo     Repository
		expected []string
	}{
		{
			Repository{Language: "Go"},
			[]string{"Go", "trending"},
		},
		{
			Repository{Language: "", Topics: []string{"ai", "ml"}},
			[]string{"trending", "ai", "ml"},
		},
		{
			Repository{Language: "Python", Topics: []string{"data-science"}},
			[]string{"Python", "trending", "data-science"},
		},
		{
			Repository{},
			[]string{"trending"},
		},
	}

	for i, tt := range tests {
		got := buildTags(tt.repo)
		if !reflect.DeepEqual(got, tt.expected) {
			t.Errorf("test %d: buildTags() = %v, want %v", i, got, tt.expected)
		}
	}
}
