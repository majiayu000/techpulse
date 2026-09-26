// Package lobsters provides a collector for Lobsters (lobste.rs) stories.
package lobsters

import "time"

// Story represents a story from Lobsters.
type Story struct {
	ShortID          string    `json:"short_id"`
	Title            string    `json:"title"`
	URL              string    `json:"url"`
	CreatedAt        time.Time `json:"created_at"`
	Score            int       `json:"score"`
	Flags            int       `json:"flags"`
	CommentCount     int       `json:"comment_count"`
	SubmitterUser    string    `json:"submitter_user"`
	UserIsAuthor     bool      `json:"user_is_author"`
	Description      string    `json:"description"`
	DescriptionPlain string    `json:"description_plain"`
	Tags             []string  `json:"tags"`
	ShortIDURL       string    `json:"short_id_url"`
	CommentsURL      string    `json:"comments_url"`
}

// FeedType represents the type of Lobsters feed.
type FeedType string

const (
	// FeedHottest returns the hottest stories.
	FeedHottest FeedType = "hottest"
	// FeedNewest returns the newest stories.
	FeedNewest FeedType = "newest"
	// FeedActive returns the most active stories (by comments).
	FeedActive FeedType = "active"
)

// ValidFeedTypes contains all valid feed types.
var ValidFeedTypes = map[FeedType]bool{
	FeedHottest: true,
	FeedNewest:  true,
	FeedActive:  true,
}

// DefaultFeedType is the default feed to fetch.
const DefaultFeedType = FeedHottest

// DefaultBaseURL is the Lobsters API base URL.
const DefaultBaseURL = "https://lobste.rs"
