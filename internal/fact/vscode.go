package fact

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type VSCodeInfo struct {
	Command    string
	Extensions map[string]bool
	Bundled    map[string]bool
}

type VSCodeCollector struct{}

func (VSCodeCollector) Collect(ctx context.Context) (*VSCodeInfo, error) {
	info := &VSCodeInfo{Extensions: make(map[string]bool), Bundled: make(map[string]bool)}
	command, err := exec.LookPath("code")
	if errors.Is(err, exec.ErrNotFound) {
		return info, nil
	}
	if err != nil {
		return nil, fmt.Errorf("locate VS Code CLI: %w", err)
	}
	info.Command = command
	out, err := exec.CommandContext(ctx, command, "--list-extensions").Output()
	if err != nil {
		return nil, fmt.Errorf("list VS Code extensions: %w", err)
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		if id := strings.ToLower(strings.TrimSpace(line)); id != "" {
			info.Extensions[id] = true
		}
	}
	resolved, err := filepath.EvalSymlinks(command)
	if err != nil {
		return nil, fmt.Errorf("resolve VS Code CLI: %w", err)
	}
	root := filepath.Join(filepath.Dir(resolved), "..")
	dir := filepath.Join(root, "extensions")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		dir = filepath.Join(root, "resources", "app", "extensions")
		entries, err = os.ReadDir(dir)
	}
	if errors.Is(err, fs.ErrNotExist) {
		return info, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read bundled VS Code extensions: %w", err)
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name(), "package.json")
		data, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read bundled extension %s: %w", path, err)
		}
		var manifest struct {
			Publisher string
			Name      string
		}
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil, fmt.Errorf("parse bundled extension %s: %w", path, err)
		}
		if manifest.Publisher != "" && manifest.Name != "" {
			info.Bundled[strings.ToLower(manifest.Publisher+"."+manifest.Name)] = true
		}
	}
	return info, nil
}
