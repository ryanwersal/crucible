package fact

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGitRepoCollector_NonExistent(t *testing.T) {
	t.Parallel()
	c := GitRepoCollector{Path: "/nonexistent/repo"}
	info, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Exists {
		t.Fatal("expected Exists=false")
	}
}

func TestGitRepoCollector_ExistingRepo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Initialize a git repo
	cmd := exec.Command("git", "init", dir)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// Configure user for the test repo
	if err := exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", dir, "config", "user.name", "Test").Run(); err != nil {
		t.Fatal(err)
	}

	// Create an initial commit so branch exists
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", dir, "add", ".").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", dir, "commit", "-m", "init").Run(); err != nil {
		t.Fatal(err)
	}

	// Add a remote
	if err := exec.Command("git", "-C", dir, "remote", "add", "origin", "https://github.com/test/repo.git").Run(); err != nil {
		t.Fatal(err)
	}

	c := GitRepoCollector{Path: dir}
	info, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !info.Exists {
		t.Fatal("expected Exists=true")
	}
	if info.RemoteURL != "https://github.com/test/repo.git" {
		t.Fatalf("unexpected remote URL: %q", info.RemoteURL)
	}
	if info.CurrentBranch == "" {
		t.Fatal("expected non-empty branch")
	}
}

func TestGitRepoCollector_NoRemote(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cmd := exec.Command("git", "init", dir)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	c := GitRepoCollector{Path: dir}
	info, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !info.Exists {
		t.Fatal("expected Exists=true")
	}
	if info.RemoteURL != "" {
		t.Fatalf("expected empty remote URL, got %q", info.RemoteURL)
	}
}
