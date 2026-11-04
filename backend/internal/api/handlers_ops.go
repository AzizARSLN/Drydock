package api

import (
	"context"
	"net/http"
	"time"

	"github.com/AzizARSLN/drydock/internal/audit"
	"github.com/AzizARSLN/drydock/internal/models"
	"github.com/AzizARSLN/drydock/internal/modules"
	"github.com/AzizARSLN/drydock/internal/sshx"
)

// dial opens a connection for the {id} route server and writes an error response
// when it cannot be established.
func (s *Server) dial(w http.ResponseWriter, r *http.Request, timeout time.Duration) (*sshx.Client, *models.Server, context.Context, context.CancelFunc, bool) {
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	client, sv, err := s.App.Connect(ctx, pathID(r))
	if err != nil {
		cancel()
		fail(w, http.StatusBadGateway, err.Error())
		return nil, sv, ctx, cancel, false
	}
	return client, sv, ctx, cancel, true
}

// statusFor maps a dry-run flag to the audit status.
func statusFor(dryRun bool) string {
	if dryRun {
		return audit.StatusDryRun
	}
	return audit.StatusSuccess
}

// --- Nginx -----------------------------------------------------------------

func (s *Server) handleNginxOverview(w http.ResponseWriter, r *http.Request) {
	client, sv, ctx, cancel, ok := s.dial(w, r, 30*time.Second)
	if !ok {
		return
	}
	defer cancel()
	defer client.Close()

	installed, version := modules.NginxInstalled(ctx, client)
	resp := map[string]any{
		"installed": installed,
		"version":   version,
		"state":     modules.NginxStatus(ctx, client),
		"sites":     []modules.NginxSite{},
	}
	if installed {
		sites, err := modules.ListNginxConfigs(ctx, client)
		if err != nil {
			fail(w, http.StatusBadGateway, err.Error())
			return
		}
		resp["sites"] = sites
	}
	s.audit(r, sv.ID, sv.Name, "nginx-overview", "", "nginx durumu görüntülendi", audit.StatusSuccess)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleNginxConfigGet(w http.ResponseWriter, r *http.Request) {
	client, sv, ctx, cancel, ok := s.dial(w, r, 30*time.Second)
	if !ok {
		return
	}
	defer cancel()
	defer client.Close()

	content, err := modules.ReadNginxConfig(ctx, client, r.URL.Query().Get("path"))
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r, sv.ID, sv.Name, "nginx-read", r.URL.Query().Get("path"), "yapılandırma okundu", audit.StatusSuccess)
	writeJSON(w, http.StatusOK, map[string]string{"path": r.URL.Query().Get("path"), "content": content})
}

type configSaveReq struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	DryRun  bool   `json:"dry_run"`
}

func (s *Server) handleNginxConfigSave(w http.ResponseWriter, r *http.Request) {
	var in configSaveReq
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, "geçersiz istek gövdesi")
		return
	}
	client, sv, ctx, cancel, ok := s.dial(w, r, 60*time.Second)
	if !ok {
		return
	}
	defer cancel()
	defer client.Close()

	out, err := modules.SaveNginxConfig(ctx, s.App, client, sv, actor(r).Username, in.Path, in.Content, in.DryRun)
	if err != nil {
		s.audit(r, sv.ID, sv.Name, "nginx-save", in.Path, err.Error(), audit.StatusError)
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": err.Error(), "outcome": out})
		return
	}
	s.audit(r, sv.ID, sv.Name, "nginx-save", in.Path, out.Message, statusFor(in.DryRun))
	writeJSON(w, http.StatusOK, out)
}

type siteActionReq struct {
	Name   string `json:"name"`
	DryRun bool   `json:"dry_run"`
}

func (s *Server) handleNginxEnable(w http.ResponseWriter, r *http.Request) {
	s.nginxSiteAction(w, r, true)
}

func (s *Server) handleNginxDisable(w http.ResponseWriter, r *http.Request) {
	s.nginxSiteAction(w, r, false)
}

func (s *Server) nginxSiteAction(w http.ResponseWriter, r *http.Request, enable bool) {
	var in siteActionReq
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, "geçersiz istek gövdesi")
		return
	}
	client, sv, ctx, cancel, ok := s.dial(w, r, 60*time.Second)
	if !ok {
		return
	}
	defer cancel()
	defer client.Close()

	var out *modules.Outcome
	var err error
	action := "nginx-disable"
	if enable {
		action = "nginx-enable"
		out, err = modules.EnableNginxSite(ctx, client, in.Name, in.DryRun)
	} else {
		out, err = modules.DisableNginxSite(ctx, client, in.Name, in.DryRun)
	}
	if err != nil {
		s.audit(r, sv.ID, sv.Name, action, in.Name, err.Error(), audit.StatusError)
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": err.Error(), "outcome": out})
		return
	}
	s.audit(r, sv.ID, sv.Name, action, in.Name, out.Message, statusFor(in.DryRun))
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleNginxTest(w http.ResponseWriter, r *http.Request) {
	client, sv, ctx, cancel, ok := s.dial(w, r, 30*time.Second)
	if !ok {
		return
	}
	defer cancel()
	defer client.Close()

	out, err := modules.TestNginx(ctx, client)
	status := audit.StatusSuccess
	if err != nil {
		status = audit.StatusError
	}
	s.audit(r, sv.ID, sv.Name, "nginx-test", "", out, status)
	writeJSON(w, http.StatusOK, map[string]any{"ok": err == nil, "output": out})
}

func (s *Server) handleNginxReload(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DryRun bool `json:"dry_run"`
	}
	_ = decode(r, &in)
	client, sv, ctx, cancel, ok := s.dial(w, r, 30*time.Second)
	if !ok {
		return
	}
	defer cancel()
	defer client.Close()

	out, err := modules.ReloadNginx(ctx, client, in.DryRun)
	if err != nil {
		s.audit(r, sv.ID, sv.Name, "nginx-reload", "", err.Error(), audit.StatusError)
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": err.Error(), "outcome": out})
		return
	}
	s.audit(r, sv.ID, sv.Name, "nginx-reload", "", out.Message, statusFor(in.DryRun))
	writeJSON(w, http.StatusOK, out)
}

// --- Certificates ----------------------------------------------------------

func (s *Server) handleCerts(w http.ResponseWriter, r *http.Request) {
	client, sv, ctx, cancel, ok := s.dial(w, r, 60*time.Second)
	if !ok {
		return
	}
	defer cancel()
	defer client.Close()

	sites, _ := modules.ListNginxConfigs(ctx, client)
	certs, err := modules.ListCertificates(ctx, client, sites)
	if err != nil {
		fail(w, http.StatusBadGateway, err.Error())
		return
	}
	s.audit(r, sv.ID, sv.Name, "certs-list", "", "sertifikalar listelendi", audit.StatusSuccess)
	writeJSON(w, http.StatusOK, certs)
}

func (s *Server) handleCertRenew(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Domain string `json:"domain"`
		DryRun bool   `json:"dry_run"`
	}
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, "geçersiz istek gövdesi")
		return
	}
	client, sv, ctx, cancel, ok := s.dial(w, r, 180*time.Second)
	if !ok {
		return
	}
	defer cancel()
	defer client.Close()

	out, err := modules.RenewCertificate(ctx, client, in.Domain, in.DryRun)
	if err != nil {
		s.audit(r, sv.ID, sv.Name, "cert-renew", in.Domain, err.Error(), audit.StatusError)
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": err.Error(), "outcome": out})
		return
	}
	s.audit(r, sv.ID, sv.Name, "cert-renew", in.Domain, out.Message, statusFor(in.DryRun))
	writeJSON(w, http.StatusOK, out)
}

// --- Services --------------------------------------------------------------

func (s *Server) handleServices(w http.ResponseWriter, r *http.Request) {
	client, sv, ctx, cancel, ok := s.dial(w, r, 45*time.Second)
	if !ok {
		return
	}
	defer cancel()
	defer client.Close()

	services, err := modules.ListServices(ctx, client, r.URL.Query().Get("kind"), r.URL.Query().Get("filter"))
	if err != nil {
		fail(w, http.StatusBadGateway, err.Error())
		return
	}
	_ = sv
	writeJSON(w, http.StatusOK, services)
}

func (s *Server) handleServiceAction(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Kind   string `json:"kind"`
		Name   string `json:"name"`
		Action string `json:"action"`
		DryRun bool   `json:"dry_run"`
	}
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, "geçersiz istek gövdesi")
		return
	}
	client, sv, ctx, cancel, ok := s.dial(w, r, 90*time.Second)
	if !ok {
		return
	}
	defer cancel()
	defer client.Close()

	out, err := modules.ServiceAction(ctx, client, in.Kind, in.Name, in.Action, in.DryRun)
	if err != nil {
		s.audit(r, sv.ID, sv.Name, "service-"+in.Action, in.Kind+"/"+in.Name, err.Error(), audit.StatusError)
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": err.Error(), "outcome": out})
		return
	}
	s.audit(r, sv.ID, sv.Name, "service-"+in.Action, in.Kind+"/"+in.Name, out.Message, statusFor(in.DryRun))
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleServiceUpdate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Kind        string `json:"kind"`
		Name        string `json:"name"`
		Image       string `json:"image"`
		ComposeFile string `json:"compose_file"`
		DryRun      bool   `json:"dry_run"`
	}
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, "geçersiz istek gövdesi")
		return
	}
	client, sv, ctx, cancel, ok := s.dial(w, r, 300*time.Second)
	if !ok {
		return
	}
	defer cancel()
	defer client.Close()

	out, err := modules.UpdateService(ctx, client, in.Kind, in.Name, in.Image, in.ComposeFile, in.DryRun)
	if err != nil {
		s.audit(r, sv.ID, sv.Name, "service-update", in.Kind+"/"+in.Name, err.Error(), audit.StatusError)
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": err.Error(), "outcome": out})
		return
	}
	s.audit(r, sv.ID, sv.Name, "service-update", in.Kind+"/"+in.Name, out.Message, statusFor(in.DryRun))
	writeJSON(w, http.StatusOK, out)
}

// handleComposeProjects scans the host for compose project folders so the UI
// can group containers by folder even when compose labels are missing.
func (s *Server) handleComposeProjects(w http.ResponseWriter, r *http.Request) {
	client, _, ctx, cancel, ok := s.dial(w, r, 90*time.Second)
	if !ok {
		return
	}
	defer cancel()
	defer client.Close()

	projects, err := modules.ListComposeProjects(ctx, client)
	if err != nil {
		fail(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

// handleComposeAction runs a project-level docker compose command in the
// project's root directory (pull / up -d / restart / down / ps / logs).
func (s *Server) handleComposeAction(w http.ResponseWriter, r *http.Request) {
	var in struct {
		WorkingDir  string `json:"working_dir"`
		ComposeFile string `json:"compose_file"`
		Action      string `json:"action"`
		DryRun      bool   `json:"dry_run"`
	}
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, "geçersiz istek gövdesi")
		return
	}
	client, sv, ctx, cancel, ok := s.dial(w, r, 300*time.Second)
	if !ok {
		return
	}
	defer cancel()
	defer client.Close()

	out, err := modules.ComposeProjectAction(ctx, client, in.WorkingDir, in.ComposeFile, in.Action, in.DryRun)
	if err != nil {
		s.audit(r, sv.ID, sv.Name, "compose-"+in.Action, in.WorkingDir, err.Error(), audit.StatusError)
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": err.Error(), "outcome": out})
		return
	}
	s.audit(r, sv.ID, sv.Name, "compose-"+in.Action, in.WorkingDir, out.Message, statusFor(in.DryRun))
	writeJSON(w, http.StatusOK, out)
}

// --- Ports -----------------------------------------------------------------

func (s *Server) handlePorts(w http.ResponseWriter, r *http.Request) {
	client, _, ctx, cancel, ok := s.dial(w, r, 30*time.Second)
	if !ok {
		return
	}
	defer cancel()
	defer client.Close()

	ports, err := modules.ListPorts(ctx, client)
	if err != nil {
		fail(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ports)
}

func (s *Server) handlePortChange(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Path    string `json:"path"`
		OldPort int    `json:"old_port"`
		NewPort int    `json:"new_port"`
		DryRun  bool   `json:"dry_run"`
	}
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, "geçersiz istek gövdesi")
		return
	}
	client, sv, ctx, cancel, ok := s.dial(w, r, 60*time.Second)
	if !ok {
		return
	}
	defer cancel()
	defer client.Close()

	out, err := modules.ChangeNginxListenPort(ctx, s.App, client, sv, actor(r).Username, in.Path, in.OldPort, in.NewPort, in.DryRun)
	if err != nil {
		s.audit(r, sv.ID, sv.Name, "port-change", in.Path, err.Error(), audit.StatusError)
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": err.Error(), "outcome": out})
		return
	}
	s.audit(r, sv.ID, sv.Name, "port-change", in.Path, out.Message, statusFor(in.DryRun))
	writeJSON(w, http.StatusOK, out)
}
