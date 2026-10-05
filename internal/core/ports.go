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
	// Manifest returns the sync inventory of notes together with the highest
	// change sequence, used to build the manifest ETag. Notes are ordered by
	// path.
	Manifest(ctx context.Context) ([]ManifestNote, int64, error)
	// VaultID returns the persistent vault identifier, generating and storing
	// one on first use.
	VaultID(ctx context.Context) (string, error)
	// RecordTombstone upserts a deletion marker for a note.
	RecordTombstone(ctx context.Context, t Tombstone) error
	// Tombstones returns all recorded deletion markers, oldest first.
	Tombstones(ctx context.Context) ([]Tombstone, error)
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

// RawWriter is an optional capability for writing raw note bytes at an
// explicit path without frontmatter processing. It is used by archive import to
// restore files faithfully.
type RawWriter interface {
	WriteRaw(ctx context.Context, rel string, raw []byte) error
}

// PathLocator exposes the absolute filesystem path of a vault-relative path.
// It is implemented by filesystem-backed repositories to support operations
// like moving entries to the trash.
type PathLocator interface {
	AbsPath(rel string) (string, error)
}

// UserStore persists user accounts, sessions and single-use user tokens.
type UserStore interface {
	CreateUser(ctx context.Context, user User, passwordHash string) error
	UserByUsername(ctx context.Context, username string) (User, string, error)
	UserByID(ctx context.Context, id string) (User, error)
	UserByEmail(ctx context.Context, email string) (User, string, error)
	ListUsers(ctx context.Context) ([]User, error)
	DeleteUser(ctx context.Context, id string) error
	UpdatePassword(ctx context.Context, id, passwordHash string) error
	CountUsers(ctx context.Context) (int, error)
	CountActiveUsers(ctx context.Context) (int, error)
	ActivateUser(ctx context.Context, id, passwordHash string, emailVerified bool) error
	SetUserStatus(ctx context.Context, id, status string) error
	SetUserEmail(ctx context.Context, id, email string) error
	SetEmailVerified(ctx context.Context, id string, verified bool) error

	CreateSession(ctx context.Context, token, userID string, expires time.Time) error
	UserBySession(ctx context.Context, token string) (User, error)
	DeleteSession(ctx context.Context, token string) error
	DeleteSessionsByUser(ctx context.Context, userID string) error
	DeleteExpiredSessions(ctx context.Context, now time.Time) error

	CreateUserToken(ctx context.Context, t UserToken, tokenHash string) error
	ConsumeUserToken(ctx context.Context, tokenHash string, purpose UserTokenPurpose, now time.Time) (UserToken, error)
	DeleteUserTokens(ctx context.Context, userID string, purpose UserTokenPurpose) error
	DeleteExpiredUserTokens(ctx context.Context, now time.Time) error
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

// AssetRestorer is an optional capability implemented by asset stores that can
// import an asset at an explicit vault-relative path (used by archive import).
type AssetRestorer interface {
	Restore(ctx context.Context, rel string, r io.Reader) error
}

// TokenStore persists long-lived API/MCP tokens. Only a hash of the token is
// stored; the plaintext is returned to the caller once at creation.
type TokenStore interface {
	CreateAPIToken(ctx context.Context, t APIToken, tokenHash string) error
	ListAPITokens(ctx context.Context) ([]APIToken, error)
	DeleteAPIToken(ctx context.Context, id string) error
	APITokenByHash(ctx context.Context, tokenHash string) (APIToken, error)
	TouchAPIToken(ctx context.Context, id string, at time.Time) error
}
