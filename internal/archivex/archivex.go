// Package archivex implements portable ZIP export and import for a vault.
//
// The archive layout mirrors the data directory so it stays legible:
//
//	manifest.json          # format/version metadata
//	notes/<path>.md        # raw note files (front-matter preserved)
//	assets/<path>          # attachment binaries
//
// Export streams directly to an io.Writer; import reads a zip and writes notes
// through the note repository and (when supported) assets through an
// AssetRestorer.
package archivex

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/Overview-Note/overview/internal/core"
	"github.com/Overview-Note/overview/internal/markdown"
)

// Format is the manifest format identifier.
const Format = "overview.archive"

// Manifest describes an exported archive.
type Manifest struct {
	Format      string    `json:"format"`
	Version     int       `json:"version"`
	App         string    `json:"app"`
	GeneratedAt time.Time `json:"generatedAt"`
	Notes       int       `json:"notes"`
	Assets      int       `json:"assets"`
}

// Exporter writes a vault to a ZIP stream.
type Exporter struct {
	repo   core.NoteRepository
	assets core.AssetStore
}

// NewExporter constructs an Exporter. assets may be nil to skip attachments.
func NewExporter(repo core.NoteRepository, assets core.AssetStore) *Exporter {
	return &Exporter{repo: repo, assets: assets}
}

// Export writes the vault as a ZIP archive to w.
func (e *Exporter) Export(ctx context.Context, w io.Writer) error {
	zw := zip.NewWriter(w)

	notes, err := e.writeNotes(ctx, zw)
	if err != nil {
		return err
	}
	assets, err := e.writeAssets(ctx, zw)
	if err != nil {
		return err
	}
	if err := writeManifest(zw, Manifest{
		Format:      Format,
		Version:     1,
		App:         "Overview",
		GeneratedAt: time.Now().UTC(),
		Notes:       notes,
		Assets:      assets,
	}); err != nil {
		return err
	}
	return zw.Close()
}

func (e *Exporter) writeNotes(ctx context.Context, zw *zip.Writer) (int, error) {
	count := 0
	err := e.repo.Walk(ctx, func(n core.Note) error {
		raw, err := e.repo.Raw(ctx, n.Path)
		if err != nil {
			return err
		}
		f, err := zw.Create("notes/" + n.Path)
		if err != nil {
			return err
		}
		if _, err := f.Write(raw); err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err
}

func (e *Exporter) writeAssets(ctx context.Context, zw *zip.Writer) (int, error) {
	if e.assets == nil {
		return 0, nil
	}
	assets, err := e.assets.ListAssets(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, a := range assets {
		rc, err := e.assets.Open(ctx, a.Path)
		if err != nil {
			continue
		}
		f, err := zw.Create("assets/" + a.Path)
		if err != nil {
			rc.Close()
			return 0, err
		}
		if _, err := io.Copy(f, rc); err != nil {
			rc.Close()
			return 0, err
		}
		rc.Close()
		count++
	}
	return count, nil
}

func writeManifest(zw *zip.Writer, m Manifest) error {
	f, err := zw.Create("manifest.json")
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}

// Importer restores notes (and assets) from a ZIP archive.
type Importer struct {
	repo     core.NoteRepository
	assets   core.AssetStore
	rawWrite core.RawWriter
}

// NewImporter constructs an Importer. When repo also implements RawWriter,
// note files are restored byte-for-byte; otherwise the body is re-serialized.
func NewImporter(repo core.NoteRepository, assets core.AssetStore) *Importer {
	imp := &Importer{repo: repo, assets: assets}
	if rw, ok := repo.(core.RawWriter); ok {
		imp.rawWrite = rw
	}
	return imp
}

// Result summarizes an import.
type Result struct {
	Notes   int `json:"notes"`
	Assets  int `json:"assets"`
	Skipped int `json:"skipped"`
}

// Import reads a ZIP archive from r and writes its contents into the vault.
// Existing notes at the same path are overwritten (last write wins).
func (i *Importer) Import(ctx context.Context, r io.ReaderAt, size int64) (Result, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return Result{}, err
	}
	var res Result
	restorer, _ := i.assets.(core.AssetRestorer)

	for _, file := range zr.File {
		name := path.Clean(file.Name)
		if name == "." || strings.HasSuffix(name, "/") || name == "manifest.json" {
			continue
		}
		if strings.HasPrefix(name, "..") || path.IsAbs(name) {
			res.Skipped++
			continue
		}
		switch {
		case strings.HasPrefix(name, "notes/"):
			rel := strings.TrimPrefix(name, "notes/")
			if err := i.importNote(ctx, file, rel); err != nil {
				return res, fmt.Errorf("import note %s: %w", rel, err)
			}
			res.Notes++
		case strings.HasPrefix(name, "assets/"):
			rel := strings.TrimPrefix(name, "assets/")
			if restorer == nil {
				res.Skipped++
				continue
			}
			if err := i.importAsset(ctx, file, rel, restorer); err != nil {
				if errors.Is(err, core.ErrNotSupported) {
					res.Skipped++
					continue
				}
				return res, fmt.Errorf("import asset %s: %w", rel, err)
			}
			res.Assets++
		default:
			res.Skipped++
		}
	}
	return res, nil
}

func (i *Importer) importNote(ctx context.Context, file *zip.File, rel string) error {
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	raw, err := io.ReadAll(rc)
	if err != nil {
		return err
	}
	if i.rawWrite != nil {
		return i.rawWrite.WriteRaw(ctx, rel, raw)
	}
	doc := markdown.Parse(string(raw))
	_, err = i.repo.Write(ctx, rel, doc.Body, "", doc.Meta.Public)
	return err
}

func (i *Importer) importAsset(ctx context.Context, file *zip.File, rel string, restorer core.AssetRestorer) error {
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	return restorer.Restore(ctx, rel, rc)
}
