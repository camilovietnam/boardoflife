package healthbars

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"boardoflife/backend/internal/auth"
	"boardoflife/backend/internal/httpx"
)

const maxBody = 1 << 10 // 1 KiB

// Handler exposes the health bars over HTTP.
type Handler struct {
	store       Store
	requireAuth func(http.Handler) http.Handler
}

// NewHandler builds the HTTP handlers. requireAuth must reject anonymous
// requests and put the user in the context (auth.Service.RequireAuth).
func NewHandler(store Store, requireAuth func(http.Handler) http.Handler) *Handler {
	return &Handler{store: store, requireAuth: requireAuth}
}

// Routes registers the /healthbars endpoints on r, which is expected to be
// mounted under /api.
func (h *Handler) Routes(r chi.Router) {
	r.Route("/healthbars", func(r chi.Router) {
		r.Use(httpx.NoStore, h.requireAuth)
		r.Get("/{section}", h.get)
		r.Put("/{section}", h.put)
	})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context()) // requireAuth guarantees presence.
	section, ok := sectionParam(w, r)
	if !ok {
		return
	}
	rec, err := h.store.Get(r.Context(), user.ID, section)
	respond(w, rec, err, "get healthbar")
}

func (h *Handler) put(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	section, ok := sectionParam(w, r)
	if !ok {
		return
	}

	var in Input
	if !httpx.DecodeJSON(w, r, &in, maxBody) {
		return
	}
	halfPoints, err := in.Validate()
	var fields FieldErrors
	if errors.As(err, &fields) {
		httpx.WriteValidationError(w, fields)
		return
	}

	rec, err := h.store.Set(r.Context(), user.ID, section, halfPoints)
	respond(w, rec, err, "set healthbar")
}

// sectionParam returns the {section} path parameter. An unknown section is
// answered with 404; only whitelisted values ever reach the store.
func sectionParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	section := chi.URLParam(r, "section")
	if !ValidSection(section) {
		httpx.WriteError(w, http.StatusNotFound, "unknown section")
		return "", false
	}
	return section, true
}

func respond(w http.ResponseWriter, rec Record, err error, op string) {
	if err != nil {
		slog.Error(op, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toHealthbar(rec))
}
