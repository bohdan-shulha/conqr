package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var configFiles = []string{".conqr.json", "conqr.json"}

type rawConfigFile struct {
	Commands     json.RawMessage `json:"commands"`
	Restart      *partialRestart `json:"restart"`
	DefaultGroup *string         `json:"defaultGroup"`
}

type partialRestart struct {
	Policy *RestartPolicy `json:"policy"`
	Delay  *int           `json:"delay"`
}

type commandObject struct {
	Command string          `json:"command"`
	Group   *string         `json:"group"`
	Restart *partialRestart `json:"restart"`
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

	commands, err := parseConfigCommands(config.Commands, config.Restart)
	if err != nil {
		return nil, "", err
	}
	return commands, defaultGroup, nil
}

func parseConfigCommands(commandsJSON json.RawMessage, global *partialRestart) ([]CommandInfo, error) {
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

		if len(raw) > 0 && raw[0] == '"' {
			var command string
			if err := json.Unmarshal(raw, &command); err != nil {
				return nil, err
			}
			restart := resolveRestartConfig(global, nil)
			commands = append(commands, CommandInfo{
				ID:      index,
				Name:    name,
				Command: command,
				Restart: &restart,
			})
			index++
			continue
		}

		var object commandObject
		if err := json.Unmarshal(raw, &object); err != nil {
			return nil, fmt.Errorf("invalid command entry %q: %w", name, err)
		}
		if object.Command == "" {
			fmt.Fprintf(os.Stderr, "Warning: Command entry %q is missing required \"command\" property, skipping.\n", name)
			continue
		}

		restart := resolveRestartConfig(global, object.Restart)
		group := ""
		if object.Group != nil {
			group = *object.Group
		}
		commands = append(commands, CommandInfo{
			ID:      index,
			Name:    name,
			Command: object.Command,
			Group:   group,
			Restart: &restart,
		})
		index++
	}

	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	return commands, nil
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
