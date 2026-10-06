package core

import "time"

// Roles for authenticated users.
const (
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// Account lifecycle statuses. Invited accounts exist but cannot authenticate
// until the invitation is accepted.
const (
	StatusActive  = "active"
	StatusInvited = "invited"
)

// User is an authenticated account. The password hash never leaves the
// persistence layer.
type User struct {
	ID            string    `json:"id"`
	Username      string    `json:"username"`
	Email         string    `json:"email,omitempty"`
	Role          string    `json:"role"`
	Status        string    `json:"status"`
	EmailVerified bool      `json:"emailVerified"`
	Created       time.Time `json:"created"`
	Updated       time.Time `json:"updated"`
}

// UserTokenPurpose scopes a single-use user token to one flow.
type UserTokenPurpose string

const (
	PurposeInvite UserTokenPurpose = "invite"
	PurposeReset  UserTokenPurpose = "reset"
	PurposeVerify UserTokenPurpose = "verify"
)

// UserToken is a single-use, purpose-scoped credential (invitation, password
// reset or email verification). Only a hash of the plaintext token is stored.
type UserToken struct {
	ID      string           `json:"id"`
	UserID  string           `json:"userId"`
	Purpose UserTokenPurpose `json:"purpose"`
	Created time.Time        `json:"created"`
	Expires time.Time        `json:"expires"`
}

// IsAdmin reports whether the user has administrative privileges.
func (u User) IsAdmin() bool { return u.Role == RoleAdmin }

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
	Public  bool      `json:"public"`
	Body    string    `json:"body"`
}

// NoteMeta is the lightweight projection of a note stored in the index. It
// deliberately excludes the body so the directory tree can be built without
// reading file contents. Version carries the content hash used by the sync
// manifest.
type NoteMeta struct {
	ID      string    `json:"id"`
	Path    string    `json:"path"`
	Title   string    `json:"title"`
	Tags    []string  `json:"tags"`
	Updated time.Time `json:"updated"`
	Size    int64     `json:"size"`
	Version string    `json:"version"`
}

// ManifestNote is a note inventory entry in the vault sync manifest.
type ManifestNote struct {
	ID      string    `json:"id"`
	Path    string    `json:"path"`
	Version string    `json:"version"`
	Updated time.Time `json:"updated"`
	Size    int64     `json:"size"`
	Public  bool      `json:"public"`
}

// VaultManifest is the inventory a desktop client uses to plan a sync:
// the persistent vault identifier, every note with its content version, and
// every folder (including empty ones).
type VaultManifest struct {
	VaultID     string         `json:"vaultId"`
	GeneratedAt time.Time      `json:"generatedAt"`
	ETag        string         `json:"etag"`
	Notes       []ManifestNote `json:"notes"`
	Folders     []string       `json:"folders"`
}

// Tombstone records the deletion of a note so a client can reconcile a sync.
type Tombstone struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	DeletedAt time.Time `json:"deletedAt"`
	Device    string    `json:"device"`
}

// FolderTombstone records the deletion or relocation of a folder so a client
// can reconcile a sync, including folders that contained no notes.
type FolderTombstone struct {
	Path      string    `json:"path"`
	DeletedAt time.Time `json:"deletedAt"`
	Device    string    `json:"device,omitempty"`
}

// SyncChange is a single note change in an incremental sync response. Op is
// currently always "upsert": the entry carries the note's current state.
type SyncChange struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Version string `json:"version"`
	Seq     int64  `json:"seq"`
	Op      string `json:"op"`
}

// SyncChanges is the incremental change feed a client uses to sync without
// fetching the full manifest. LatestSeq is the head of the note change
// sequence; LatestTs is the highest tombstone timestamp currently recorded.
// A client paginates sessions by feeding the last change's seq back as since,
// and passes LatestTs back as sinceTs to receive only newer tombstones.
type SyncChanges struct {
	LatestSeq        int64             `json:"latestSeq"`
	LatestTs         time.Time         `json:"latestTs"`
	HasMore          bool              `json:"hasMore"`
	Changes          []SyncChange      `json:"changes"`
	Tombstones       []Tombstone       `json:"tombstones"`
	FolderTombstones []FolderTombstone `json:"folderTombstones"`
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
	Path        string    `json:"path"`
	URL         string    `json:"url"`
	Name        string    `json:"name"`
	Size        int64     `json:"size"`
	ContentType string    `json:"contentType"`
	Created     time.Time `json:"created,omitempty"`
}

// APIToken is a long-lived bearer credential for the API and MCP. The secret is
// only shown once at creation; the store keeps a hash plus a short prefix.
type APIToken struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Prefix   string     `json:"prefix"`
	Created  time.Time  `json:"created"`
	LastUsed *time.Time `json:"lastUsed,omitempty"`
	Expires  *time.Time `json:"expires,omitempty"`
}
