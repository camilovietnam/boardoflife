// Package booksearch looks books up in public catalogues (Google Books, with
// Open Library as a fallback) and normalizes the answers to a single shape.
package booksearch

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const (
	cacheTTL     = 10 * time.Minute
	cacheMaxSize = 500

	unknownAuthor = "Unknown author"

	// Limits matching what the books package accepts when storing a book,
	// so a looked-up result can always be saved as is.
	maxNameLen        = 300
	maxAuthorLen      = 300
	maxDescriptionLen = 5000
	maxCoverURLLen    = 2000
	maxExternalIDLen  = 100
)

// ErrUnavailable means no catalogue could answer the search.
var ErrUnavailable = errors.New("book search unavailable")

// Result is one catalogue entry.
type Result struct {
	ExternalID    string  `json:"external_id"`
	Name          string  `json:"name"`
	Author        string  `json:"author"`
	Description   *string `json:"description"`
	CoverURL      *string `json:"cover_url"`
	PublishedYear *int    `json:"published_year"`
}

// Provider is a book catalogue. Its errors must never contain request URLs
// (they may carry an API key).
type Provider interface {
	Name() string
	Search(ctx context.Context, query string, limit int) ([]Result, error)
}

// Service searches the primary provider, falls back to the secondary one when
// the primary fails or finds nothing, and caches successful answers.
type Service struct {
	primary  Provider
	fallback Provider // may be nil
	cache    *cache[[]Result]
}

// NewService builds a Service. fallback may be nil.
func NewService(primary, fallback Provider) *Service {
	return newService(primary, fallback, time.Now)
}

func newService(primary, fallback Provider, now func() time.Time) *Service {
	return &Service{
		primary:  primary,
		fallback: fallback,
		cache:    newCache[[]Result](cacheTTL, cacheMaxSize, now),
	}
}

// Search returns at most limit results for query. It returns ErrUnavailable
// when no provider could answer, and the context's error when ctx is done.
func (s *Service) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	query = strings.Join(strings.Fields(strings.ToLower(query)), " ")
	key := fmt.Sprintf("%d:%s", limit, query)

	if results, ok := s.cache.get(key); ok {
		return results, nil
	}
	results, err := s.search(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	s.cache.set(key, results)
	return results, nil
}

func (s *Service) search(ctx context.Context, query string, limit int) ([]Result, error) {
	results, err := s.primary.Search(ctx, query, limit)
	if err == nil && len(results) > 0 {
		return results, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		slog.Warn("book search failed", "provider", s.primary.Name(), "err", err)
	}
	if s.fallback == nil {
		return empty(results), unavailableIf(err)
	}

	fallbackResults, fallbackErr := s.fallback.Search(ctx, query, limit)
	switch {
	case fallbackErr == nil:
		return empty(fallbackResults), nil
	case ctx.Err() != nil:
		return nil, ctx.Err()
	}
	slog.Warn("book search failed", "provider", s.fallback.Name(), "err", fallbackErr)
	// The primary answered (with nothing): that is still a valid answer.
	return empty(results), unavailableIf(err)
}

func unavailableIf(err error) error {
	if err != nil {
		return ErrUnavailable
	}
	return nil
}

// empty turns nil into an empty, non-nil slice so it serializes as [].
func empty(r []Result) []Result {
	if r == nil {
		return []Result{}
	}
	return r
}

// truncate cuts s to at most n runes.
func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// optional trims s, caps it at n runes and maps the empty string to nil.
func optional(s string, n int) *string {
	s = truncate(strings.TrimSpace(s), n)
	if s == "" {
		return nil
	}
	return &s
}

// joinAuthors renders an author list, or "Unknown author" when there is none.
func joinAuthors(authors []string) string {
	names := make([]string, 0, len(authors))
	for _, a := range authors {
		if a = strings.TrimSpace(a); a != "" {
			names = append(names, a)
		}
	}
	if len(names) == 0 {
		return unknownAuthor
	}
	return truncate(strings.Join(names, ", "), maxAuthorLen)
}
