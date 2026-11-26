# Board of Life

A personal dashboard for tracking progress in different areas of life (reading, exercise, and so on). It starts small, with a book tracker, and is meant to grow one area at a time.

The app opens on a login page. After signing in, the login columns expand into the main board: four full-height columns, one per area. Clicking a column slides the others aside, accordion style, and opens that area's panel. Clicking the open column closes it. Only **Books** is implemented so far. The other three columns (Exercise, Learning, Mindfulness) are placeholders.

## Note

AI was extensively used to design this application. The code has been reviewed and
tested as much as possible, but I shouldn't be considered responsible for any problems that
might originate from running this code on your personal machine.

## Stack

| Part | Technology |
|---|---|
| Reverse proxy | nginx 1.27, serves everything on port 8585 |
| Frontend | Svelte 5 + Vite, built and served by its own nginx container |
| Backend | Go 1.27, Chi router, `database/sql` with pgx, `log/slog`, `encoding/json/v2` |
| Database | PostgreSQL 17 |
| Migrations | [goose](https://github.com/pressly/goose), SQL files embedded in the backend binary |

## Running it

Requires Docker with Compose.

```sh
docker compose up -d --build
```

Then open http://localhost:8585 and sign in with the local development account:

- username: `admin`
- password: `change-me-please`

These credentials are set in `docker-compose.yml` and are **for local development only**. The admin account is created on first start, only when the users table is empty.

To stop everything: `docker compose down`. The database lives in the `db-data` volume and survives restarts.

After changing code, run `docker compose up -d --build` again. The frontend is a production build with no hot reload, and the migrations are embedded in the backend binary, so a new migration file also needs a rebuild.

### Configuration

| Variable | Where | Purpose |
|---|---|---|
| `DATABASE_URL` | compose, backend | Postgres connection string |
| `ADMIN_USERNAME`, `ADMIN_PASSWORD` | compose, backend | First admin user, created only if no users exist |
| `COOKIE_SECURE` | compose, backend | Set `true` to mark the session cookie `Secure` (needed behind HTTPS). Defaults to `false` |
| `GOOGLE_BOOKS_API_KEY` | `.env`, optional | Enables authenticated Google Books lookups. See below |

Copy `.env.skel` to `.env` to set the optional key. `.env` is git-ignored and must never be committed.

The Postgres container is published on `localhost:54320` for development tools. Remove that mapping in `docker-compose.yml` if you don't want it.

## Project layout

```
docker-compose.yml     nginx, frontend, backend, db
nginx/nginx.conf       / -> frontend, /api/ -> backend (re-resolves names so restarts don't cause 502s)
backend/
  main.go, config.go, db.go
  migrations/          numbered goose SQL migrations, applied on startup
  docs/openapi.yaml    OpenAPI documentation for every /api endpoint
  internal/
    auth/              login, logout, sessions, requireAuth/requireRole middleware
    books/             books CRUD handlers, validation, store
    booksearch/        book metadata lookup (Google Books, Open Library fallback)
    ratelimit/         small in-memory per-user rate limiter
    httpx/             JSON helpers and error responses
frontend/src/
  App.svelte           session check, login/board switcher
  api.js               all API calls (relative paths, same-origin cookies)
  components/          Board, Login, Books, BookForm, BookCover, TitleCombobox,
                       StarRating, ProgressBar, Pagination
```

## Authentication and roles

- Cookie-based sessions. `POST /api/auth/login` sets an `HttpOnly`, `SameSite=Lax` cookie named `session`, valid for 7 days.
- Only a SHA-256 hash of the session token is stored in the database. Passwords are hashed with bcrypt.
- Roles are `admin` and `user`. Role checks exist as middleware for future endpoints.
- The frontend calls `GET /api/me` on load. A 200 shows the board and a 401 shows the login page. Any later 401 sends the user back to login.
- Every user only sees and modifies their own data. Requesting another user's book returns 404.
- There are no user-management endpoints yet. New users have to be inserted into the database directly.

## API

All endpoints are documented in [`backend/docs/openapi.yaml`](backend/docs/openapi.yaml). Every `/api/*` endpoint must be documented there in the same change that adds or alters it.

| Method | Path | Notes |
|---|---|---|
| GET | `/api/health` | Public |
| POST | `/api/auth/login` | Sets the session cookie |
| POST | `/api/auth/logout` | Clears the session, always 204 |
| GET | `/api/me` | Current user, or 401 Unauthorized |
| GET | `/api/books` | Filters `status`, `q`, `page`, `page_size`. Returns per-status `counts` |
| POST | `/api/books` | Create |
| GET | `/api/books/{id}` | Read |
| PUT | `/api/books/{id}` | Full replace. Omitted optional fields reset to defaults |
| DELETE | `/api/books/{id}` | 204 |
| GET | `/api/books/lookup` | Search book metadata (`q`, `limit`) |
| GET | `/api/healthbars/{section}` | Health 0 to 10 (steps of 0.5). Created at 10 on first read. Unknown section is 404 |
| PUT | `/api/healthbars/{section}` | Set health to an absolute value, e.g. `{"health": 7.5}` |

Errors are JSON: `{"error": "message"}`, and validation errors add `"fields": {"name": "message"}`.

## Books

Books have a status (`to_read`, `reading`, `finished`), a progress percentage (0 to 100), an optional rating (1 to 5 in 0.5 steps), and optional `cover_url`, `description` and `external_id`.

- A `to_read` book is forced to 0% progress and a `finished` book to 100%.
- Progress is a percentage, not pages, because page counts differ between editions.
- The UI has status tabs with counts, debounced search, pagination, and add, edit and delete forms. Cards show a cover (with a generated fallback), a progress bar, stars, and an expandable description.
- The add form has a title search. Picking a result fills in the title, author, cover and description. It is optional, and a failed search never blocks saving.

### Book search

`GET /api/books/lookup` queries Google Books first and falls back to Open Library when Google fails or returns nothing. Results are cached briefly, and each user is limited to 30 searches per minute.

Anonymous Google Books requests were returning 429 (quota 0) when this was last tested, so lookups were being served by Open Library. Open Library needs no key but returns no descriptions. For descriptions, create a free Google Books API key and set `GOOGLE_BOOKS_API_KEY` in `.env`. That path has not been tested against the real Google API.

Cover images are loaded by the browser directly from the cover host (Open Library or Google).

## Design

- Palette: Rosy Taupe `#9b514d`, Old Rose `#8e4168`, Amethyst Smoke `#783d78`, Dusty Lavender `#68466f`, Space Indigo `#3b3b58`, on white and light gray. These are brand colors, not status colors. Errors use a separate red with an icon and text.
- Fonts: Fraunces (headings) and Instrument Sans (body), both self-hosted.
- Accessibility: WCAG AA contrast, visible keyboard focus, status never shown by color alone, and `prefers-reduced-motion` respected.

## Conventions

- Migrations are numbered SQL files in `backend/migrations/`.
- Match the existing hand-written `database/sql` style. No ORM.
- Pin exact dependency versions.
- Before finishing backend changes, run `go build ./...` and `go vet ./...` in `backend/`. For the frontend, run `npm run build` in `frontend/`. Then run `docker compose config --quiet`.

## Known limitations and ideas

- The login, board and Books UI were verified by build and API checks only. They have not been reviewed visually in a browser.
- The rate limiter and lookup cache are in-memory, so they are per backend instance.
- The other three areas have no features yet.
- No user-management endpoints or UI.
- Ideas: Google Books key for descriptions, proxying cover images through the backend, a hot-reload dev setup for the frontend, and more areas such as exercise tracking.
