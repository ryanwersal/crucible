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
	}{
		{
			name:        "new clone",
			desired:     DesiredGitRepo{Path: "/home/user/.oh-my-zsh", URL: "https://github.com/ohmyzsh/ohmyzsh.git", Branch: "master"},
			actual:      nil,
			wantActions: 1,
			wantType:    CloneRepo,
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
				LocalSHA:      "abc123",
			},
			wantActions: 1,
			wantType:    PullRepo,
		},
		{
			name:    "local drift pulls",
			desired: DesiredGitRepo{Path: "/home/user/.oh-my-zsh", URL: "https://github.com/ohmyzsh/ohmyzsh.git", Branch: "master"},
			actual: &fact.GitRepoInfo{
				Exists:        true,
				RemoteURL:     "https://github.com/ohmyzsh/ohmyzsh.git",
				CurrentBranch: "master",
				LocalSHA:      "abc123",
				RemoteSHA:     "def456",
			},
			wantActions: 1,
			wantType:    PullRepo,
		},
		{
			name:    "in sync skips pull",
			desired: DesiredGitRepo{Path: "/home/user/.oh-my-zsh", URL: "https://github.com/ohmyzsh/ohmyzsh.git", Branch: "master"},
			actual: &fact.GitRepoInfo{
				Exists:        true,
				RemoteURL:     "https://github.com/ohmyzsh/ohmyzsh.git",
				CurrentBranch: "master",
				LocalSHA:      "abc123",
				RemoteSHA:     "abc123",
			},
			wantActions:      0,
			wantObservations: 0,
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
		})
	}
}
