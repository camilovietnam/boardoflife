// Package auth implements cookie-based sessions, login/logout and
// role-based access control.
package auth

import (
	"context"
	"errors"
	"time"
)

// Role names seeded by the migrations.
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

var (
	// ErrInvalidCredentials is returned for an unknown user or a wrong password.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrUnauthenticated is returned when a session token is missing, unknown or expired.
	ErrUnauthenticated = errors.New("unauthenticated")
	// ErrNotFound is returned by a Store when a row does not exist.
	ErrNotFound = errors.New("not found")
)

// User is the public view of an account.
type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// Credentials couples a user with the bcrypt hash used to verify a login.
type Credentials struct {
	User
	PasswordHash string
}

// Session is a freshly created login session. Token is the raw secret that
// goes into the cookie; only its hash is persisted.
type Session struct {
	Token     string
	ExpiresAt time.Time
}

// Store is the persistence port of the auth package.
type Store interface {
	// UserByUsername looks a user up case-insensitively.
	UserByUsername(ctx context.Context, username string) (Credentials, error)
	CreateSession(ctx context.Context, tokenHash []byte, userID int64, expiresAt time.Time) error
	// UserBySession returns the owner of a session that has not expired at now.
	UserBySession(ctx context.Context, tokenHash []byte, now time.Time) (User, error)
	DeleteSession(ctx context.Context, tokenHash []byte) error
	DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error)
	// CreateFirstUser inserts the user only if the users table is empty and
	// reports whether it did so. The check and insert are atomic.
	CreateFirstUser(ctx context.Context, username, passwordHash, role string) (bool, error)
}
