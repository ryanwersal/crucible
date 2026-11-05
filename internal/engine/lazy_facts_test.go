package engine

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These tests use the production fact store and fake executables, so they cover
// lazy collection through both JavaScript and resource planning.
func lazyFactsTools(t *testing.T, brewBody string) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	t.Setenv("PATH", dir)
	t.Setenv("CRUCIBLE_TEST_FACT_CALLS", log)
	logCall := "#!/bin/sh\nprintf '%s %s\\n' \"${0##*/}\" \"$*\" >> \"$CRUCIBLE_TEST_FACT_CALLS\"\n"
	mustWriteFile(t, filepath.Join(dir, "brew"), []byte(logCall+brewBody), 0o755)
	mustWriteFile(t, filepath.Join(dir, "mas"), []byte(logCall+"exit 42\n"), 0o755)
	return log
}

func lazyFactsEngine(t *testing.T, source string) *Engine {
	t.Helper()
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "crucible.js"), []byte(`var c = require("crucible");`+source), 0o644)
	return New(dir, t.TempDir(), slog.New(slog.DiscardHandler))
}

func TestPlanCertificateOnlySkipsPackageTools(t *testing.T) {
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "retained root"},
		NotBefore: time.Unix(0, 0), NotAfter: time.Unix(2000000000, 0),
		IsCA: true, BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, pub, key)
	if err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(t.TempDir(), "cert.pem")
	mustWriteFile(t, bundle, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644)
	source := fmt.Sprintf(`
if (!c.facts.os.name) throw new Error("missing OS facts");
c.certBundle(%q, {absent: [%q]});`, bundle, strings.Repeat("A", 64))
	for _, refresh := range []bool{true, false} {
		t.Run(fmt.Sprintf("refresh=%t", refresh), func(t *testing.T) {
			log := lazyFactsTools(t, "exit 42\n")
			eng := lazyFactsEngine(t, source)
			if !refresh {
				eng.SetHomebrewRefresh(false)
			}
			result, err := eng.Plan(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Actions) != 0 || len(result.Observations) != 1 || !strings.Contains(result.Observations[0].Description, "denied certificates absent") {
				t.Fatalf("unexpected certificate plan: %+v", result)
			}
			if calls, err := os.ReadFile(log); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("certificate-only plan invoked package tools: %q (read error: %v)", calls, err)
			}
		})
	}
}

func TestPlanRequiredPackageFactsPropagateFailures(t *testing.T) {
	for _, tc := range []struct {
		name, source, tool string
	}{
		{"JavaScript", `c.facts.homebrew.available;`, "brew"},
		{"package", `c.brew("example");`, "brew"},
		{"tap", `c.brew.tap("example/tools");`, "brew"},
		{"App Store", `c.mas(123, "Example");`, "mas"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log := lazyFactsTools(t, "exit 42\n")
			eng := lazyFactsEngine(t, tc.source)
			eng.SetHomebrewRefresh(false)
			if _, err := eng.Plan(context.Background()); err == nil || !strings.Contains(err.Error(), tc.tool) {
				t.Fatalf("expected %s collection error, got %v", tc.tool, err)
			}
			calls, err := os.ReadFile(log)
			if err != nil || !strings.HasPrefix(string(calls), tc.tool+" ") {
				t.Fatalf("required tool was not called: %q, %v", calls, err)
			}
		})
	}
}

func TestPlanLazyHomebrewPreservesRefreshAndCaching(t *testing.T) {
	for _, source := range []string{
		`c.brew("example");`,
		`c.brew.tap("example/tools");`,
		`var first = c.facts.homebrew;
if (!first.available || first !== c.facts.homebrew) throw new Error("uncached Homebrew facts");
c.brew("example"); c.brew.tap("example/tools");`,
	} {
		for _, refresh := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/refresh=%t", source, refresh), func(t *testing.T) {
				log := lazyFactsTools(t, `case "$1" in
update) exit 0 ;;
info|outdated) printf '{"formulae":[],"casks":[]}\n' ;;
tap) exit 0 ;;
*) exit 43 ;;
esac
`)
				eng := lazyFactsEngine(t, source)
				if !refresh {
					eng.SetHomebrewRefresh(false)
				}
				if _, err := eng.Plan(context.Background()); err != nil {
					t.Fatal(err)
				}
				calls, err := os.ReadFile(log)
				if err != nil {
					t.Fatal(err)
				}
				want := "brew info --json=v2 --installed\nbrew outdated --json=v2 --greedy\nbrew tap\n"
				if refresh {
					want = "brew update --quiet\n" + want
				}
				if string(calls) != want {
					t.Fatalf("tool calls = %q, want %q", calls, want)
				}
			})
		}
	}
}
