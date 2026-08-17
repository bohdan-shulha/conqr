package main

import (
	"regexp"
	"time"
)

type RestartPolicy string

const (
	RestartNever   RestartPolicy = "never"
	RestartOnError RestartPolicy = "on-error"
	RestartOnExit  RestartPolicy = "on-exit"
)

type RestartConfig struct {
	Policy RestartPolicy `json:"policy"`
	Delay  int           `json:"delay"`
}

type CommandInfo struct {
	ID      int
	Name    string
	Command string
	Group   string
	// DependsOn holds command or group names. DependsOnCommands holds the command
	// names they resolve to.
	DependsOn         []string
	DependsOnCommands []string
	Ready             *regexp.Regexp
	Busy              *regexp.Regexp
	ReadyTimeout      time.Duration
	Restart           *RestartConfig
}

type ProcessStatus string

const (
	StatusRunning ProcessStatus = "running"
	StatusStopped ProcessStatus = "stopped"
	StatusError   ProcessStatus = "error"
	StatusUnknown ProcessStatus = "unknown"
)

type LogSource string

const (
	SourceStdout LogSource = "stdout"
	SourceStderr LogSource = "stderr"
)

var defaultRestartConfig = RestartConfig{
	Policy: RestartNever,
	Delay:  1000,
}

const defaultReadyTimeout = 120 * time.Second
