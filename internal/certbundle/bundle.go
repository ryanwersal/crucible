// Package certbundle inspects and filters PEM certificate bundles without
// taking ownership of the vendor's remaining certificates or text.
package certbundle

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"strings"
)

// Certificate identifies an exact DER certificate and its PEM byte range.
type Certificate struct {
	Fingerprint string
	Subject     string
	Start, End  int
}

// NormalizeFingerprints validates SHA-256 fingerprints and removes duplicates.
func NormalizeFingerprints(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("absent must contain at least one SHA-256 fingerprint")
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), ":", ""))
		decoded, err := hex.DecodeString(value)
		if err != nil || len(decoded) != sha256.Size {
			return nil, fmt.Errorf("invalid SHA-256 certificate fingerprint %q", value)
		}
		if !seen[value] {
			result = append(result, value)
			seen[value] = true
		}
	}
	return result, nil
}

// Parse rejects malformed, non-certificate, or empty PEM bundles. Text outside
// PEM blocks is retained, including Apple's human-readable certificate dumps.
func Parse(data []byte) ([]Certificate, error) {
	var certs []Certificate
	start := -1
	offset := 0
	for line := range bytes.SplitAfterSeq(data, []byte("\n")) {
		trimmed := bytes.TrimSpace(line)
		if bytes.Contains(line, []byte("-----BEGIN")) {
			if start >= 0 || !bytes.Equal(trimmed, []byte("-----BEGIN CERTIFICATE-----")) {
				return nil, fmt.Errorf("invalid PEM opening at byte %d", offset)
			}
			start = offset
		} else if bytes.Contains(line, []byte("-----END")) {
			if start < 0 || !bytes.Equal(trimmed, []byte("-----END CERTIFICATE-----")) {
				return nil, fmt.Errorf("invalid PEM closing at byte %d", offset)
			}
			end := offset + len(line)
			block, rest := pem.Decode(data[start:end])
			if block == nil || len(bytes.TrimSpace(rest)) != 0 || len(block.Headers) != 0 {
				return nil, fmt.Errorf("invalid certificate PEM at byte %d", start)
			}
			cert, err := parseCertificate(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("parse certificate at byte %d: %w", start, err)
			}
			certs = append(certs, Certificate{Fingerprint: Digest(block.Bytes), Subject: cert.Subject.String(), Start: start, End: end})
			start = -1
		}
		offset += len(line)
	}
	if start >= 0 {
		return nil, fmt.Errorf("unterminated certificate PEM at byte %d", start)
	}
	if len(certs) == 0 {
		return nil, fmt.Errorf("bundle contains no certificates")
	}
	return certs, nil
}

// Digest returns an uppercase SHA-256 digest.
func Digest(data []byte) string { return fmt.Sprintf("%X", sha256.Sum256(data)) }

// Filter removes every occurrence of the exact denied certificates. All other
// bytes remain untouched. Removing the final certificate is refused.
func Filter(data []byte, absent []string) ([]byte, []Certificate, error) {
	fingerprints, err := NormalizeFingerprints(absent)
	if err != nil {
		return nil, nil, err
	}
	certs, err := Parse(data)
	if err != nil {
		return nil, nil, err
	}
	denied := make(map[string]bool, len(fingerprints))
	for _, fingerprint := range fingerprints {
		denied[fingerprint] = true
	}
	var removed []Certificate
	var out bytes.Buffer
	offset := 0
	for _, cert := range certs {
		if denied[cert.Fingerprint] {
			out.Write(data[offset:cert.Start])
			offset = cert.End
			removed = append(removed, cert)
		}
	}
	out.Write(data[offset:])
	if len(removed) == len(certs) {
		return nil, nil, fmt.Errorf("refusing to remove every certificate from bundle")
	}
	return out.Bytes(), removed, nil
}

// parseCertificate accepts legacy negative serials found in vendor root bundles
// without relaxing process-wide TLS verification. The normal x509 parser still
// validates the entire structure. For inspection only, substitute a positive serial in
// a private copy; identity and output always use the untouched original DER.
// No certificate signature or trust decision is made by this package.
func parseCertificate(der []byte) (*x509.Certificate, error) {
	cert, err := x509.ParseCertificate(der)
	if err == nil {
		return cert, nil
	}
	// crypto/x509 exposes no sentinel for this legacy compatibility case.
	// Fail closed if another error (or a changed Go error) is reported.
	if err.Error() != "x509: negative serial number" {
		return nil, err
	}
	var outer, tbs, field asn1.RawValue
	rest, decodeErr := asn1.Unmarshal(der, &outer)
	if decodeErr != nil || len(rest) != 0 || outer.Tag != asn1.TagSequence {
		return nil, err
	}
	if _, decodeErr = asn1.Unmarshal(outer.Bytes, &tbs); decodeErr != nil || tbs.Tag != asn1.TagSequence {
		return nil, err
	}
	fields := tbs.Bytes
	offset := len(der) - len(outer.Bytes) + len(tbs.FullBytes) - len(tbs.Bytes)
	rest, decodeErr = asn1.Unmarshal(fields, &field)
	if decodeErr != nil {
		return nil, err
	}
	if field.Class == asn1.ClassContextSpecific && field.Tag == 0 {
		offset += len(field.FullBytes)
		if _, decodeErr = asn1.Unmarshal(rest, &field); decodeErr != nil {
			return nil, err
		}
	}
	if field.Class != asn1.ClassUniversal || field.Tag != asn1.TagInteger || len(field.Bytes) == 0 || field.Bytes[0]&0x80 == 0 {
		return nil, err
	}
	inspection := bytes.Clone(der)
	// A nonzero leading byte also preserves minimal DER for values like 80 00.
	inspection[offset+len(field.FullBytes)-len(field.Bytes)] = 1
	return x509.ParseCertificate(inspection)
}
