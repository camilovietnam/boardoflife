package healthbars

import (
	"context"
	"database/sql"
)

// SQLStore is the database/sql implementation of Store.
type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore { return &SQLStore{db: db} }

func (s *SQLStore) Get(ctx context.Context, userID int64, section string) (Record, error) {
	const insert = `
		INSERT INTO healthbars (user_id, section, health)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, section) DO NOTHING`
	if _, err := s.db.ExecContext(ctx, insert, userID, section, maxHalfPoints); err != nil {
		return Record{}, err
	}

	const query = `SELECT health, last_updated FROM healthbars WHERE user_id = $1 AND section = $2`
	var r Record
	err := s.db.QueryRowContext(ctx, query, userID, section).Scan(&r.HalfPoints, &r.LastUpdated)
	return r, err
}

func (s *SQLStore) Set(ctx context.Context, userID int64, section string, halfPoints int) (Record, error) {
	const q = `
		INSERT INTO healthbars (user_id, section, health)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, section)
		DO UPDATE SET health = EXCLUDED.health, last_updated = now()
		RETURNING health, last_updated`
	var r Record
	err := s.db.QueryRowContext(ctx, q, userID, section, halfPoints).Scan(&r.HalfPoints, &r.LastUpdated)
	return r, err
}
