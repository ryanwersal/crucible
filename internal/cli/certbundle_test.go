package cli

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ryanwersal/crucible/internal/certbundle"
)

func bundleFixture(t *testing.T) (string, []byte, []byte, string) {
	t.Helper()
	makeCert := func(name string) []byte {
		pub, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: name}, NotBefore: time.Unix(0, 0), NotAfter: time.Unix(2000000000, 0), IsCA: true, BasicConstraintsValid: true}
		der, err := x509.CreateCertificate(rand.Reader, template, template, pub, key)
		if err != nil {
			t.Fatal(err)
		}
		return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	}
	denied, keep := makeCert("denied"), makeCert("keep")
	certs, err := certbundle.Parse(denied)
	if err != nil {
		t.Fatal(err)
	}
	original := append(bytes.Clone(denied), keep...)
	path := filepath.Join(t.TempDir(), "cert.pem")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	return path, original, keep, certs[0].Fingerprint
}

func TestCertBundleCLIPlanApply(t *testing.T) {
	t.Parallel()
	path, original, keep, fingerprint := bundleFixture(t)
	dir := t.TempDir()
	script := fmt.Sprintf(`var c=require("crucible"); c.certBundle(%q,{absent:[%q]});`, path, fingerprint)
	if err := os.WriteFile(filepath.Join(dir, "crucible.js"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, run := testCmdDirs(dir, t.TempDir())
	if err := run("apply", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), fingerprint) || !strings.Contains(out.String(), "CN=denied") {
		t.Fatalf("plan omitted identity: %s", out)
	}
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, original) {
		t.Fatal("dry-run modified bundle")
	}
	backups, err := filepath.Glob(path + ".crucible-backup-*")
	if err != nil || len(backups) != 0 {
		t.Fatal("dry-run created backups")
	}
	_, _, apply := testCmdDirs(dir, t.TempDir())
	if err := apply("apply", "--yes"); err != nil {
		t.Fatal(err)
	}
	actual, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, keep) {
		t.Fatal("apply did not filter bundle")
	}
	out, _, verify := testCmdDirs(dir, t.TempDir())
	if err := verify("apply", "--dry-run", "--show-plan"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "denied certificates absent") || !strings.Contains(out.String(), "Everything up to date") {
		t.Fatalf("not converged: %s", out)
	}
}

func TestCertBundleCLIHelper(t *testing.T) {
	t.Parallel()
	path, _, keep, fingerprint := bundleFixture(t)
	snapshot, err := certbundle.Read(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	_, _, run := testCmd(t)
	if err := run("_filter-cert-bundle", path, snapshot.Token, fingerprint); err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, keep) {
		t.Fatal("helper did not filter approved bundle")
	}
	_, _, stale := testCmd(t)
	if err := stale("_filter-cert-bundle", path, snapshot.Token, fingerprint); err == nil {
		t.Fatal("helper accepted stale snapshot")
	}
}

func TestCertBundleCLIRejectsConflicts(t *testing.T) {
	t.Parallel()
	for _, bundleFirst := range []bool{true, false} {
		t.Run(fmt.Sprint(bundleFirst), func(t *testing.T) {
			t.Parallel()
			path, _, _, fingerprint := bundleFixture(t)
			bundle := fmt.Sprintf(`c.certBundle(%q,{absent:[%q]});`, path, fingerprint)
			file := fmt.Sprintf(`c.file(%q,{content:"overwrite"});`, path)
			script := "var c=require(\"crucible\");" + file + bundle
			if bundleFirst {
				script = "var c=require(\"crucible\");" + bundle + file
			}
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "crucible.js"), []byte(script), 0o644); err != nil {
				t.Fatal(err)
			}
			_, _, run := testCmdDirs(dir, t.TempDir())
			if err := run("apply", "--dry-run"); err == nil {
				t.Fatal("accepted conflicting ownership")
			}
		})
	}
}
