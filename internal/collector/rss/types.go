// Package rss provides a collector for RSS feeds.
package rss

import "encoding/xml"

// Feed represents an RSS feed.
type Feed struct {
	XMLName xml.Name `xml:"rss"`
	Channel Channel  `xml:"channel"`
}

// Channel represents the channel element in an RSS feed.
type Channel struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Items       []Item `xml:"item"`
}

// Item represents a single item in an RSS feed.
type Item struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	Description string   `xml:"description"`
	PubDate     string   `xml:"pubDate"`
	Author      string   `xml:"author"`
	Creator     string   `xml:"creator"` // dc:creator
	GUID        string   `xml:"guid"`
	Categories  []string `xml:"category"`
}

// AtomFeed represents an Atom feed document (root element <feed>).
type AtomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Title   string      `xml:"title"`
	Entries []AtomEntry `xml:"entry"`
}

// AtomEntry represents a single entry in an Atom feed.
type AtomEntry struct {
	ID         string         `xml:"id"`
	Title      string         `xml:"title"`
	Links      []AtomLink     `xml:"link"`
	Published  string         `xml:"published"`
	Updated    string         `xml:"updated"`
	Summary    string         `xml:"summary"`
	Content    string         `xml:"content"`
	Authors    []AtomPerson   `xml:"author"`
	Categories []AtomCategory `xml:"category"`
}

// AtomLink represents a link element in an Atom feed.
type AtomLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
}

// AtomPerson represents an author or contributor in an Atom feed.
type AtomPerson struct {
	Name  string `xml:"name"`
	Email string `xml:"email"`
}

// AtomCategory represents a category element in an Atom feed.
type AtomCategory struct {
	Term  string `xml:"term,attr"`
	Label string `xml:"label,attr"`
}

// toItem normalizes an Atom entry into the shared Item shape so that
// article conversion and time filtering behave identically for both formats.
func (e AtomEntry) toItem() Item {
	item := Item{
		Title:       e.Title,
		Description: e.Summary,
		GUID:        e.ID,
		PubDate:     e.Published,
	}
	if item.Description == "" {
		item.Description = e.Content
	}
	if item.PubDate == "" {
		item.PubDate = e.Updated
	}

	// Prefer the alternate link (Atom's default rel when the attribute is
	// absent); ignore self/enclosure/replies links.
	for _, l := range e.Links {
		if l.Rel == "alternate" {
			item.Link = l.Href
			break
		}
		if l.Rel == "" && item.Link == "" {
			item.Link = l.Href
		}
	}

	for _, a := range e.Authors {
		if a.Name != "" {
			item.Creator = a.Name
			break
		}
	}

	for _, cat := range e.Categories {
		switch {
		case cat.Term != "":
			item.Categories = append(item.Categories, cat.Term)
		case cat.Label != "":
			item.Categories = append(item.Categories, cat.Label)
		}
	}

	return item
}

// Source represents a configured RSS source.
type Source struct {
	Name string
	URL  string
}

// DefaultSources provides a list of default tech RSS feeds.
var DefaultSources = []Source{
	{Name: "TechCrunch", URL: "https://techcrunch.com/feed/"},
	{Name: "Ars Technica", URL: "https://feeds.arstechnica.com/arstechnica/index"},
	{Name: "The Verge", URL: "https://www.theverge.com/rss/index.xml"},
	{Name: "Wired", URL: "https://www.wired.com/feed/rss"},
	{Name: "MIT Technology Review", URL: "https://www.technologyreview.com/feed"},
}
