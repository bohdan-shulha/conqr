package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

func parseCommands(args []string) []CommandInfo {
	if len(args) == 0 {
		return nil
	}

	commands := make([]CommandInfo, 0, len(args))
	for i, arg := range args {
		equalsIndex := strings.Index(arg, "=")
		var name, command string

		if equalsIndex > 0 && equalsIndex < len(arg)-1 {
			name = strings.TrimSpace(arg[:equalsIndex])
			command = strings.TrimSpace(arg[equalsIndex+1:])
			name = stripMatchingQuotes(name)
			command = stripMatchingQuotes(command)
			if name == "" {
				name = extractCommandName(command)
			}
		} else {
			command = arg
			name = extractCommandName(command)
		}

		commands = append(commands, CommandInfo{
			ID:      i,
			Name:    name,
			Command: command,
		})
	}

	return commands
}

func parseCLICommands() []CommandInfo {
	return parseCommands(os.Args[1:])
}

func stripMatchingQuotes(value string) string {
	if len(value) < 2 {
		return value
	}
	if (value[0] == '\'' && value[len(value)-1] == '\'') ||
		(value[0] == '"' && value[len(value)-1] == '"') {
		return value[1 : len(value)-1]
	}
	return value
}

func extractCommandName(command string) string {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) == 0 {
		return "cmd" + time.Now().Format("150405")
	}
	base := filepath.Base(fields[0])
	if base == "." || base == string(filepath.Separator) || base == "" {
		return "cmd" + time.Now().Format("150405")
	}
	return base
}
