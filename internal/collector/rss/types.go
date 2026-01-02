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
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Author      string `xml:"author"`
	Creator     string `xml:"creator"` // dc:creator
	GUID        string `xml:"guid"`
	Categories  []string `xml:"category"`
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
