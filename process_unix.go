//go:build !windows

package main

import (
	"os"
	"syscall"
)

var (
	terminateSignal os.Signal = syscall.SIGTERM
	killSignal      os.Signal = syscall.SIGKILL
)

func processAttributes() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

func killProcessGroup(pid int, signal os.Signal) bool {
	sysSignal, ok := signal.(syscall.Signal)
	if !ok {
		return false
	}
	return syscall.Kill(-pid, sysSignal) == nil
}
