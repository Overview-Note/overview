// Package service contains the application use cases. It orchestrates the
// note repository, the search index and the asset store, keeping transport
// concerns (HTTP) out of the business logic. Future adapters (WebDAV, CLI,
// scheduled jobs) can reuse this layer directly.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Overview-Note/overview/internal/archivex"
	"github.com/Overview-Note/overview/internal/core"
	"github.com/Overview-Note/overview/internal/history"
	"github.com/Overview-Note/overview/internal/markdown"
	"github.com/Overview-Note/overview/internal/trash"
)

// assetRefPattern matches asset references in any of the accepted URL forms,
// capturing the vault-relative path.
var assetRefPattern = regexp.MustCompile(`(?:/api/v1/assets/|/api/assets/|/assets/|assets/)([A-Za-z0-9._/-]+)`)

// Service is the application service facade.
type Service struct {
	repo     core.NoteRepository
	index    core.Index
	assets   core.AssetStore
	history  *history.Store
	trash    *trash.Store
	exporter *archivex.Exporter
	importer *archivex.Importer
	bus      *ChangeBus
}

// New constructs a Service. history and trash may be nil to disable those
// features.
func New(repo core.NoteRepository, index core.Index, assets core.AssetStore, hist *history.Store, tr *trash.Store) *Service {
	return &Service{
		repo:     repo,
		index:    index,
		assets:   assets,
		history:  hist,
		trash:    tr,
		exporter: archivex.NewExporter(repo, assets),
		importer: archivex.NewImporter(repo, assets),
		bus:      NewChangeBus(),
	}
}

// ChangeBus returns the change broadcast bus used to drive real-time sync
// subscribers. It is never nil.
func (s *Service) ChangeBus() *ChangeBus { return s.bus }

// CurrentChangeCursor returns the latest note sequence and tombstone timestamp
// so a subscriber can start from a known point.
func (s *Service) CurrentChangeCursor(ctx context.Context) ChangeEvent {
	return s.changeEvent(ctx)
}

// changeEvent reads the current change cursors, tolerating an index that does
// not expose the change feed.
func (s *Service) changeEvent(ctx context.Context) ChangeEvent {
	log, ok := s.index.(core.ChangeLog)
	if !ok {
		return ChangeEvent{}
	}
	var ev ChangeEvent
	if seq, err := log.LatestSeq(ctx); err == nil {
		ev.LatestSeq = seq
	}
	if ts, err := log.LatestTombstoneTime(ctx); err == nil {
		ev.LatestTs = ts
	}
	return ev
}

// notifyChange publishes the current change cursors to real-time subscribers.
func (s *Service) notifyChange(ctx context.Context) {
	if s.bus == nil {
		return
	}
	s.bus.Publish(s.changeEvent(ctx))
}

// Changes returns the incremental change feed after the given note sequence and
// tombstone timestamp. since is exclusive for note sequences; sinceTs is
// exclusive for note and folder tombstones.
func (s *Service) Changes(ctx context.Context, since int64, sinceTs time.Time, limit int) (core.SyncChanges, error) {
	log, ok := s.index.(core.ChangeLog)
	if !ok {
		return core.SyncChanges{}, fmt.Errorf("%w: incremental changes are not supported", core.ErrNotSupported)
	}
	changes, hasMore, err := log.ChangesSince(ctx, since, limit)
	if err != nil {
		return core.SyncChanges{}, err
	}
	tombstones, err := log.TombstonesSince(ctx, sinceTs)
	if err != nil {
		return core.SyncChanges{}, err
	}
	folders, err := log.FolderTombstonesSince(ctx, sinceTs)
	if err != nil {
		return core.SyncChanges{}, err
	}
	latestSeq, err := log.LatestSeq(ctx)
	if err != nil {
		return core.SyncChanges{}, err
	}
	latestTs, err := log.LatestTombstoneTime(ctx)
	if err != nil {
		return core.SyncChanges{}, err
	}
	if changes == nil {
		changes = []core.SyncChange{}
	}
	if tombstones == nil {
		tombstones = []core.Tombstone{}
	}
	if folders == nil {
		folders = []core.FolderTombstone{}
	}
	return core.SyncChanges{
		LatestSeq:        latestSeq,
		LatestTs:         latestTs,
		HasMore:          hasMore,
		Changes:          changes,
		Tombstones:       tombstones,
		FolderTombstones: folders,
	}, nil
}

// ExportArchive writes the whole vault (notes + assets) as a ZIP stream.
func (s *Service) ExportArchive(ctx context.Context, w io.Writer) error {
	return s.exporter.Export(ctx, w)
}

// ImportArchive restores a vault from a ZIP archive and rebuilds the index.
func (s *Service) ImportArchive(ctx context.Context, r io.ReaderAt, size int64) (archivex.Result, error) {
	res, err := s.importer.Import(ctx, r, size)
	if err != nil {
		return res, err
	}
	if err := s.Reindex(ctx); err != nil {
		return res, err
	}
	return res, nil
}

// Tree assembles the directory tree from a cheap filesystem listing merged
// with note metadata from the index (so file bodies are never read).
func (s *Service) Tree(ctx context.Context) ([]core.TreeNode, error) {
	entries, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	metas, err := s.index.Tree(ctx)
	if err != nil {
		return nil, err
	}
	byPath := make(map[string]core.NoteMeta, len(metas))
	for _, m := range metas {
		byPath[m.Path] = m
	}
	return buildTree(entries, byPath), nil
}

// Manifest builds the vault sync inventory: the persistent vault id, every note
// with its content version, and every folder (including empty ones). The ETag
// is derived from the highest change sequence and the note count.
func (s *Service) Manifest(ctx context.Context) (core.VaultManifest, error) {
	vaultID, err := s.index.VaultID(ctx)
	if err != nil {
		return core.VaultManifest{}, err
	}
	notes, maxSeq, err := s.index.Manifest(ctx)
	if err != nil {
		return core.VaultManifest{}, err
	}
	if notes == nil {
		notes = []core.ManifestNote{}
	}

	entries, err := s.repo.List(ctx)
	if err != nil {
		return core.VaultManifest{}, err
	}
	folders := make([]string, 0, 8)
	for _, e := range entries {
		if e.IsDir {
			folders = append(folders, e.Path)
		}
	}
	sort.Strings(folders)

	return core.VaultManifest{
		VaultID:     vaultID,
		GeneratedAt: time.Now().UTC(),
		ETag:        manifestETag(vaultID, maxSeq, len(notes)),
		Notes:       notes,
		Folders:     folders,
	}, nil
}

// VaultID returns the persistent vault identifier, generating it on first use.
func (s *Service) VaultID(ctx context.Context) (string, error) {
	return s.index.VaultID(ctx)
}

// manifestETag is a strong validator that changes whenever a note is written,
// created or removed.
func manifestETag(vaultID string, maxSeq int64, count int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d", vaultID, maxSeq, count)))
	return hex.EncodeToString(sum[:16])
}

// GetNote returns a single note.
func (s *Service) GetNote(ctx context.Context, path string) (core.Note, error) {
	if strings.TrimSpace(path) == "" {
		return core.Note{}, core.Invalidf("path is required")
	}
	return s.repo.Read(ctx, path)
}

// RawNote returns the complete Markdown file bytes of a note (frontmatter
// included). It is used by the desktop sync client to pull a note faithfully:
// the bytes hash to the note version advertised by the manifest.
func (s *Service) RawNote(ctx context.Context, path string) ([]byte, error) {
	if strings.TrimSpace(path) == "" {
		return nil, core.Invalidf("path is required")
	}
	return s.repo.Raw(ctx, path)
}

// SaveNote writes a note and updates the index.
func (s *Service) SaveNote(ctx context.Context, path, body, expectedVersion string, public bool) (core.Note, error) {
	if strings.TrimSpace(path) == "" {
		return core.Note{}, core.Invalidf("path is required")
	}
	if s.history != nil {
		s.snapshot(ctx, path)
	}
	note, err := s.repo.Write(ctx, path, body, expectedVersion, public)
	if err != nil {
		return core.Note{}, err
	}
	if err := s.index.Upsert(ctx, note); err != nil {
		return core.Note{}, err
	}
	s.notifyChange(ctx)
	return note, nil
}

// SaveNoteRaw writes a note from complete Markdown bytes (frontmatter
// included) without rewriting the document. The client's id and created
// timestamp are preserved and the stored version equals the hash of content.
// baseVersion follows the create/update contract, but an empty value is
// rejected: a sync engine must never write unconditionally.
func (s *Service) SaveNoteRaw(ctx context.Context, path string, content []byte, expectedVersion string) (core.Note, error) {
	if strings.TrimSpace(path) == "" {
		return core.Note{}, core.Invalidf("path is required")
	}
	if !isMarkdownPath(path) {
		return core.Note{}, core.Invalidf("path must be a markdown file")
	}
	if expectedVersion == "" {
		return core.Note{}, core.Invalidf("baseVersion is required")
	}
	writer, ok := s.repo.(core.RawWriter)
	if !ok {
		return core.Note{}, fmt.Errorf("%w: raw note writes are not supported", core.ErrNotSupported)
	}

	if s.history != nil {
		s.snapshot(ctx, path)
	}
	exists := false
	current := ""
	if existing, err := s.repo.Read(ctx, path); err == nil {
		exists = true
		current = existing.Version
	}
	if err := checkBaseVersion(expectedVersion, exists, current); err != nil {
		return core.Note{}, err
	}
	if err := writer.WriteRaw(ctx, path, content); err != nil {
		return core.Note{}, err
	}
	note, err := s.repo.Read(ctx, path)
	if err != nil {
		return core.Note{}, err
	}
	if err := s.index.Upsert(ctx, note); err != nil {
		return core.Note{}, err
	}
	s.notifyChange(ctx)
	return note, nil
}

// checkBaseVersion implements the raw sync write version contract. Unlike the
// repository contract it never allows an unconditional write.
func checkBaseVersion(expected string, exists bool, current string) error {
	switch expected {
	case "*":
		if exists {
			return core.Conflictf("note already exists")
		}
		return nil
	case "":
		return core.Invalidf("baseVersion is required")
	default:
		if !exists {
			return core.Conflictf("note no longer exists")
		}
		if expected != current {
			return core.Conflictf("note was modified by another client")
		}
		return nil
	}
}

// isMarkdownPath reports whether rel names a Markdown file.
func isMarkdownPath(rel string) bool {
	switch strings.ToLower(path.Ext(rel)) {
	case ".md", ".markdown":
		return true
	default:
		return false
	}
}

// snapshot records the current on-disk revision of a note, if it exists.
func (s *Service) snapshot(ctx context.Context, path string) {
	existing, err := s.repo.Read(ctx, path)
	if err != nil {
		return
	}
	raw, err := s.repo.Raw(ctx, path)
	if err != nil {
		return
	}
	_, _ = s.history.Snapshot(existing.Path, existing.Version, raw)
}

// Revisions lists stored revisions for a note.
func (s *Service) Revisions(ctx context.Context, path string) ([]history.Revision, error) {
	if s.history == nil {
		return []history.Revision{}, nil
	}
	if strings.TrimSpace(path) == "" {
		return nil, core.Invalidf("path is required")
	}
	return s.history.List(path)
}

// RevisionContent returns the raw markdown of a stored revision.
func (s *Service) RevisionContent(ctx context.Context, path, id string) (string, error) {
	if s.history == nil {
		return "", core.ErrNotFound
	}
	raw, err := s.history.Content(path, id)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// RestoreRevision replaces a note's body with a stored revision, preserving the
// note's current public flag.
func (s *Service) RestoreRevision(ctx context.Context, path, id string) (core.Note, error) {
	raw, err := s.RevisionContent(ctx, path, id)
	if err != nil {
		return core.Note{}, err
	}
	public := false
	if current, err := s.repo.Read(ctx, path); err == nil {
		public = current.Public
	}
	doc := markdown.Parse(raw)
	return s.SaveNote(ctx, path, doc.Body, "", public)
}

// PublicNotes returns metadata for all publicly shared notes.
func (s *Service) PublicNotes(ctx context.Context) ([]core.NoteMeta, error) {
	notes, err := s.index.PublicNotes(ctx)
	if err != nil {
		return nil, err
	}
	if notes == nil {
		notes = []core.NoteMeta{}
	}
	return notes, nil
}

// PublicTree builds a directory tree containing only publicly shared notes,
// used by the read-only documentation-site mode.
func (s *Service) PublicTree(ctx context.Context) ([]core.TreeNode, error) {
	metas, err := s.index.PublicNotes(ctx)
	if err != nil {
		return nil, err
	}
	entries := make([]core.Entry, 0, len(metas)*2)
	byPath := make(map[string]core.NoteMeta, len(metas))
	seen := map[string]struct{}{}
	for _, m := range metas {
		byPath[m.Path] = m
		parts := strings.Split(m.Path, "/")
		for i := 1; i < len(parts); i++ {
			dir := strings.Join(parts[:i], "/")
			if _, ok := seen[dir]; ok {
				continue
			}
			seen[dir] = struct{}{}
			entries = append(entries, core.Entry{Path: dir, Name: parts[i-1], IsDir: true})
		}
		entries = append(entries, core.Entry{Path: m.Path, Name: parts[len(parts)-1]})
	}
	return buildTree(entries, byPath), nil
}

// PublicNote returns a public note (metadata + body).
func (s *Service) PublicNote(ctx context.Context, path string) (core.Note, error) {
	note, err := s.repo.Read(ctx, path)
	if err != nil {
		return core.Note{}, err
	}
	if !note.Public {
		return core.Note{}, core.ErrNotFound
	}
	return note, nil
}

// Delete moves a note or folder to the trash (soft delete) and reindexes the
// affected subtree. When trash is unavailable it falls back to a hard delete.
func (s *Service) Delete(ctx context.Context, path string) error {
	if strings.TrimSpace(path) == "" {
		return core.Invalidf("path is required")
	}
	existing, readErr := s.repo.Read(ctx, path)
	isNote := readErr == nil
	victims := s.deletionVictims(ctx, path, existing, isNote)
	var folders []string
	if !isNote {
		folders = s.folderVictims(ctx, path)
	}

	if s.trash != nil {
		locator, ok := s.repo.(core.PathLocator)
		if !ok {
			if err := s.repo.Delete(ctx, path); err != nil {
				return err
			}
			return s.finishDelete(ctx, path, isNote, victims, folders)
		}
		full, err := locator.AbsPath(path)
		if err != nil {
			return err
		}
		if _, err := s.trash.Move(full, path, !isNote); err != nil {
			return err
		}
	} else if err := s.repo.Delete(ctx, path); err != nil {
		return err
	}
	return s.finishDelete(ctx, path, isNote, victims, folders)
}

// folderVictims returns every folder at or under path. It must run before the
// folders disappear so an empty folder's deletion can be recorded.
func (s *Service) folderVictims(ctx context.Context, prefix string) []string {
	entries, err := s.repo.List(ctx)
	if err != nil {
		return nil
	}
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		return nil
	}
	out := make([]string, 0, 8)
	for _, e := range entries {
		if !e.IsDir {
			continue
		}
		if e.Path == prefix || strings.HasPrefix(e.Path, prefix+"/") {
			out = append(out, e.Path)
		}
	}
	return out
}

// deletionVictims gathers the notes removed by a delete so their tombstones can
// be recorded after the index is updated. It must run before the files are
// moved to the trash.
func (s *Service) deletionVictims(ctx context.Context, path string, existing core.Note, isNote bool) []core.NoteMeta {
	if isNote {
		return []core.NoteMeta{{ID: existing.ID, Path: existing.Path}}
	}
	notes, err := s.repo.ListUnder(ctx, path)
	if err != nil {
		return nil
	}
	victims := make([]core.NoteMeta, 0, len(notes))
	for _, n := range notes {
		victims = append(victims, core.NoteMeta{ID: n.ID, Path: n.Path})
	}
	return victims
}

// finishDelete updates the index for a removed note or folder and records a
// tombstone for every note and folder that disappeared.
func (s *Service) finishDelete(ctx context.Context, path string, isNote bool, victims []core.NoteMeta, folders []string) error {
	var err error
	if isNote {
		err = s.index.DeleteByPath(ctx, path)
	} else {
		err = s.index.ReplacePrefix(ctx, path, nil)
	}
	if err != nil {
		return err
	}
	if err := s.recordTombstones(ctx, victims, ""); err != nil {
		return err
	}
	if err := s.recordFolderTombstones(ctx, folders, ""); err != nil {
		return err
	}
	s.notifyChange(ctx)
	return nil
}

// recordTombstones persists deletion markers so a sync client can reconcile
// removals. Notes without an id (not yet indexed) are skipped.
func (s *Service) recordTombstones(ctx context.Context, victims []core.NoteMeta, device string) error {
	now := time.Now().UTC()
	for _, v := range victims {
		if v.ID == "" {
			continue
		}
		if err := s.index.RecordTombstone(ctx, core.Tombstone{
			ID:        v.ID,
			Path:      v.Path,
			DeletedAt: now,
			Device:    device,
		}); err != nil {
			return err
		}
	}
	return nil
}

// recordFolderTombstones persists deletion markers for removed folders so a
// sync client can drop folders that held no notes. It is a no-op when the index
// does not support the change feed.
func (s *Service) recordFolderTombstones(ctx context.Context, paths []string, device string) error {
	if len(paths) == 0 {
		return nil
	}
	log, ok := s.index.(core.ChangeLog)
	if !ok {
		return nil
	}
	now := time.Now().UTC()
	for _, p := range paths {
		if p == "" {
			continue
		}
		if err := log.RecordFolderTombstone(ctx, core.FolderTombstone{
			Path:      p,
			DeletedAt: now,
			Device:    device,
		}); err != nil {
			return err
		}
	}
	return nil
}

// recordFolderChange persists a folder creation marker so a sync client can
// observe a folder that holds no notes. It is a no-op when the index does not
// support the change feed.
func (s *Service) recordFolderChange(ctx context.Context, path string) error {
	path = strings.Trim(strings.TrimSpace(path), "/")
	if path == "" {
		return nil
	}
	log, ok := s.index.(core.ChangeLog)
	if !ok {
		return nil
	}
	return log.RecordFolderChange(ctx, path)
}

// Trash lists soft-deleted items.
func (s *Service) Trash(ctx context.Context) ([]trash.Entry, error) {
	if s.trash == nil {
		return []trash.Entry{}, nil
	}
	return s.trash.List()
}

// RestoreTrash restores a trashed item to its original (or given) path and
// reindexes it.
func (s *Service) RestoreTrash(ctx context.Context, id string) error {
	if s.trash == nil {
		return core.ErrNotFound
	}
	entry, err := s.trash.Meta(id)
	if err != nil {
		return err
	}
	locator, ok := s.repo.(core.PathLocator)
	if !ok {
		return core.Invalidf("repository cannot locate paths")
	}
	dest, err := locator.AbsPath(entry.Path)
	if err != nil {
		return err
	}
	if err := s.trash.Restore(id, dest); err != nil {
		return err
	}
	return s.Reindex(ctx)
}

// PurgeTrash permanently removes a trashed item.
func (s *Service) PurgeTrash(ctx context.Context, id string) error {
	if s.trash == nil {
		return core.ErrNotFound
	}
	return s.trash.Purge(id)
}

// Move renames or relocates a note or folder, reindexing only the affected
// subtrees (source and destination).
func (s *Service) Move(ctx context.Context, from, to string) error {
	if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
		return core.Invalidf("from and to are required")
	}
	moved, err := s.repo.ListUnder(ctx, from)
	if err != nil {
		return err
	}
	folders := s.folderVictims(ctx, from)
	if err := s.repo.Move(ctx, from, to); err != nil {
		return err
	}
	if err := s.index.ReplacePrefix(ctx, from, nil); err != nil {
		return err
	}
	notes, err := s.repo.ListUnder(ctx, to)
	if err != nil {
		return err
	}
	if err := s.index.ReplacePrefix(ctx, to, notes); err != nil {
		return err
	}
	victims := make([]core.NoteMeta, 0, len(moved))
	for _, n := range moved {
		victims = append(victims, core.NoteMeta{ID: n.ID, Path: n.Path})
	}
	if err := s.recordTombstones(ctx, victims, ""); err != nil {
		return err
	}
	if err := s.recordFolderTombstones(ctx, folders, ""); err != nil {
		return err
	}
	s.notifyChange(ctx)
	return nil
}

// Mkdir creates a folder.
func (s *Service) Mkdir(ctx context.Context, path string) error {
	if strings.TrimSpace(path) == "" {
		return core.Invalidf("path is required")
	}
	if err := s.repo.Mkdir(ctx, path); err != nil {
		return err
	}
	if err := s.recordFolderChange(ctx, path); err != nil {
		return err
	}
	s.notifyChange(ctx)
	return nil
}

// Search runs a full-text query.
func (s *Service) Search(ctx context.Context, query string, limit, offset int) ([]core.SearchHit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []core.SearchHit{}, nil
	}
	return s.index.Search(ctx, query, limit, offset)
}

// LinksResult bundles the outgoing and incoming links of a note.
type LinksResult struct {
	Outgoing  []core.Link     `json:"outgoing"`
	Backlinks []core.NoteMeta `json:"backlinks"`
}

// Links returns the outgoing and incoming wiki-links of the note at path.
func (s *Service) Links(ctx context.Context, path string) (LinksResult, error) {
	if strings.TrimSpace(path) == "" {
		return LinksResult{}, core.Invalidf("path is required")
	}
	outgoing, err := s.index.OutgoingLinks(ctx, path)
	if err != nil {
		return LinksResult{}, err
	}
	backlinks, err := s.index.Backlinks(ctx, path)
	if err != nil {
		return LinksResult{}, err
	}
	if outgoing == nil {
		outgoing = []core.Link{}
	}
	if backlinks == nil {
		backlinks = []core.NoteMeta{}
	}
	return LinksResult{Outgoing: outgoing, Backlinks: backlinks}, nil
}

// ResolveLink resolves a raw wiki-link target to a note.
func (s *Service) ResolveLink(ctx context.Context, raw string) (core.NoteMeta, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return core.NoteMeta{}, core.Invalidf("target is required")
	}
	return s.index.ResolveLink(ctx, raw)
}

// Upload stores an asset.
func (s *Service) Upload(ctx context.Context, name string, r io.Reader) (core.Asset, error) {
	return s.assets.Save(ctx, name, r)
}

// RestoreAsset writes an attachment body at an explicit vault-relative path.
// Backends without explicit-path restore return ErrNotSupported.
func (s *Service) RestoreAsset(ctx context.Context, rel string, r io.Reader) error {
	restorer, ok := s.assets.(core.AssetRestorer)
	if !ok {
		return fmt.Errorf("%w: asset path restore is not supported by the active backend", core.ErrNotSupported)
	}
	return restorer.Restore(ctx, rel, r)
}

// OpenAsset returns a reader for an asset.
func (s *Service) OpenAsset(ctx context.Context, rel string) (io.ReadSeekCloser, error) {
	return s.assets.Open(ctx, rel)
}

// OrphanAsset describes a stored asset not referenced by any note.
type OrphanAsset struct {
	Path string    `json:"path"`
	Size int64     `json:"size"`
	Age  time.Time `json:"created"`
}

// OrphanAssets returns stored assets that no note references.
func (s *Service) OrphanAssets(ctx context.Context) ([]OrphanAsset, error) {
	assets, err := s.assets.ListAssets(ctx)
	if err != nil {
		return nil, err
	}
	referenced := map[string]struct{}{}
	if err := s.repo.Walk(ctx, func(n core.Note) error {
		for _, ref := range assetRefs(n.Body) {
			referenced[ref] = struct{}{}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	orphans := make([]OrphanAsset, 0)
	for _, a := range assets {
		if _, ok := referenced[a.Path]; ok {
			continue
		}
		orphans = append(orphans, OrphanAsset{Path: a.Path, Size: a.Size, Age: a.Created})
	}
	sort.Slice(orphans, func(i, j int) bool { return orphans[i].Age.Before(orphans[j].Age) })
	return orphans, nil
}

// PurgeOrphanAssets deletes unreferenced assets and returns how many were
// removed.
func (s *Service) PurgeOrphanAssets(ctx context.Context) (int, error) {
	orphans, err := s.OrphanAssets(ctx)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, o := range orphans {
		if err := s.assets.DeleteAsset(ctx, o.Path); err == nil {
			removed++
		}
	}
	return removed, nil
}

// assetRefs extracts asset paths referenced by a Markdown body, normalising
// the various URL forms to the vault-relative form used by the store.
func assetRefs(body string) []string {
	var refs []string
	for _, m := range assetRefPattern.FindAllStringSubmatch(body, -1) {
		refs = append(refs, m[1])
	}
	return refs
}

// ReindexPath reindexes only the subtree rooted at path (used by the file
// watcher for external edits).
func (s *Service) ReindexPath(ctx context.Context, path string) error {
	path = strings.Trim(strings.TrimSpace(path), "/")
	notes, err := s.repo.ListUnder(ctx, path)
	if err != nil {
		return err
	}
	if err := s.index.ReplacePrefix(ctx, path, notes); err != nil {
		return err
	}
	s.notifyChange(ctx)
	return nil
}

// Reindex rebuilds the search index from the repository.
func (s *Service) Reindex(ctx context.Context) error {
	notes := make([]core.Note, 0, 128)
	if err := s.repo.Walk(ctx, func(n core.Note) error {
		notes = append(notes, n)
		return nil
	}); err != nil {
		return err
	}
	if err := s.index.Sync(ctx, notes); err != nil {
		return err
	}
	s.notifyChange(ctx)
	return nil
}

// ---------------------------------------------------------------------------
// tree assembly
// ---------------------------------------------------------------------------

type trieNode struct {
	tree     core.TreeNode
	children map[string]*trieNode
}

func buildTree(entries []core.Entry, metas map[string]core.NoteMeta) []core.TreeNode {
	root := &trieNode{children: map[string]*trieNode{}}
	for _, e := range entries {
		parts := strings.Split(e.Path, "/")
		cur := root
		prefix := ""
		for i, part := range parts {
			prefix = path.Join(prefix, part)
			child, ok := cur.children[prefix]
			if !ok {
				child = &trieNode{
					tree:     core.TreeNode{Name: part, Path: prefix, Type: core.NodeFolder},
					children: map[string]*trieNode{},
				}
				cur.children[prefix] = child
			}
			if i == len(parts)-1 && !e.IsDir {
				child.tree.Type = core.NodeNote
				if m, ok := metas[e.Path]; ok {
					child.tree.ID = m.ID
					child.tree.Title = m.Title
					child.tree.Size = m.Size
					updated := m.Updated
					child.tree.Updated = &updated
				} else {
					child.tree.Title = strings.TrimSuffix(e.Name, path.Ext(e.Name))
					updated := e.ModTime
					child.tree.Updated = &updated
					child.tree.Size = e.Size
				}
			}
			cur = child
		}
	}
	return flatten(root)
}

func flatten(n *trieNode) []core.TreeNode {
	nodes := make([]core.TreeNode, 0, len(n.children))
	for _, child := range n.children {
		child.tree.Children = flatten(child)
		nodes = append(nodes, child.tree)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Type != nodes[j].Type {
			return nodes[i].Type == core.NodeFolder
		}
		return strings.ToLower(nodes[i].Name) < strings.ToLower(nodes[j].Name)
	})
	return nodes
}
