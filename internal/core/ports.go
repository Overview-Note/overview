package core

import (
	"context"
	"io"
	"time"
)

// NoteRepository persists notes on a backing store (currently the filesystem).
type NoteRepository interface {
	// Read returns the note at rel, including its current version.
	Read(ctx context.Context, rel string) (Note, error)

	// Raw returns the raw bytes of the note file at rel.
	Raw(ctx context.Context, rel string) ([]byte, error)

	// Write creates or updates a note. Version semantics:
	//   expectedVersion == "*"  -> the note must not exist (create)
	//   expectedVersion == ""   -> unconditional write
	//   otherwise               -> the current version must match
	// A mismatch returns an error wrapping ErrConflict. When public is true the
	// note's frontmatter is marked as publicly readable.
	Write(ctx context.Context, rel, body, expectedVersion string, public bool) (Note, error)

	// Delete removes a note or folder recursively.
	Delete(ctx context.Context, rel string) error

	// Move renames or relocates a note or folder.
	Move(ctx context.Context, from, to string) error

	// Mkdir creates a folder (and parents).
	Mkdir(ctx context.Context, rel string) error

	// List returns a flat, content-free listing of the notes tree.
	List(ctx context.Context) ([]Entry, error)

	// ListUnder returns notes at or under the given path prefix.
	ListUnder(ctx context.Context, prefix string) ([]Note, error)

	// Walk visits every note, used to (re)build the index.
	Walk(ctx context.Context, fn func(Note) error) error
}

// Index is the queryable projection of the note repository.
type Index interface {
	// Upsert inserts or replaces a single note.
	Upsert(ctx context.Context, n Note) error
	// DeleteByPath removes any indexed note at path.
	DeleteByPath(ctx context.Context, path string) error
	// Sync replaces the entire index contents with notes.
	Sync(ctx context.Context, notes []Note) error
	// ReplacePrefix reindexes only the subtree rooted at prefix: it deletes all
	// indexed notes whose path equals or is under prefix, then inserts notes.
	ReplacePrefix(ctx context.Context, prefix string, notes []Note) error
	// Tree returns metadata for every indexed note.
	Tree(ctx context.Context) ([]NoteMeta, error)
	// Search runs a full-text query and returns escaped snippets.
	Search(ctx context.Context, query string, limit, offset int) ([]SearchHit, error)
	// OutgoingLinks returns the wiki-links originating from the note at path,
	// resolving each target to an existing note when possible.
	OutgoingLinks(ctx context.Context, path string) ([]Link, error)
	// Backlinks returns notes that link to the note at path.
	Backlinks(ctx context.Context, path string) ([]NoteMeta, error)
	// ResolveLink resolves a raw wiki-link target to a stored note.
	ResolveLink(ctx context.Context, raw string) (NoteMeta, error)
	// PublicNotes returns metadata for all notes marked public.
	PublicNotes(ctx context.Context) ([]NoteMeta, error)
	// PublicBody returns the body of a public note at path, or ErrNotFound when
	// the note does not exist or is not public.
	PublicBody(ctx context.Context, path string) (string, error)
}

// PathLocator exposes the absolute filesystem path of a vault-relative path.
// It is implemented by filesystem-backed repositories to support operations
// like moving entries to the trash.
type PathLocator interface {
	AbsPath(rel string) (string, error)
}

// UserStore persists user accounts and sessions.
type UserStore interface {
	CreateUser(ctx context.Context, user User, passwordHash string) error
	UserByUsername(ctx context.Context, username string) (User, string, error)
	UserByID(ctx context.Context, id string) (User, error)
	ListUsers(ctx context.Context) ([]User, error)
	DeleteUser(ctx context.Context, id string) error
	UpdatePassword(ctx context.Context, id, passwordHash string) error
	CountUsers(ctx context.Context) (int, error)

	CreateSession(ctx context.Context, token, userID string, expires time.Time) error
	UserBySession(ctx context.Context, token string) (User, error)
	DeleteSession(ctx context.Context, token string) error
	DeleteExpiredSessions(ctx context.Context, now time.Time) error
}

// AssetStore persists binary attachments.
type AssetStore interface {
	// Save stores r under name and returns the resulting asset.
	Save(ctx context.Context, name string, r io.Reader) (Asset, error)
	// Open returns a reader for the asset at rel.
	Open(ctx context.Context, rel string) (io.ReadSeekCloser, error)
	// ListAssets returns metadata for every stored asset.
	ListAssets(ctx context.Context) ([]Asset, error)
	// DeleteAsset removes the asset at rel.
	DeleteAsset(ctx context.Context, rel string) error
}
