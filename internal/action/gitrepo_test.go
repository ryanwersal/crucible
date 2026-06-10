package action

import (
	"testing"

	"github.com/ryanwersal/crucible/internal/fact"
)

func TestDiffGitRepo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		desired          DesiredGitRepo
		actual           *fact.GitRepoInfo
		wantActions      int
		wantObservations int
		wantType         Type
		// wantDesc, if set, is matched against the first action's description
		// when an action is expected, otherwise the first observation's.
		wantDesc string
	}{
		{
			name:        "new clone",
			desired:     DesiredGitRepo{Path: "/home/user/.oh-my-zsh", URL: "https://github.com/ohmyzsh/ohmyzsh.git", Branch: "master"},
			actual:      nil,
			wantActions: 1,
			wantType:    CloneRepo,
			wantDesc:    "git clone https://github.com/ohmyzsh/ohmyzsh.git → /home/user/.oh-my-zsh (branch master)",
		},
		{
			name:        "clone without branch omits branch annotation",
			desired:     DesiredGitRepo{Path: "/home/user/.oh-my-zsh", URL: "https://github.com/ohmyzsh/ohmyzsh.git"},
			actual:      nil,
			wantActions: 1,
			wantType:    CloneRepo,
			wantDesc:    "git clone https://github.com/ohmyzsh/ohmyzsh.git → /home/user/.oh-my-zsh",
		},
		{
			name:        "repo does not exist",
			desired:     DesiredGitRepo{Path: "/home/user/.oh-my-zsh", URL: "https://github.com/ohmyzsh/ohmyzsh.git", Branch: "master"},
			actual:      &fact.GitRepoInfo{Exists: false},
			wantActions: 1,
			wantType:    CloneRepo,
		},
		{
			name:    "correct remote pulls when remote SHA unknown",
			desired: DesiredGitRepo{Path: "/home/user/.oh-my-zsh", URL: "https://github.com/ohmyzsh/ohmyzsh.git", Branch: "master"},
			actual: &fact.GitRepoInfo{
				Exists:        true,
				RemoteURL:     "https://github.com/ohmyzsh/ohmyzsh.git",
				CurrentBranch: "master",
				LocalSHA:      "abc1234def",
			},
			wantActions: 1,
			wantType:    PullRepo,
			wantDesc:    "git pull /home/user/.oh-my-zsh (master: abc1234 → remote unknown)",
		},
		{
			name:    "local drift pulls with SHA transition",
			desired: DesiredGitRepo{Path: "/home/user/.oh-my-zsh", URL: "https://github.com/ohmyzsh/ohmyzsh.git", Branch: "master"},
			actual: &fact.GitRepoInfo{
				Exists:        true,
				RemoteURL:     "https://github.com/ohmyzsh/ohmyzsh.git",
				CurrentBranch: "master",
				LocalSHA:      "abc1234def",
				RemoteSHA:     "def4567abc",
			},
			wantActions: 1,
			wantType:    PullRepo,
			wantDesc:    "git pull /home/user/.oh-my-zsh (master: abc1234 → def4567)",
		},
		{
			name:    "in sync reports current branch and SHA",
			desired: DesiredGitRepo{Path: "/home/user/.oh-my-zsh", URL: "https://github.com/ohmyzsh/ohmyzsh.git", Branch: "master"},
			actual: &fact.GitRepoInfo{
				Exists:        true,
				RemoteURL:     "https://github.com/ohmyzsh/ohmyzsh.git",
				CurrentBranch: "master",
				LocalSHA:      "abc1234def",
				RemoteSHA:     "abc1234def",
			},
			wantActions:      0,
			wantObservations: 1,
			wantDesc:         "/home/user/.oh-my-zsh (up to date, master@abc1234)",
		},
		{
			name:    "branch mismatch warns and still pulls",
			desired: DesiredGitRepo{Path: "/home/user/.oh-my-zsh", URL: "https://github.com/ohmyzsh/ohmyzsh.git", Branch: "main"},
			actual: &fact.GitRepoInfo{
				Exists:        true,
				RemoteURL:     "https://github.com/ohmyzsh/ohmyzsh.git",
				CurrentBranch: "master",
				LocalSHA:      "abc1234def",
				RemoteSHA:     "def4567abc",
			},
			wantActions:      1,
			wantObservations: 1,
			wantType:         PullRepo,
		},
		{
			name:    "wrong remote warns",
			desired: DesiredGitRepo{Path: "/home/user/.oh-my-zsh", URL: "https://github.com/ohmyzsh/ohmyzsh.git", Branch: "master"},
			actual: &fact.GitRepoInfo{
				Exists:        true,
				RemoteURL:     "https://github.com/other/repo.git",
				CurrentBranch: "master",
			},
			wantActions:      0,
			wantObservations: 1,
			wantDesc:         "/home/user/.oh-my-zsh: remote URL mismatch (want https://github.com/ohmyzsh/ohmyzsh.git, have https://github.com/other/repo.git)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actions, observations := DiffGitRepo(tt.desired, tt.actual)
			if len(actions) != tt.wantActions {
				t.Fatalf("expected %d actions, got %d: %v", tt.wantActions, len(actions), actions)
			}
			if len(observations) != tt.wantObservations {
				t.Fatalf("expected %d observations, got %d: %v", tt.wantObservations, len(observations), observations)
			}
			if tt.wantActions > 0 && actions[0].Type != tt.wantType {
				t.Fatalf("expected type %s, got %s", tt.wantType, actions[0].Type)
			}
			if tt.wantDesc != "" {
				var got string
				if tt.wantActions > 0 {
					got = actions[0].Description
				} else {
					got = observations[0].Description
				}
				if got != tt.wantDesc {
					t.Fatalf("description:\n got %q\nwant %q", got, tt.wantDesc)
				}
			}
		})
	}
}
