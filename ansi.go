package main

import (
	"regexp"
	"strconv"
	"strings"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)
var ansiColorRegex = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

func stripANSI(text string) string {
	return ansiRegex.ReplaceAllString(text, "")
}

func detectANSIColor(line string) string {
	matches := ansiColorRegex.FindAllStringSubmatch(line, -1)
	color := ""
	for _, match := range matches {
		parts := strings.Split(match[1], ";")
		if len(parts) == 0 {
			continue
		}
		switch parts[0] {
		case "0":
			color = ""
		case "31", "91":
			color = "31"
		case "33", "93":
			color = "33"
		case "32", "92":
			color = "32"
		case "34", "94":
			color = "34"
		case "35", "95":
			color = "35"
		case "36", "96":
			color = "36"
		case "37", "97":
			color = "37"
		case "30", "90":
			color = "30"
		case "38":
			if len(parts) >= 3 && parts[1] == "5" {
				value, err := strconv.Atoi(parts[2])
				if err == nil {
					switch {
					case value == 1 || value == 9:
						color = "31"
					case value == 3 || value == 11:
						color = "33"
					case value == 2 || value == 10:
						color = "32"
					case value == 4 || value == 12:
						color = "34"
					case value == 5 || value == 13:
						color = "35"
					case value == 6 || value == 14:
						color = "36"
					case value == 7 || value == 15:
						color = "37"
					}
				}
			}
		}
	}
	return color
}

func colorSeq(color string) string {
	if color == "" {
		return ""
	}
	return "\x1b[" + color + "m"
}
