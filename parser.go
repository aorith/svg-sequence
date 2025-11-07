// SPDX-License-Identifier: MIT

package svgsequence

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// GenerateFromCFG generates the sequence by parsing a config file
func GenerateFromCFG(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("error reading file '%s': %v", filename, err)
	}

	r := bytes.NewReader(data)
	scanner := bufio.NewScanner(r)

	// Initialize the sequence
	s := NewSequence()

	lineNum := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineNum++

		// Skip empty lines and comments
		if line == "" || line[0] == '#' {
			continue
		}

		// Sequence properties
		key, val, ok := strings.Cut(line, "=")
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		if ok {
			switch key {
			case "distance_between_actors":
				s.SetDistance(parseIntDefault(val, 180))
			case "width":
				s.SetWidth(val)
			case "height":
				s.SetHeight(val)
			}
		}

		parts := strings.Split(line, " ")
		property := parts[0]
		if property[0] != '@' {
			continue
		}

		switch property {
		case "@actors":
			s.AddActors(parseProperty(line, property)...)

		case "@start":
			values := parseProperty(line, property)
			var name, color string
			switch len(values) {
			case 0:
				return "", fmt.Errorf("section needs a name at line %d", lineNum)
			case 1:
				name = values[0]
			default:
				name = values[0]
				color = values[1]
			}
			s.OpenSection(name, color)

		case "@end":
			s.CloseSection()

		case "@step":
			values := parseProperty(line, property)
			var src, tgt, desc, color string
			switch len(values) {
			case 0, 1:
				return "", fmt.Errorf("not enough values for step at line %d", lineNum)
			case 2:
				src = values[0]
				tgt = values[1]
			case 3:
				src = values[0]
				tgt = values[1]
				desc = values[2]
			default:
				src = values[0]
				tgt = values[1]
				desc = values[2]
				color = values[3]
			}
			s.AddStep(Step{
				Description: desc,
				SourceActor: src,
				TargetActor: tgt,
				Color:       color,
			})

		default:
			return "", fmt.Errorf(`unknown property: "%s" at line %d`, property, lineNum)
		}
	}

	return s.Generate()
}

// parseIntDefault is a helper function to convert a string to int
// returns the default value if parsing fails
func parseIntDefault(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// parseProperty is a helper function to separate values from properties
func parseProperty(line, property string) []string {
	// remove the prefix (@actors, @start, ...)
	rest := strings.TrimPrefix(line, property)

	parts := strings.Split(rest, ",")
	values := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		trimmed = strings.ReplaceAll(trimmed, `\n`, "\n")
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}
