package healthbars

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"boardoflife/backend/internal/auth"
)

type barKey struct {
	userID  int64
	section string
}

// memStore is an in-memory Store for tests. A fake clock advances on every
// write so last_updated is deterministic.
type memStore struct {
	clock time.Time
	bars  map[barKey]Record
}

func newMemStore() *memStore {
	return &memStore{
		clock: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		bars:  map[barKey]Record{},
	}
}

func (m *memStore) tick() time.Time {
	m.clock = m.clock.Add(time.Minute)
	return m.clock
}

func (m *memStore) Get(_ context.Context, userID int64, section string) (Record, error) {
	key := barKey{userID, section}
	if _, ok := m.bars[key]; !ok {
		m.bars[key] = Record{HalfPoints: maxHalfPoints, LastUpdated: m.tick()}
	}
	return m.bars[key], nil
}

func (m *memStore) Set(_ context.Context, userID int64, section string, halfPoints int) (Record, error) {
	rec := Record{HalfPoints: halfPoints, LastUpdated: m.tick()}
	m.bars[barKey{userID, section}] = rec
	return rec, nil
}

// testAPI wires the real handler to a fake authentication middleware that
// trusts the X-Test-User header (a user id); no header means anonymous.
func testAPI() http.Handler {
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
	r.Route("/api", NewHandler(newMemStore(), fakeAuth).Routes)
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

func put(t *testing.T, api http.Handler, user, section, health string) *httptest.ResponseRecorder {
	t.Helper()
	return do(t, api, user, http.MethodPut, "/api/healthbars/"+section, `{"health":`+health+`}`)
}

func TestRequiresAuthentication(t *testing.T) {
	api := testAPI()
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		rec := do(t, api, "", method, "/api/healthbars/books", `{"health":5}`)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s = %d", method, rec.Code)
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s: missing no-store", method)
		}
	}
}

func TestGetCreatesFullBarLazily(t *testing.T) {
	api := testAPI()
	rec := do(t, api, "1", http.MethodGet, "/api/healthbars/books", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("missing no-store")
	}
	bar := decode[Healthbar](t, rec)
	if bar.Health != 10 || bar.Max != 10 || bar.LastUpdated.IsZero() {
		t.Fatalf("bar = %+v", bar)
	}

	// A second read returns the same bar instead of creating a new one.
	again := decode[Healthbar](t, do(t, api, "1", http.MethodGet, "/api/healthbars/books", ""))
	if !again.LastUpdated.Equal(bar.LastUpdated) {
		t.Errorf("last_updated changed on read: %v -> %v", bar.LastUpdated, again.LastUpdated)
	}
}

func TestPutThenGet(t *testing.T) {
	api := testAPI()
	first := decode[Healthbar](t, do(t, api, "1", http.MethodGet, "/api/healthbars/exercise", ""))

	rec := put(t, api, "1", "exercise", "7.5")
	if rec.Code != http.StatusOK {
		t.Fatalf("put: %d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("missing no-store")
	}
	updated := decode[Healthbar](t, rec)
	if updated.Health != 7.5 || updated.Max != 10 || !updated.LastUpdated.After(first.LastUpdated) {
		t.Fatalf("updated = %+v (first %+v)", updated, first)
	}
	if !strings.Contains(rec.Body.String(), `"health":7.5`) {
		t.Errorf("body = %s", rec.Body)
	}

	got := decode[Healthbar](t, do(t, api, "1", http.MethodGet, "/api/healthbars/exercise", ""))
	if got != updated {
		t.Errorf("get = %+v, want %+v", got, updated)
	}
}

func TestPutWithoutPriorGetUpserts(t *testing.T) {
	api := testAPI()
	if rec := put(t, api, "1", "learning", "3"); rec.Code != http.StatusOK {
		t.Fatalf("put: %d %s", rec.Code, rec.Body)
	}
	if got := decode[Healthbar](t, do(t, api, "1", http.MethodGet, "/api/healthbars/learning", "")); got.Health != 3 {
		t.Errorf("health = %v, want 3", got.Health)
	}
}

func TestPutValidation(t *testing.T) {
	api := testAPI()
	for _, tc := range []struct {
		name, body string
		code       int
		field      string
	}{
		{"zero", `{"health":0}`, 200, ""},
		{"full", `{"health":10}`, 200, ""},
		{"fraction", `{"health":7.5}`, 200, ""},
		{"integer as float", `{"health":4.0}`, 200, ""},
		{"negative", `{"health":-0.5}`, 400, "health"},
		{"above max", `{"health":11}`, 400, "health"},
		{"just above max", `{"health":10.5}`, 400, "health"},
		{"not a half step", `{"health":2.3}`, 400, "health"},
		{"quarter", `{"health":2.25}`, 400, "health"},
		{"missing", `{}`, 400, "health"},
		{"null", `{"health":null}`, 400, "health"},
		{"string", `{"health":"5"}`, 400, ""},
		{"unknown field", `{"health":5,"max":20}`, 400, ""},
		{"malformed json", `{"health":`, 400, ""},
		{"too large", `{"health":5` + strings.Repeat(" ", 2<<10) + `}`, 413, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, api, "1", http.MethodPut, "/api/healthbars/books", tc.body)
			if rec.Code != tc.code {
				t.Fatalf("code = %d, body %s", rec.Code, rec.Body)
			}
			if tc.code == http.StatusOK {
				return
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
}

func TestRejectedPutLeavesBarUntouched(t *testing.T) {
	api := testAPI()
	put(t, api, "1", "books", "6")
	if rec := put(t, api, "1", "books", "2.3"); rec.Code != http.StatusBadRequest {
		t.Fatalf("put: %d", rec.Code)
	}
	if got := decode[Healthbar](t, do(t, api, "1", http.MethodGet, "/api/healthbars/books", "")); got.Health != 6 {
		t.Errorf("health = %v, want 6", got.Health)
	}
}

func TestUnknownSectionIsNotFound(t *testing.T) {
	api := testAPI()
	for _, section := range []string{"nonsense", "Books", "books%27%3B--", "1"} {
		for _, method := range []string{http.MethodGet, http.MethodPut} {
			rec := do(t, api, "1", method, "/api/healthbars/"+section, `{"health":5}`)
			if rec.Code != http.StatusNotFound {
				t.Errorf("%s %s = %d, want 404", method, section, rec.Code)
			}
			if decode[struct {
				Error string `json:"error"`
			}](t, rec).Error == "" {
				t.Errorf("%s %s: missing error message", method, section)
			}
		}
	}
}

func TestAllSectionsAreServed(t *testing.T) {
	api := testAPI()
	for _, section := range sections {
		if rec := do(t, api, "1", http.MethodGet, "/api/healthbars/"+section, ""); rec.Code != http.StatusOK {
			t.Errorf("%s = %d", section, rec.Code)
		}
	}
}

func TestOwnershipIsolation(t *testing.T) {
	api := testAPI()
	put(t, api, "1", "books", "2")

	// User 2 starts with their own full bar and does not see user 1's value.
	if got := decode[Healthbar](t, do(t, api, "2", http.MethodGet, "/api/healthbars/books", "")); got.Health != 10 {
		t.Fatalf("user 2 sees health %v, want 10", got.Health)
	}

	// User 2 changing their bar does not affect user 1's.
	put(t, api, "2", "books", "9")
	if got := decode[Healthbar](t, do(t, api, "1", http.MethodGet, "/api/healthbars/books", "")); got.Health != 2 {
		t.Fatalf("user 1 health = %v, want 2", got.Health)
	}
}
