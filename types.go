package main

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
	Restart *RestartConfig
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
