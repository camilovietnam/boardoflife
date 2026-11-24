package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const tokenBytes = 32

// Service holds the authentication use cases. It is independent of HTTP.
type Service struct {
	store Store
	ttl   time.Duration
	now   func() time.Time

	// dummyHash is verified against when the user is unknown so that unknown
	// users and wrong passwords take about the same time.
	dummyHash []byte
}

// NewService builds a Service whose sessions live for ttl.
func NewService(store Store, ttl time.Duration) (*Service, error) {
	dummy, err := bcrypt.GenerateFromPassword(randomBytes(tokenBytes), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("prepare dummy hash: %w", err)
	}
	return &Service{store: store, ttl: ttl, now: time.Now, dummyHash: dummy}, nil
}

// TTL is the lifetime of a session.
func (s *Service) TTL() time.Duration { return s.ttl }

// Login verifies the credentials and opens a new session.
func (s *Service) Login(ctx context.Context, username, password string) (User, Session, error) {
	creds, err := s.store.UserByUsername(ctx, username)
	switch {
	case errors.Is(err, ErrNotFound):
		_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password))
		return User{}, Session{}, ErrInvalidCredentials
	case err != nil:
		return User{}, Session{}, fmt.Errorf("find user: %w", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(creds.PasswordHash), []byte(password)) != nil {
		return User{}, Session{}, ErrInvalidCredentials
	}

	token := base64.RawURLEncoding.EncodeToString(randomBytes(tokenBytes))
	expiresAt := s.now().Add(s.ttl)
	if err := s.store.CreateSession(ctx, hashToken(token), creds.ID, expiresAt); err != nil {
		return User{}, Session{}, fmt.Errorf("create session: %w", err)
	}
	return creds.User, Session{Token: token, ExpiresAt: expiresAt}, nil
}

// Authenticate resolves a session token to its user.
func (s *Service) Authenticate(ctx context.Context, token string) (User, error) {
	if token == "" {
		return User{}, ErrUnauthenticated
	}
	user, err := s.store.UserBySession(ctx, hashToken(token), s.now())
	switch {
	case errors.Is(err, ErrNotFound):
		return User{}, ErrUnauthenticated
	case err != nil:
		return User{}, fmt.Errorf("find session: %w", err)
	}
	return user, nil
}

// Logout destroys the session. It is idempotent.
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.store.DeleteSession(ctx, hashToken(token))
}

// PurgeExpired removes sessions that can no longer be used.
func (s *Service) PurgeExpired(ctx context.Context) (int64, error) {
	return s.store.DeleteExpiredSessions(ctx, s.now())
}

// BootstrapAdmin creates the first admin account when no user exists yet.
// It reports whether an account was created.
func (s *Service) BootstrapAdmin(ctx context.Context, username, password string) (bool, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return false, fmt.Errorf("hash admin password: %w", err)
	}
	return s.store.CreateFirstUser(ctx, username, string(hash), RoleAdmin)
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing means the OS entropy source is broken.
		panic(fmt.Sprintf("crypto/rand: %v", err))
	}
	return b
}
