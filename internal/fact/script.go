package fact

import (
	"context"
	"os/exec"
)

// ScriptInfo holds whether a script-installed tool is present.
type ScriptInfo struct {
	Installed bool
}

// ScriptCollector runs a check command to determine if a tool is installed.
// The tool is considered installed if the check command exits with status 0.
type ScriptCollector struct {
	Check string // shell command to run (e.g. "claude --version")
}

// Collect runs the check command and reports whether it succeeded.
func (c ScriptCollector) Collect(ctx context.Context) (*ScriptInfo, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", c.Check)
	err := cmd.Run()
	return &ScriptInfo{Installed: err == nil}, nil
}
