package summarizer

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/majiayu000/techpulse/internal/collector"
	"github.com/majiayu000/techpulse/internal/collector/github"
	"github.com/majiayu000/techpulse/internal/collector/rss"
	"github.com/majiayu000/techpulse/internal/filter"
	"github.com/majiayu000/techpulse/internal/httpclient"
)

func TestEnrich_CollectorSourceWeights(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/trending" {
			fmt.Fprint(w, `<article class="Box-row"><h2><a href="/example/project">example/project</a></h2></article>`)
			return
		}
		fmt.Fprintf(w, `<rss><channel><item><title>%s</title><link>https://example.com%s</link></item></channel></rss>`, r.URL.Path, r.URL.Path)
	}))
	defer server.Close()

	ctx := context.Background()
	articles, err := github.NewWithBaseURL(server.URL).Collect(ctx, collector.Options{})
	if err != nil || len(articles) != 1 {
		t.Fatalf("GitHub Collect() = %d articles, %v; want one article", len(articles), err)
	}
	if articles[0].Source != "github" {
		t.Fatalf("GitHub article source = %q, want github", articles[0].Source)
	}
	wantImportance := map[string]float64{articles[0].Title: 5.8}
	feedNames := []string{
		rss.DefaultSources[0].Name,
		rss.DefaultSources[1].Name,
		rss.DefaultSources[2].Name,
		rss.DefaultSources[3].Name,
		rss.DefaultSources[4].Name,
		"  aRs TeChNiCa  ",
		"Unknown Publication",
		"",
	}
	feedImportance := []float64{5.6, 5.6, 5.4, 5.6, 5.8, 5.6, 5.0, 5.0}
	sources := make([]rss.Source, len(feedNames))
	for i, name := range feedNames {
		path := fmt.Sprintf("/feed/%d", i)
		sources[i] = rss.Source{Name: name, URL: server.URL + path}
		wantImportance[path] = feedImportance[i]
	}
	feedArticles, err := rss.New(sources, httpclient.WithAllowPrivateHosts(true)).Collect(ctx, collector.Options{})
	if err != nil || len(feedArticles) != len(sources) {
		t.Fatalf("RSS Collect() = %d articles, %v; want %d", len(feedArticles), err, len(sources))
	}
	for i, a := range feedArticles {
		if a.Source != "rss" || a.Metadata["feed_name"] != feedNames[i] {
			t.Fatalf("RSS article %d source/metadata = %q/%v", i, a.Source, a.Metadata)
		}
	}
	articles = append(articles, feedArticles...)
	articles = append(articles, collector.Article{Title: "RSS without metadata", Source: "rss"})
	wantImportance["RSS without metadata"] = 5.0

	s := NewBasicSummarizer()
	enriched, err := s.Enrich(ctx, filter.ToFiltered(articles))
	if err != nil || len(enriched) != len(articles) {
		t.Fatalf("Enrich() = %d articles, %v; want %d", len(enriched), err, len(articles))
	}
	for _, a := range enriched {
		if want := wantImportance[a.Title]; math.Abs(a.Importance-want) > 0.0001 {
			t.Errorf("%q (%s, %v) importance = %v, want %v", a.Title, a.Source, a.Metadata, a.Importance, want)
		}
	}
	if enriched[0].Source != "github" || enriched[1].Metadata["feed_name"] != "MIT Technology Review" {
		t.Errorf("highest-weight stories = %q, %q; want GitHub then MIT Technology Review", enriched[0].Title, enriched[1].Title)
	}
	report, err := s.GenerateReport(ctx, enriched)
	if err != nil {
		t.Fatalf("GenerateReport() error = %v", err)
	}
	if report.TopStories[0].Source != "github" || report.Stats.BySource["rss"] != len(feedArticles)+1 {
		t.Errorf("report does not retain weighted order and RSS source grouping: %+v", report)
	}
}

func TestNewBasicSummarizer(t *testing.T) {
	s := NewBasicSummarizer()
	if s == nil {
		t.Error("expected non-nil summarizer")
	}
}

func TestEnrich_Empty(t *testing.T) {
	s := NewBasicSummarizer()
	result, err := s.Enrich(context.Background(), nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d", len(result))
	}
}

func TestEnrich_SingleArticle(t *testing.T) {
	s := NewBasicSummarizer()
	articles := []filter.FilteredArticle{
		{
			Article: collector.Article{
				ID:       "1",
				Title:    "AI News",
				Score:    100,
				Comments: 50,
			},
			MatchedKeywords: []string{"AI"},
		},
	}

	result, err := s.Enrich(context.Background(), articles)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	// Base 5.0 + score>50: 0.5 + 1 keyword: 0.5 = 6.0
	if result[0].Importance != 6.0 {
		t.Errorf("expected importance 6.0, got %f", result[0].Importance)
	}
}

func TestEnrich_Sorting(t *testing.T) {
	s := NewBasicSummarizer()
	articles := []filter.FilteredArticle{
		{Article: collector.Article{ID: "1", Score: 10}},
		{Article: collector.Article{ID: "2", Score: 600}},
		{Article: collector.Article{ID: "3", Score: 200}},
	}

	result, err := s.Enrich(context.Background(), articles)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result[0].ID != "2" {
		t.Errorf("expected ID '2' first, got %s", result[0].ID)
	}
	if result[1].ID != "3" {
		t.Errorf("expected ID '3' second, got %s", result[1].ID)
	}
	if result[2].ID != "1" {
		t.Errorf("expected ID '1' third, got %s", result[2].ID)
	}
}

func TestEnrich_TieBreakDeterministic(t *testing.T) {
	s := NewBasicSummarizer()

	// All four articles score identically (no score/comments/keywords and
	// sources matching no weight rule), so ordering is decided purely by the
	// deterministic tie-breakers: source, then title.
	articles := []filter.FilteredArticle{
		{Article: collector.Article{ID: "1", Source: "zeta-blog", Title: "Beta Post"}},
		{Article: collector.Article{ID: "2", Source: "alpha-blog", Title: "Zulu Post"}},
		{Article: collector.Article{ID: "3", Source: "alpha-blog", Title: "Alpha Post"}},
		{Article: collector.Article{ID: "4", Source: "mike-blog", Title: "Yankee Post"}},
	}

	tests := []struct {
		name  string
		input []filter.FilteredArticle
	}{
		{"as declared", articles},
		{"reversed", []filter.FilteredArticle{articles[3], articles[2], articles[1], articles[0]}},
		{"rotated", []filter.FilteredArticle{articles[2], articles[0], articles[3], articles[1]}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := s.Enrich(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, a := range result {
				if a.Importance != 5.0 {
					t.Errorf("article %s importance = %v, want 5.0 (test requires equal scores)", a.ID, a.Importance)
				}
			}

			got := make([]string, 0, len(result))
			for _, a := range result {
				got = append(got, a.ID)
			}
			want := []string{"3", "2", "4", "1"} // source asc, then title asc
			if !reflect.DeepEqual(got, want) {
				t.Errorf("tie order = %v, want %v", got, want)
			}
		})
	}
}

func TestEnrich_MultipleArticles(t *testing.T) {
	s := NewBasicSummarizer()
	articles := []filter.FilteredArticle{
		{
			Article:         collector.Article{ID: "1", Score: 50},
			MatchedKeywords: []string{"AI"},
		},
		{
			Article:         collector.Article{ID: "2", Score: 100, Comments: 60},
			MatchedKeywords: []string{"GPT", "LLM"},
		},
		{
			Article: collector.Article{ID: "3", Score: 30},
		},
	}

	result, err := s.Enrich(context.Background(), articles)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result))
	}

	// Verify sorted by importance
	for i := 0; i < len(result)-1; i++ {
		if result[i].Importance < result[i+1].Importance {
			t.Errorf("expected descending order, got %f < %f at index %d",
				result[i].Importance, result[i+1].Importance, i)
		}
	}
}

func TestEnrich_PreservesOriginalData(t *testing.T) {
	s := NewBasicSummarizer()
	articles := []filter.FilteredArticle{
		{
			Article: collector.Article{
				ID:     "test-id",
				Title:  "Test Title",
				URL:    "https://example.com",
				Source: "test-source",
			},
			MatchedKeywords: []string{"AI"},
		},
	}

	result, err := s.Enrich(context.Background(), articles)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result[0].ID != "test-id" {
		t.Errorf("expected ID 'test-id', got %s", result[0].ID)
	}
	if result[0].Title != "Test Title" {
		t.Errorf("expected title 'Test Title', got %s", result[0].Title)
	}
	if result[0].URL != "https://example.com" {
		t.Errorf("expected URL 'https://example.com', got %s", result[0].URL)
	}
	if result[0].Source != "test-source" {
		t.Errorf("expected source 'test-source', got %s", result[0].Source)
	}
	if len(result[0].MatchedKeywords) != 1 {
		t.Errorf("expected 1 keyword, got %d", len(result[0].MatchedKeywords))
	}
}
