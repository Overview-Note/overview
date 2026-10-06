package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

// DefaultIntervalSec is the polling interval used before a configuration is
// saved.
const DefaultIntervalSec = 60

// noteState is the last synced state of a note, keyed by its frontmatter id.
type noteState struct {
	Path        string    `json:"path"`
	BaseVersion string    `json:"baseVersion"`
	SyncedAt    time.Time `json:"syncedAt"`
}

// folderState records that a folder has been synchronized.
type folderState struct {
	SyncedAt time.Time `json:"syncedAt"`
}

// assetState is the last synced state of an attachment, keyed by its
// vault-relative path.
type assetState struct {
	BaseHash string `json:"baseHash"`
	Size     int64  `json:"size"`
}

// cursorState records the remote change feed head the client last synced
// against. It lets the engine skip a full reconciliation when the remote has
// not advanced.
type cursorState struct {
	LatestSeq int64     `json:"latestSeq"`
	LatestTs  time.Time `json:"latestTs"`
}

// Conflict records a note whose local and remote copies diverged. The remote
// original stays at OriginalPath; the local copy lives at ConflictPath as a
// standalone note.
type Conflict struct {
	ID            string    `json:"id"`
	OriginalPath  string    `json:"originalPath"`
	ConflictPath  string    `json:"conflictPath"`
	LocalVersion  string    `json:"localVersion"`
	RemoteVersion string    `json:"remoteVersion"`
	DetectedAt    time.Time `json:"detectedAt"`
}

// stateData is the on-disk sync state. It never contains the API token.
type stateData struct {
	ServerURL   string                 `json:"serverURL"`
	VaultID     string                 `json:"vaultId"`
	DeviceID    string                 `json:"deviceId"`
	Enabled     bool                   `json:"enabled"`
	IntervalSec int                    `json:"intervalSec"`
	Direction   string                 `json:"direction"`
	LastSyncAt  time.Time              `json:"lastSyncAt"`
	Cursor      cursorState            `json:"cursor"`
	Notes       map[string]noteState   `json:"notes"`
	Folders     map[string]folderState `json:"folders"`
	Assets      map[string]assetState  `json:"assets"`
	Conflicts   []Conflict             `json:"conflicts"`
}

// State persists the sync engine's bookkeeping to <DataDir>/sync.json with
// owner-only permissions. All access is serialized; every mutation is written
// through to disk.
type State struct {
	mu   sync.Mutex
	path string
	data stateData
}

// OpenState loads the state for a data directory, creating defaults (including
// a device id) when absent.
func OpenState(dataDir string) (*State, error) {
	s := &State{path: filepath.Join(dataDir, "sync.json")}
	raw, err := os.ReadFile(s.path)
	switch {
	case err == nil:
		if err := json.Unmarshal(raw, &s.data); err != nil {
			return nil, fmt.Errorf("parse %s: %w", s.path, err)
		}
	case os.IsNotExist(err):
	default:
		return nil, err
	}
	if s.data.Notes == nil {
		s.data.Notes = map[string]noteState{}
	}
	if s.data.Folders == nil {
		s.data.Folders = map[string]folderState{}
	}
	if s.data.Assets == nil {
		s.data.Assets = map[string]assetState{}
	}
	if s.data.Direction == "" {
		s.data.Direction = string(DirectionBoth)
	}
	if s.data.IntervalSec <= 0 {
		s.data.IntervalSec = DefaultIntervalSec
	}
	if s.data.DeviceID == "" {
		s.data.DeviceID = ulid.Make().String()
	}
	return s, nil
}

// Snapshot returns a deep copy of the persisted state.
func (s *State) Snapshot() stateData {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.copyLocked()
}

func (s *State) copyLocked() stateData {
	out := s.data
	out.Notes = make(map[string]noteState, len(s.data.Notes))
	for k, v := range s.data.Notes {
		out.Notes[k] = v
	}
	out.Folders = make(map[string]folderState, len(s.data.Folders))
	for k, v := range s.data.Folders {
		out.Folders[k] = v
	}
	out.Assets = make(map[string]assetState, len(s.data.Assets))
	for k, v := range s.data.Assets {
		out.Assets[k] = v
	}
	out.Conflicts = append([]Conflict(nil), s.data.Conflicts...)
	return out
}

// Mutate applies fn to the state and persists the result. fn runs while the
// state lock is held; it must not call back into State.
func (s *State) Mutate(fn func(*stateData)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.data)
	return s.saveLocked()
}

// saveLocked writes the state atomically with 0600 permissions.
func (s *State) saveLocked() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(s.path, raw)
}

// writeFileAtomic writes data to path via a same-directory temp file with
// owner-only permissions, then renames it into place.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".overview-sync-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// tokenPath returns the file holding the API token, which is deliberately kept
// out of sync.json so a state export never leaks a credential.
func tokenPath(dataDir string) string { return filepath.Join(dataDir, ".sync-token") }

// ReadToken returns the stored API token, or an empty string when none exists.
func ReadToken(dataDir string) (string, error) {
	raw, err := os.ReadFile(tokenPath(dataDir))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// WriteToken stores the API token with owner-only permissions.
func WriteToken(dataDir, token string) error {
	if token == "" {
		return ClearToken(dataDir)
	}
	return writeFileAtomic(tokenPath(dataDir), []byte(token))
}

// ClearToken removes the stored API token.
func ClearToken(dataDir string) error {
	err := os.Remove(tokenPath(dataDir))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
