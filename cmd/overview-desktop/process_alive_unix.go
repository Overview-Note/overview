//go:build darwin || (linux && desktop)

package main

import (
	"errors"
	"syscall"
)

// processAlive reports whether a process with the given PID is still running.
func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	// EPERM means the process exists but belongs to another user.
	return errors.Is(err, syscall.EPERM)
}
