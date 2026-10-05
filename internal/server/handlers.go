package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Overview-Note/overview/internal/core"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"name":      "Overview",
		"version":   s.opts.Version,
		"render":    s.opts.Render,
		"siteTitle": s.opts.SiteTitle,
	})
}

func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	var (
		nodes []core.TreeNode
		err   error
	)
	if s.opts.Render {
		nodes, err = s.svc.PublicTree(r.Context())
	} else {
		nodes, err = s.svc.Tree(r.Context())
	}
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"nodes": nodes})
}

func (s *Server) handleGetNote(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	if r.URL.Query().Get("raw") == "1" {
		s.handleGetNoteRaw(w, r, rel)
		return
	}
	note, err := s.svc.GetNote(r.Context(), rel)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	if s.opts.Render && !note.Public {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	w.Header().Set("ETag", etag(note.Version))
	writeJSON(w, http.StatusOK, note)
}

// handleGetNoteRaw returns the complete Markdown file bytes of a note so a
// sync client can reconstruct the exact file (and its version hash). The ETag
// carries the note version.
func (s *Server) handleGetNoteRaw(w http.ResponseWriter, r *http.Request, rel string) {
	note, err := s.svc.GetNote(r.Context(), rel)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	if s.opts.Render && !note.Public {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	raw, err := s.svc.RawNote(r.Context(), rel)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	w.Header().Set("ETag", etag(note.Version))
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

type saveNoteRequest struct {
	Path        string `json:"path"`
	Body        string `json:"body"`
	BaseVersion string `json:"baseVersion"`
	Public      bool   `json:"public"`
	// Raw requests a faithful write: Content holds the complete Markdown file
	// bytes (frontmatter included) and is stored verbatim.
	Raw     bool   `json:"raw"`
	Content string `json:"content"`
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

	var (
		note core.Note
		err  error
	)
	if req.Raw {
		note, err = s.svc.SaveNoteRaw(r.Context(), req.Path, []byte(req.Content), expected)
	} else {
		note, err = s.svc.SaveNote(r.Context(), req.Path, req.Body, expected, req.Public)
	}
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	w.Header().Set("ETag", etag(note.Version))
	writeJSON(w, http.StatusOK, note)
}

// handleSyncManifest returns the vault sync inventory. The response carries a
// strong ETag derived from the index change sequence; a matching If-None-Match
// yields 304.
func (s *Server) handleSyncManifest(w http.ResponseWriter, r *http.Request) {
	manifest, err := s.svc.Manifest(r.Context())
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	tag := etag(manifest.ETag)
	w.Header().Set("ETag", tag)
	w.Header().Set("Cache-Control", "no-cache")
	if match := r.Header.Get("If-None-Match"); match != "" && trimETag(match) == manifest.ETag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	writeJSON(w, http.StatusOK, manifest)
}

// handleRestoreAsset writes a request body to an explicit vault-relative asset
// path. Backends that cannot restore an explicit path (S3) return 501.
func (s *Server) handleRestoreAsset(w http.ResponseWriter, r *http.Request) {
	rel := r.PathValue("path")
	if err := validateAssetPath(rel); err != nil {
		s.writeDomainError(w, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.opts.MaxUploadBytes)
	if err := s.svc.RestoreAsset(r.Context(), rel, r.Body); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "asset exceeds the maximum upload size")
			return
		}
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": rel, "url": "/assets/" + rel})
}

// validateAssetPath rejects empty, absolute, traversal and malformed asset
// paths before they reach the asset store.
func validateAssetPath(rel string) error {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return core.Invalidf("path is required")
	}
	if strings.ContainsAny(rel, "\\\x00:") || strings.HasPrefix(rel, "/") || filepath.IsAbs(rel) {
		return core.Invalidf("invalid asset path %q", rel)
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return core.Invalidf("invalid asset path %q", rel)
		}
	}
	return nil
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

func (s *Server) handleOrphanAssets(w http.ResponseWriter, r *http.Request) {
	orphans, err := s.svc.OrphanAssets(r.Context())
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"orphans": orphans})
}

func (s *Server) handlePurgeOrphans(w http.ResponseWriter, r *http.Request) {
	removed, err := s.svc.PurgeOrphanAssets(r.Context())
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"removed": removed})
}

func (s *Server) handlePublicNotes(w http.ResponseWriter, r *http.Request) {
	notes, err := s.svc.PublicNotes(r.Context())
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notes": notes})
}

func (s *Server) handlePublicNote(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	note, err := s.svc.PublicNote(r.Context(), rel)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	// Expose only render-safe fields for anonymous readers.
	writeJSON(w, http.StatusOK, map[string]any{
		"id":      note.ID,
		"path":    note.Path,
		"title":   note.Title,
		"tags":    note.Tags,
		"created": note.Created,
		"updated": note.Updated,
		"body":    note.Body,
	})
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
	// Embedded frontend assets share the /assets/ prefix; prefer them so the
	// SPA bundle wins over an upload with a colliding name. They still go
	// through the static handler for compression and long-term caching.
	if s.static != nil {
		if _, err := fs.Stat(s.opts.Static, "assets/"+rel); err == nil {
			r.URL.Path = "/assets/" + rel
			s.static.serve(w, r)
			return
		}
	}
	reader, err := s.svc.OpenAsset(r.Context(), rel)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	defer reader.Close()

	ct := assetMIMEType(path.Ext(rel))
	w.Header().Set("Content-Type", ct)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if !isInlineType(ct) {
		w.Header().Set("Content-Disposition", contentDisposition(path.Base(rel)))
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

// extraMIMETypes supplements the platform MIME database with common attachment
// types so portable builds still serve a sensible Content-Type (and browsers
// offer the right download name).
var extraMIMETypes = map[string]string{
	".pdf":  "application/pdf",
	".zip":  "application/zip",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	".txt":  "text/plain; charset=utf-8",
	".csv":  "text/csv; charset=utf-8",
	".md":   "text/markdown; charset=utf-8",
	".json": "application/json; charset=utf-8",
}

func assetMIMEType(ext string) string {
	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct
	}
	if ct, ok := extraMIMETypes[strings.ToLower(ext)]; ok {
		return ct
	}
	return "application/octet-stream"
}

// contentDisposition builds an attachment header that preserves the original
// filename. Non-ASCII names get an RFC 5987 filename* parameter plus an ASCII
// fallback for older clients.
func contentDisposition(filename string) string {
	var ascii strings.Builder
	hasNonASCII := false
	for _, r := range filename {
		if r >= 0x20 && r < 0x7f && r != '"' && r != '\\' {
			ascii.WriteRune(r)
			continue
		}
		if r > 0x7f {
			hasNonASCII = true
		}
		ascii.WriteByte('_')
	}
	fallback := ascii.String()
	if fallback == "" {
		fallback = "download"
	}
	if !hasNonASCII {
		return fmt.Sprintf("attachment; filename=%q", fallback)
	}
	return fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", fallback, encodeRFC5987(filename))
}

func encodeRFC5987(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			strings.IndexByte("!#$&+-.^_`|~", c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
