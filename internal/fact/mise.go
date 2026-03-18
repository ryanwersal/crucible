package fact

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"
	"strings"
)

// MiseInfo holds the set of globally installed mise tools.
type MiseInfo struct {
	Available bool            // is `mise` on PATH?
	Globals   map[string]bool // tool names installed globally (e.g. "python", "node")
}

// MiseCollector collects the list of globally installed mise tools.
type MiseCollector struct{}

// Collect checks whether mise is available and lists globally installed tools.
func (c MiseCollector) Collect(ctx context.Context) (*MiseInfo, error) {
	misePath, err := exec.LookPath("mise")
	if err != nil {
		return &MiseInfo{Available: false, Globals: make(map[string]bool)}, nil
	}

	cmd := exec.CommandContext(ctx, misePath, "ls", "--global", "--installed")
	out, err := cmd.Output()
	if err != nil {
		// mise is available but ls failed — return available with empty globals
		return &MiseInfo{Available: true, Globals: make(map[string]bool)}, nil
	}

	globals := parseMiseLsOutput(out)
	return &MiseInfo{Available: true, Globals: globals}, nil
}

// parseMiseLsOutput parses the output of `mise ls --global --installed`.
// Each line has the format: "tool  version  ..." — we extract the tool name.
func parseMiseLsOutput(out []byte) map[string]bool {
	globals := make(map[string]bool)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 1 {
			globals[fields[0]] = true
		}
	}
	return globals
}
