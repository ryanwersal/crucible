package keychain

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSSHKeyProbe(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Apple OpenSSH integration")
	}
	for _, password := range []string{"", "crucible-test-only"} {
		name := "unencrypted"
		if password != "" {
			name = "needs-enrollment"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "key")
			cmd := exec.CommandContext(t.Context(), "/usr/bin/ssh-keygen", "-q", "-t", "ed25519", "-N", password, "-f", path)
			if err := cmd.Run(); err != nil {
				t.Fatal(err)
			}
			client := SSHClient{}
			available, err := client.Available(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			if available != (password == "") {
				t.Fatalf("available = %v", available)
			}
			err = client.Store(t.Context(), path, strings.NewReader("must not become a passphrase"), io.Discard, io.Discard)
			if password != "" && (err == nil || !strings.Contains(err.Error(), "interactive terminal")) {
				t.Fatalf("expected terminal requirement, got %v", err)
			}
			if password == "" && err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSSHKeyProbeRejectsInvalidKeys(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Apple OpenSSH integration")
	}
	directory := t.TempDir()
	invalid := filepath.Join(directory, "invalid")
	if err := os.WriteFile(invalid, []byte("not a private key"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{directory, invalid, filepath.Join(directory, "missing"), "relative"} {
		if _, err := (SSHClient{}).Available(t.Context(), path); err == nil {
			t.Fatalf("accepted %s", path)
		}
	}
}

func TestTemporaryAgentIsRemovedOnFailureAndCancellation(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Apple OpenSSH integration")
	}
	sentinel := errors.New("callback failed")
	var socket string
	err := withAgent(t.Context(), func(path string) error { socket = path; return sentinel })
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(filepath.Dir(socket)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary agent directory remains: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	started := time.Now()
	if err := withAgent(ctx, func(string) error { t.Fatal("canceled agent ran callback"); return nil }); err == nil {
		t.Fatal("expected cancellation")
	}
	if time.Since(started) > time.Second {
		t.Fatal("cancellation took too long")
	}
}

func TestEnrollmentRequiresRetainedKeychainAccess(t *testing.T) {
	sentinel := errors.New("keychain unavailable")
	for _, test := range []struct {
		name                             string
		initial, retained                bool
		initialErr, enrollErr, verifyErr error
		wantEnroll                       bool
		wantErr                          bool
	}{
		{name: "already available", initial: true},
		{name: "probe failure", initialErr: sentinel, wantErr: true},
		{name: "enrolled", retained: true, wantEnroll: true},
		{name: "canceled", enrollErr: context.Canceled, wantEnroll: true, wantErr: true},
		{name: "not retained", wantEnroll: true, wantErr: true},
		{name: "verification failed", verifyErr: sentinel, wantEnroll: true, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			enrolled := false
			probes := 0
			err := ensureStored(func() (bool, error) {
				probes++
				if !enrolled {
					return test.initial, test.initialErr
				}
				return test.retained, test.verifyErr
			}, func() error { enrolled = true; return test.enrollErr })
			if enrolled != test.wantEnroll || (err != nil) != test.wantErr {
				t.Fatalf("enrolled=%v error=%v", enrolled, err)
			}
			if test.enrollErr == nil && enrolled && probes != 2 {
				t.Fatal("enrollment was not independently verified")
			}
			if test.enrollErr != nil && !errors.Is(err, test.enrollErr) {
				t.Fatal("enrollment error was lost")
			}
		})
	}
}
