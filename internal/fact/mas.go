package fact

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"
	"strconv"
	"strings"
)

// MasInfo holds the observed state of Mac App Store apps.
type MasInfo struct {
	Available bool             // is `mas` on PATH?
	Apps      map[int64]string // ADAM ID → app name
}

// MasCollector collects installed Mac App Store apps.
type MasCollector struct{}

func (MasCollector) Collect(ctx context.Context) (*MasInfo, error) {
	masPath, err := exec.LookPath("mas")
	if err != nil {
		return &MasInfo{Available: false}, nil
	}

	cmd := exec.CommandContext(ctx, masPath, "list")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return &MasInfo{
		Available: true,
		Apps:      parseMasList(out),
	}, nil
}

// parseMasList parses the output of `mas list`.
// Each line has the format: "497799835 Xcode (15.0)"
// Returns a map of ADAM ID to app name (without version).
func parseMasList(data []byte) map[int64]string {
	apps := make(map[int64]string)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		id, name, ok := parseMasLine(line)
		if ok {
			apps[id] = name
		}
	}
	return apps
}

// parseMasLine parses a single line of `mas list` output.
// Format: "497799835 Xcode (15.0)" → (497799835, "Xcode", true)
func parseMasLine(line string) (int64, string, bool) {
	idx := strings.IndexByte(line, ' ')
	if idx < 0 {
		return 0, "", false
	}
	id, err := strconv.ParseInt(line[:idx], 10, 64)
	if err != nil {
		return 0, "", false
	}
	rest := strings.TrimSpace(line[idx+1:])
	// Strip trailing version like "(15.0)"
	if i := strings.LastIndex(rest, "("); i > 0 {
		rest = strings.TrimSpace(rest[:i])
	}
	return id, rest, true
}
