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

	// Use a local file:// URL that points at a nonexistent path so
	// ls-remote fails fast without touching the network.
	remoteURL := "file://" + filepath.Join(t.TempDir(), "does-not-exist")
	if err := exec.Command("git", "-C", dir, "remote", "add", "origin", remoteURL).Run(); err != nil {
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
	if info.RemoteURL != remoteURL {
		t.Fatalf("unexpected remote URL: %q", info.RemoteURL)
	}
	if info.CurrentBranch == "" {
		t.Fatal("expected non-empty branch")
	}
	if info.LocalSHA == "" {
		t.Fatal("expected non-empty local SHA")
	}
	if info.RemoteSHA != "" {
		t.Fatalf("expected empty RemoteSHA for unreachable remote, got %q", info.RemoteSHA)
	}
}

func TestGitRepoCollector_LocalRemoteInSync(t *testing.T) {
	t.Parallel()

	// Create an origin (bare) repo and a working clone so ls-remote
	// actually resolves, letting us verify SHA comparison end-to-end.
	origin := t.TempDir()
	if out, err := exec.Command("git", "init", "--bare", "-b", "main", origin).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}

	// Init a working repo, seed it, then push to the bare origin.
	// Cloning first won't work — the bare repo has no branches yet.
	work := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main", work},
		{"-C", work, "config", "user.email", "test@test.com"},
		{"-C", work, "config", "user.name", "Test"},
		{"-C", work, "remote", "add", "origin", origin},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(work, "README"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"-C", work, "add", "."},
		{"-C", work, "commit", "-m", "init"},
		{"-C", work, "push", "-u", "origin", "main"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	c := GitRepoCollector{Path: work}
	info, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.LocalSHA == "" || info.RemoteSHA == "" {
		t.Fatalf("expected both SHAs populated: local=%q remote=%q", info.LocalSHA, info.RemoteSHA)
	}
	if info.LocalSHA != info.RemoteSHA {
		t.Fatalf("expected SHAs to match: local=%q remote=%q", info.LocalSHA, info.RemoteSHA)
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
