package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/AzizARSLN/drydock/internal/models"
)

// Routes builds the complete HTTP handler for the Drydock API and web UI.
func (s *Server) Routes(webDir string) http.Handler {
	mux := http.NewServeMux()

	open := func(h http.HandlerFunc) http.Handler { return h }
	authed := func(h http.HandlerFunc) http.Handler { return s.withAuth(h) }
	operator := func(h http.HandlerFunc) http.Handler {
		return chain(h, s.withAuth, s.requireRole(models.RoleOperator))
	}
	admin := func(h http.HandlerFunc) http.Handler {
		return chain(h, s.withAuth, s.requireRole(models.RoleAdmin))
	}

	// --- Public -----------------------------------------------------------
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": Version})
	})
	mux.Handle("GET /api/meta", open(s.handleMeta))
	mux.Handle("GET /api/setup/status", open(s.handleSetupStatus))
	mux.Handle("POST /api/setup", open(s.handleSetup))
	mux.Handle("POST /api/auth/login", open(s.handleLogin))

	// --- Session ----------------------------------------------------------
	mux.Handle("GET /api/auth/me", authed(s.handleMe))
	mux.Handle("POST /api/auth/logout", authed(s.handleLogout))
	mux.Handle("POST /api/auth/password", authed(s.handleChangePassword))

	// --- Servers ----------------------------------------------------------
	mux.Handle("GET /api/servers", authed(s.handleListServers))
	mux.Handle("GET /api/servers/{id}", authed(s.handleGetServer))
	mux.Handle("POST /api/servers", admin(s.handleCreateServer))
	mux.Handle("PUT /api/servers/{id}", admin(s.handleUpdateServer))
	mux.Handle("DELETE /api/servers/{id}", admin(s.handleDeleteServer))
	mux.Handle("POST /api/servers/{id}/test", operator(s.handleTestServer))
	mux.Handle("GET /api/servers/{id}/health", authed(s.handleServerHealth))

	// --- Nginx ------------------------------------------------------------
	mux.Handle("GET /api/servers/{id}/nginx", authed(s.handleNginxOverview))
	mux.Handle("GET /api/servers/{id}/nginx/config", authed(s.handleNginxConfigGet))
	mux.Handle("POST /api/servers/{id}/nginx/config", operator(s.handleNginxConfigSave))
	mux.Handle("POST /api/servers/{id}/nginx/enable", operator(s.handleNginxEnable))
	mux.Handle("POST /api/servers/{id}/nginx/disable", operator(s.handleNginxDisable))
	mux.Handle("POST /api/servers/{id}/nginx/test", operator(s.handleNginxTest))
	mux.Handle("POST /api/servers/{id}/nginx/reload", operator(s.handleNginxReload))

	// --- Certificates -----------------------------------------------------
	mux.Handle("GET /api/servers/{id}/certs", authed(s.handleCerts))
	mux.Handle("POST /api/servers/{id}/certs/renew", operator(s.handleCertRenew))

	// --- Services ---------------------------------------------------------
	mux.Handle("GET /api/servers/{id}/services", authed(s.handleServices))
	mux.Handle("GET /api/servers/{id}/compose", authed(s.handleComposeProjects))
	mux.Handle("POST /api/servers/{id}/apps/deploy", operator(s.handleDeployApp))
	mux.Handle("POST /api/servers/{id}/services/action", operator(s.handleServiceAction))
	mux.Handle("POST /api/servers/{id}/services/update", operator(s.handleServiceUpdate))
	mux.Handle("POST /api/servers/{id}/services/compose", operator(s.handleComposeAction))

	// --- Ports ------------------------------------------------------------
	mux.Handle("GET /api/servers/{id}/ports", authed(s.handlePorts))
	mux.Handle("POST /api/servers/{id}/ports/change", operator(s.handlePortChange))

	// --- Files ------------------------------------------------------------
	mux.Handle("GET /api/servers/{id}/files", authed(s.handleFilesList))
	mux.Handle("GET /api/servers/{id}/files/content", authed(s.handleFileContent))
	mux.Handle("GET /api/servers/{id}/files/download", authed(s.handleFileDownload))
	mux.Handle("POST /api/servers/{id}/files/content", operator(s.handleFileSave))
	mux.Handle("POST /api/servers/{id}/files/mkdir", operator(s.handleFileMkdir))
	mux.Handle("POST /api/servers/{id}/files/move", operator(s.handleFileMove))
	mux.Handle("POST /api/servers/{id}/files/delete", operator(s.handleFileDelete))
	mux.Handle("POST /api/servers/{id}/files/upload", operator(s.handleFileUpload))

	// --- Snapshots --------------------------------------------------------
	mux.Handle("GET /api/servers/{id}/snapshots", authed(s.handleSnapshots))
	mux.Handle("POST /api/servers/{id}/snapshots/restore", operator(s.handleSnapshotRestore))

	// --- Terminal (WebSocket) --------------------------------------------
	mux.Handle("GET /api/servers/{id}/terminal", authed(s.handleTerminal))

	// --- Audit ------------------------------------------------------------
	mux.Handle("GET /api/audit", authed(s.handleAuditList))

	// --- Users ------------------------------------------------------------
	mux.Handle("GET /api/users", admin(s.handleListUsers))
	mux.Handle("POST /api/users", admin(s.handleCreateUser))
	mux.Handle("PUT /api/users/{id}", admin(s.handleUpdateUser))
	mux.Handle("DELETE /api/users/{id}", admin(s.handleDeleteUser))

	// --- Static web UI ----------------------------------------------------
	if webDir != "" {
		mux.Handle("/", spaHandler(webDir))
	}

	return s.withCORS(mux)
}

// spaHandler serves the built React application with history fallback.
func spaHandler(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	index := filepath.Join(dir, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		clean := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		target := filepath.Join(dir, clean)
		if info, err := os.Stat(target); err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			fail(w, http.StatusNotFound, "uç nokta bulunamadı")
			return
		}
		http.ServeFile(w, r, index)
	})
}
