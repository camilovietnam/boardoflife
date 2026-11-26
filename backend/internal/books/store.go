package books

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

const bookColumns = `id, name, author, status, progress, rating, cover_url, description, external_id, created_at, updated_at`

// SQLStore is the database/sql implementation of Store.
type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore { return &SQLStore{db: db} }

func (s *SQLStore) Create(ctx context.Context, userID int64, d Draft) (Book, error) {
	const q = `
		INSERT INTO books (user_id, name, author, status, progress, rating, cover_url, description, external_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING ` + bookColumns

	return scanBook(s.db.QueryRowContext(ctx, q,
		userID, d.Name, d.Author, d.Status, d.Progress, halfStars(d.Rating),
		nullString(d.CoverURL), nullString(d.Description), nullString(d.ExternalID)))
}

func (s *SQLStore) Get(ctx context.Context, userID, id int64) (Book, error) {
	const q = `SELECT ` + bookColumns + ` FROM books WHERE id = $1 AND user_id = $2`
	return scanBook(s.db.QueryRowContext(ctx, q, id, userID))
}

func (s *SQLStore) Update(ctx context.Context, userID, id int64, d Draft) (Book, error) {
	const q = `
		UPDATE books
		SET name = $3, author = $4, status = $5, progress = $6, rating = $7,
		    cover_url = $8, description = $9, external_id = $10, updated_at = now()
		WHERE id = $1 AND user_id = $2
		RETURNING ` + bookColumns

	return scanBook(s.db.QueryRowContext(ctx, q,
		id, userID, d.Name, d.Author, d.Status, d.Progress, halfStars(d.Rating),
		nullString(d.CoverURL), nullString(d.Description), nullString(d.ExternalID)))
}

func (s *SQLStore) Delete(ctx context.Context, userID, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM books WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLStore) List(ctx context.Context, userID int64, q ListQuery) (Page, error) {
	// Empty status / pattern disable the respective filter.
	pattern := ""
	if q.Search != "" {
		pattern = "%" + likeEscaper.Replace(q.Search) + "%"
	}
	const filter = `
		FROM books
		WHERE user_id = $1
		  AND ($2::text = '' OR status = $2)
		  AND ($3::text = '' OR name ILIKE $3 OR author ILIKE $3)`

	var page Page
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) `+filter, userID, q.Status, pattern).
		Scan(&page.Total); err != nil {
		return Page{}, err
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT `+bookColumns+filter+` ORDER BY updated_at DESC, id DESC LIMIT $4 OFFSET $5`,
		userID, q.Status, pattern, q.Limit, q.Offset)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()

	page.Items = []Book{}
	for rows.Next() {
		b, err := scanBook(rows)
		if err != nil {
			return Page{}, err
		}
		page.Items = append(page.Items, b)
	}
	if err := rows.Err(); err != nil {
		return Page{}, err
	}

	page.Counts, err = s.counts(ctx, userID)
	return page, err
}

func (s *SQLStore) counts(ctx context.Context, userID int64) (Counts, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT status, count(*) FROM books WHERE user_id = $1 GROUP BY status`, userID)
	if err != nil {
		return Counts{}, err
	}
	defer rows.Close()

	var c Counts
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return Counts{}, err
		}
		switch status {
		case StatusToRead:
			c.ToRead = n
		case StatusReading:
			c.Reading = n
		case StatusFinished:
			c.Finished = n
		}
	}
	return c, rows.Err()
}

// likeEscaper neutralizes the LIKE metacharacters (and the escape character
// itself) so user input is always matched literally.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

type rowScanner interface{ Scan(dest ...any) error }

func scanBook(row rowScanner) (Book, error) {
	var (
		b                              Book
		rating                         sql.NullInt16
		cover, description, externalID sql.NullString
	)
	err := row.Scan(&b.ID, &b.Name, &b.Author, &b.Status, &b.Progress, &rating,
		&cover, &description, &externalID, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Book{}, ErrNotFound
	}
	if err != nil {
		return Book{}, err
	}
	if rating.Valid {
		stars := float64(rating.Int16) / 2
		b.Rating = &stars
	}
	b.CoverURL, b.Description, b.ExternalID = stringPtr(cover), stringPtr(description), stringPtr(externalID)
	return b, nil
}

func nullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func stringPtr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	return &s.String
}

// halfStars converts a 1..5 star rating to its stored half-star value.
func halfStars(rating *float64) sql.NullInt16 {
	if rating == nil {
		return sql.NullInt16{}
	}
	return sql.NullInt16{Int16: int16(*rating * 2), Valid: true}
}
