// Package index implements core.Index on top of SQLite. It stores note
// metadata for fast tree assembly and an FTS5 table for full-text search.
// CJK text is segmented before indexing (see internal/textproc) so substring
// queries work. Snippets are generated in Go and HTML-escaped, making them
// safe to render.
package index

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/Overview-Note/overview/internal/core"
	"github.com/Overview-Note/overview/internal/textproc"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Index is the SQLite-backed note index.
type Index struct {
	db     *sql.DB
	logger *slog.Logger
}

var _ core.Index = (*Index)(nil)

// Open opens (and migrates) the SQLite index at path.
func Open(path string) (*Index, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	ix := &Index{db: db, logger: slog.Default()}
	if err := ix.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return ix, nil
}

// Close releases the database handle.
func (ix *Index) Close() error { return ix.db.Close() }

func (ix *Index) migrate(ctx context.Context) error {
	if _, err := ix.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`); err != nil {
		return err
	}

	names, err := fs.Glob(migrationsFS, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)

	applied := map[int]bool{}
	rows, err := ix.db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		applied[v] = true
	}
	rows.Close()

	for _, name := range names {
		version, err := migrationVersion(name)
		if err != nil {
			return err
		}
		if applied[version] {
			continue
		}
		stmt, err := migrationsFS.ReadFile(name)
		if err != nil {
			return err
		}
		tx, err := ix.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(stmt)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			version, time.Now().UTC().Format(time.RFC3339)); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func migrationVersion(name string) (int, error) {
	base := name[strings.LastIndex(name, "/")+1:]
	idx := strings.IndexByte(base, '_')
	if idx <= 0 {
		return 0, fmt.Errorf("invalid migration name %q", name)
	}
	return strconv.Atoi(base[:idx])
}

// ---------------------------------------------------------------------------
// Core.Index
// ---------------------------------------------------------------------------

// Upsert inserts or replaces a note in both tables.
func (ix *Index) Upsert(ctx context.Context, n core.Note) error {
	tx, err := ix.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := upsert(ctx, tx, n); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteByPath removes any indexed note at path.
func (ix *Index) DeleteByPath(ctx context.Context, path string) error {
	tx, err := ix.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := deleteByPath(ctx, tx, path); err != nil {
		return err
	}
	return tx.Commit()
}

// Sync replaces the entire index with the provided notes.
func (ix *Index) Sync(ctx context.Context, notes []core.Note) error {
	tx, err := ix.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM notes`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM notes_fts`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM links`); err != nil {
		return err
	}
	for _, n := range notes {
		if err := insertGuarded(ctx, tx, ix.logger, n); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ReplacePrefix reindexes only the subtree rooted at prefix.
func (ix *Index) ReplacePrefix(ctx context.Context, prefix string, notes []core.Note) error {
	tx, err := ix.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	like := escapeLike(prefix) + "/%"
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM notes_fts WHERE id IN (SELECT id FROM notes WHERE path = ? OR path LIKE ? ESCAPE '\')`,
		prefix, like); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM links WHERE source_id IN (SELECT id FROM notes WHERE path = ? OR path LIKE ? ESCAPE '\')`,
		prefix, like); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM notes WHERE path = ? OR path LIKE ? ESCAPE '\'`, prefix, like); err != nil {
		return err
	}
	for _, n := range notes {
		if err := insertGuarded(ctx, tx, ix.logger, n); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// Tree returns metadata for every indexed note.
func (ix *Index) Tree(ctx context.Context) ([]core.NoteMeta, error) {
	rows, err := ix.db.QueryContext(ctx,
		`SELECT id, path, title, tags, updated, size FROM notes ORDER BY path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	metas := make([]core.NoteMeta, 0, 64)
	for rows.Next() {
		var (
			m        core.NoteMeta
			tags, up string
		)
		if err := rows.Scan(&m.ID, &m.Path, &m.Title, &tags, &up, &m.Size); err != nil {
			return nil, err
		}
		m.Tags = splitTags(tags)
		m.Updated = parseTime(up)
		metas = append(metas, m)
	}
	return metas, rows.Err()
}

// Search runs a full-text query and returns escaped snippets.
func (ix *Index) Search(ctx context.Context, query string, limit, offset int) ([]core.SearchHit, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}
	match := matchExpr(textproc.Tokens(query))
	if match == "" {
		return []core.SearchHit{}, nil
	}

	rows, err := ix.db.QueryContext(ctx, `
		SELECT n.id, n.path, n.title, n.tags, n.updated, n.body
		FROM notes_fts f
		JOIN notes n ON n.id = f.id
		WHERE notes_fts MATCH ?
		ORDER BY rank
		LIMIT ? OFFSET ?`, match, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hits := make([]core.SearchHit, 0, limit)
	for rows.Next() {
		var (
			h        core.SearchHit
			tags, up string
			body     string
		)
		if err := rows.Scan(&h.ID, &h.Path, &h.Title, &tags, &up, &body); err != nil {
			return nil, err
		}
		h.Tags = splitTags(tags)
		h.Updated = parseTime(up)
		h.Snippet = textproc.Snippet(body, query, 60)
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

// OutgoingLinks returns the wiki-links originating from the note at path.
func (ix *Index) OutgoingLinks(ctx context.Context, path string) ([]core.Link, error) {
	rows, err := ix.db.QueryContext(ctx,
		`SELECT target_raw FROM links WHERE source_path = ? ORDER BY rowid`, path)
	if err != nil {
		return nil, err
	}
	var raws []string
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			rows.Close()
			return nil, err
		}
		raws = append(raws, raw)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	links := make([]core.Link, 0, len(raws))
	seen := make(map[string]bool, len(raws))
	for _, raw := range raws {
		if seen[raw] {
			continue
		}
		seen[raw] = true
		link := core.Link{Raw: raw}
		if meta, err := ix.ResolveLink(ctx, raw); err == nil {
			target := meta
			link.Target = &target
		}
		links = append(links, link)
	}
	return links, nil
}

// Backlinks returns notes that link to the note at path.
func (ix *Index) Backlinks(ctx context.Context, path string) ([]core.NoteMeta, error) {
	var id, title, p, name string
	err := ix.db.QueryRowContext(ctx,
		`SELECT id, title, path, name FROM notes WHERE path = ?`, path).
		Scan(&id, &title, &p, &name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []core.NoteMeta{}, nil
		}
		return nil, err
	}

	keys := uniqueLower(p, strings.TrimSuffix(strings.ToLower(p), ".md"), title, name, id)
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(keys)), ",")
	args := make([]any, 0, len(keys)+1)
	for _, k := range keys {
		args = append(args, k)
	}
	args = append(args, id)

	rows, err := ix.db.QueryContext(ctx, `
		SELECT DISTINCT n.id, n.path, n.title, n.tags, n.updated, n.size
		FROM links l
		JOIN notes n ON n.id = l.source_id
		WHERE l.target_key IN (`+placeholders+`) AND n.id <> ?
		ORDER BY n.updated DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	metas := make([]core.NoteMeta, 0, 8)
	for rows.Next() {
		var (
			m        core.NoteMeta
			tags, up string
		)
		if err := rows.Scan(&m.ID, &m.Path, &m.Title, &tags, &up, &m.Size); err != nil {
			return nil, err
		}
		m.Tags = splitTags(tags)
		m.Updated = parseTime(up)
		metas = append(metas, m)
	}
	return metas, rows.Err()
}

// ResolveLink resolves a raw wiki-link target to a stored note.
func (ix *Index) ResolveLink(ctx context.Context, raw string) (core.NoteMeta, error) {
	key := strings.ToLower(strings.TrimSpace(raw))
	if key == "" {
		return core.NoteMeta{}, core.ErrNotFound
	}
	var (
		m        core.NoteMeta
		tags, up string
	)
	err := ix.db.QueryRowContext(ctx, `
		SELECT id, path, title, tags, updated, size FROM notes
		WHERE LOWER(path) = ? OR LOWER(path) = ? OR LOWER(title) = ? OR LOWER(name) = ? OR id = ?
		ORDER BY CASE
			WHEN LOWER(path) = ? THEN 0
			WHEN LOWER(path) = ? THEN 1
			WHEN LOWER(title) = ? THEN 2
			WHEN LOWER(name) = ? THEN 3
			ELSE 4 END
		LIMIT 1`,
		key, key+".md", key, key, strings.TrimSpace(raw),
		key, key+".md", key, key).Scan(&m.ID, &m.Path, &m.Title, &tags, &up, &m.Size)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return core.NoteMeta{}, core.ErrNotFound
		}
		return core.NoteMeta{}, err
	}
	m.Tags = splitTags(tags)
	m.Updated = parseTime(up)
	return m, nil
}

func uniqueLower(values ...string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// ---------------------------------------------------------------------------
// internal helpers
// ---------------------------------------------------------------------------

func upsert(ctx context.Context, tx *sql.Tx, n core.Note) error {
	if err := deleteByPath(ctx, tx, n.Path); err != nil {
		return err
	}
	// A note moved to this path may still hold its old id; drop it too.
	if _, err := tx.ExecContext(ctx, `DELETE FROM notes_fts WHERE id = ?`, n.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM links WHERE source_id = ?`, n.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM notes WHERE id = ?`, n.ID); err != nil {
		return err
	}
	return insert(ctx, tx, n)
}

// insertSavepoint scopes a single note insert so one failure can be rolled
// back without discarding the surrounding full-rebuild transaction.
const insertSavepoint = "overview_note_insert"

// ensureNoteID gives notes without frontmatter an id so they never collide on
// the empty primary key. The id is derived deterministically from the note
// path, which keeps repeated reindexes idempotent.
func ensureNoteID(n core.Note) core.Note {
	if n.ID != "" || n.Path == "" {
		return n
	}
	sum := sha256.Sum256([]byte(n.Path))
	n.ID = "auto-" + hex.EncodeToString(sum[:12])
	return n
}

// insertGuarded inserts a single note inside a savepoint. A failure only
// discards that note, logs a warning, and lets the caller's transaction and
// the remaining notes commit.
func insertGuarded(ctx context.Context, tx *sql.Tx, logger *slog.Logger, n core.Note) error {
	if _, err := tx.ExecContext(ctx, "SAVEPOINT "+insertSavepoint); err != nil {
		return err
	}
	if err := insert(ctx, tx, n); err != nil {
		if _, rbErr := tx.ExecContext(ctx, "ROLLBACK TO "+insertSavepoint); rbErr != nil {
			return err
		}
		if _, relErr := tx.ExecContext(ctx, "RELEASE "+insertSavepoint); relErr != nil {
			return err
		}
		logger.Warn("skipped note while rebuilding index", "path", n.Path, "error", err)
		return nil
	}
	if _, err := tx.ExecContext(ctx, "RELEASE "+insertSavepoint); err != nil {
		return err
	}
	return nil
}

func insert(ctx context.Context, tx *sql.Tx, n core.Note) error {
	n = ensureNoteID(n)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO notes (id, path, name, title, tags, body, created, updated, size, version, public)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.Path, noteName(n.Path), n.Title, strings.Join(n.Tags, ","), n.Body,
		n.Created.UTC().Format(time.RFC3339), n.Updated.UTC().Format(time.RFC3339),
		n.Size, n.Version, boolToInt(n.Public)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO notes_fts (id, title, tags, body) VALUES (?, ?, ?, ?)`,
		n.ID,
		textproc.Segment(n.Title),
		textproc.Segment(strings.Join(n.Tags, " ")),
		textproc.Segment(n.Body)); err != nil {
		return err
	}
	for _, target := range textproc.ExtractWikiLinks(n.Body) {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO links (source_id, source_path, target_raw, target_key) VALUES (?, ?, ?, ?)`,
			n.ID, n.Path, target, strings.ToLower(target)); err != nil {
			return err
		}
	}
	return nil
}

func deleteByPath(ctx context.Context, tx *sql.Tx, path string) error {
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM notes_fts WHERE id IN (SELECT id FROM notes WHERE path = ?)`, path); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM links WHERE source_id IN (SELECT id FROM notes WHERE path = ?)`, path); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM notes WHERE path = ?`, path)
	return err
}

// matchExpr builds a safe FTS5 MATCH expression from pre-segmented tokens.
func matchExpr(tokens []string) string {
	seen := make(map[string]struct{}, len(tokens))
	parts := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		escaped := strings.ReplaceAll(t, `"`, `""`)
		if textproc.IsASCIIWord(t) {
			parts = append(parts, `"`+escaped+`"*`)
		} else {
			parts = append(parts, `"`+escaped+`"`)
		}
	}
	return strings.Join(parts, " ")
}

// noteName returns the file base name without extension, used to resolve
// wiki-links like [[并发模型]] against nested notes.
// GetSetting returns a stored setting value, or "" when unset.
func (ix *Index) GetSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := ix.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return value, err
}

// SetSetting upserts a setting value.
func (ix *Index) SetSetting(ctx context.Context, key, value string) error {
	_, err := ix.db.ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// PublicNotes returns metadata for all notes marked public.
func (ix *Index) PublicNotes(ctx context.Context) ([]core.NoteMeta, error) {
	rows, err := ix.db.QueryContext(ctx,
		`SELECT id, path, title, tags, updated, size FROM notes WHERE public = 1 ORDER BY path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	metas := make([]core.NoteMeta, 0, 16)
	for rows.Next() {
		var (
			m        core.NoteMeta
			tags, up string
		)
		if err := rows.Scan(&m.ID, &m.Path, &m.Title, &tags, &up, &m.Size); err != nil {
			return nil, err
		}
		m.Tags = splitTags(tags)
		m.Updated = parseTime(up)
		metas = append(metas, m)
	}
	return metas, rows.Err()
}

// PublicBody returns the body of a public note.
func (ix *Index) PublicBody(ctx context.Context, path string) (string, error) {
	var body string
	err := ix.db.QueryRowContext(ctx,
		`SELECT body FROM notes WHERE path = ? AND public = 1`, path).Scan(&body)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", core.ErrNotFound
		}
		return "", err
	}
	return body, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func noteName(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		p = p[i+1:]
	}
	if i := strings.LastIndexByte(p, '.'); i > 0 {
		p = p[:i]
	}
	return p
}

func splitTags(s string) []string {
	if s == "" {
		return []string{}
	}
	return strings.Split(s, ",")
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}
