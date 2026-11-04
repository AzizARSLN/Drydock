package api

import (
	"net/http"

	"github.com/AzizARSLN/drydock/internal/audit"
	"github.com/AzizARSLN/drydock/internal/auth"
	"github.com/AzizARSLN/drydock/internal/models"
)

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	list, err := s.App.Store.ListUsers()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []models.User{}
	}
	writeJSON(w, http.StatusOK, list)
}

type userCreateReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
	FullName string `json:"full_name"`
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var in userCreateReq
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, "geçersiz istek gövdesi")
		return
	}
	if len(in.Username) < 3 || len(in.Password) < 8 {
		fail(w, http.StatusBadRequest, "kullanıcı adı en az 3, şifre en az 8 karakter olmalı")
		return
	}
	if !auth.ValidRole(in.Role) {
		fail(w, http.StatusBadRequest, "geçersiz rol")
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	u := &models.User{Username: in.Username, PasswordHash: hash, Role: in.Role, FullName: in.FullName}
	if err := s.App.Store.CreateUser(u); err != nil {
		fail(w, http.StatusConflict, "kullanıcı oluşturulamadı: "+err.Error())
		return
	}
	s.audit(r, 0, "", "user-create", u.Username, "kullanıcı oluşturuldu ("+u.Role+")", audit.StatusSuccess)
	writeJSON(w, http.StatusCreated, u)
}

type userUpdateReq struct {
	Role     string `json:"role"`
	FullName string `json:"full_name"`
	Password string `json:"password"`
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	target, err := s.App.Store.GetUserByID(pathID(r))
	if err != nil || target == nil {
		fail(w, http.StatusNotFound, "kullanıcı bulunamadı")
		return
	}
	var in userUpdateReq
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, "geçersiz istek gövdesi")
		return
	}
	if in.Role != "" && !auth.ValidRole(in.Role) {
		fail(w, http.StatusBadRequest, "geçersiz rol")
		return
	}
	// Prevent locking out the last administrator.
	if target.Role == models.RoleAdmin && in.Role != "" && in.Role != models.RoleAdmin {
		if admins, _ := countAdmins(s, target.ID); admins == 0 {
			fail(w, http.StatusBadRequest, "son yöneticinin rolü düşürülemez")
			return
		}
	}
	role := target.Role
	if in.Role != "" {
		role = in.Role
	}
	if err := s.App.Store.UpdateUser(target.ID, role, in.FullName); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if in.Password != "" {
		if len(in.Password) < 8 {
			fail(w, http.StatusBadRequest, "şifre en az 8 karakter olmalı")
			return
		}
		hash, err := auth.HashPassword(in.Password)
		if err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = s.App.Store.UpdatePassword(target.ID, hash)
	}
	s.audit(r, 0, "", "user-update", target.Username, "kullanıcı güncellendi", audit.StatusSuccess)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	target, err := s.App.Store.GetUserByID(pathID(r))
	if err != nil || target == nil {
		fail(w, http.StatusNotFound, "kullanıcı bulunamadı")
		return
	}
	if me := actor(r); me != nil && me.ID == target.ID {
		fail(w, http.StatusBadRequest, "kendi hesabınızı silemezsiniz")
		return
	}
	if target.Role == models.RoleAdmin {
		if admins, _ := countAdmins(s, target.ID); admins == 0 {
			fail(w, http.StatusBadRequest, "son yönetici silinemez")
			return
		}
	}
	if err := s.App.Store.DeleteUser(target.ID); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.audit(r, 0, "", "user-delete", target.Username, "kullanıcı silindi", audit.StatusSuccess)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// countAdmins counts admin accounts excluding the given id.
func countAdmins(s *Server, excludeID int64) (int, error) {
	list, err := s.App.Store.ListUsers()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, u := range list {
		if u.Role == models.RoleAdmin && u.ID != excludeID {
			n++
		}
	}
	return n, nil
}

func (s *Server) handleAuditList(w http.ResponseWriter, r *http.Request) {
	list, err := s.App.Store.ListAudit(queryInt(r, "limit", 200), queryInt(r, "server_id", 0))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []models.AuditEntry{}
	}
	writeJSON(w, http.StatusOK, list)
}
