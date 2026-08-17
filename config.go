package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var configFiles = []string{".conqr.json", "conqr.json"}

type rawConfigFile struct {
	Commands     json.RawMessage `json:"commands"`
	Restart      *partialRestart `json:"restart"`
	DefaultGroup *string         `json:"defaultGroup"`
	Ready        *string         `json:"ready"`
	Busy         *string         `json:"busy"`
	ReadyTimeout *int            `json:"readyTimeout"`
}

type partialRestart struct {
	Policy *RestartPolicy `json:"policy"`
	Delay  *int           `json:"delay"`
}

type commandObject struct {
	Command   string          `json:"command"`
	Group     *string         `json:"group"`
	DependsOn []string        `json:"dependsOn"`
	Ready     *string         `json:"ready"`
	Busy      *string         `json:"busy"`
	Restart   *partialRestart `json:"restart"`
}

type globalDefaults struct {
	restart      *partialRestart
	ready        *string
	busy         *string
	readyTimeout time.Duration
}

func loadConfig() ([]CommandInfo, string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}

	for _, name := range configFiles {
		path := filepath.Join(cwd, name)
		content, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, "", fmt.Errorf("error reading config file %s: %w", name, err)
		}

		commands, defaultGroup, err := parseConfig(content)
		if err != nil {
			return nil, "", fmt.Errorf("error reading config file %s: %w", name, err)
		}
		return commands, defaultGroup, nil
	}

	return nil, "", nil
}

func parseConfig(content []byte) ([]CommandInfo, string, error) {
	var config rawConfigFile
	if err := json.Unmarshal(content, &config); err != nil {
		return nil, "", err
	}
	defaultGroup := ""
	if config.DefaultGroup != nil {
		defaultGroup = *config.DefaultGroup
	}
	if len(config.Commands) == 0 || string(config.Commands) == "null" {
		return []CommandInfo{}, defaultGroup, nil
	}
	if len(config.Commands) > 0 && config.Commands[0] == '[' {
		return nil, "", errors.New("array format for commands is no longer supported; use object format")
	}

	defaults := globalDefaults{
		restart:      config.Restart,
		ready:        config.Ready,
		busy:         config.Busy,
		readyTimeout: defaultReadyTimeout,
	}
	if config.ReadyTimeout != nil {
		defaults.readyTimeout = time.Duration(*config.ReadyTimeout) * time.Millisecond
	}

	commands, err := parseConfigCommands(config.Commands, defaults)
	if err != nil {
		return nil, "", err
	}
	if err := validateDependencies(commands); err != nil {
		return nil, "", err
	}
	return commands, defaultGroup, nil
}

func parseConfigCommands(commandsJSON json.RawMessage, defaults globalDefaults) ([]CommandInfo, error) {
	decoder := json.NewDecoder(bytes.NewReader(commandsJSON))
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delimiter, ok := token.(json.Delim)
	if !ok || delimiter != '{' {
		return nil, errors.New("commands must be an object")
	}

	var commands []CommandInfo
	index := 0
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		name, ok := keyToken.(string)
		if !ok {
			return nil, errors.New("command name must be a string")
		}

		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return nil, err
		}

		var object commandObject
		if len(raw) > 0 && raw[0] == '"' {
			if err := json.Unmarshal(raw, &object.Command); err != nil {
				return nil, err
			}
		} else if err := json.Unmarshal(raw, &object); err != nil {
			return nil, fmt.Errorf("invalid command entry %q: %w", name, err)
		}
		if object.Command == "" {
			fmt.Fprintf(os.Stderr, "Warning: Command entry %q is missing required \"command\" property, skipping.\n", name)
			continue
		}

		ready, err := resolvePattern(defaults.ready, object.Ready)
		if err != nil {
			return nil, fmt.Errorf("invalid ready pattern for command %q: %w", name, err)
		}
		busy, err := resolvePattern(defaults.busy, object.Busy)
		if err != nil {
			return nil, fmt.Errorf("invalid busy pattern for command %q: %w", name, err)
		}

		restart := resolveRestartConfig(defaults.restart, object.Restart)
		group := ""
		if object.Group != nil {
			group = *object.Group
		}
		commands = append(commands, CommandInfo{
			ID:           index,
			Name:         name,
			Command:      object.Command,
			Group:        group,
			DependsOn:    object.DependsOn,
			Ready:        ready,
			Busy:         busy,
			ReadyTimeout: defaults.readyTimeout,
			Restart:      &restart,
		})
		index++
	}

	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	return commands, nil
}

func resolvePattern(global, perProcess *string) (*regexp.Regexp, error) {
	pattern := global
	if perProcess != nil {
		pattern = perProcess
	}
	if pattern == nil || *pattern == "" {
		return nil, nil
	}
	return regexp.Compile(*pattern)
}

func validateDependencies(commands []CommandInfo) error {
	byName := make(map[string]CommandInfo, len(commands))
	for _, command := range commands {
		byName[command.Name] = command
	}

	for _, command := range commands {
		for _, dependency := range command.DependsOn {
			if _, ok := byName[dependency]; !ok {
				return fmt.Errorf("command %q depends on unknown command %q", command.Name, dependency)
			}
		}
	}

	const (
		visiting = 1
		visited  = 2
	)
	state := make(map[string]int, len(commands))
	path := make([]string, 0, len(commands))

	var visit func(string) error
	visit = func(name string) error {
		switch state[name] {
		case visiting:
			return fmt.Errorf("dependency cycle: %s", strings.Join(append(path, name), " -> "))
		case visited:
			return nil
		}
		state[name] = visiting
		path = append(path, name)
		for _, dependency := range byName[name].DependsOn {
			if err := visit(dependency); err != nil {
				return err
			}
		}
		path = path[:len(path)-1]
		state[name] = visited
		return nil
	}

	for _, command := range commands {
		if err := visit(command.Name); err != nil {
			return err
		}
	}
	return nil
}

func resolveRestartConfig(global, perProcess *partialRestart) RestartConfig {
	result := defaultRestartConfig
	if global != nil {
		mergeRestart(&result, global)
	}
	if perProcess != nil {
		mergeRestart(&result, perProcess)
	}
	return result
}

func mergeRestart(target *RestartConfig, patch *partialRestart) {
	if patch.Policy != nil {
		target.Policy = *patch.Policy
	}
	if patch.Delay != nil {
		target.Delay = *patch.Delay
	}
}
