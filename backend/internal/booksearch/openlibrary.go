package booksearch

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Open Library endpoints.
const (
	OpenLibraryURL      = "https://openlibrary.org/search.json"
	openLibraryCoverFmt = "https://covers.openlibrary.org/b/id/%d-M.jpg"
)

// OpenLibrary searches Open Library. It needs no API key.
type OpenLibrary struct {
	client  *http.Client
	baseURL string
}

// NewOpenLibrary builds the provider. baseURL is injectable for tests.
func NewOpenLibrary(client *http.Client, baseURL string) *OpenLibrary {
	return &OpenLibrary{client: client, baseURL: baseURL}
}

func (o *OpenLibrary) Name() string { return "open_library" }

type openLibraryResponse struct {
	Docs []struct {
		Key              string   `json:"key"`
		Title            string   `json:"title"`
		AuthorName       []string `json:"author_name"`
		FirstPublishYear int      `json:"first_publish_year"`
		CoverID          int64    `json:"cover_i"`
	} `json:"docs"`
}

func (o *OpenLibrary) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	v := url.Values{}
	v.Set("q", query)
	v.Set("fields", "key,title,author_name,first_publish_year,cover_i")
	v.Set("limit", strconv.Itoa(limit))

	var resp openLibraryResponse
	if err := getJSON(ctx, o.client, o.Name(), o.baseURL, v, &resp); err != nil {
		return nil, err
	}

	results := make([]Result, 0, len(resp.Docs))
	for _, d := range resp.Docs {
		name := truncate(strings.TrimSpace(d.Title), maxNameLen)
		key := strings.TrimSpace(d.Key)
		if name == "" || key == "" || len(key) > maxExternalIDLen {
			continue
		}
		r := Result{ExternalID: key, Name: name, Author: joinAuthors(d.AuthorName)}
		if d.CoverID > 0 {
			cover := fmt.Sprintf(openLibraryCoverFmt, d.CoverID)
			r.CoverURL = &cover
		}
		if d.FirstPublishYear > 0 {
			r.PublishedYear = &d.FirstPublishYear
		}
		results = append(results, r)
	}
	return results, nil
}
