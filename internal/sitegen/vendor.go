package sitegen

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// vendorFS holds the frontend libraries used for runtime enhancement. They are
// copied verbatim from web/node_modules so the exported site renders with the
// exact same versions the app uses.
//
//go:embed vendor
var vendorFS embed.FS

//go:embed readonly-enhance.js
var readonlyEnhanceJS string

// writeVendor writes only the libraries the generated site actually needs,
// keeping the export small for sites without math/diagrams/highlighted code.
func writeVendor(outDir string, needCode, needMath, needMermaid bool) error {
	if err := os.WriteFile(filepath.Join(outDir, "readonly-enhance.js"), []byte(readonlyEnhanceJS), 0o644); err != nil {
		return err
	}
	subs := make([]string, 0, 3)
	if needCode {
		subs = append(subs, "highlight")
	}
	if needMath {
		subs = append(subs, "katex")
	}
	if needMermaid {
		subs = append(subs, "mermaid")
	}
	for _, sub := range subs {
		root := "vendor/" + sub
		err := fs.WalkDir(vendorFS, root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			data, err := vendorFS.ReadFile(p)
			if err != nil {
				return err
			}
			rel := strings.TrimPrefix(p, "vendor/")
			dest := filepath.Join(outDir, "vendor", filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}
			return os.WriteFile(dest, data, 0o644)
		})
		if err != nil {
			return err
		}
	}
	return nil
}
