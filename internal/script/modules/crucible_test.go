package modules

import (
	"log/slog"
	"testing"

	"github.com/dop251/goja"

	"github.com/ryanwersal/crucible/internal/script/decl"
)

func setupModule(t *testing.T) (*goja.Runtime, *[]decl.Declaration) {
	t.Helper()
	vm := goja.New()
	declarations := &[]decl.Declaration{}
	mod := NewCrucibleModule(vm, slog.New(slog.DiscardHandler), "/home/user", declarations)
	vm.Set("c", mod.Export())
	return vm, declarations
}

func TestFile_InlineContent(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.file("~/.gitconfig", { content: "[user]\n  name = Test", mode: 0o600 })`)
	if err != nil {
		t.Fatal(err)
	}

	if len(*decls) != 1 {
		t.Fatalf("expected 1 declaration, got %d", len(*decls))
	}

	d := (*decls)[0]
	if d.Type != decl.File {
		t.Errorf("type = %v, want DeclFile", d.Type)
	}
	if d.Path != "/home/user/.gitconfig" {
		t.Errorf("path = %q, want /home/user/.gitconfig", d.Path)
	}
	if string(d.Content) != "[user]\n  name = Test" {
		t.Errorf("content = %q", d.Content)
	}
	if d.Mode != 0o600 {
		t.Errorf("mode = %o, want 600", d.Mode)
	}
}

func TestFile_SourceRef(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.file("~/.config/fish/config.fish", { source: "fish/config.fish" })`)
	if err != nil {
		t.Fatal(err)
	}

	d := (*decls)[0]
	if d.SourceFile != "fish/config.fish" {
		t.Errorf("source = %q", d.SourceFile)
	}
}

func TestFile_TemplateRef(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.file("~/.config/starship.toml", { template: "starship.toml.tmpl", data: { prompt: ">" } })`)
	if err != nil {
		t.Fatal(err)
	}

	d := (*decls)[0]
	if d.TemplateFile != "starship.toml.tmpl" {
		t.Errorf("template = %q", d.TemplateFile)
	}
	if d.TemplateData["prompt"] != ">" {
		t.Errorf("data.prompt = %v", d.TemplateData["prompt"])
	}
}

func TestFile_DefaultMode(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.file("~/test", { content: "x" })`)
	if err != nil {
		t.Fatal(err)
	}

	if (*decls)[0].Mode != 0o644 {
		t.Errorf("default mode = %o, want 644", (*decls)[0].Mode)
	}
}

func TestFile_NoArgs(t *testing.T) {
	t.Parallel()
	vm, _ := setupModule(t)

	_, err := vm.RunString(`c.file()`)
	if err == nil {
		t.Fatal("expected error for file() with no args")
	}
}

func TestDir_Basic(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.dir("~/.config/fish", { mode: 0o700 })`)
	if err != nil {
		t.Fatal(err)
	}

	d := (*decls)[0]
	if d.Type != decl.Dir {
		t.Errorf("type = %v, want DeclDir", d.Type)
	}
	if d.Path != "/home/user/.config/fish" {
		t.Errorf("path = %q", d.Path)
	}
	if d.Mode != 0o700 {
		t.Errorf("mode = %o, want 700", d.Mode)
	}
}

func TestDir_DefaultMode(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.dir("~/.config")`)
	if err != nil {
		t.Fatal(err)
	}

	if (*decls)[0].Mode != 0o755 {
		t.Errorf("default mode = %o, want 755", (*decls)[0].Mode)
	}
}

func TestSymlink_Basic(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.symlink("~/.vimrc", { target: "~/.config/nvim/init.vim" })`)
	if err != nil {
		t.Fatal(err)
	}

	d := (*decls)[0]
	if d.Type != decl.Symlink {
		t.Errorf("type = %v, want DeclSymlink", d.Type)
	}
	if d.Path != "/home/user/.vimrc" {
		t.Errorf("path = %q", d.Path)
	}
	if d.LinkTarget != "/home/user/.config/nvim/init.vim" {
		t.Errorf("target = %q", d.LinkTarget)
	}
}

func TestSymlink_NoTarget(t *testing.T) {
	t.Parallel()
	vm, _ := setupModule(t)

	_, err := vm.RunString(`c.symlink("~/.vimrc", {})`)
	if err == nil {
		t.Fatal("expected error for symlink without target")
	}
}

func TestBrew_Formula(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.brew("coreutils")`)
	if err != nil {
		t.Fatal(err)
	}

	d := (*decls)[0]
	if d.Type != decl.Package {
		t.Errorf("type = %v, want DeclPackage", d.Type)
	}
	if d.PackageName != "coreutils" {
		t.Errorf("name = %q", d.PackageName)
	}
	if d.PackageType != "formula" {
		t.Errorf("type = %q, want formula", d.PackageType)
	}
}

func TestBrew_Cask(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.brew("firefox", { type: "cask" })`)
	if err != nil {
		t.Fatal(err)
	}

	d := (*decls)[0]
	if d.PackageType != "cask" {
		t.Errorf("type = %q, want cask", d.PackageType)
	}
}

func TestLog(t *testing.T) {
	t.Parallel()
	vm, _ := setupModule(t)

	// Should not panic
	_, err := vm.RunString(`c.log("hello from script")`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestExpandPath(t *testing.T) {
	t.Parallel()

	vm := goja.New()
	decls := &[]decl.Declaration{}
	mod := NewCrucibleModule(vm, slog.New(slog.DiscardHandler), "/home/user", decls)

	tests := []struct {
		input string
		want  string
	}{
		{"~/.bashrc", "/home/user/.bashrc"},
		{"~", "/home/user"},
		{"/etc/hosts", "/etc/hosts"},
		{"~/a/../b", "/home/user/b"},
	}

	for _, tt := range tests {
		if got := mod.expandPath(tt.input); got != tt.want {
			t.Errorf("expandPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
