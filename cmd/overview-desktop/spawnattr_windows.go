//go:build windows

package main

import "syscall"

// detachedProcAttr starts the restart helper without a console window and
// independent of this process, so it survives our exit.
func detachedProcAttr() *syscall.SysProcAttr {
	const createNoWindow = 0x08000000
	return &syscall.SysProcAttr{CreationFlags: createNoWindow}
}
