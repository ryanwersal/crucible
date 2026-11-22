package keychain

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type SSHKeys interface {
	Available(context.Context, string) (bool, error)
	Store(context.Context, string, io.Reader, io.Writer, io.Writer) error
}

type SSHClient struct{}

func validateKey(path string) error {
	if runtime.GOOS != "darwin" {
		return errors.New("SSH Keychain management requires macOS")
	}
	if !filepath.IsAbs(path) {
		return errors.New("SSH key path must be absolute")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect SSH key: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("SSH key must be a regular file")
	}
	return nil
}

// A private agent tests Keychain access independently of keys already loaded in
// the user's agent. Apple handles all private-key and passphrase data.
func withAgent(ctx context.Context, run func(string) error) (err error) {
	directory, err := os.MkdirTemp("", "crucible-ssh-")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(directory)) }()
	socket := filepath.Join(directory, "agent")
	agentContext, stop := context.WithCancel(ctx)
	defer stop()
	agent := exec.CommandContext(agentContext, "/usr/bin/ssh-agent", "-D", "-a", socket)
	if err := agent.Start(); err != nil {
		return fmt.Errorf("start temporary SSH agent: %w", err)
	}
	defer func() {
		stop()
		waitErr := agent.Wait()
		if waitErr != nil {
			if exit, ok := errors.AsType[*exec.ExitError](waitErr); !ok || exit.ExitCode() != -1 {
				err = errors.Join(err, waitErr)
			}
		}
	}()
	ready, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		info, statErr := os.Stat(socket)
		if statErr == nil && info.Mode()&os.ModeSocket != 0 {
			return run(socket)
		}
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return statErr
		}
		select {
		case <-ready.Done():
			return fmt.Errorf("temporary SSH agent did not start: %w", ready.Err())
		case <-ticker.C:
		}
	}
}

func sshAdd(ctx context.Context, socket, path string, prompt bool) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "/usr/bin/ssh-add", "-q", "--apple-use-keychain", path)
	cmd.Env = append(os.Environ(), "SSH_AUTH_SOCK="+socket, "APPLE_SSH_ADD_BEHAVIOR=macos")
	if prompt {
		cmd.Env = append(cmd.Env, "SSH_ASKPASS_REQUIRE=never")
		// Give OpenSSH time to restore terminal echo when enrollment is canceled.
		cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
		cmd.WaitDelay = 2 * time.Second
	} else {
		cmd.Env = append(cmd.Env, "SSH_ASKPASS_REQUIRE=force", "SSH_ASKPASS=/usr/bin/false", "DISPLAY=crucible")
	}
	return cmd
}

func probeResult(ctx context.Context, err error, diagnostic string) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if err == nil {
		return true, nil
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok && exit.ExitCode() == 1 && strings.TrimSpace(diagnostic) == "" {
		return false, nil
	}
	return false, fmt.Errorf("check SSH Keychain access: %w (%s)", err, strings.TrimSpace(diagnostic))
}

func (SSHClient) Available(ctx context.Context, path string) (available bool, err error) {
	if err := validateKey(path); err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	err = withAgent(ctx, func(socket string) error {
		var diagnostic bytes.Buffer
		cmd := sshAdd(ctx, socket, path, false)
		cmd.Stderr = &diagnostic
		runErr := cmd.Run()
		var probeErr error
		available, probeErr = probeResult(ctx, runErr, diagnostic.String())
		return probeErr
	})
	return available, err
}

func (client SSHClient) Store(ctx context.Context, path string, stdin io.Reader, stdout, stderr io.Writer) error {
	return ensureStored(func() (bool, error) { return client.Available(ctx, path) }, func() error {
		if !terminalInput(stdin) {
			return errors.New("SSH passphrase enrollment requires an interactive terminal; rerun apply in your terminal")
		}
		err := withAgent(ctx, func(socket string) error {
			cmd := sshAdd(ctx, socket, path, true)
			cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
			return cmd.Run()
		})
		if err != nil {
			return fmt.Errorf("store SSH passphrase in Keychain: %w", err)
		}
		return nil
	})
}

func ensureStored(probe func() (bool, error), enroll func() error) error {
	available, err := probe()
	if err != nil || available {
		return err
	}
	if err := enroll(); err != nil {
		return err
	}
	available, err = probe()
	if err != nil {
		return err
	}
	if !available {
		return errors.New("SSH key was unlocked but its passphrase was not retained in Keychain; unlock your login Keychain and retry apply")
	}
	return nil
}
