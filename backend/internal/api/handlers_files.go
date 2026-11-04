package api

import (
	"net/http"
	"time"

	"github.com/AzizARSLN/drydock/internal/audit"
	"github.com/AzizARSLN/drydock/internal/models"
	"github.com/AzizARSLN/drydock/internal/modules"
)

func (s *Server) handleFilesList(w http.ResponseWriter, r *http.Request) {
	client, _, ctx, cancel, ok := s.dial(w, r, 30*time.Second)
	if !ok {
		return
	}
	defer cancel()
	defer client.Close()

	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/etc"
	}
	entries, err := modules.ListFiles(ctx, client, path)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": path, "entries": entries})
}

func (s *Server) handleFileContent(w http.ResponseWriter, r *http.Request) {
	client, _, ctx, cancel, ok := s.dial(w, r, 30*time.Second)
	if !ok {
		return
	}
	defer cancel()
	defer client.Close()

	path := r.URL.Query().Get("path")
	content, err := modules.ReadFileContent(ctx, client, path)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": path, "content": content})
}

func (s *Server) handleFileSave(w http.ResponseWriter, r *http.Request) {
	var in configSaveReq
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, "geçersiz istek gövdesi")
		return
	}
	client, sv, ctx, cancel, ok := s.dial(w, r, 45*time.Second)
	if !ok {
		return
	}
	defer cancel()
	defer client.Close()

	out, err := modules.WriteFileContent(ctx, s.App, client, sv, actor(r).Username, in.Path, in.Content, in.DryRun)
	if err != nil {
		s.audit(r, sv.ID, sv.Name, "file-save", in.Path, err.Error(), audit.StatusError)
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": err.Error(), "outcome": out})
		return
	}
	s.audit(r, sv.ID, sv.Name, "file-save", in.Path, out.Message, statusFor(in.DryRun))
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleFileMkdir(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Path   string `json:"path"`
		DryRun bool   `json:"dry_run"`
	}
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, "geçersiz istek gövdesi")
		return
	}
	client, sv, ctx, cancel, ok := s.dial(w, r, 30*time.Second)
	if !ok {
		return
	}
	defer cancel()
	defer client.Close()

	out, err := modules.CreateDirectory(ctx, client, in.Path, in.DryRun)
	if err != nil {
		s.audit(r, sv.ID, sv.Name, "file-mkdir", in.Path, err.Error(), audit.StatusError)
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": err.Error(), "outcome": out})
		return
	}
	s.audit(r, sv.ID, sv.Name, "file-mkdir", in.Path, out.Message, statusFor(in.DryRun))
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleFileMove(w http.ResponseWriter, r *http.Request) {
	var in struct {
		From   string `json:"from"`
		To     string `json:"to"`
		DryRun bool   `json:"dry_run"`
	}
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, "geçersiz istek gövdesi")
		return
	}
	client, sv, ctx, cancel, ok := s.dial(w, r, 30*time.Second)
	if !ok {
		return
	}
	defer cancel()
	defer client.Close()

	out, err := modules.RenamePath(ctx, client, in.From, in.To, in.DryRun)
	if err != nil {
		s.audit(r, sv.ID, sv.Name, "file-move", in.From, err.Error(), audit.StatusError)
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": err.Error(), "outcome": out})
		return
	}
	s.audit(r, sv.ID, sv.Name, "file-move", in.From+" -> "+in.To, out.Message, statusFor(in.DryRun))
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleFileDelete(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
		DryRun    bool   `json:"dry_run"`
		Confirm   string `json:"confirm"`
	}
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, "geçersiz istek gövdesi")
		return
	}
	// Two-step confirmation: the caller must echo the final path segment.
	base := in.Path
	for i := len(base) - 1; i >= 0; i-- {
		if base[i] == '/' {
			base = base[i+1:]
			break
		}
	}
	if !in.DryRun && in.Confirm != base {
		fail(w, http.StatusBadRequest, "silme onayı için öğe adını aynen yazmalısınız")
		return
	}
	client, sv, ctx, cancel, ok := s.dial(w, r, 30*time.Second)
	if !ok {
		return
	}
	defer cancel()
	defer client.Close()

	out, err := modules.DeletePath(ctx, client, in.Path, in.Recursive, in.DryRun)
	if err != nil {
		s.audit(r, sv.ID, sv.Name, "file-delete", in.Path, err.Error(), audit.StatusError)
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": err.Error(), "outcome": out})
		return
	}
	s.audit(r, sv.ID, sv.Name, "file-delete", in.Path, out.Message, statusFor(in.DryRun))
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleFileDownload(w http.ResponseWriter, r *http.Request) {
	client, sv, ctx, cancel, ok := s.dial(w, r, 60*time.Second)
	if !ok {
		return
	}
	defer cancel()
	defer client.Close()

	data, err := modules.DownloadFile(ctx, client, r.URL.Query().Get("path"))
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r, sv.ID, sv.Name, "file-download", r.URL.Query().Get("path"), "dosya indirildi", audit.StatusSuccess)
	writeJSON(w, http.StatusOK, map[string]string{"path": r.URL.Query().Get("path"), "content_base64": data})
}

func (s *Server) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Path           string `json:"path"`
		ContentBase64  string `json:"content_base64"`
		DryRun         bool   `json:"dry_run"`
	}
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, "geçersiz istek gövdesi")
		return
	}
	client, sv, ctx, cancel, ok := s.dial(w, r, 120*time.Second)
	if !ok {
		return
	}
	defer cancel()
	defer client.Close()

	out, err := modules.UploadFile(ctx, client, in.Path, in.ContentBase64, in.DryRun)
	if err != nil {
		s.audit(r, sv.ID, sv.Name, "file-upload", in.Path, err.Error(), audit.StatusError)
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": err.Error(), "outcome": out})
		return
	}
	s.audit(r, sv.ID, sv.Name, "file-upload", in.Path, out.Message, statusFor(in.DryRun))
	writeJSON(w, http.StatusOK, out)
}

// --- Snapshots -------------------------------------------------------------

func (s *Server) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	list, err := s.App.Store.ListSnapshots(pathID(r), r.URL.Query().Get("path"))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []models.Snapshot{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleSnapshotRestore(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SnapshotID int64 `json:"snapshot_id"`
		DryRun     bool  `json:"dry_run"`
	}
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, "geçersiz istek gövdesi")
		return
	}
	snap, err := s.App.Store.GetSnapshot(in.SnapshotID)
	if err != nil || snap == nil {
		fail(w, http.StatusNotFound, "yedek bulunamadı")
		return
	}
	if snap.ServerID != pathID(r) {
		fail(w, http.StatusBadRequest, "yedek bu sunucuya ait değil")
		return
	}
	client, sv, ctx, cancel, ok := s.dial(w, r, 60*time.Second)
	if !ok {
		return
	}
	defer cancel()
	defer client.Close()

	current, _ := modules.ReadFileContent(ctx, client, snap.Path)
	out := &modules.Outcome{
		DryRun:  in.DryRun,
		Diff:    modules.UnifiedDiff(snap.Path, current, snap.Content),
		Message: "Yedek geri yüklenecek.",
	}
	if in.DryRun {
		s.audit(r, sv.ID, sv.Name, "snapshot-restore", snap.Path, "dry-run geri yükleme", audit.StatusDryRun)
		writeJSON(w, http.StatusOK, out)
		return
	}
	if snap.Kind == "nginx" {
		res, err := modules.SaveNginxConfig(ctx, s.App, client, sv, actor(r).Username, snap.Path, snap.Content, false)
		if err != nil {
			s.audit(r, sv.ID, sv.Name, "snapshot-restore", snap.Path, err.Error(), audit.StatusError)
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": err.Error(), "outcome": res})
			return
		}
		out = res
	} else {
		res, err := modules.WriteFileContent(ctx, s.App, client, sv, actor(r).Username, snap.Path, snap.Content, false)
		if err != nil {
			s.audit(r, sv.ID, sv.Name, "snapshot-restore", snap.Path, err.Error(), audit.StatusError)
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": err.Error(), "outcome": res})
			return
		}
		out = res
	}
	s.audit(r, sv.ID, sv.Name, "snapshot-restore", snap.Path, "yedek geri yüklendi", audit.StatusSuccess)
	writeJSON(w, http.StatusOK, out)
}
