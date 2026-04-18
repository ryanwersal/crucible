package resource

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

func TestSymlinkHandler_Plan_TargetResolution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		sourceDir  string
		linkTarget string
		wantTarget string
	}{
		{
			name:       "relative target resolved against source dir",
			sourceDir:  "/home/user/loom",
			linkTarget: "dotfiles/zsh/zshrc",
			wantTarget: "/home/user/loom/dotfiles/zsh/zshrc",
		},
		{
			name:       "absolute target passed through",
			sourceDir:  "/home/user/loom",
			linkTarget: "/etc/shared/config",
			wantTarget: "/etc/shared/config",
		},
		{
			name:       "target cleaned via filepath.Join",
			sourceDir:  "/home/user/loom",
			linkTarget: "dotfiles/../dotfiles/vim/vimrc",
			wantTarget: "/home/user/loom/dotfiles/vim/vimrc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := fact.NewStore()
			// Seed a "symlink does not exist" fact so Plan takes the create path.
			linkPath := filepath.Join(t.TempDir(), "link")
			_, _ = fact.Get(context.Background(), store, "symlink:"+linkPath, fact.SymlinkCollector{Path: linkPath})

			out, err := SymlinkHandler{}.Plan(
				context.Background(),
				store,
				Env{SourceDir: tt.sourceDir},
				decl.Declaration{Type: decl.Symlink, Path: linkPath, LinkTarget: tt.linkTarget},
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out.Actions) != 1 {
				t.Fatalf("expected 1 action, got %d", len(out.Actions))
			}
			got := out.Actions[0].LinkTarget
			if got != tt.wantTarget {
				t.Errorf("LinkTarget = %q, want %q", got, tt.wantTarget)
			}
			if !strings.Contains(out.Actions[0].Description, tt.wantTarget) {
				t.Errorf("description %q should mention resolved target %q",
					out.Actions[0].Description, tt.wantTarget)
			}
		})
	}
}
