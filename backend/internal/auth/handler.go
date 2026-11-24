package auth

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"boardoflife/backend/internal/httpx"
)

const maxLoginBody = 4 << 10 // 4 KiB

// Handler exposes the auth service over HTTP.
type Handler struct {
	svc          *Service
	secureCookie bool
}

// NewHandler builds the HTTP handlers. secureCookie sets the Secure attribute
// on the session cookie and should be true whenever served over HTTPS.
func NewHandler(svc *Service, secureCookie bool) *Handler {
	return &Handler{svc: svc, secureCookie: secureCookie}
}

// Routes registers POST /auth/login, POST /auth/logout and GET /me on r,
// which is expected to be mounted under /api.
func (h *Handler) Routes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(httpx.NoStore)
		r.Post("/auth/login", h.login)
		r.Post("/auth/logout", h.logout)
		r.With(h.svc.RequireAuth).Get("/me", h.me)
	})
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !httpx.DecodeJSON(w, r, &req, maxLoginBody) {
		return
	}
	if req.Username == "" || req.Password == "" {
		httpx.WriteError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	user, session, err := h.svc.Login(r.Context(), req.Username, req.Password)
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		httpx.WriteError(w, http.StatusUnauthorized, "invalid username or password")
		return
	case err != nil:
		slog.Error("login", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	http.SetCookie(w, h.cookie(session.Token, int(h.svc.TTL().Seconds())))
	httpx.WriteJSON(w, http.StatusOK, user)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(CookieName); err == nil {
		if err := h.svc.Logout(r.Context(), c.Value); err != nil {
			slog.Error("logout", "err", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
			return
		}
	}
	http.SetCookie(w, h.cookie("", -1))
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFrom(r.Context()) // RequireAuth guarantees presence.
	httpx.WriteJSON(w, http.StatusOK, user)
}

func (h *Handler) cookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
	}
}
