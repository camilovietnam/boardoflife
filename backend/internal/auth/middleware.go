package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"

	"boardoflife/backend/internal/httpx"
)

// CookieName is the name of the session cookie.
const CookieName = "session"

type ctxKey struct{}

// UserFrom returns the authenticated user placed in ctx by RequireAuth.
func UserFrom(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(ctxKey{}).(User)
	return u, ok
}

// WithUser returns a copy of ctx carrying user, as RequireAuth does. It lets
// other packages' tests stand in for the real authentication middleware.
func WithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, ctxKey{}, user)
}

// RequireAuth rejects requests without a valid session (401) and otherwise
// stores the user in the request context.
func (s *Service) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var token string
		if c, err := r.Cookie(CookieName); err == nil {
			token = c.Value
		}

		user, err := s.Authenticate(r.Context(), token)
		switch {
		case errors.Is(err, ErrUnauthenticated):
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		case err != nil:
			slog.Error("authenticate request", "err", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
	})
}

// RequireRole answers 403 to authenticated users whose role is not listed.
// It must run after RequireAuth.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFrom(r.Context())
			if !ok {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			if !slices.Contains(roles, user.Role) {
				httpx.WriteError(w, http.StatusForbidden, "forbidden")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
