
# Project Rules for Kiro

Persistent instructions for working in the kraken repository.
These are loaded automatically every session.

## Shell / File Safety

- Never run `rm` unless the target is directly in the repository root. 
  Never `rm` inside subdirectories, and never use recursive or force
  deletes (`rm -rf`, `rm -r`, `rm -f`) without explicit, per-command
  confirmation from me.
- Prefer moving files to a temp location over deleting them when unsure.


## Files Never to Read

- Never read `.env` or any `.env.*` file (except `.env.skel`, which is a
  safe template). These contain live credentials (DB passwords, the app API key).
- Never echo or log secret values. Refer to secrets by their key name only.

## Secrets Handling

- Never add debug logging that prints credentials, API keys, tokens, or full
  auth headers.
- When adding dependencies, pin exact versions.

## API Documentation (MANDATORY — do not skip)

Every `/api/*` endpoint MUST be documented in `backend/docs/openapi.yaml`.
This is not optional and is not "nice to have" — an endpoint without docs is
incomplete work. 

When you add, change, or remove an `/api/*` route (anywhere a
`r.Route("/api/...")`, `r.Get`, `r.Post`, `r.Put`, `r.Delete`, or
`register*Routes` helper is touched), in the SAME change you must:

1. Add/update the path under `paths:` with every method, parameter, and
   response code the handler actually returns.
2. Reference request/response bodies via `$ref` to schemas under
   `components/schemas` — add the schema if it does not exist, modeling it on
   the Go struct's JSON tags.
3. Add the `tag` under the top-level `tags:` section if it is new.

## Conventions

- Backend is Go (Go 1.27, Chi router, `encoding/json/v2`, `log/slog`).
  Match the existing hand-written `database/sql` style; do not introduce an ORM.
- Frontend is Svelte 5 + Vite.
- Database migrations are numbered SQL files in `backend/migrations/` except for the initial creation of a "schema_migrations" table which is found in `db/init`.
- After backend changes, run `go build ./...` and `go vet ./...` in
  `backend/` before reporting done.
- **Verification Checklist:** When implementing new features, run these in order:
  1. Backend: `go build ./...` and `go vet ./...` in `backend/`
  2. Frontend: `npm run build` in `frontend/`
  3. Compose: `docker compose config --quiet`