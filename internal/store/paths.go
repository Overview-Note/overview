package store

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Overview-Note/overview/internal/core"
)

// AbsPath returns the absolute filesystem path of rel inside the notes root.
func (s *Store) AbsPath(rel string) (string, error) {
	return resolve(s.notesDir, rel)
}

// resolve turns a slash-separated relative path into an absolute path that is
// guaranteed to stay inside root. It rejects empty, absolute and escaping
// paths with an error wrapping core.ErrInvalid.
func resolve(root, rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", core.Invalidf("empty path")
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." ||
		strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", core.Invalidf("invalid path %q", rel)
	}
	full := filepath.Join(root, clean)
	back, err := filepath.Rel(root, full)
	if err != nil || back == ".." || strings.HasPrefix(back, ".."+string(os.PathSeparator)) {
		return "", core.Invalidf("path escapes root %q", rel)
	}
	return full, nil
}
