// Package reddit provides a collector for Reddit posts.
package reddit

import "time"

// Post represents a Reddit post.
type Post struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	Permalink   string  `json:"permalink"`
	Author      string  `json:"author"`
	Selftext    string  `json:"selftext"`
	Score       int     `json:"score"`
	NumComments int     `json:"num_comments"`
	Created     float64 `json:"created_utc"`
	Subreddit   string  `json:"subreddit"`
	Flair       string  `json:"link_flair_text"`
	Domain      string  `json:"domain"`
	IsSelf      bool    `json:"is_self"`
}

// CreatedTime returns the post creation time.
func (p *Post) CreatedTime() time.Time {
	return time.Unix(int64(p.Created), 0)
}

// ListingResponse is the Reddit JSON API response for a listing.
type ListingResponse struct {
	Data struct {
		Children []struct {
			Data Post `json:"data"`
		} `json:"children"`
		After  string `json:"after"`
		Before string `json:"before"`
	} `json:"data"`
}

// Subreddit represents a subreddit configuration.
type Subreddit struct {
	Name     string
	Category string // e.g., "ai", "programming", "tech"
}

// DefaultSubreddits contains the default subreddits to collect from.
var DefaultSubreddits = []Subreddit{
	{Name: "MachineLearning", Category: "ai"},
	{Name: "artificial", Category: "ai"},
	{Name: "programming", Category: "programming"},
	{Name: "golang", Category: "programming"},
	{Name: "rust", Category: "programming"},
}

// SortType represents how posts are sorted.
type SortType string

const (
	SortHot SortType = "hot"
	SortNew SortType = "new"
	SortTop SortType = "top"
)

// ValidSortTypes contains all valid sort types.
var ValidSortTypes = map[SortType]bool{
	SortHot: true,
	SortNew: true,
	SortTop: true,
}

// DefaultSortType is the default sort order.
const DefaultSortType = SortHot

// BaseURL is the Reddit JSON API base URL.
const BaseURL = "https://www.reddit.com"
