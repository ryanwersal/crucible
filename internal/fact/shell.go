package fact

import (
	"context"
	"os/exec"
	"os/user"
	"strings"
)

// ShellInfo holds the current login shell for a user.
type ShellInfo struct {
	Path string // e.g. "/bin/zsh"
}

// ShellCollector reads the current user's login shell.
type ShellCollector struct {
	Username string
}

// Collect reads the login shell for the specified user via `dscl` on macOS.
func (c ShellCollector) Collect(ctx context.Context) (*ShellInfo, error) {
	username := c.Username
	if username == "" {
		u, err := user.Current()
		if err != nil {
			return nil, err
		}
		username = u.Username
	}

	cmd := exec.CommandContext(ctx, "dscl", ".", "-read", "/Users/"+username, "UserShell")
	out, err := cmd.Output()
	if err != nil {
		return &ShellInfo{}, nil
	}

	// Output is "UserShell: /bin/zsh\n"
	line := strings.TrimSpace(string(out))
	if _, after, ok := strings.Cut(line, "UserShell: "); ok {
		return &ShellInfo{Path: strings.TrimSpace(after)}, nil
	}

	return &ShellInfo{}, nil
}
