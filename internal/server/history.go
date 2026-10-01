package server

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleHistoryList(w http.ResponseWriter, r *http.Request) {
	revisions, err := s.svc.Revisions(r.Context(), r.URL.Query().Get("path"))
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"revisions": revisions})
}

func (s *Server) handleHistoryGet(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	id := r.URL.Query().Get("id")
	content, err := s.svc.RevisionContent(r.Context(), path, id)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"content": content})
}

type restoreRequest struct {
	Path string `json:"path"`
	ID   string `json:"id"`
}

func (s *Server) handleHistoryRestore(w http.ResponseWriter, r *http.Request) {
	var req restoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	note, err := s.svc.RestoreRevision(r.Context(), req.Path, req.ID)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, note)
}

func (s *Server) handleTrashList(w http.ResponseWriter, r *http.Request) {
	entries, err := s.svc.Trash(r.Context())
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

type trashIDRequest struct {
	ID string `json:"id"`
}

func (s *Server) handleTrashRestore(w http.ResponseWriter, r *http.Request) {
	var req trashIDRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := s.svc.RestoreTrash(r.Context(), req.ID); err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"restored": true})
}

func (s *Server) handleTrashPurge(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.PurgeTrash(r.Context(), r.URL.Query().Get("id")); err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"purged": true})
}
