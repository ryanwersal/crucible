package modules

import (
	"github.com/ryanwersal/crucible/internal/script/decl"
	"strings"
	"testing"
)

func TestCertBundleDeclaration(t *testing.T) {
	t.Parallel()
	fingerprint := strings.Repeat("ab", 32)
	vm, declarations := setupModule(t)
	_, err := vm.RunString(`c.certBundle("/tmp/cert.pem", {absent:["` + fingerprint + `"],sudo:true})`)
	if err != nil {
		t.Fatal(err)
	}
	d := (*declarations)[0]
	if d.Type != decl.CertBundle || !d.CertSudo || d.CertAbsent[0] != strings.ToUpper(fingerprint) {
		t.Fatalf("unexpected declaration: %+v", d)
	}
}

func TestCertBundleRejectsInvalidDeclarations(t *testing.T) {
	t.Parallel()
	for _, source := range []string{
		`c.certBundle()`,
		`c.certBundle("relative", {absent:[]})`,
		`c.certBundle("/tmp/cert.pem", null)`,
		`c.certBundle("/tmp/cert.pem", {})`,
		`c.certBundle("/tmp/cert.pem", {absent:[]})`,
		`c.certBundle("/tmp/cert.pem", {absent:["bad"]})`,
		`c.certBundle("/tmp/cert.pem", {absent:[123]})`,
		`c.certBundle("/tmp/cert.pem", {absent:[null]})`,
		`c.certBundle("/tmp/cert.pem", {absent:new Array(1)})`,
		`c.certBundle("/tmp/cert.pem", {absent:["` + strings.Repeat("ab", 32) + `"],sudo:"true"})`,
		`c.certBundle("/tmp/cert.pem", {absent:["` + strings.Repeat("ab", 32) + `"],state:"absent"})`,
	} {
		t.Run(source, func(t *testing.T) {
			t.Parallel()
			vm, declarations := setupModule(t)
			if _, err := vm.RunString(source); err == nil {
				t.Fatal("accepted invalid declaration")
			}
			if len(*declarations) != 0 {
				t.Fatal("invalid declaration was recorded")
			}
		})
	}
}
