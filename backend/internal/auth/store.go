package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// SQLStore is the database/sql implementation of Store.
type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore { return &SQLStore{db: db} }

func (s *SQLStore) UserByUsername(ctx context.Context, username string) (Credentials, error) {
	const q = `
		SELECT u.id, u.username, r.name, u.password_hash
		FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE lower(u.username) = lower($1)`

	var c Credentials
	err := s.db.QueryRowContext(ctx, q, username).Scan(&c.ID, &c.Username, &c.Role, &c.PasswordHash)
	return c, notFound(err)
}

func (s *SQLStore) CreateSession(ctx context.Context, tokenHash []byte, userID int64, expiresAt time.Time) error {
	const q = `INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`
	_, err := s.db.ExecContext(ctx, q, tokenHash, userID, expiresAt)
	return err
}

func (s *SQLStore) UserBySession(ctx context.Context, tokenHash []byte, now time.Time) (User, error) {
	const q = `
		SELECT u.id, u.username, r.name
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		JOIN roles r ON r.id = u.role_id
		WHERE s.token_hash = $1 AND s.expires_at > $2`

	var u User
	err := s.db.QueryRowContext(ctx, q, tokenHash, now).Scan(&u.ID, &u.Username, &u.Role)
	return u, notFound(err)
}

func (s *SQLStore) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

func (s *SQLStore) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= $1`, now)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *SQLStore) CreateFirstUser(ctx context.Context, username, passwordHash, role string) (bool, error) {
	const q = `
		INSERT INTO users (username, password_hash, role_id)
		SELECT $1, $2, r.id FROM roles r
		WHERE r.name = $3 AND NOT EXISTS (SELECT 1 FROM users)`

	res, err := s.db.ExecContext(ctx, q, username, passwordHash, role)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
