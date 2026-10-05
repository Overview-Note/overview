//go:build windows

package config

// DefaultDataDir is the per-user vault location for the desktop build on
// Windows: %USERPROFILE%\Documents\Overview. When Documents is unavailable it
// falls back to the user configuration directory.
func DefaultDataDir() string {
	if dir, ok := documentsDataDir(); ok {
		return dir
	}
	return configDataDir()
}
