package booksearch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const googleBody = `{"items":[
  {"id":"abc123","volumeInfo":{"title":" Dune ","authors":["Frank Herbert","Brian Herbert"],
    "description":"  Spice. ","publishedDate":"1965-08",
    "imageLinks":{"thumbnail":"http://books.google.com/books/content?id=abc123&printsec=frontcover&img=1&zoom=1&edge=curl&source=gbs_api"}}},
  {"id":"noauthor","volumeInfo":{"title":"Anonymous Tales"}},
  {"id":"badcover","volumeInfo":{"title":"Odd","imageLinks":{"thumbnail":"javascript:alert(1)"}}},
  {"id":"notitle","volumeInfo":{"authors":["Nobody"]}}
]}`

// upstream is a fake catalogue endpoint that counts requests and records the
// last query string.
type upstream struct {
	*httptest.Server
	calls atomic.Int32
	query atomic.Value // url.Values
}

func newUpstream(t *testing.T, status int, body string) *upstream {
	t.Helper()
	u := &upstream{}
	u.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u.calls.Add(1)
		u.query.Store(r.URL.Query())
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(u.Close)
	return u
}

func TestGoogleMapsAndNormalizes(t *testing.T) {
	up := newUpstream(t, 200, googleBody)
	g := NewGoogle(NewHTTPClient(), up.URL, "")

	got, err := g.Search(context.Background(), "dune", 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 { // the title-less volume is dropped
		t.Fatalf("got %d results: %+v", len(got), got)
	}

	dune := got[0]
	if dune.ExternalID != "abc123" || dune.Name != "Dune" || dune.Author != "Frank Herbert, Brian Herbert" {
		t.Errorf("dune = %+v", dune)
	}
	if dune.Description == nil || *dune.Description != "Spice." {
		t.Errorf("description = %v", dune.Description)
	}
	if dune.PublishedYear == nil || *dune.PublishedYear != 1965 {
		t.Errorf("year = %v", dune.PublishedYear)
	}
	if dune.CoverURL == nil || !strings.HasPrefix(*dune.CoverURL, "https://books.google.com/") ||
		strings.Contains(*dune.CoverURL, "edge") || !strings.Contains(*dune.CoverURL, "zoom=1") {
		t.Errorf("cover = %v", dune.CoverURL)
	}

	anon := got[1]
	if anon.Author != "Unknown author" || anon.CoverURL != nil || anon.Description != nil || anon.PublishedYear != nil {
		t.Errorf("anon = %+v", anon)
	}
	if got[2].CoverURL != nil {
		t.Errorf("non-http cover must be dropped: %v", *got[2].CoverURL)
	}
}

func TestGoogleRequestParameters(t *testing.T) {
	up := newUpstream(t, 200, `{}`)

	if _, err := NewGoogle(NewHTTPClient(), up.URL, "").Search(context.Background(), "a&b=c d", 5); err != nil {
		t.Fatal(err)
	}
	q := up.query.Load().(url.Values)
	if q.Get("q") != "a&b=c d" || q.Get("maxResults") != "5" || q.Get("printType") != "books" ||
		q.Get("fields") == "" || q.Has("key") || q.Has("langRestrict") {
		t.Errorf("query = %v", q)
	}

	if _, err := NewGoogle(NewHTTPClient(), up.URL, "k3y").Search(context.Background(), "dune", 5); err != nil {
		t.Fatal(err)
	}
	if k := up.query.Load().(url.Values).Get("key"); k != "k3y" {
		t.Errorf("key = %q", k)
	}
}

func TestGoogleErrorsDoNotLeakTheKey(t *testing.T) {
	up := newUpstream(t, 429, `{}`)
	_, err := NewGoogle(NewHTTPClient(), up.URL, "s3cr3t-key").Search(context.Background(), "dune", 5)
	if err == nil || strings.Contains(err.Error(), "s3cr3t-key") || !strings.Contains(err.Error(), "429") {
		t.Fatalf("err = %v", err)
	}

	// Transport failures (here: connection refused) embed the URL in net/http errors.
	dead := httptest.NewServer(http.NotFoundHandler())
	deadURL := dead.URL
	dead.Close()
	_, err = NewGoogle(NewHTTPClient(), deadURL, "s3cr3t-key").Search(context.Background(), "dune", 5)
	if err == nil || strings.Contains(err.Error(), "s3cr3t-key") || strings.Contains(err.Error(), deadURL) {
		t.Fatalf("err leaks the URL: %v", err)
	}
}

func TestGoogleRejectsOversizedBody(t *testing.T) {
	huge := `{"items":[{"id":"x","volumeInfo":{"title":"` + strings.Repeat("a", maxResponseBytes) + `"}}]}`
	up := newUpstream(t, 200, huge)
	if _, err := NewGoogle(NewHTTPClient(), up.URL, "").Search(context.Background(), "dune", 5); err == nil {
		t.Fatal("expected an error for an oversized body")
	}
}

func TestOpenLibraryMaps(t *testing.T) {
	up := newUpstream(t, 200, `{"docs":[
	  {"key":"/works/OL1W","title":"Dune","author_name":["Frank Herbert"],"first_publish_year":1965,"cover_i":42},
	  {"key":"/works/OL2W","title":"Bare"},
	  {"key":"/works/OL3W","title":""}]}`)

	got, err := NewOpenLibrary(NewHTTPClient(), up.URL).Search(context.Background(), "dune", 2)
	if err != nil || len(got) != 2 {
		t.Fatalf("got %+v, err %v", got, err)
	}
	if got[0].ExternalID != "/works/OL1W" || got[0].Author != "Frank Herbert" ||
		*got[0].CoverURL != "https://covers.openlibrary.org/b/id/42-M.jpg" || *got[0].PublishedYear != 1965 {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[1].Author != "Unknown author" || got[1].CoverURL != nil || got[1].PublishedYear != nil {
		t.Errorf("got[1] = %+v", got[1])
	}
}

// stub is a Provider returning canned answers.
type stub struct {
	name    string
	results []Result
	err     error
	calls   int
}

func (s *stub) Name() string { return s.name }
func (s *stub) Search(context.Context, string, int) ([]Result, error) {
	s.calls++
	return s.results, s.err
}

var hit = []Result{{ExternalID: "1", Name: "Dune", Author: "FH"}}

func TestServiceFallback(t *testing.T) {
	boom := errors.New("boom")
	for _, tc := range []struct {
		name         string
		primary      *stub
		fallback     *stub
		want         int
		wantErr      error
		wantFallback int
	}{
		{"primary answers", &stub{results: hit}, &stub{}, 1, nil, 0},
		{"primary fails", &stub{err: boom}, &stub{results: hit}, 1, nil, 1},
		{"primary empty", &stub{}, &stub{results: hit}, 1, nil, 1},
		{"both fail", &stub{err: boom}, &stub{err: boom}, 0, ErrUnavailable, 1},
		{"primary empty, fallback fails", &stub{}, &stub{err: boom}, 0, nil, 1},
		{"primary fails, fallback empty", &stub{err: boom}, &stub{}, 0, nil, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewService(tc.primary, tc.fallback)
			got, err := svc.Search(context.Background(), "dune", 8)
			if !errors.Is(err, tc.wantErr) || len(got) != tc.want {
				t.Fatalf("got %v, err %v", got, err)
			}
			if tc.wantErr == nil && got == nil {
				t.Error("results must be non-nil")
			}
			if tc.fallback.calls != tc.wantFallback {
				t.Errorf("fallback calls = %d", tc.fallback.calls)
			}
		})
	}
}

func TestServiceNoFallbackConfigured(t *testing.T) {
	svc := NewService(&stub{err: errors.New("boom")}, nil)
	if _, err := svc.Search(context.Background(), "dune", 8); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestServiceStopsWhenContextIsDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fallback := &stub{results: hit}
	svc := NewService(&stub{err: context.Canceled}, fallback)
	if _, err := svc.Search(ctx, "dune", 8); !errors.Is(err, context.Canceled) || fallback.calls != 0 {
		t.Fatalf("err = %v, fallback calls = %d", err, fallback.calls)
	}
}

func TestServiceCache(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	primary := &stub{results: hit}
	svc := newService(primary, nil, func() time.Time { return now })
	ctx := context.Background()

	for _, q := range []string{"dune", "  DUNE ", "Dune"} { // normalized to one key
		if _, err := svc.Search(ctx, q, 8); err != nil {
			t.Fatal(err)
		}
	}
	if primary.calls != 1 {
		t.Fatalf("calls = %d, want 1 (cache hit)", primary.calls)
	}

	if _, _ = svc.Search(ctx, "dune", 9); primary.calls != 2 {
		t.Fatal("limit must be part of the cache key")
	}

	now = now.Add(cacheTTL + time.Second)
	if _, _ = svc.Search(ctx, "dune", 8); primary.calls != 3 {
		t.Fatal("entries must expire after the TTL")
	}
}

func TestServiceDoesNotCacheFailures(t *testing.T) {
	primary := &stub{err: errors.New("boom")}
	svc := NewService(primary, nil)
	for range 2 {
		_, _ = svc.Search(context.Background(), "dune", 8)
	}
	if primary.calls != 2 {
		t.Fatalf("calls = %d, want 2", primary.calls)
	}
}

func TestCacheIsBounded(t *testing.T) {
	now := time.Now()
	c := newCache[int](time.Hour, 3, func() time.Time { return now })
	for i, k := range []string{"a", "b", "c", "d", "e"} {
		c.set(k, i)
		if len(c.entries) > 3 {
			t.Fatalf("size = %d after %q", len(c.entries), k)
		}
	}
	if _, ok := c.get("e"); !ok {
		t.Error("newest entry must be present")
	}
}
