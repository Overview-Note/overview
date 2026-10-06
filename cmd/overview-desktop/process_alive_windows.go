//go:build windows

package main

import "syscall"

// processAlive reports whether a process with the given PID is still running.
func processAlive(pid int) bool {
	const (
		stillActive  = 259
		queryLimited = 0x1000
	)
	h, err := syscall.OpenProcess(queryLimited, false, uint32(pid))
	if err != nil {
		return false
	}
	defer func() { _ = syscall.CloseHandle(h) }()
	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == stillActive
}
