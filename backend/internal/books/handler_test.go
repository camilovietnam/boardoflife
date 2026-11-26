package books

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"boardoflife/backend/internal/auth"
	"boardoflife/backend/internal/booksearch"
	"boardoflife/backend/internal/ratelimit"
)

// memStore is an in-memory Store for tests. A fake clock advances on every
// write so updated_at ordering is deterministic.
type memStore struct {
	nextID int64
	clock  time.Time
	owner  map[int64]int64 // book id -> user id
	books  map[int64]Book
}

func newMemStore() *memStore {
	return &memStore{
		clock: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		owner: map[int64]int64{},
		books: map[int64]Book{},
	}
}

func (m *memStore) tick() time.Time {
	m.clock = m.clock.Add(time.Minute)
	return m.clock
}

func (m *memStore) owned(userID, id int64) (Book, bool) {
	b, ok := m.books[id]
	return b, ok && m.owner[id] == userID
}

func (m *memStore) Create(_ context.Context, userID int64, d Draft) (Book, error) {
	m.nextID++
	now := m.tick()
	b := Book{ID: m.nextID, Name: d.Name, Author: d.Author, Status: d.Status,
		Progress: d.Progress, Rating: d.Rating,
		CoverURL: d.CoverURL, Description: d.Description, ExternalID: d.ExternalID,
		CreatedAt: now, UpdatedAt: now}
	m.books[b.ID], m.owner[b.ID] = b, userID
	return b, nil
}

func (m *memStore) Get(_ context.Context, userID, id int64) (Book, error) {
	b, ok := m.owned(userID, id)
	if !ok {
		return Book{}, ErrNotFound
	}
	return b, nil
}

func (m *memStore) Update(_ context.Context, userID, id int64, d Draft) (Book, error) {
	b, ok := m.owned(userID, id)
	if !ok {
		return Book{}, ErrNotFound
	}
	b.Name, b.Author, b.Status, b.Progress, b.Rating = d.Name, d.Author, d.Status, d.Progress, d.Rating
	b.CoverURL, b.Description, b.ExternalID = d.CoverURL, d.Description, d.ExternalID
	b.UpdatedAt = m.tick()
	m.books[id] = b
	return b, nil
}

func (m *memStore) Delete(_ context.Context, userID, id int64) error {
	if _, ok := m.owned(userID, id); !ok {
		return ErrNotFound
	}
	delete(m.books, id)
	return nil
}

func (m *memStore) List(_ context.Context, userID int64, q ListQuery) (Page, error) {
	var mine, matching []Book
	for id, b := range m.books {
		if m.owner[id] == userID {
			mine = append(mine, b)
		}
	}
	var page Page
	for _, b := range mine {
		switch b.Status {
		case StatusToRead:
			page.Counts.ToRead++
		case StatusReading:
			page.Counts.Reading++
		case StatusFinished:
			page.Counts.Finished++
		}
		text := strings.ToLower(b.Name + "\n" + b.Author)
		if (q.Status == "" || b.Status == q.Status) && strings.Contains(text, strings.ToLower(q.Search)) {
			matching = append(matching, b)
		}
	}
	slices.SortFunc(matching, func(a, b Book) int {
		if c := b.UpdatedAt.Compare(a.UpdatedAt); c != 0 {
			return c
		}
		return int(b.ID - a.ID)
	})
	page.Total = len(matching)
	page.Items = []Book{}
	if q.Offset < len(matching) {
		page.Items = matching[q.Offset:min(q.Offset+q.Limit, len(matching))]
	}
	return page, nil
}

// testAPI wires the real handler to a fake authentication middleware that
// trusts the X-Test-User header (a user id); no header means anonymous.
func testAPI() http.Handler {
	return testAPIWith(&fakeSearcher{}, allowAll{})
}

// fakeSearcher records its calls and answers with canned results or an error.
type fakeSearcher struct {
	results []booksearch.Result
	err     error
	query   string
	limit   int
}

func (f *fakeSearcher) Search(_ context.Context, query string, limit int) ([]booksearch.Result, error) {
	f.query, f.limit = query, limit
	return f.results, f.err
}

type allowAll struct{}

func (allowAll) Allow(int64) (bool, time.Duration) { return true, 0 }

func testAPIWith(searcher Searcher, limiter Limiter) http.Handler {
	fakeAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, err := strconv.ParseInt(r.Header.Get("X-Test-User"), 10, 64)
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), auth.User{ID: id, Role: auth.RoleUser})))
		})
	}
	r := chi.NewRouter()
	r.Route("/api", NewHandler(newMemStore(), fakeAuth, searcher, limiter).Routes)
	return r
}

func do(t *testing.T, api http.Handler, user, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if user != "" {
		req.Header.Set("X-Test-User", user)
	}
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

func mustCreate(t *testing.T, api http.Handler, user, body string) Book {
	t.Helper()
	rec := do(t, api, user, http.MethodPost, "/api/books", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %s: %d %s", body, rec.Code, rec.Body)
	}
	return decode[Book](t, rec)
}

func TestRequiresAuthentication(t *testing.T) {
	api := testAPI()
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/books"},
		{http.MethodPost, "/api/books"},
		{http.MethodGet, "/api/books/lookup?q=dune"},
		{http.MethodGet, "/api/books/1"},
		{http.MethodPut, "/api/books/1"},
		{http.MethodDelete, "/api/books/1"},
	} {
		rec := do(t, api, "", tc.method, tc.path, `{}`)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s = %d", tc.method, tc.path, rec.Code)
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s %s: missing no-store", tc.method, tc.path)
		}
	}
}

func TestCreateGetReplaceDelete(t *testing.T) {
	api := testAPI()

	rec := do(t, api, "1", http.MethodPost, "/api/books",
		`{"name":"  Dune ","author":"Frank Herbert","status":"reading","progress":30,"rating":4.5}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("missing no-store")
	}
	book := decode[Book](t, rec)
	if loc := rec.Header().Get("Location"); loc != "/api/books/"+strconv.FormatInt(book.ID, 10) {
		t.Errorf("Location = %q", loc)
	}
	if book.Name != "Dune" || book.Progress != 30 || book.Rating == nil || *book.Rating != 4.5 {
		t.Fatalf("book = %+v", book)
	}
	path := "/api/books/" + strconv.FormatInt(book.ID, 10)

	if rec := do(t, api, "1", http.MethodGet, path, ""); rec.Code != http.StatusOK {
		t.Fatalf("get: %d", rec.Code)
	}

	rec = do(t, api, "1", http.MethodPut, path,
		`{"name":"Dune","author":"Frank Herbert","status":"finished","rating":null}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("put: %d %s", rec.Code, rec.Body)
	}
	updated := decode[Book](t, rec)
	if updated.Progress != 100 || updated.Rating != nil || !updated.UpdatedAt.After(book.UpdatedAt) {
		t.Fatalf("updated = %+v", updated)
	}
	if !strings.Contains(rec.Body.String(), `"rating":null`) {
		t.Errorf("rating must be serialized as null: %s", rec.Body)
	}

	if rec := do(t, api, "1", http.MethodDelete, path, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	if rec := do(t, api, "1", http.MethodGet, path, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("get after delete: %d", rec.Code)
	}
	if rec := do(t, api, "1", http.MethodDelete, path, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("second delete: %d", rec.Code)
	}
}

func TestOwnershipIsolation(t *testing.T) {
	api := testAPI()
	book := mustCreate(t, api, "1", `{"name":"Private","author":"A","status":"to_read"}`)
	path := "/api/books/" + strconv.FormatInt(book.ID, 10)
	body := `{"name":"Hijacked","author":"B","status":"finished"}`

	for _, tc := range []struct{ method, body string }{
		{http.MethodGet, ""},
		{http.MethodPut, body},
		{http.MethodDelete, ""},
	} {
		if rec := do(t, api, "2", tc.method, path, tc.body); rec.Code != http.StatusNotFound {
			t.Errorf("user 2 %s = %d, want 404", tc.method, rec.Code)
		}
	}

	list := decode[listResponse](t, do(t, api, "2", http.MethodGet, "/api/books", ""))
	if list.Total != 0 || len(list.Items) != 0 || list.Counts != (Counts{}) {
		t.Fatalf("user 2 sees user 1 data: %+v", list)
	}

	// The owner's book is untouched.
	got := decode[Book](t, do(t, api, "1", http.MethodGet, path, ""))
	if got.Name != "Private" || got.Status != StatusToRead {
		t.Fatalf("book was modified: %+v", got)
	}
}

func TestValidationErrors(t *testing.T) {
	api := testAPI()
	for _, tc := range []struct {
		name, body string
		code       int
		field      string
	}{
		{"missing name", `{"author":"A","status":"reading"}`, 400, "name"},
		{"bad status", `{"name":"N","author":"A","status":"nope"}`, 400, "status"},
		{"bad rating", `{"name":"N","author":"A","status":"reading","rating":4.3}`, 400, "rating"},
		{"bad progress", `{"name":"N","author":"A","status":"reading","progress":101}`, 400, "progress"},
		{"unknown field", `{"name":"N","author":"A","status":"reading","id":9}`, 400, ""},
		{"malformed json", `{"name":`, 400, ""},
		{"too large", `{"name":"` + strings.Repeat("x", 9<<10) + `"}`, 413, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, api, "1", http.MethodPost, "/api/books", tc.body)
			if rec.Code != tc.code {
				t.Fatalf("code = %d, body %s", rec.Code, rec.Body)
			}
			body := decode[struct {
				Error  string            `json:"error"`
				Fields map[string]string `json:"fields"`
			}](t, rec)
			if body.Error == "" {
				t.Error("missing error message")
			}
			if tc.field != "" && body.Fields[tc.field] == "" {
				t.Errorf("fields = %v, want %q", body.Fields, tc.field)
			}
		})
	}

	// Validation applies to PUT as well, and a bad id is simply not found.
	book := mustCreate(t, api, "1", `{"name":"N","author":"A","status":"reading"}`)
	rec := do(t, api, "1", http.MethodPut, "/api/books/"+strconv.FormatInt(book.ID, 10), `{"name":"","author":"A","status":"reading"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("put invalid = %d", rec.Code)
	}
	if rec := do(t, api, "1", http.MethodGet, "/api/books/abc", ""); rec.Code != http.StatusNotFound {
		t.Errorf("non numeric id = %d", rec.Code)
	}
}

func TestListFilterSearchPaginationCounts(t *testing.T) {
	api := testAPI()
	mustCreate(t, api, "1", `{"name":"Dune","author":"Frank Herbert","status":"finished"}`)
	mustCreate(t, api, "1", `{"name":"Emma","author":"Jane Austen","status":"reading","progress":10}`)
	mustCreate(t, api, "1", `{"name":"Persuasion","author":"Jane Austen","status":"to_read"}`)
	mustCreate(t, api, "1", `{"name":"Hyperion","author":"Dan Simmons","status":"to_read"}`)
	mustCreate(t, api, "2", `{"name":"Other user","author":"X","status":"reading"}`)

	list := func(query string) (int, listResponse) {
		rec := do(t, api, "1", http.MethodGet, "/api/books"+query, "")
		if rec.Code != http.StatusOK {
			return rec.Code, listResponse{}
		}
		return rec.Code, decode[listResponse](t, rec)
	}

	_, all := list("")
	if all.Page != 1 || all.PageSize != 12 || all.Total != 4 || all.TotalPages != 1 ||
		all.Counts != (Counts{ToRead: 2, Reading: 1, Finished: 1}) {
		t.Fatalf("defaults: %+v", all)
	}
	if all.Items[0].Name != "Hyperion" || all.Items[3].Name != "Dune" {
		t.Errorf("order must be updated_at DESC: %+v", all.Items)
	}

	_, byStatus := list("?status=to_read")
	if byStatus.Total != 2 || byStatus.Counts.Reading != 1 {
		t.Errorf("status filter must not change counts: %+v", byStatus)
	}

	_, search := list("?q=JANE")
	if search.Total != 2 {
		t.Errorf("case-insensitive search: %+v", search)
	}
	_, both := list("?q=jane&status=reading")
	if both.Total != 1 || both.Items[0].Name != "Emma" {
		t.Errorf("status+q: %+v", both)
	}

	_, p1 := list("?page_size=3")
	_, p2 := list("?page_size=3&page=2")
	_, p3 := list("?page_size=3&page=3")
	if p1.TotalPages != 2 || len(p1.Items) != 3 || len(p2.Items) != 1 || p3.Items == nil || len(p3.Items) != 0 {
		t.Errorf("pagination: %d %d %d", len(p1.Items), len(p2.Items), len(p3.Items))
	}
	if p3.Page != 3 || p3.Total != 4 {
		t.Errorf("past the end: %+v", p3)
	}

	for _, bad := range []string{"?status=x", "?page=0", "?page=a", "?page_size=0", "?page_size=51", "?page=-1"} {
		if code, _ := list(bad); code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", bad, code)
		}
	}
}

func TestMetadataRoundTrip(t *testing.T) {
	api := testAPI()
	body := `{"name":"Dune","author":"FH","status":"to_read",
		"cover_url":"https://books.google.com/c?id=1","description":" Spice. ","external_id":"abc"}`
	book := mustCreate(t, api, "1", body)
	if book.CoverURL == nil || *book.CoverURL != "https://books.google.com/c?id=1" ||
		book.Description == nil || *book.Description != "Spice." ||
		book.ExternalID == nil || *book.ExternalID != "abc" {
		t.Fatalf("book = %+v", book)
	}
	path := "/api/books/" + strconv.FormatInt(book.ID, 10)

	got := decode[Book](t, do(t, api, "1", http.MethodGet, path, ""))
	if got.CoverURL == nil || got.ExternalID == nil {
		t.Fatalf("metadata lost on read: %+v", got)
	}

	// PUT is a full replace: omitted metadata becomes null.
	rec := do(t, api, "1", http.MethodPut, path, `{"name":"Dune","author":"FH","status":"to_read"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("put: %d %s", rec.Code, rec.Body)
	}
	for _, field := range []string{"cover_url", "description", "external_id"} {
		if !strings.Contains(rec.Body.String(), `"`+field+`":null`) {
			t.Errorf("%s must be null after PUT: %s", field, rec.Body)
		}
	}

	// Omitted on create is null as well.
	plain := mustCreate(t, api, "1", `{"name":"N","author":"A","status":"to_read"}`)
	if plain.CoverURL != nil || plain.Description != nil || plain.ExternalID != nil {
		t.Fatalf("plain = %+v", plain)
	}
}

func TestMetadataValidationOverHTTP(t *testing.T) {
	api := testAPI()
	for _, tc := range []struct{ field, value string }{
		{"cover_url", `"http://example.com/c.jpg"`},
		{"cover_url", `"javascript:alert(1)"`},
		{"description", `"` + strings.Repeat("d", 5001) + `"`},
		{"external_id", `"` + strings.Repeat("e", 101) + `"`},
	} {
		body := `{"name":"N","author":"A","status":"to_read","` + tc.field + `":` + tc.value + `}`
		rec := do(t, api, "1", http.MethodPost, "/api/books", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s=%.30s: %d", tc.field, tc.value, rec.Code)
			continue
		}
		if decode[httpxValidation](t, rec).Fields[tc.field] == "" {
			t.Errorf("no field error for %s: %s", tc.field, rec.Body)
		}
	}
}

type httpxValidation struct {
	Fields map[string]string `json:"fields"`
}

func TestLookupIsNotTreatedAsID(t *testing.T) {
	searcher := &fakeSearcher{results: []booksearch.Result{{ExternalID: "1", Name: "Dune", Author: "FH"}}}
	api := testAPIWith(searcher, allowAll{})

	rec := do(t, api, "1", http.MethodGet, "/api/books/lookup?q=dune", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("lookup: %d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("missing no-store")
	}
	if got := decode[lookupResponse](t, rec); len(got.Items) != 1 || got.Items[0].ExternalID != "1" {
		t.Fatalf("body = %s", rec.Body)
	}
	if searcher.query != "dune" || searcher.limit != 8 {
		t.Errorf("searcher got %q, %d", searcher.query, searcher.limit)
	}
}

func TestLookupEmptyResultIsAnArray(t *testing.T) {
	rec := do(t, testAPI(), "1", http.MethodGet, "/api/books/lookup?q=zzzz", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestLookupValidation(t *testing.T) {
	api := testAPI()
	for _, tc := range []struct{ query, field string }{
		{"", "q"},
		{"?q=", "q"},
		{"?q=a", "q"},
		{"?q=%20a%20", "q"},
		{"?q=" + strings.Repeat("x", 201), "q"},
		{"?q=dune&limit=0", "limit"},
		{"?q=dune&limit=21", "limit"},
		{"?q=dune&limit=x", "limit"},
	} {
		rec := do(t, api, "1", http.MethodGet, "/api/books/lookup"+tc.query, "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%.40s = %d", tc.query, rec.Code)
			continue
		}
		if decode[httpxValidation](t, rec).Fields[tc.field] == "" {
			t.Errorf("%.40s: no field error for %s: %s", tc.query, tc.field, rec.Body)
		}
	}

	if rec := do(t, api, "1", http.MethodGet, "/api/books/lookup?q=ab&limit=20", ""); rec.Code != http.StatusOK {
		t.Errorf("boundaries must be accepted: %d", rec.Code)
	}
}

func TestLookupUpstreamFailureIs502(t *testing.T) {
	api := testAPIWith(&fakeSearcher{err: booksearch.ErrUnavailable}, allowAll{})
	rec := do(t, api, "1", http.MethodGet, "/api/books/lookup?q=dune", "")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("code = %d", rec.Code)
	}
	if got := decode[struct {
		Error string `json:"error"`
	}](t, rec).Error; got != "book search is temporarily unavailable" {
		t.Errorf("error = %q", got)
	}
}

func TestLookupRateLimitPerUser(t *testing.T) {
	api := testAPIWith(&fakeSearcher{}, ratelimit.New[int64](2, time.Minute))
	get := func(user string) *httptest.ResponseRecorder {
		return do(t, api, user, http.MethodGet, "/api/books/lookup?q=dune", "")
	}

	for range 2 {
		if rec := get("1"); rec.Code != http.StatusOK {
			t.Fatalf("within quota: %d", rec.Code)
		}
	}
	rec := get("1")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("over quota: %d", rec.Code)
	}
	if secs, err := strconv.Atoi(rec.Header().Get("Retry-After")); err != nil || secs < 1 || secs > 60 {
		t.Errorf("Retry-After = %q", rec.Header().Get("Retry-After"))
	}
	if decode[struct {
		Error string `json:"error"`
	}](t, rec).Error == "" {
		t.Error("missing error message")
	}
	if rec := get("2"); rec.Code != http.StatusOK {
		t.Errorf("another user must not be limited: %d", rec.Code)
	}
}

func TestInvalidLookupDoesNotConsumeQuota(t *testing.T) {
	api := testAPIWith(&fakeSearcher{}, ratelimit.New[int64](1, time.Minute))
	do(t, api, "1", http.MethodGet, "/api/books/lookup", "")
	if rec := do(t, api, "1", http.MethodGet, "/api/books/lookup?q=dune", ""); rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
}
