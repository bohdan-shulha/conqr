//go:build windows

package main

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

var (
	terminateSignal os.Signal = os.Interrupt
	killSignal      os.Signal = os.Kill
)

func processAttributes() *syscall.SysProcAttr {
	return nil
}

func killProcessGroup(pid int, _ os.Signal) bool {
	return exec.Command("taskkill", "/pid", strconv.Itoa(pid), "/t", "/f").Run() == nil
}
