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

	"github.com/oklog/ulid/v2"
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

	// A full rebuild must not look like a change to sync clients, so carry the
	// existing change sequence forward for notes whose id, path and version are
	// unchanged. Everything else gets a fresh, higher sequence.
	existing, err := readChangeState(ctx, tx)
	if err != nil {
		return err
	}
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
		if prev, ok := existing[ensureNoteID(n).ID]; ok &&
			prev.path == n.Path && prev.version == n.Version {
			if err := insertGuarded(ctx, tx, ix.logger, n, prev.seq); err != nil {
				return err
			}
			continue
		}
		seq, err := nextChangeSeq(ctx, tx)
		if err != nil {
			return err
		}
		if err := insertGuarded(ctx, tx, ix.logger, n, seq); err != nil {
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
	// A subtree replacement may be a move, so every reinserted note is treated
	// as changed and receives a fresh sequence.
	for _, n := range notes {
		seq, err := nextChangeSeq(ctx, tx)
		if err != nil {
			return err
		}
		if err := insertGuarded(ctx, tx, ix.logger, n, seq); err != nil {
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
		`SELECT id, path, title, tags, updated, size, version FROM notes ORDER BY path`)
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
		if err := rows.Scan(&m.ID, &m.Path, &m.Title, &tags, &up, &m.Size, &m.Version); err != nil {
			return nil, err
		}
		m.Tags = splitTags(tags)
		m.Updated = parseTime(up)
		metas = append(metas, m)
	}
	return metas, rows.Err()
}

// Manifest returns the sync inventory of notes (ordered by path) together with
// the highest change sequence. The sequence grows on every indexed write and
// lets callers derive a manifest ETag.
func (ix *Index) Manifest(ctx context.Context) ([]core.ManifestNote, int64, error) {
	rows, err := ix.db.QueryContext(ctx,
		`SELECT id, path, version, updated, size, public FROM notes ORDER BY path`)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	notes := make([]core.ManifestNote, 0, 64)
	for rows.Next() {
		var (
			n        core.ManifestNote
			up       string
			isPublic int
		)
		if err := rows.Scan(&n.ID, &n.Path, &n.Version, &up, &n.Size, &isPublic); err != nil {
			return nil, 0, err
		}
		n.Updated = parseTime(up)
		n.Public = isPublic != 0
		notes = append(notes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var seq int64
	if err := ix.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(changed_seq), 0) FROM notes`).Scan(&seq); err != nil {
		return nil, 0, err
	}
	return notes, seq, nil
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
	seq, err := nextChangeSeq(ctx, tx)
	if err != nil {
		return err
	}
	return insert(ctx, tx, n, seq)
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
func insertGuarded(ctx context.Context, tx *sql.Tx, logger *slog.Logger, n core.Note, seq int64) error {
	if _, err := tx.ExecContext(ctx, "SAVEPOINT "+insertSavepoint); err != nil {
		return err
	}
	if err := insert(ctx, tx, n, seq); err != nil {
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

func insert(ctx context.Context, tx *sql.Tx, n core.Note, seq int64) error {
	n = ensureNoteID(n)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO notes (id, path, name, title, tags, body, created, updated, size, version, public, changed_seq)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.Path, noteName(n.Path), n.Title, strings.Join(n.Tags, ","), n.Body,
		n.Created.UTC().Format(time.RFC3339), n.Updated.UTC().Format(time.RFC3339),
		n.Size, n.Version, boolToInt(n.Public), seq); err != nil {
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

// changeState captures the fields needed to decide whether a full rebuild can
// preserve a note's change sequence.
type changeState struct {
	path    string
	version string
	seq     int64
}

// readChangeState snapshots the current change sequences before a full rebuild
// and returns them keyed by note id.
func readChangeState(ctx context.Context, tx *sql.Tx) (map[string]changeState, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, path, version, changed_seq FROM notes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := map[string]changeState{}
	for rows.Next() {
		var (
			id string
			s  changeState
		)
		if err := rows.Scan(&id, &s.path, &s.version, &s.seq); err != nil {
			return nil, err
		}
		states[id] = s
	}
	return states, rows.Err()
}

// settingChangeSeq is the settings key holding the monotonic change counter.
const settingChangeSeq = "change_seq"

// nextChangeSeq allocates the next change sequence. The counter is persisted so
// a sequence is never reused after its note is deleted, which keeps the manifest
// ETag strictly monotonic. On first use it is seeded from the highest indexed
// sequence.
func nextChangeSeq(ctx context.Context, tx *sql.Tx) (int64, error) {
	var cur int64
	var raw string
	err := tx.QueryRowContext(ctx,
		`SELECT value FROM settings WHERE key = ?`, settingChangeSeq).Scan(&raw)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if err := tx.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(changed_seq), 0) FROM notes`).Scan(&cur); err != nil {
			return 0, err
		}
	case err != nil:
		return 0, err
	default:
		v, perr := strconv.ParseInt(raw, 10, 64)
		if perr != nil {
			return 0, fmt.Errorf("invalid change_seq %q: %w", raw, perr)
		}
		cur = v
	}

	cur++
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		settingChangeSeq, strconv.FormatInt(cur, 10)); err != nil {
		return 0, err
	}
	return cur, nil
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

// settingVaultID is the settings key holding the persistent vault identifier.
const settingVaultID = "vault_id"

// VaultID returns the persistent vault identifier, generating and persisting a
// new ULID the first time it is requested.
func (ix *Index) VaultID(ctx context.Context) (string, error) {
	id, err := ix.GetSetting(ctx, settingVaultID)
	if err != nil {
		return "", err
	}
	if id != "" {
		return id, nil
	}
	id = ulid.Make().String()
	if err := ix.SetSetting(ctx, settingVaultID, id); err != nil {
		return "", err
	}
	return id, nil
}

// RecordTombstone upserts a deletion marker for a note. A note that is deleted
// again (or moved) overwrites its earlier marker.
func (ix *Index) RecordTombstone(ctx context.Context, t core.Tombstone) error {
	_, err := ix.db.ExecContext(ctx, `
		INSERT INTO note_tombstones (id, path, deleted_at, device) VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			path = excluded.path, deleted_at = excluded.deleted_at, device = excluded.device`,
		t.ID, t.Path, t.DeletedAt.UTC().Format(time.RFC3339), t.Device)
	return err
}

// Tombstones returns every recorded deletion marker, oldest first.
func (ix *Index) Tombstones(ctx context.Context) ([]core.Tombstone, error) {
	rows, err := ix.db.QueryContext(ctx,
		`SELECT id, path, deleted_at, device FROM note_tombstones ORDER BY deleted_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]core.Tombstone, 0, 16)
	for rows.Next() {
		var (
			t  core.Tombstone
			at string
		)
		if err := rows.Scan(&t.ID, &t.Path, &at, &t.Device); err != nil {
			return nil, err
		}
		t.DeletedAt = parseTime(at)
		out = append(out, t)
	}
	return out, rows.Err()
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
