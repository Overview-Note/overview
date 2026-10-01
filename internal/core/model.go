package core

import "time"

// NodeType distinguishes folders from notes in the tree.
type NodeType string

const (
	NodeFolder NodeType = "folder"
	NodeNote   NodeType = "note"
)

// Note is the domain representation of a single note.
type Note struct {
	ID      string    `json:"id"`
	Path    string    `json:"path"`
	Title   string    `json:"title"`
	Tags    []string  `json:"tags"`
	Icon    string    `json:"icon,omitempty"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
	Size    int64     `json:"size"`
	Version string    `json:"version"`
	Body    string    `json:"body"`
}

// NoteMeta is the lightweight projection of a note stored in the index. It
// deliberately excludes the body so the directory tree can be built without
// reading file contents.
type NoteMeta struct {
	ID      string    `json:"id"`
	Path    string    `json:"path"`
	Title   string    `json:"title"`
	Tags    []string  `json:"tags"`
	Updated time.Time `json:"updated"`
	Size    int64     `json:"size"`
}

// Entry is a single filesystem entry (folder or note) without its content.
type Entry struct {
	Path    string
	Name    string
	IsDir   bool
	Size    int64
	ModTime time.Time
}

// TreeNode is a node in the assembled directory tree.
type TreeNode struct {
	Name     string     `json:"name"`
	Path     string     `json:"path"`
	Type     NodeType   `json:"type"`
	ID       string     `json:"id,omitempty"`
	Title    string     `json:"title,omitempty"`
	Updated  *time.Time `json:"updated,omitempty"`
	Size     int64      `json:"size,omitempty"`
	Children []TreeNode `json:"children,omitempty"`
}

// Link is an outgoing wiki-link from a note. Target is nil when the link does
// not resolve to any existing note.
type Link struct {
	Raw    string    `json:"raw"`
	Target *NoteMeta `json:"target,omitempty"`
}

// SearchHit is a single full-text search result.
type SearchHit struct {
	ID      string    `json:"id"`
	Path    string    `json:"path"`
	Title   string    `json:"title"`
	Tags    []string  `json:"tags"`
	Updated time.Time `json:"updated"`
	Snippet string    `json:"snippet"` // pre-escaped HTML with <mark> highlights
}

// Asset describes a stored upload.
type Asset struct {
	Path        string `json:"path"`
	URL         string `json:"url"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
}
