// Package httpx holds small HTTP helpers shared by all handlers: JSON
// responses, JSON error bodies and bounded JSON request decoding.
package httpx

import (
	"encoding/json/v2"
	"errors"
	"log/slog"
	"net/http"
)

// ErrorBody is the JSON shape of every error response.
type ErrorBody struct {
	Error string `json:"error"`
}

// ValidationBody is the JSON shape of a 400 caused by invalid fields.
type ValidationBody struct {
	Error  string            `json:"error"`
	Fields map[string]string `json:"fields"`
}

// WriteJSON writes v as a JSON response with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.MarshalWrite(w, v); err != nil {
		slog.Error("write json response", "err", err)
	}
}

// WriteError writes {"error": message} with the given status.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, ErrorBody{Error: message})
}

// WriteValidationError writes a 400 listing what is wrong per field.
func WriteValidationError(w http.ResponseWriter, fields map[string]string) {
	WriteJSON(w, http.StatusBadRequest, ValidationBody{Error: "validation failed", Fields: fields})
}

// DecodeJSON reads a single JSON object of at most maxBytes into dst.
// Unknown members are rejected. On failure it writes the error response
// (400, or 413 when the body is too large) and returns false.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	err := json.UnmarshalRead(r.Body, dst, json.RejectUnknownMembers(true))
	if err == nil {
		return true
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		WriteError(w, http.StatusRequestEntityTooLarge, "request body too large")
	} else {
		WriteError(w, http.StatusBadRequest, "malformed JSON body")
	}
	return false
}

// NoStore marks responses as uncacheable; use it on anything user-specific.
func NoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
