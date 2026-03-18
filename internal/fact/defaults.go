package fact

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// DefaultsInfo holds the current value of a macOS defaults key.
type DefaultsInfo struct {
	Value  any
	Exists bool
}

// DefaultsCollector reads a single macOS defaults key.
type DefaultsCollector struct {
	Domain string
	Key    string
}

// Collect reads the current value of a macOS defaults key.
func (c DefaultsCollector) Collect(ctx context.Context) (*DefaultsInfo, error) {
	// Get the type of the key
	typeCmd := exec.CommandContext(ctx, "defaults", "read-type", c.Domain, c.Key)
	typeOut, err := typeCmd.Output()
	if err != nil {
		// Non-zero exit means key doesn't exist
		return &DefaultsInfo{Exists: false}, nil
	}

	// Get the value
	valueCmd := exec.CommandContext(ctx, "defaults", "read", c.Domain, c.Key)
	valueOut, err := valueCmd.Output()
	if err != nil {
		return &DefaultsInfo{Exists: false}, nil
	}

	val, err := parseDefaultsOutput(strings.TrimSpace(string(typeOut)), strings.TrimSpace(string(valueOut)))
	if err != nil {
		return nil, fmt.Errorf("parse defaults %s %s: %w", c.Domain, c.Key, err)
	}

	return &DefaultsInfo{Value: val, Exists: true}, nil
}

// parseDefaultsOutput interprets the output of `defaults read-type` and
// `defaults read` to produce a typed Go value.
func parseDefaultsOutput(typeOutput, valueOutput string) (any, error) {
	// typeOutput looks like "Type is boolean", "Type is integer", etc.
	typ := strings.TrimPrefix(typeOutput, "Type is ")

	switch typ {
	case "boolean":
		// defaults read returns "1" or "0" for booleans
		switch valueOutput {
		case "1":
			return true, nil
		case "0":
			return false, nil
		default:
			return nil, fmt.Errorf("unexpected boolean value: %q", valueOutput)
		}
	case "integer":
		return strconv.ParseInt(valueOutput, 10, 64)
	case "float":
		return strconv.ParseFloat(valueOutput, 64)
	case "string":
		return valueOutput, nil
	default:
		return nil, fmt.Errorf("unsupported defaults type: %q", typ)
	}
}
