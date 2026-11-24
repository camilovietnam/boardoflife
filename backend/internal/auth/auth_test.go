package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// memStore is an in-memory Store for tests.
type memStore struct {
	users    map[string]Credentials // keyed by lower-case username
	sessions map[string]memSession  // keyed by string(tokenHash)
}

type memSession struct {
	user      User
	expiresAt time.Time
}

func newMemStore() *memStore {
	return &memStore{users: map[string]Credentials{}, sessions: map[string]memSession{}}
}

func (m *memStore) UserByUsername(_ context.Context, name string) (Credentials, error) {
	c, ok := m.users[strings.ToLower(name)]
	if !ok {
		return Credentials{}, ErrNotFound
	}
	return c, nil
}

func (m *memStore) CreateSession(_ context.Context, h []byte, uid int64, exp time.Time) error {
	for _, c := range m.users {
		if c.ID == uid {
			m.sessions[string(h)] = memSession{user: c.User, expiresAt: exp}
		}
	}
	return nil
}

func (m *memStore) UserBySession(_ context.Context, h []byte, now time.Time) (User, error) {
	s, ok := m.sessions[string(h)]
	if !ok || !s.expiresAt.After(now) {
		return User{}, ErrNotFound
	}
	return s.user, nil
}

func (m *memStore) DeleteSession(_ context.Context, h []byte) error {
	delete(m.sessions, string(h))
	return nil
}

func (m *memStore) DeleteExpiredSessions(context.Context, time.Time) (int64, error) { return 0, nil }

func (m *memStore) CreateFirstUser(_ context.Context, name, hash, role string) (bool, error) {
	if len(m.users) > 0 {
		return false, nil
	}
	m.users[strings.ToLower(name)] = Credentials{User: User{ID: 1, Username: name, Role: role}, PasswordHash: hash}
	return true, nil
}

func newTestService(t *testing.T) (*Service, *memStore) {
	t.Helper()
	store := newMemStore()
	svc, err := NewService(store, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if created, err := svc.BootstrapAdmin(context.Background(), "Admin", "s3cret"); err != nil || !created {
		t.Fatalf("bootstrap: created=%v err=%v", created, err)
	}
	return svc, store
}

func TestLoginAuthenticateLogout(t *testing.T) {
	ctx := context.Background()
	svc, store := newTestService(t)

	user, session, err := svc.Login(ctx, "ADMIN", "s3cret") // case-insensitive
	if err != nil {
		t.Fatal(err)
	}
	if user.Role != RoleAdmin {
		t.Fatalf("role = %q", user.Role)
	}
	if _, ok := store.sessions[session.Token]; ok {
		t.Fatal("raw token must not be stored")
	}

	if got, err := svc.Authenticate(ctx, session.Token); err != nil || got.ID != user.ID {
		t.Fatalf("authenticate: %v %v", got, err)
	}

	if err := svc.Logout(ctx, session.Token); err != nil {
		t.Fatal(err)
	}
	if err := svc.Logout(ctx, session.Token); err != nil {
		t.Fatalf("logout must be idempotent: %v", err)
	}
	if _, err := svc.Authenticate(ctx, session.Token); err != ErrUnauthenticated {
		t.Fatalf("after logout: %v", err)
	}
}

func TestLoginRejectsBadCredentials(t *testing.T) {
	svc, _ := newTestService(t)
	for _, tc := range []struct{ user, pass string }{
		{"admin", "wrong"},
		{"nobody", "s3cret"},
	} {
		if _, _, err := svc.Login(context.Background(), tc.user, tc.pass); err != ErrInvalidCredentials {
			t.Errorf("%s: err = %v", tc.user, err)
		}
	}
}

func TestSessionExpires(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	_, session, err := svc.Login(ctx, "admin", "s3cret")
	if err != nil {
		t.Fatal(err)
	}
	svc.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	if _, err := svc.Authenticate(ctx, session.Token); err != ErrUnauthenticated {
		t.Fatalf("expired session: %v", err)
	}
}

func TestBootstrapOnlyOnce(t *testing.T) {
	svc, _ := newTestService(t)
	created, err := svc.BootstrapAdmin(context.Background(), "other", "pw")
	if err != nil || created {
		t.Fatalf("second bootstrap: created=%v err=%v", created, err)
	}
}

func TestDummyHashIsValidBcrypt(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := bcrypt.Cost(svc.dummyHash); err != nil {
		t.Fatal(err)
	}
}

func TestMiddleware(t *testing.T) {
	svc, _ := newTestService(t)
	_, session, err := svc.Login(context.Background(), "admin", "s3cret")
	if err != nil {
		t.Fatal(err)
	}
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	call := func(h http.Handler, token string) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if token != "" {
			req.AddCookie(&http.Cookie{Name: CookieName, Value: token})
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	adminOnly := svc.RequireAuth(RequireRole(RoleAdmin)(ok))
	userOnly := svc.RequireAuth(RequireRole(RoleUser)(ok))

	if got := call(adminOnly, ""); got != http.StatusUnauthorized {
		t.Errorf("no cookie: %d", got)
	}
	if got := call(adminOnly, session.Token); got != http.StatusOK {
		t.Errorf("admin: %d", got)
	}
	if got := call(userOnly, session.Token); got != http.StatusForbidden {
		t.Errorf("wrong role: %d", got)
	}
}
