//go:build darwin || (linux && desktop)

package main

import "syscall"

// detachedProcAttr starts the restart helper in its own session so it is not
// killed when this process exits.
func detachedProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
