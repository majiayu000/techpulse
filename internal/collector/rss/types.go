// Package rss provides a collector for RSS feeds.
package rss

import (
	"encoding/xml"
	"net/url"
	"strings"
)

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
	XMLBase string      `xml:"http://www.w3.org/XML/1998/namespace base,attr"`
	Title   AtomText    `xml:"title"`
	Entries []AtomEntry `xml:"entry"`
}

// AtomEntry represents a single entry in an Atom feed.
type AtomEntry struct {
	XMLBase    string         `xml:"http://www.w3.org/XML/1998/namespace base,attr"`
	ID         string         `xml:"id"`
	Title      AtomText       `xml:"title"`
	Links      []AtomLink     `xml:"link"`
	Published  string         `xml:"published"`
	Updated    string         `xml:"updated"`
	Summary    AtomText       `xml:"summary"`
	Content    AtomText       `xml:"content"`
	Authors    []AtomPerson   `xml:"author"`
	Categories []AtomCategory `xml:"category"`
}

// AtomText captures an Atom text construct (title, summary, or content).
// Plain text/html types store character data; type="xhtml" nests markup,
// which encoding/xml would leave empty if decoded into a bare string.
type AtomText struct {
	Type  string
	Value string
}

// UnmarshalXML decodes Atom text constructs, including nested XHTML bodies.
func (t *AtomText) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Local == "type" {
			t.Type = attr.Value
			break
		}
	}

	var b strings.Builder
	depth := 1
	for depth > 0 {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		case xml.CharData:
			b.Write(v)
		}
	}
	t.Value = strings.TrimSpace(b.String())
	return nil
}

// AtomLink represents a link element in an Atom feed.
type AtomLink struct {
	Rel     string `xml:"rel,attr"`
	Href    string `xml:"href,attr"`
	XMLBase string `xml:"http://www.w3.org/XML/1998/namespace base,attr"`
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
// feedURL is the retrieval URL used as the outermost base for relative links;
// feedBase is the feed-level xml:base when present.
func (e AtomEntry) toItem(feedURL, feedBase string) Item {
	item := Item{
		Title:       e.Title.Value,
		Description: e.Summary.Value,
		GUID:        e.ID,
		PubDate:     e.Published,
	}
	if item.Description == "" {
		item.Description = e.Content.Value
	}
	if item.PubDate == "" {
		item.PubDate = e.Updated
	}

	// Prefer the alternate link (Atom's default rel when the attribute is
	// absent); ignore self/enclosure/replies links. Relative hrefs resolve
	// against the composed xml:base chain (link → entry → feed → retrieval URL),
	// so relative inner bases inherit from outer absolute ones.
	for _, l := range e.Links {
		href := resolveAtomHref(l.Href, l.XMLBase, e.XMLBase, feedBase, feedURL)
		if l.Rel == "alternate" {
			item.Link = href
			break
		}
		if l.Rel == "" && item.Link == "" {
			item.Link = href
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

// resolveAtomHref turns a possibly-relative Atom link href into an absolute
// URL. bases are ordered most-specific-first (link xml:base, entry xml:base,
// feed xml:base, feed retrieval URL). Relative xml:base values are composed
// against outer bases per XML Base before the href is resolved. Absolute
// hrefs are returned unchanged. When no absolute base is available the
// original href is kept.
func resolveAtomHref(href string, bases ...string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	ref, err := url.Parse(href)
	if err != nil {
		return href
	}
	if ref.IsAbs() {
		return ref.String()
	}

	// Walk outermost → innermost so relative inner xml:base values inherit
	// from an absolute outer base (e.g. feed https://ex/base/ + entry posts/
	// + href 1 → https://ex/base/posts/1).
	var effective *url.URL
	for i := len(bases) - 1; i >= 0; i-- {
		base := strings.TrimSpace(bases[i])
		if base == "" {
			continue
		}
		baseURL, err := url.Parse(base)
		if err != nil {
			continue
		}
		if effective == nil {
			if !baseURL.IsAbs() {
				continue
			}
			effective = baseURL
			continue
		}
		effective = effective.ResolveReference(baseURL)
	}
	if effective == nil {
		return href
	}
	return effective.ResolveReference(ref).String()
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
