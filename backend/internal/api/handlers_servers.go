package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/AzizARSLN/drydock/internal/audit"
	"github.com/AzizARSLN/drydock/internal/models"
	"github.com/AzizARSLN/drydock/internal/modules"
)

// handleListServers returns every managed host.
func (s *Server) handleListServers(w http.ResponseWriter, r *http.Request) {
	list, err := s.App.Store.ListServers()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []models.Server{}
	}
	writeJSON(w, http.StatusOK, list)
}

// handleGetServer returns a single host.
func (s *Server) handleGetServer(w http.ResponseWriter, r *http.Request) {
	sv, err := s.App.Store.GetServer(pathID(r))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if sv == nil {
		fail(w, http.StatusNotFound, "sunucu bulunamadı")
		return
	}
	writeJSON(w, http.StatusOK, sv)
}

// handleCreateServer registers a new host.
func (s *Server) handleCreateServer(w http.ResponseWriter, r *http.Request) {
	var in models.ServerInput
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, "geçersiz istek gövdesi")
		return
	}
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Host) == "" || strings.TrimSpace(in.Username) == "" {
		fail(w, http.StatusBadRequest, "ad, sunucu adresi ve kullanıcı adı zorunludur")
		return
	}
	sv, err := s.buildServer(&models.Server{}, in)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.App.Store.CreateServer(sv); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.audit(r, sv.ID, sv.Name, "server-create", sv.Host, "sunucu eklendi", audit.StatusSuccess)
	writeJSON(w, http.StatusCreated, sv)
}

// handleUpdateServer edits an existing host.
func (s *Server) handleUpdateServer(w http.ResponseWriter, r *http.Request) {
	sv, err := s.App.Store.GetServer(pathID(r))
	if err != nil || sv == nil {
		fail(w, http.StatusNotFound, "sunucu bulunamadı")
		return
	}
	var in models.ServerInput
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, "geçersiz istek gövdesi")
		return
	}
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Host) == "" || strings.TrimSpace(in.Username) == "" {
		fail(w, http.StatusBadRequest, "ad, sunucu adresi ve kullanıcı adı zorunludur")
		return
	}
	updated, err := s.buildServer(sv, in)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.App.Store.UpdateServer(updated); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.audit(r, updated.ID, updated.Name, "server-update", updated.Host, "sunucu güncellendi", audit.StatusSuccess)
	writeJSON(w, http.StatusOK, updated)
}

// handleDeleteServer removes a host.
func (s *Server) handleDeleteServer(w http.ResponseWriter, r *http.Request) {
	sv, _ := s.App.Store.GetServer(pathID(r))
	if sv == nil {
		fail(w, http.StatusNotFound, "sunucu bulunamadı")
		return
	}
	if err := s.App.Store.DeleteServer(sv.ID); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.audit(r, sv.ID, sv.Name, "server-delete", sv.Host, "sunucu silindi", audit.StatusSuccess)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleTestServer verifies connectivity and returns a short system banner.
func (s *Server) handleTestServer(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	client, sv, err := s.App.Connect(ctx, pathID(r))
	if err != nil {
		s.audit(r, pathID(r), "", "server-test", sv.Host, err.Error(), audit.StatusError)
		fail(w, http.StatusBadGateway, err.Error())
		return
	}
	defer client.Close()

	res, err := client.Run(ctx, "echo DRYDOCK_OK; hostname; uname -srm")
	if err != nil {
		fail(w, http.StatusBadGateway, err.Error())
		return
	}
	s.audit(r, sv.ID, sv.Name, "server-test", sv.Host, "bağlantı testi başarılı", audit.StatusSuccess)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          strings.Contains(res.Stdout, "DRYDOCK_OK"),
		"fingerprint": client.Fingerprint,
		"output":      strings.TrimSpace(res.Stdout),
	})
}

// handleServerHealth collects a health snapshot.
func (s *Server) handleServerHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()

	client, sv, err := s.App.Connect(ctx, pathID(r))
	if err != nil {
		writeJSON(w, http.StatusOK, &modules.HealthReport{
			ServerID: pathID(r), Reachable: false, Error: err.Error(),
			CheckedAt: time.Now().UTC().Format(time.RFC3339),
		})
		return
	}
	defer client.Close()
	writeJSON(w, http.StatusOK, modules.CollectHealth(ctx, client, sv.ID))
}

// buildServer merges request input onto a server record, encrypting secrets.
func (s *Server) buildServer(sv *models.Server, in models.ServerInput) (*models.Server, error) {
	sv.Name = strings.TrimSpace(in.Name)
	sv.Host = strings.TrimSpace(in.Host)
	sv.Username = strings.TrimSpace(in.Username)
	sv.Port = in.Port
	if sv.Port == 0 {
		sv.Port = 22
	}
	sv.AuthType = in.AuthType
	if sv.AuthType != "password" {
		sv.AuthType = "key"
	}
	sv.Tags = strings.TrimSpace(in.Tags)
	sv.Description = strings.TrimSpace(in.Description)

	enc := func(v, current string) (string, error) {
		if strings.TrimSpace(v) == "" {
			return current, nil
		}
		return s.App.Cipher.Encrypt(v)
	}
	var err error
	if sv.PrivateKeyEnc, err = enc(in.PrivateKey, sv.PrivateKeyEnc); err != nil {
		return nil, err
	}
	if sv.PasswordEnc, err = enc(in.Password, sv.PasswordEnc); err != nil {
		return nil, err
	}
	if sv.PassphraseEnc, err = enc(in.Passphrase, sv.PassphraseEnc); err != nil {
		return nil, err
	}
	if sv.SudoPasswordEnc, err = enc(in.SudoPassword, sv.SudoPasswordEnc); err != nil {
		return nil, err
	}
	if in.ResetHostKey {
		sv.HostKeyFingerprint = ""
	}
	return sv, nil
}
