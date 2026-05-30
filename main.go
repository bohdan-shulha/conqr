package main

import (
	"fmt"
	"os"
)

func main() {
	cliCommands := parseCLICommands()
	configCommands, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var commands []CommandInfo
	if len(cliCommands) > 0 {
		commands = cliCommands
	} else if len(configCommands) > 0 {
		commands = configCommands
	} else {
		fmt.Fprintln(os.Stderr, "No commands provided. Use CLI arguments or create a conqr.json config file.")
		os.Exit(1)
	}

	logBuffer := NewLogBuffer()
	processManager := NewProcessManager(logBuffer)

	RenderTUI(commands, processManager, logBuffer)
}
