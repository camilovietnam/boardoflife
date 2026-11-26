// Package books implements the per-user reading list: validation rules, the
// persistence port and the HTTP handlers.
package books

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// Reading statuses.
const (
	StatusToRead   = "to_read"
	StatusReading  = "reading"
	StatusFinished = "finished"
)

const (
	maxTextLen        = 300
	maxCoverURLLen    = 2000
	maxDescriptionLen = 5000
	maxExternalIDLen  = 100
	minRating         = 1.0
	maxRating         = 5.0
)

// ErrNotFound is returned by a Store when the book does not exist for that user.
var ErrNotFound = errors.New("book not found")

// Book is a stored book as exposed over the API.
type Book struct {
	ID       int64    `json:"id"`
	Name     string   `json:"name"`
	Author   string   `json:"author"`
	Status   string   `json:"status"`
	Progress int      `json:"progress"`
	Rating   *float64 `json:"rating"`

	// Optional catalogue metadata.
	CoverURL    *string `json:"cover_url"`
	Description *string `json:"description"`
	ExternalID  *string `json:"external_id"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Draft is a validated, normalized book payload ready to be stored.
type Draft struct {
	Name     string
	Author   string
	Status   string
	Progress int
	Rating   *float64

	CoverURL    *string
	Description *string
	ExternalID  *string
}

// Counts holds a user's number of books per status.
type Counts struct {
	ToRead   int `json:"to_read"`
	Reading  int `json:"reading"`
	Finished int `json:"finished"`
}

// ListQuery selects one page of a user's books. Empty Status or Search means
// "no filter"; Search is plain text (the store escapes it).
type ListQuery struct {
	Status string
	Search string
	Limit  int
	Offset int
}

// Page is the result of Store.List.
type Page struct {
	Items  []Book
	Total  int    // books matching the filters, across all pages
	Counts Counts // user totals per status, ignoring the filters
}

// Store is the persistence port of the books package. Every method is scoped
// to userID: another user's books behave as if they did not exist.
type Store interface {
	Create(ctx context.Context, userID int64, d Draft) (Book, error)
	Get(ctx context.Context, userID, id int64) (Book, error)
	Update(ctx context.Context, userID, id int64, d Draft) (Book, error)
	Delete(ctx context.Context, userID, id int64) error
	List(ctx context.Context, userID int64, q ListQuery) (Page, error)
}

// ValidStatus reports whether s is a known reading status.
func ValidStatus(s string) bool {
	return s == StatusToRead || s == StatusReading || s == StatusFinished
}

// FieldErrors maps a JSON field name to a human readable problem.
type FieldErrors map[string]string

func (e FieldErrors) Error() string { return "validation failed" }

// Input is the writable part of a book as sent by clients (POST and PUT).
// Progress is a float only so that fractions can be reported as a field
// error instead of an opaque decoding failure.
type Input struct {
	Name     string   `json:"name"`
	Author   string   `json:"author"`
	Status   string   `json:"status"`
	Progress *float64 `json:"progress"`
	Rating   *float64 `json:"rating"`

	CoverURL    *string `json:"cover_url"`
	Description *string `json:"description"`
	ExternalID  *string `json:"external_id"`
}

// Validate checks the input and returns the normalized Draft: text is
// trimmed, blank optional text becomes nil, progress is forced to 0 for to_read and to 100 for finished.
// On failure the returned error is a non-empty FieldErrors.
func (in Input) Validate() (Draft, error) {
	errs := FieldErrors{}
	d := Draft{
		Name:   strings.TrimSpace(in.Name),
		Author: strings.TrimSpace(in.Author),
		Status: in.Status,
	}

	checkText(errs, "name", d.Name)
	checkText(errs, "author", d.Author)

	if !ValidStatus(d.Status) {
		errs["status"] = "must be one of to_read, reading, finished"
	}

	if in.Progress != nil {
		p := *in.Progress
		if p != math.Trunc(p) || p < 0 || p > 100 {
			errs["progress"] = "must be an integer between 0 and 100"
		} else {
			d.Progress = int(p)
		}
	}
	switch d.Status {
	case StatusToRead:
		d.Progress = 0
	case StatusFinished:
		d.Progress = 100
	}

	if r := in.Rating; r != nil {
		if *r < minRating || *r > maxRating || *r*2 != math.Trunc(*r*2) {
			errs["rating"] = "must be null or a multiple of 0.5 between 1 and 5"
		} else {
			d.Rating = r
		}
	}

	d.CoverURL = optionalText(errs, "cover_url", in.CoverURL, maxCoverURLLen, checkHTTPSURL)
	d.Description = optionalText(errs, "description", in.Description, maxDescriptionLen, nil)
	d.ExternalID = optionalText(errs, "external_id", in.ExternalID, maxExternalIDLen, nil)

	if len(errs) > 0 {
		return Draft{}, errs
	}
	return d, nil
}

func checkText(errs FieldErrors, field, value string) {
	switch {
	case value == "":
		errs[field] = "is required"
	case utf8.RuneCountInString(value) > maxTextLen:
		errs[field] = "must be at most 300 characters"
	}
}

// optionalText normalizes an optional text field: it is trimmed and blank
// becomes nil. A value longer than max characters, or one rejected by check
// (which may be nil), is recorded in errs and yields nil.
func optionalText(errs FieldErrors, field string, in *string, max int, check func(string) string) *string {
	if in == nil {
		return nil
	}
	v := strings.TrimSpace(*in)
	switch {
	case v == "":
		return nil
	case utf8.RuneCountInString(v) > max:
		errs[field] = fmt.Sprintf("must be at most %d characters", max)
		return nil
	}
	if check != nil {
		if problem := check(v); problem != "" {
			errs[field] = problem
			return nil
		}
	}
	return &v
}

// checkHTTPSURL returns what is wrong with v as an absolute https URL, or "".
func checkHTTPSURL(v string) string {
	u, err := url.Parse(v)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" {
		return "must be an absolute https URL"
	}
	return ""
}
