package api

import (
	"net/http"
	"strings"

	"github.com/AzizARSLN/drydock/internal/auth"
	"github.com/AzizARSLN/drydock/internal/models"
	"github.com/AzizARSLN/drydock/internal/modules"
)

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

type passwordChange struct {
	Current string `json:"current"`
	New     string `json:"new"`
}

func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"name":     "Drydock",
		"version":  Version,
		"terminal": s.App.Cfg.EnableTerminal,
		"sftp":     s.App.Cfg.EnableSFTP,
		"fsscope":  modules.DefaultFSScope,
	})
}

// handleSetupStatus reports whether Drydock still needs its first administrator.
func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	n, err := s.App.Store.CountUsers()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"needs_setup": n == 0})
}

// handleSetup creates the initial administrator account.
func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	n, err := s.App.Store.CountUsers()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n > 0 {
		fail(w, http.StatusConflict, "kurulum zaten tamamlanmış")
		return
	}
	var in credentials
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, "geçersiz istek gövdesi")
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	if len(in.Username) < 3 || len(in.Password) < 8 {
		fail(w, http.StatusBadRequest, "kullanıcı adı en az 3, şifre en az 8 karakter olmalı")
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	user := &models.User{Username: in.Username, PasswordHash: hash, Role: models.RoleAdmin, FullName: in.FullName}
	if err := s.App.Store.CreateUser(user); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.audit(r, 0, "", "setup", user.Username, "ilk yönetici hesabı oluşturuldu", "success")
	writeJSON(w, http.StatusCreated, user)
}

// handleLogin authenticates a user and issues a session cookie.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, "geçersiz istek gövdesi")
		return
	}
	user, err := s.App.Store.GetUserByUsername(strings.TrimSpace(in.Username))
	if err != nil || user == nil || !auth.VerifyPassword(user.PasswordHash, in.Password) {
		s.audit(r, 0, "", "login", in.Username, "başarısız giriş denemesi", "error")
		fail(w, http.StatusUnauthorized, "kullanıcı adı veya şifre hatalı")
		return
	}
	token, exp, err := s.App.Auth.Issue(user)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     tokenCookie,
		Value:    token,
		Path:     "/",
		Expires:  exp,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
	_ = s.App.Store.TouchLastLogin(user.ID)
	s.audit(r, 0, "", "login", user.Username, "giriş başarılı", "success")
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "expires_at": exp})
}

// handleLogout clears the session cookie.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: tokenCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil,
	})
	if u := actor(r); u != nil {
		s.audit(r, 0, "", "logout", u.Username, "oturum kapatıldı", "success")
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleMe returns the current session user.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, actor(r))
}

// handleChangePassword updates the caller's own password.
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var in passwordChange
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, "geçersiz istek gövdesi")
		return
	}
	u := actor(r)
	if !auth.VerifyPassword(u.PasswordHash, in.Current) {
		fail(w, http.StatusBadRequest, "mevcut şifre hatalı")
		return
	}
	if len(in.New) < 8 {
		fail(w, http.StatusBadRequest, "yeni şifre en az 8 karakter olmalı")
		return
	}
	hash, err := auth.HashPassword(in.New)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.App.Store.UpdatePassword(u.ID, hash); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.audit(r, 0, "", "password-change", u.Username, "şifre güncellendi", "success")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
