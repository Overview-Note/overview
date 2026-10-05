//go:build !windows && !darwin && !linux

package config

// DefaultDataDir falls back to the user configuration directory on platforms
// without a dedicated implementation.
func DefaultDataDir() string {
	return configDataDir()
}
