package modules

import (
	"github.com/ryanwersal/crucible/internal/script/decl"
	"path/filepath"
	"testing"
)

func TestSSHKeychainDeclaration(t *testing.T) {
	vm, declarations := setupModule(t)
	if _, err := vm.RunString(`c.keychain.sshKey("~/.ssh/id_ed25519")`); err != nil {
		t.Fatal(err)
	}
	if len(*declarations) != 1 {
		t.Fatal("missing declaration")
	}
	d := (*declarations)[0]
	if d.Type != decl.SSHKeychain || !filepath.IsAbs(d.Path) || filepath.Base(d.Path) != "id_ed25519" {
		t.Fatalf("declaration: %+v", d)
	}
}
func TestSSHKeychainRejectsInvalidDeclarations(t *testing.T) {
	for _, source := range []string{`c.keychain.sshKey()`, `c.keychain.sshKey(null)`, `c.keychain.sshKey(123)`, `c.keychain.sshKey("")`, `c.keychain.sshKey("relative")`, `c.keychain.sshKey("/key",{password:"never"})`, `c.keychain.sshKey("/key\u0000")`} {
		t.Run(source, func(t *testing.T) {
			vm, declarations := setupModule(t)
			if _, err := vm.RunString(source); err == nil {
				t.Fatal("accepted invalid declaration")
			}
			if len(*declarations) != 0 {
				t.Fatal("recorded invalid declaration")
			}
		})
	}
}
