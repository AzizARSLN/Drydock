package api

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/AzizARSLN/drydock/internal/auth"
	"github.com/AzizARSLN/drydock/internal/models"
)

type ctxKey string

const ctxUserKey ctxKey = "drydock.user"

const tokenCookie = "drydock_token"

// tokenFromRequest resolves the session token from the cookie or the header.
func tokenFromRequest(r *http.Request) string {
	if c, err := r.Cookie(tokenCookie); err == nil && c.Value != "" {
		return c.Value
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}

// withAuth validates the session and injects the user into the request context.
func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := tokenFromRequest(r)
		if tok == "" {
			fail(w, http.StatusUnauthorized, "kimlik doğrulama gerekli")
			return
		}
		claims, err := s.App.Auth.Parse(tok)
		if err != nil {
			fail(w, http.StatusUnauthorized, "oturum geçersiz veya süresi dolmuş")
			return
		}
		user, err := s.App.Store.GetUserByID(claims.UserID)
		if err != nil || user == nil {
			fail(w, http.StatusUnauthorized, "kullanıcı bulunamadı")
			return
		}
		ctx := context.WithValue(r.Context(), ctxUserKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireRole rejects callers below the requested privilege level.
func (s *Server) requireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := actor(r)
			if u == nil || !auth.Can(u.Role, role) {
				fail(w, http.StatusForbidden, "bu işlem için yetkiniz yok")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// chain applies middleware in the order supplied.
func chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// clientIP extracts the best-effort origin address for audit records.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// withCORS allows the configured development origins to call the API.
func (s *Server) withCORS(next http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range s.App.Cfg.CORSOrigins {
		allowed[strings.TrimSpace(o)] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// audit records an action performed by the current caller.
func (s *Server) audit(r *http.Request, serverID int64, serverName, action, target, detail, status string) {
	u := actor(r)
	e := models.AuditEntry{
		ServerID:   serverID,
		ServerName: serverName,
		Action:     action,
		Target:     target,
		Detail:     detail,
		Status:     status,
		IP:         clientIP(r),
	}
	if u != nil {
		e.UserID = u.ID
		e.Username = u.Username
	}
	s.Audit.Record(e)
}
