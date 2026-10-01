package server

import (
	"encoding/json"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"name":    "Overview",
		"version": s.opts.Version,
	})
}

func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	nodes, err := s.svc.Tree(r.Context())
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"nodes": nodes})
}

func (s *Server) handleGetNote(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	note, err := s.svc.GetNote(r.Context(), rel)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	w.Header().Set("ETag", etag(note.Version))
	writeJSON(w, http.StatusOK, note)
}

type saveNoteRequest struct {
	Path        string `json:"path"`
	Body        string `json:"body"`
	BaseVersion string `json:"baseVersion"`
}

func (s *Server) handleSaveNote(w http.ResponseWriter, r *http.Request) {
	var req saveNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	expected := req.BaseVersion
	if im := r.Header.Get("If-Match"); im != "" {
		expected = trimETag(im)
	}
	note, err := s.svc.SaveNote(r.Context(), req.Path, req.Body, expected)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	w.Header().Set("ETag", etag(note.Version))
	writeJSON(w, http.StatusOK, note)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	if err := s.svc.Delete(r.Context(), rel); err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

type pathRequest struct {
	Path string `json:"path"`
}

func (s *Server) handleCreateFolder(w http.ResponseWriter, r *http.Request) {
	var req pathRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := s.svc.Mkdir(r.Context(), req.Path); err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": req.Path})
}

type renameRequest struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	var req renameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := s.svc.Move(r.Context(), req.From, req.To); err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"from": req.From, "to": req.To})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	results, err := s.svc.Search(r.Context(), q, limit, offset)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (s *Server) handleLinks(w http.ResponseWriter, r *http.Request) {
	result, err := s.svc.Links(r.Context(), r.URL.Query().Get("path"))
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleResolve(w http.ResponseWriter, r *http.Request) {
	meta, err := s.svc.ResolveLink(r.Context(), r.URL.Query().Get("target"))
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

func (s *Server) handleUploadAsset(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.opts.MaxUploadBytes)
	if err := r.ParseMultipartForm(s.opts.MaxUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or malformed form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	asset, err := s.svc.Upload(r.Context(), header.Filename, file)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, asset)
}

func (s *Server) handleAsset(w http.ResponseWriter, r *http.Request) {
	rel := r.PathValue("path")
	reader, err := s.svc.OpenAsset(r.Context(), rel)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	defer reader.Close()

	ct := mime.TypeByExtension(path.Ext(rel))
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if !isInlineType(ct) {
		w.Header().Set("Content-Disposition", "attachment")
	}
	http.ServeContent(w, r, path.Base(rel), time.Time{}, reader)
}

func (s *Server) handleReindex(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.Reindex(r.Context()); err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"reindexed": true})
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func etag(version string) string {
	return `"` + version + `"`
}

func trimETag(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "W/")
	return strings.Trim(v, `"`)
}

func isInlineType(ct string) bool {
	if strings.HasPrefix(ct, "image/") && ct != "image/svg+xml" {
		return true
	}
	return false
}
