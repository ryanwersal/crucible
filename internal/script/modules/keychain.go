package modules

import (
	"fmt"
	"github.com/dop251/goja"
	"github.com/ryanwersal/crucible/internal/script/decl"
	"path/filepath"
	"strings"
)

func (m *CrucibleModule) sshKeychain(call goja.FunctionCall) goja.Value {
	path, ok := call.Argument(0).Export().(string)
	if len(call.Arguments) != 1 || !ok || strings.TrimSpace(path) == "" || strings.ContainsRune(path, 0) {
		panic(m.vm.NewGoError(fmt.Errorf("keychain.sshKey(): requires one SSH private-key path")))
	}
	path = m.expandPath(path)
	if !filepath.IsAbs(path) {
		panic(m.vm.NewGoError(fmt.Errorf("keychain.sshKey(): path must be absolute or begin with ~/")))
	}
	*m.declarations = append(*m.declarations, decl.Declaration{Type: decl.SSHKeychain, Path: path})
	return goja.Undefined()
}
