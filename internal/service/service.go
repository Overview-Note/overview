// Package service contains the application use cases. It orchestrates the
// note repository, the search index and the asset store, keeping transport
// concerns (HTTP) out of the business logic. Future adapters (WebDAV, CLI,
// scheduled jobs) can reuse this layer directly.
package service

import (
	"context"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/overview-app/overview/internal/core"
)

// Service is the application service facade.
type Service struct {
	repo   core.NoteRepository
	index  core.Index
	assets core.AssetStore
}

// New constructs a Service.
func New(repo core.NoteRepository, index core.Index, assets core.AssetStore) *Service {
	return &Service{repo: repo, index: index, assets: assets}
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
	note, err := s.repo.Write(ctx, path, body, expectedVersion, public)
	if err != nil {
		return core.Note{}, err
	}
	if err := s.index.Upsert(ctx, note); err != nil {
		return core.Note{}, err
	}
	return note, nil
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

// Delete removes a note or folder. Notes are removed from the index directly;
// folder removals trigger a full reindex.
func (s *Service) Delete(ctx context.Context, path string) error {
	if strings.TrimSpace(path) == "" {
		return core.Invalidf("path is required")
	}
	if _, err := s.repo.Read(ctx, path); err == nil {
		if err := s.repo.Delete(ctx, path); err != nil {
			return err
		}
		return s.index.DeleteByPath(ctx, path)
	}
	if err := s.repo.Delete(ctx, path); err != nil {
		return err
	}
	return s.Reindex(ctx)
}

// Move renames or relocates a note or folder and rebuilds the index.
func (s *Service) Move(ctx context.Context, from, to string) error {
	if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
		return core.Invalidf("from and to are required")
	}
	if err := s.repo.Move(ctx, from, to); err != nil {
		return err
	}
	return s.Reindex(ctx)
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
