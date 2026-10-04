package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Overview-Note/overview/internal/core"
	"github.com/Overview-Note/overview/internal/service"
)

type capturePreviewRequest struct {
	URL string `json:"url"`
}

// handleCapturePreview fetches a remote page and returns a title/text preview.
// Invalid URLs are rejected with 400; network failures and blocked addresses
// with 502.
func (s *Server) handleCapturePreview(w http.ResponseWriter, r *http.Request) {
	var req capturePreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	preview, err := service.FetchPreview(r.Context(), req.URL)
	if err != nil {
		if errors.Is(err, core.ErrInvalid) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusBadGateway, "failed to fetch url")
		return
	}
	writeJSON(w, http.StatusOK, preview)
}
