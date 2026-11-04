package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/AzizARSLN/drydock/internal/app"
	"github.com/AzizARSLN/drydock/internal/audit"
	"github.com/AzizARSLN/drydock/internal/models"
)

// Version is the Drydock release reported by the API.
const Version = "0.1.0"

// Server bundles the dependencies used by every HTTP handler.
type Server struct {
	App   *app.App
	Audit *audit.Logger
}

// NewServer builds the HTTP layer.
func NewServer(a *app.App, al *audit.Logger) *Server {
	return &Server{App: a, Audit: al}
}

// writeJSON serializes v as a JSON response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// fail writes a structured error payload.
func fail(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// decode parses a JSON request body into dst.
func decode(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func queryInt(r *http.Request, key string, def int64) int64 {
	if v := r.URL.Query().Get(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func queryBool(r *http.Request, key string) bool {
	v := r.URL.Query().Get(key)
	return v == "1" || v == "true" || v == "yes"
}

// pathID extracts the {id} route parameter.
func pathID(r *http.Request) int64 {
	n, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return n
}

// actor returns the authenticated user attached by the auth middleware.
func actor(r *http.Request) *models.User {
	u, _ := r.Context().Value(ctxUserKey).(*models.User)
	return u
}
