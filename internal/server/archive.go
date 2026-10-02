package server

import (
	"bytes"
	"net/http"
	"time"
)

// handleExport streams the entire vault as a ZIP archive.
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	name := "overview-export-" + time.Now().UTC().Format("20060102-150405") + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	if err := s.svc.ExportArchive(r.Context(), w); err != nil {
		// Headers are already written; the stream is simply truncated.
		s.logger.Error("export archive", "error", err)
	}
}

// handleImport restores a vault from an uploaded ZIP archive.
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.opts.MaxUploadBytes)
	data, err := readAll(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "file too large or malformed request")
		return
	}
	res, err := s.svc.ImportArchive(r.Context(), bytes.NewReader(data), int64(len(data)))
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func readAll(r *http.Request) ([]byte, error) {
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r.Body); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
