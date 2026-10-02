// Package service contains the application use cases. It orchestrates the
// note repository, the search index and the asset store, keeping transport
// concerns (HTTP) out of the business logic. Future adapters (WebDAV, CLI,
// scheduled jobs) can reuse this layer directly.
package service

import (
	"context"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

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
	repo    core.NoteRepository
	index   core.Index
	assets  core.AssetStore
	history *history.Store
	trash   *trash.Store
}

// New constructs a Service. history and trash may be nil to disable those
// features.
func New(repo core.NoteRepository, index core.Index, assets core.AssetStore, hist *history.Store, tr *trash.Store) *Service {
	return &Service{repo: repo, index: index, assets: assets, history: hist, trash: tr}
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

// GetNote returns a single note.
func (s *Service) GetNote(ctx context.Context, path string) (core.Note, error) {
	if strings.TrimSpace(path) == "" {
		return core.Note{}, core.Invalidf("path is required")
	}
	return s.repo.Read(ctx, path)
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
	return note, nil
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
	_, readErr := s.repo.Read(ctx, path)
	isNote := readErr == nil

	if s.trash != nil {
		locator, ok := s.repo.(core.PathLocator)
		if !ok {
			return s.repo.Delete(ctx, path)
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

	if isNote {
		return s.index.DeleteByPath(ctx, path)
	}
	return s.index.ReplacePrefix(ctx, path, nil)
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
	return s.index.ReplacePrefix(ctx, to, notes)
}

// Mkdir creates a folder.
func (s *Service) Mkdir(ctx context.Context, path string) error {
	if strings.TrimSpace(path) == "" {
		return core.Invalidf("path is required")
	}
	return s.repo.Mkdir(ctx, path)
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
	return s.index.ReplacePrefix(ctx, path, notes)
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
	return s.index.Sync(ctx, notes)
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
