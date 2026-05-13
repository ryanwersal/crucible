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
	Available bool              // is `mise` on PATH?
	Globals   map[string]string // tool name → installed version (e.g. "python": "3.12.0")
}

// MiseCollector collects the list of globally installed mise tools.
type MiseCollector struct{}

// Collect checks whether mise is available and lists globally installed tools.
func (c MiseCollector) Collect(ctx context.Context) (*MiseInfo, error) {
	misePath, err := exec.LookPath("mise")
	if err != nil {
		return &MiseInfo{Available: false, Globals: make(map[string]string)}, nil
	}

	cmd := exec.CommandContext(ctx, misePath, "ls", "--global", "--installed")
	out, err := cmd.Output()
	if err != nil {
		// mise is available but ls failed — return available with empty globals
		return &MiseInfo{Available: true, Globals: make(map[string]string)}, nil
	}

	globals := parseMiseLsOutput(out)
	return &MiseInfo{Available: true, Globals: globals}, nil
}

// MiseResolver resolves a mise version spec (e.g. "latest", "2", "2.92") to a
// concrete version by shelling out to `mise latest <name>@<spec>`. This lets
// callers distinguish "installed but spec implies a newer version" from
// "installed and already up to date for this spec".
type MiseResolver struct{}

// Resolve returns the concrete version mise would install for spec.
// An empty spec is treated as "latest".
func (MiseResolver) Resolve(ctx context.Context, name, spec string) (string, error) {
	misePath, err := exec.LookPath("mise")
	if err != nil {
		return "", err
	}
	arg := name
	if spec != "" {
		arg = name + "@" + spec
	}
	out, err := exec.CommandContext(ctx, misePath, "latest", arg).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// parseMiseLsOutput parses the output of `mise ls --global --installed`.
// Each line has the format: "tool  version  ..." — we extract the tool name and version.
func parseMiseLsOutput(out []byte) map[string]string {
	globals := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			globals[fields[0]] = fields[1]
		}
	}
	return globals
}
