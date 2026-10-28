package certbundle

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testCert(t *testing.T, name string) ([]byte, string) {
	t.Helper()
	return testCertWithSerial(t, name, 1)
}

func testCertWithSerial(t *testing.T, name string, serial int64) ([]byte, string) {
	t.Helper()
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name}, NotBefore: time.Unix(0, 0), NotAfter: time.Unix(2000000000, 0), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, template, template, pub, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), Digest(der)
}

func TestFilterExactCertificates(t *testing.T) {
	t.Parallel()
	denied, fingerprint := testCert(t, "same name")
	keep, _ := testCert(t, "same name")
	original := bytes.Join([][]byte{[]byte("# vendor header\n"), denied, []byte("=== retained text\n"), keep, denied, []byte("# end\n")}, nil)
	filtered, removed, err := Filter(original, []string{strings.ToLower(fingerprint)})
	if err != nil {
		t.Fatal(err)
	}
	want := bytes.Join([][]byte{[]byte("# vendor header\n=== retained text\n"), keep, []byte("# end\n")}, nil)
	if !bytes.Equal(filtered, want) || len(removed) != 2 {
		t.Fatal("must remove duplicates by DER identity, preserving all other bytes")
	}
	second, removed, err := Filter(filtered, []string{fingerprint})
	if err != nil || len(removed) != 0 || !bytes.Equal(second, filtered) {
		t.Fatalf("not idempotent: %v", err)
	}
}

func TestFilterRejectsUnsafeBundles(t *testing.T) {
	t.Parallel()
	cert, fingerprint := testCert(t, "only root")
	for name, data := range map[string][]byte{
		"empty":                  nil,
		"text only":              []byte("# no certs"),
		"truncated":              bytes.TrimSuffix(cert, []byte("-----END CERTIFICATE-----\n")),
		"malformed before valid": append([]byte("-----BEGIN CERTIFICATE-----\ninvalid\n-----END CERTIFICATE-----\n"), cert...),
		"malformed after valid":  append(bytes.Clone(cert), []byte("-----BEGIN CERTIFICATE-----\n")...),
		"non certificate":        append(bytes.Clone(cert), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("bad")})...),
		"last certificate":       cert,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, _, err := Filter(data, []string{fingerprint}); err == nil {
				t.Fatal("expected refusal")
			}
		})
	}
}

func TestApplyBackupMetadataAndDrift(t *testing.T) {
	t.Parallel()
	denied, fingerprint := testCert(t, "denied")
	keep, _ := testCert(t, "keep")
	original := append(bytes.Clone(denied), keep...)
	path := filepath.Join(t.TempDir(), "cert.pem")
	if err := os.WriteFile(path, original, 0o640); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	before, err := Read(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	backup, err := Apply(ctx, path, before.Token, []string{fingerprint})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(backup)
	if err != nil || !bytes.Equal(saved, original) {
		t.Fatalf("backup: %v", err)
	}
	after, err := Read(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after.Content, keep) || after.Mode != before.Mode || after.UID != before.UID || after.GID != before.GID {
		t.Fatal("content or metadata incorrect")
	}
	if _, err := Apply(ctx, path, before.Token, []string{fingerprint}); err == nil {
		t.Fatal("accepted stale plan")
	}
	if backup, err := Apply(ctx, path, after.Token, []string{fingerprint}); err != nil || backup != "" {
		t.Fatalf("no-op apply: %q %v", backup, err)
	}
	// A vendor update reintroduces the root along with a new unrelated certificate.
	newCert, _ := testCert(t, "vendor addition")
	update := append(bytes.Clone(original), newCert...)
	if err := os.WriteFile(path, update, 0o640); err != nil {
		t.Fatal(err)
	}
	updated, err := Read(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	nextBackup, err := Apply(ctx, path, updated.Token, []string{fingerprint})
	if err != nil || nextBackup == backup {
		t.Fatalf("versioned backup: %v", err)
	}
	actual, _ := os.ReadFile(path)
	if !bytes.Equal(actual, append(bytes.Clone(keep), newCert...)) {
		t.Fatal("vendor addition was lost")
	}
	saved, _ = os.ReadFile(backup)
	if !bytes.Equal(saved, original) {
		t.Fatal("original backup overwritten")
	}
}

func TestApplyRefusesChangedOrUnsafeFiles(t *testing.T) {
	t.Parallel()
	denied, fingerprint := testCert(t, "denied")
	keep, _ := testCert(t, "keep")
	original := append(bytes.Clone(denied), keep...)
	for _, scenario := range []string{"content", "permissions", "replacement", "symlink", "backup collision", "backup symlink", "relative symlink", "relative backup symlink", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := filepath.Join(dir, "cert.pem")
			if err := os.WriteFile(path, original, 0o644); err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			snapshot, err := Read(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			backup := path + ".crucible-backup-" + Digest(original)
			switch scenario {
			case "content":
				err = os.WriteFile(path, append(bytes.Clone(original), []byte("# changed\n")...), 0o644)
			case "permissions":
				err = os.Chmod(path, 0o600)
			case "replacement":
				if err = os.Remove(path); err == nil {
					err = os.WriteFile(path, original, 0o644)
				}
			case "symlink":
				if err = os.Rename(path, path+".real"); err == nil {
					err = os.Symlink(path+".real", path)
				}
			case "relative symlink":
				if err = os.Rename(path, path+".real"); err == nil {
					err = os.Symlink("cert.pem.real", path)
				}
			case "relative backup symlink":
				err = os.Symlink("cert.pem", backup)
			case "backup collision":
				err = os.WriteFile(backup, []byte("do not overwrite"), 0o644)
			case "backup symlink":
				err = os.Symlink(path, backup)
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if err != nil {
				t.Fatal(err)
			}
			unchanged, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Apply(ctx, path, snapshot.Token, []string{fingerprint}); err == nil {
				t.Fatal("expected refusal")
			}
			actual, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(unchanged, actual) {
				t.Fatal("unsafe apply changed target")
			}
		})
	}
}

func TestReadRejectsMissingAndNonRegular(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, path := range []string{dir, filepath.Join(dir, "missing")} {
		if _, err := Read(context.Background(), path); err == nil {
			t.Fatalf("accepted %s", path)
		}
	}
}

func TestLegacyNegativeSerialKeepsOriginalIdentity(t *testing.T) {
	t.Parallel()
	positive, _ := testCert(t, "legacy root")
	block, _ := pem.Decode(positive)
	// The fixture uses serial 1: locate the serial immediately following the
	// explicit v3 version, and make it negative without changing DER lengths.
	marker := []byte{0xa0, 0x03, 0x02, 0x01, 0x02, 0x02, 0x01, 0x01}
	index := bytes.Index(block.Bytes, marker)
	if index < 0 {
		t.Fatal("fixture serial not found")
	}
	block.Bytes[index+len(marker)-1] = 0x81
	if _, err := x509.ParseCertificate(block.Bytes); err == nil {
		t.Fatal("fixture should exercise strict negative-serial rejection")
	}
	negative := pem.EncodeToMemory(block)
	certs, err := Parse(negative)
	if err != nil {
		t.Fatal(err)
	}
	if certs[0].Fingerprint != Digest(block.Bytes) || certs[0].Subject != "CN=legacy root" {
		t.Fatal("inspection changed certificate identity")
	}
	keep, fingerprint := testCert(t, "removed root")
	bundle := append(bytes.Clone(negative), keep...)
	filtered, _, err := Filter(bundle, []string{fingerprint})
	if err != nil || !bytes.Equal(filtered, negative) {
		t.Fatalf("legacy bytes changed: %v", err)
	}
	filtered, removed, err := Filter(bundle, []string{Digest(block.Bytes)})
	if err != nil || len(removed) != 1 || !bytes.Equal(filtered, keep) {
		t.Fatalf("legacy identity removal failed: %v", err)
	}
}

func TestLegacyMultibyteSerialAndMalformedSignature(t *testing.T) {
	t.Parallel()
	positive, _ := testCertWithSerial(t, "legacy multibyte", 256)
	block, _ := pem.Decode(positive)
	marker := []byte{0xa0, 0x03, 0x02, 0x01, 0x02, 0x02, 0x02, 0x01, 0x00}
	index := bytes.Index(block.Bytes, marker)
	if index < 0 {
		t.Fatal("fixture serial not found")
	}
	block.Bytes[index+len(marker)-2] = 0x80
	if _, err := Parse(pem.EncodeToMemory(block)); err != nil {
		t.Fatal(err)
	}
	// An Ed25519 signature is a 65-byte BIT STRING (one unused-bit byte).
	signature := bytes.LastIndex(block.Bytes, []byte{0x03, 0x41, 0x00})
	if signature < 0 {
		t.Fatal("fixture signature not found")
	}
	block.Bytes[signature] = 0x04
	if _, err := Parse(pem.EncodeToMemory(block)); err == nil {
		t.Fatal("negative serial masked an invalid signature field")
	}
}
