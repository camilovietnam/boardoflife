package books

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"boardoflife/backend/internal/auth"
	"boardoflife/backend/internal/booksearch"
	"boardoflife/backend/internal/httpx"
)

const (
	maxBookBody     = 8 << 10 // 8 KiB
	defaultPage     = 1
	maxPage         = 1_000_000
	defaultPageSize = 12
	maxPageSize     = 50
	maxSearchLen    = 300

	minLookupLen       = 2
	maxLookupLen       = 200
	defaultLookupLimit = 8
	maxLookupLimit     = 20
)

// Searcher finds books in an external catalogue.
type Searcher interface {
	Search(ctx context.Context, query string, limit int) ([]booksearch.Result, error)
}

// Limiter decides whether a user may perform another lookup. When it says no,
// retryAfter is how long the user has to wait.
type Limiter interface {
	Allow(userID int64) (ok bool, retryAfter time.Duration)
}

// Handler exposes the books over HTTP.
type Handler struct {
	store       Store
	requireAuth func(http.Handler) http.Handler
	searcher    Searcher
	limiter     Limiter
}

// NewHandler builds the HTTP handlers. requireAuth must reject anonymous
// requests and put the user in the context (auth.Service.RequireAuth).
// searcher backs GET /books/lookup and limiter caps how often a user may call it.
func NewHandler(store Store, requireAuth func(http.Handler) http.Handler, searcher Searcher, limiter Limiter) *Handler {
	return &Handler{store: store, requireAuth: requireAuth, searcher: searcher, limiter: limiter}
}

// Routes registers the /books endpoints on r, which is expected to be
// mounted under /api.
func (h *Handler) Routes(r chi.Router) {
	r.Route("/books", func(r chi.Router) {
		r.Use(httpx.NoStore, h.requireAuth)
		r.Get("/", h.list)
		r.Post("/", h.create)
		r.Get("/lookup", h.lookup) // static routes win over /{id} in chi
		r.Get("/{id}", h.get)
		r.Put("/{id}", h.replace)
		r.Delete("/{id}", h.delete)
	})
}

// listResponse is the JSON body of GET /books.
type listResponse struct {
	Items      []Book `json:"items"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	Total      int    `json:"total"`
	TotalPages int    `json:"total_pages"`
	Counts     Counts `json:"counts"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context()) // requireAuth guarantees presence.

	q, page, pageSize, fields := parseListQuery(r)
	if len(fields) > 0 {
		httpx.WriteValidationError(w, fields)
		return
	}
	q.Limit, q.Offset = pageSize, (page-1)*pageSize

	result, err := h.store.List(r.Context(), user.ID, q)
	if err != nil {
		serverError(w, "list books", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, listResponse{
		Items:      result.Items,
		Page:       page,
		PageSize:   pageSize,
		Total:      result.Total,
		TotalPages: (result.Total + pageSize - 1) / pageSize,
		Counts:     result.Counts,
	})
}

// lookupResponse is the JSON body of GET /books/lookup.
type lookupResponse struct {
	Items []booksearch.Result `json:"items"`
}

func (h *Handler) lookup(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())

	query, limit, fields := parseLookupQuery(r)
	if len(fields) > 0 {
		httpx.WriteValidationError(w, fields)
		return
	}
	if ok, retryAfter := h.limiter.Allow(user.ID); !ok {
		seconds := max(1, int(math.Ceil(retryAfter.Seconds())))
		w.Header().Set("Retry-After", strconv.Itoa(seconds))
		httpx.WriteError(w, http.StatusTooManyRequests, "too many searches, try again later")
		return
	}

	items, err := h.searcher.Search(r.Context(), query, limit)
	if err != nil {
		// Provider errors never contain URLs or keys; they are safe to log.
		slog.Warn("book lookup failed", "err", err)
		httpx.WriteError(w, http.StatusBadGateway, "book search is temporarily unavailable")
		return
	}
	if items == nil {
		items = []booksearch.Result{}
	}
	httpx.WriteJSON(w, http.StatusOK, lookupResponse{Items: items})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	draft, ok := decodeDraft(w, r)
	if !ok {
		return
	}
	book, err := h.store.Create(r.Context(), user.ID, draft)
	if err != nil {
		serverError(w, "create book", err)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/api/books/%d", book.ID))
	httpx.WriteJSON(w, http.StatusCreated, book)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	id, ok := bookID(w, r)
	if !ok {
		return
	}
	book, err := h.store.Get(r.Context(), user.ID, id)
	respond(w, http.StatusOK, book, err, "get book")
}

func (h *Handler) replace(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	id, ok := bookID(w, r)
	if !ok {
		return
	}
	draft, ok := decodeDraft(w, r)
	if !ok {
		return
	}
	book, err := h.store.Update(r.Context(), user.ID, id, draft)
	respond(w, http.StatusOK, book, err, "update book")
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	id, ok := bookID(w, r)
	if !ok {
		return
	}
	switch err := h.store.Delete(r.Context(), user.ID, id); {
	case errors.Is(err, ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "book not found")
	case err != nil:
		serverError(w, "delete book", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// respond writes book as JSON with status, or the matching error response.
func respond(w http.ResponseWriter, status int, book Book, err error, op string) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "book not found")
	case err != nil:
		serverError(w, op, err)
	default:
		httpx.WriteJSON(w, status, book)
	}
}

// decodeDraft reads and validates a request body. On failure it has already
// written the 400/413 response and returns false.
func decodeDraft(w http.ResponseWriter, r *http.Request) (Draft, bool) {
	var in Input
	if !httpx.DecodeJSON(w, r, &in, maxBookBody) {
		return Draft{}, false
	}
	draft, err := in.Validate()
	var fields FieldErrors
	if errors.As(err, &fields) {
		httpx.WriteValidationError(w, fields)
		return Draft{}, false
	}
	return draft, true
}

// bookID parses the {id} path parameter. A malformed id cannot match any
// book, so it is answered with 404.
func bookID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		httpx.WriteError(w, http.StatusNotFound, "book not found")
		return 0, false
	}
	return id, true
}

func parseListQuery(r *http.Request) (q ListQuery, page, pageSize int, fields FieldErrors) {
	fields = FieldErrors{}
	v := r.URL.Query()

	if s := v.Get("status"); s != "" {
		if ValidStatus(s) {
			q.Status = s
		} else {
			fields["status"] = "must be one of to_read, reading, finished"
		}
	}
	if s := strings.TrimSpace(v.Get("q")); utf8.RuneCountInString(s) > maxSearchLen {
		fields["q"] = "must be at most 300 characters"
	} else {
		q.Search = s
	}
	page = intParam(v.Get("page"), "page", defaultPage, maxPage, fields)
	pageSize = intParam(v.Get("page_size"), "page_size", defaultPageSize, maxPageSize, fields)
	return q, page, pageSize, fields
}

func parseLookupQuery(r *http.Request) (query string, limit int, fields FieldErrors) {
	fields = FieldErrors{}
	v := r.URL.Query()

	query = strings.TrimSpace(v.Get("q"))
	if n := utf8.RuneCountInString(query); n < minLookupLen || n > maxLookupLen {
		fields["q"] = fmt.Sprintf("is required and must be between %d and %d characters", minLookupLen, maxLookupLen)
	}
	limit = intParam(v.Get("limit"), "limit", defaultLookupLimit, maxLookupLimit, fields)
	return query, limit, fields
}

// intParam parses an optional integer in [1, max], recording a field error
// when it is malformed or out of range.
func intParam(raw, name string, def, max int, fields FieldErrors) int {
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > max {
		fields[name] = fmt.Sprintf("must be an integer between 1 and %d", max)
		return def
	}
	return n
}

func serverError(w http.ResponseWriter, op string, err error) {
	slog.Error(op, "err", err)
	httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
}
