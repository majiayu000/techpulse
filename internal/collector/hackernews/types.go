// Package hackernews provides a collector for Hacker News.
package hackernews

// Item represents a Hacker News item (story, comment, etc.)
type Item struct {
	ID          int    `json:"id"`
	Type        string `json:"type"`
	By          string `json:"by"`
	Time        int64  `json:"time"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Text        string `json:"text"`
	Score       int    `json:"score"`
	Descendants int    `json:"descendants"` // Comment count
	Kids        []int  `json:"kids"`        // Child comment IDs
	Parent      int    `json:"parent"`
	Dead        bool   `json:"dead"`
	Deleted     bool   `json:"deleted"`
}

// ValidCategories lists all valid HN story categories.
var ValidCategories = map[string]bool{
	"top":  true,
	"new":  true,
	"best": true,
	"ask":  true,
	"show": true,
	"job":  true,
}
