package booksearch

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// GoogleBooksURL is the Google Books volumes endpoint.
const GoogleBooksURL = "https://www.googleapis.com/books/v1/volumes"

const googleFields = "items(id,volumeInfo(title,authors,description,publishedDate,imageLinks/thumbnail))"

// Google searches Google Books.
type Google struct {
	client  *http.Client
	baseURL string
	apiKey  string // optional
}

// NewGoogle builds the provider. baseURL is injectable for tests; apiKey may
// be empty, in which case no key is sent.
func NewGoogle(client *http.Client, baseURL, apiKey string) *Google {
	return &Google{client: client, baseURL: baseURL, apiKey: apiKey}
}

func (g *Google) Name() string { return "google_books" }

type googleResponse struct {
	Items []googleVolume `json:"items"`
}

type googleVolume struct {
	ID         string `json:"id"`
	VolumeInfo struct {
		Title         string   `json:"title"`
		Authors       []string `json:"authors"`
		Description   string   `json:"description"`
		PublishedDate string   `json:"publishedDate"`
		ImageLinks    struct {
			Thumbnail string `json:"thumbnail"`
		} `json:"imageLinks"`
	} `json:"volumeInfo"`
}

func (g *Google) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	v := url.Values{}
	v.Set("q", query)
	v.Set("maxResults", strconv.Itoa(limit))
	v.Set("printType", "books")
	v.Set("fields", googleFields)
	if g.apiKey != "" {
		v.Set("key", g.apiKey)
	}

	var resp googleResponse
	if err := getJSON(ctx, g.client, g.Name(), g.baseURL, v, &resp); err != nil {
		return nil, err
	}

	results := make([]Result, 0, len(resp.Items))
	for _, item := range resp.Items {
		if r, ok := item.result(); ok {
			results = append(results, r)
		}
	}
	return results, nil
}

// result maps a volume, reporting false for volumes without id or title.
func (v googleVolume) result() (Result, bool) {
	info := v.VolumeInfo
	name := truncate(strings.TrimSpace(info.Title), maxNameLen)
	id := strings.TrimSpace(v.ID)
	if name == "" || id == "" || len(id) > maxExternalIDLen {
		return Result{}, false
	}
	return Result{
		ExternalID:    id,
		Name:          name,
		Author:        joinAuthors(info.Authors),
		Description:   optional(info.Description, maxDescriptionLen),
		CoverURL:      httpsCover(info.ImageLinks.Thumbnail),
		PublishedYear: yearOf(info.PublishedDate),
	}, true
}

// httpsCover normalizes a cover URL: http becomes https and the page-curl
// effect (`edge=curl`) is removed. Anything that is not an absolute http(s)
// URL, or is too long, yields nil.
func httpsCover(raw string) *string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil
	}
	u.Scheme = "https"
	q := u.Query()
	q.Del("edge")
	u.RawQuery = q.Encode()

	s := u.String()
	if len(s) > maxCoverURLLen {
		return nil
	}
	return &s
}

// yearOf extracts the year from "2019", "2019-05" or "2019-05-12".
func yearOf(date string) *int {
	if len(date) < 4 {
		return nil
	}
	y, err := strconv.Atoi(date[:4])
	if err != nil || y < 1 {
		return nil
	}
	return &y
}
