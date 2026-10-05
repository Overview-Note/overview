package app

import (
	"fmt"
	"os"
	"path/filepath"
)

const lockFileName = ".overview.lock"

// InstanceLock is an exclusive, cross-process advisory lock over a vault's data
// directory. It also protects the SQLite index from being opened by two server
// processes at once (for example the desktop shell and a manual `overview
// serve`).
type InstanceLock struct {
	path string
	file *os.File
}

// AcquireInstanceLock takes the single-instance lock for dataDir. It returns an
// error when another process already holds it.
func AcquireInstanceLock(dataDir string) (*InstanceLock, error) {
	path := filepath.Join(dataDir, lockFileName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := lockFile(f); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("another Overview instance is already using %s", dataDir)
	}
	return &InstanceLock{path: path, file: f}, nil
}

// Release unlocks and closes the lock file. It is safe to call multiple times.
func (l *InstanceLock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	err := unlockFile(l.file)
	if closeErr := l.file.Close(); err == nil {
		err = closeErr
	}
	l.file = nil
	return err
}
