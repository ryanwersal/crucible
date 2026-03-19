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
	_ = vm.Set("c", mod.Export())
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
}

func TestBrew_TapQualified(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.brew("ryanwersal/tools/helios")`)
	if err != nil {
		t.Fatal(err)
	}

	d := (*decls)[0]
	if d.PackageName != "ryanwersal/tools/helios" {
		t.Errorf("name = %q", d.PackageName)
	}
}

func TestBrew_Array(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.brew(["ripgrep", "fd", "bat"])`)
	if err != nil {
		t.Fatal(err)
	}

	if len(*decls) != 3 {
		t.Fatalf("expected 3 declarations, got %d", len(*decls))
	}
	want := []string{"ripgrep", "fd", "bat"}
	for i, name := range want {
		d := (*decls)[i]
		if d.Type != decl.Package {
			t.Errorf("[%d] type = %v, want Package", i, d.Type)
		}
		if d.PackageName != name {
			t.Errorf("[%d] name = %q, want %q", i, d.PackageName, name)
		}
	}
}

func TestBrew_InvalidArg(t *testing.T) {
	t.Parallel()
	vm, _ := setupModule(t)

	_, err := vm.RunString(`c.brew(42)`)
	if err == nil {
		t.Fatal("expected error for brew(number)")
	}
}

func TestDefaults_ThreeArg(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.defaults("com.apple.dock", "autohide", true)`)
	if err != nil {
		t.Fatal(err)
	}

	if len(*decls) != 1 {
		t.Fatalf("expected 1 declaration, got %d", len(*decls))
	}
	d := (*decls)[0]
	if d.Type != decl.Defaults {
		t.Errorf("type = %v, want Defaults", d.Type)
	}
	if d.DefaultsDomain != "com.apple.dock" {
		t.Errorf("domain = %q", d.DefaultsDomain)
	}
	if d.DefaultsKey != "autohide" {
		t.Errorf("key = %q", d.DefaultsKey)
	}
	if d.DefaultsValue != true {
		t.Errorf("value = %v, want true", d.DefaultsValue)
	}
}

func TestDefaults_ObjectForm(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.defaults("com.apple.dock", { autohide: true, tilesize: 36 })`)
	if err != nil {
		t.Fatal(err)
	}

	if len(*decls) != 2 {
		t.Fatalf("expected 2 declarations, got %d", len(*decls))
	}

	// Find by key since object iteration order may vary
	found := map[string]decl.Declaration{}
	for _, d := range *decls {
		found[d.DefaultsKey] = d
	}

	ah, ok := found["autohide"]
	if !ok {
		t.Fatal("missing autohide declaration")
	}
	if ah.DefaultsDomain != "com.apple.dock" {
		t.Errorf("domain = %q", ah.DefaultsDomain)
	}
	if ah.DefaultsValue != true {
		t.Errorf("autohide value = %v, want true", ah.DefaultsValue)
	}

	ts, ok := found["tilesize"]
	if !ok {
		t.Fatal("missing tilesize declaration")
	}
	if ts.DefaultsValue != int64(36) {
		t.Errorf("tilesize value = %v (%T), want int64(36)", ts.DefaultsValue, ts.DefaultsValue)
	}
}

func TestDefaults_IntConversion(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	// JS whole numbers should become int64
	_, err := vm.RunString(`c.defaults("com.apple.dock", "tilesize", 42)`)
	if err != nil {
		t.Fatal(err)
	}

	d := (*decls)[0]
	if _, ok := d.DefaultsValue.(int64); !ok {
		t.Errorf("value type = %T, want int64", d.DefaultsValue)
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

func TestFont_SingleFile(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.font("fonts/Mono.ttf")`)
	if err != nil {
		t.Fatal(err)
	}

	if len(*decls) != 1 {
		t.Fatalf("expected 1 declaration, got %d", len(*decls))
	}
	d := (*decls)[0]
	if d.Type != decl.Font {
		t.Errorf("type = %v, want Font", d.Type)
	}
	if d.FontSource != "fonts/Mono.ttf" {
		t.Errorf("source = %q", d.FontSource)
	}
	if d.FontName != "Mono.ttf" {
		t.Errorf("name = %q", d.FontName)
	}
	if d.FontDestDir != "/home/user/Library/Fonts" {
		t.Errorf("dest = %q", d.FontDestDir)
	}
}

func TestFont_Array(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.font(["fonts/Mono.ttf", "fonts/Sans.otf"])`)
	if err != nil {
		t.Fatal(err)
	}

	if len(*decls) != 2 {
		t.Fatalf("expected 2 declarations, got %d", len(*decls))
	}
	if (*decls)[0].FontName != "Mono.ttf" {
		t.Errorf("[0] name = %q", (*decls)[0].FontName)
	}
	if (*decls)[1].FontName != "Sans.otf" {
		t.Errorf("[1] name = %q", (*decls)[1].FontName)
	}
}

func TestFont_CustomDest(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.font("fonts/Mono.ttf", { dest: "~/.local/share/fonts" })`)
	if err != nil {
		t.Fatal(err)
	}

	d := (*decls)[0]
	if d.FontDestDir != "/home/user/.local/share/fonts" {
		t.Errorf("dest = %q", d.FontDestDir)
	}
}

func TestMise_Basic(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.mise("python", "3.12")`)
	if err != nil {
		t.Fatal(err)
	}

	if len(*decls) != 1 {
		t.Fatalf("expected 1 declaration, got %d", len(*decls))
	}
	d := (*decls)[0]
	if d.Type != decl.MiseTool {
		t.Errorf("type = %v, want MiseTool", d.Type)
	}
	if d.MiseToolName != "python" {
		t.Errorf("name = %q", d.MiseToolName)
	}
	if d.MiseToolVersion != "3.12" {
		t.Errorf("version = %q", d.MiseToolVersion)
	}
}

func TestMise_NoArgs(t *testing.T) {
	t.Parallel()
	vm, _ := setupModule(t)

	_, err := vm.RunString(`c.mise("python")`)
	if err == nil {
		t.Fatal("expected error for mise() with only one arg")
	}
}

func TestShell_Basic(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.shell("/opt/homebrew/bin/zsh")`)
	if err != nil {
		t.Fatal(err)
	}

	if len(*decls) != 1 {
		t.Fatalf("expected 1 declaration, got %d", len(*decls))
	}
	d := (*decls)[0]
	if d.Type != decl.Shell {
		t.Errorf("type = %v, want Shell", d.Type)
	}
	if d.ShellPath != "/opt/homebrew/bin/zsh" {
		t.Errorf("path = %q", d.ShellPath)
	}
	if d.ShellUsername != "" {
		t.Errorf("username = %q, want empty", d.ShellUsername)
	}
}

func TestShell_WithUser(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.shell("/opt/homebrew/bin/zsh", { user: "ryan" })`)
	if err != nil {
		t.Fatal(err)
	}

	d := (*decls)[0]
	if d.ShellUsername != "ryan" {
		t.Errorf("username = %q, want ryan", d.ShellUsername)
	}
}

func TestFile_Absent(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.file("~/.old-config", { state: "absent" })`)
	if err != nil {
		t.Fatal(err)
	}

	if len(*decls) != 1 {
		t.Fatalf("expected 1 declaration, got %d", len(*decls))
	}
	d := (*decls)[0]
	if d.Type != decl.File {
		t.Errorf("type = %v, want File", d.Type)
	}
	if d.State != decl.Absent {
		t.Errorf("state = %v, want Absent", d.State)
	}
	if d.Path != "/home/user/.old-config" {
		t.Errorf("path = %q", d.Path)
	}
	if len(d.Content) != 0 {
		t.Errorf("content should be empty for absent file, got %q", d.Content)
	}
}

func TestDir_Absent(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.dir("~/.cache/old", { state: "absent" })`)
	if err != nil {
		t.Fatal(err)
	}

	d := (*decls)[0]
	if d.Type != decl.Dir {
		t.Errorf("type = %v, want Dir", d.Type)
	}
	if d.State != decl.Absent {
		t.Errorf("state = %v, want Absent", d.State)
	}
}

func TestSymlink_Absent(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.symlink("~/.vimrc", { state: "absent" })`)
	if err != nil {
		t.Fatal(err)
	}

	d := (*decls)[0]
	if d.Type != decl.Symlink {
		t.Errorf("type = %v, want Symlink", d.Type)
	}
	if d.State != decl.Absent {
		t.Errorf("state = %v, want Absent", d.State)
	}
	if d.LinkTarget != "" {
		t.Errorf("target should be empty for absent symlink, got %q", d.LinkTarget)
	}
}

func TestBrew_Absent(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.brew("wget", { state: "absent" })`)
	if err != nil {
		t.Fatal(err)
	}

	d := (*decls)[0]
	if d.Type != decl.Package {
		t.Errorf("type = %v, want Package", d.Type)
	}
	if d.State != decl.Absent {
		t.Errorf("state = %v, want Absent", d.State)
	}
	if d.PackageName != "wget" {
		t.Errorf("name = %q", d.PackageName)
	}
}

func TestBrew_AbsentArray(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.brew(["wget", "curl"], { state: "absent" })`)
	if err != nil {
		t.Fatal(err)
	}

	if len(*decls) != 2 {
		t.Fatalf("expected 2 declarations, got %d", len(*decls))
	}
	for i, d := range *decls {
		if d.State != decl.Absent {
			t.Errorf("[%d] state = %v, want Absent", i, d.State)
		}
	}
}

func TestDefaults_Absent(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.defaults("com.apple.dock", "expose-animation-duration", { state: "absent" })`)
	if err != nil {
		t.Fatal(err)
	}

	d := (*decls)[0]
	if d.Type != decl.Defaults {
		t.Errorf("type = %v, want Defaults", d.Type)
	}
	if d.State != decl.Absent {
		t.Errorf("state = %v, want Absent", d.State)
	}
	if d.DefaultsDomain != "com.apple.dock" {
		t.Errorf("domain = %q", d.DefaultsDomain)
	}
	if d.DefaultsKey != "expose-animation-duration" {
		t.Errorf("key = %q", d.DefaultsKey)
	}
}

func TestFont_Absent(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.font("fonts/Old.ttf", { state: "absent" })`)
	if err != nil {
		t.Fatal(err)
	}

	d := (*decls)[0]
	if d.Type != decl.Font {
		t.Errorf("type = %v, want Font", d.Type)
	}
	if d.State != decl.Absent {
		t.Errorf("state = %v, want Absent", d.State)
	}
	if d.FontName != "Old.ttf" {
		t.Errorf("name = %q", d.FontName)
	}
}

func TestMise_Absent(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.mise("python", { state: "absent" })`)
	if err != nil {
		t.Fatal(err)
	}

	d := (*decls)[0]
	if d.Type != decl.MiseTool {
		t.Errorf("type = %v, want MiseTool", d.Type)
	}
	if d.State != decl.Absent {
		t.Errorf("state = %v, want Absent", d.State)
	}
	if d.MiseToolName != "python" {
		t.Errorf("name = %q", d.MiseToolName)
	}
}

func TestMise_InvalidSecondArg(t *testing.T) {
	t.Parallel()
	vm, _ := setupModule(t)

	_, err := vm.RunString(`c.mise("python", { foo: "bar" })`)
	if err == nil {
		t.Fatal("expected error for mise() with invalid options object")
	}
}

func TestMas_SingleApp(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.mas(497799835, "Xcode")`)
	if err != nil {
		t.Fatal(err)
	}

	if len(*decls) != 1 {
		t.Fatalf("expected 1 declaration, got %d", len(*decls))
	}
	d := (*decls)[0]
	if d.Type != decl.MasApp {
		t.Errorf("type = %v, want MasApp", d.Type)
	}
	if d.MasAppID != 497799835 {
		t.Errorf("id = %d, want 497799835", d.MasAppID)
	}
	if d.MasAppName != "Xcode" {
		t.Errorf("name = %q, want Xcode", d.MasAppName)
	}
}

func TestMas_SingleAppNoName(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.mas(497799835)`)
	if err != nil {
		t.Fatal(err)
	}

	if len(*decls) != 1 {
		t.Fatalf("expected 1 declaration, got %d", len(*decls))
	}
	d := (*decls)[0]
	if d.MasAppID != 497799835 {
		t.Errorf("id = %d, want 497799835", d.MasAppID)
	}
	if d.MasAppName != "" {
		t.Errorf("name = %q, want empty", d.MasAppName)
	}
}

func TestMas_ArrayForm(t *testing.T) {
	t.Parallel()
	vm, decls := setupModule(t)

	_, err := vm.RunString(`c.mas([{id: 497799835, name: "Xcode"}, {id: 409183694, name: "Keynote"}])`)
	if err != nil {
		t.Fatal(err)
	}

	if len(*decls) != 2 {
		t.Fatalf("expected 2 declarations, got %d", len(*decls))
	}
	if (*decls)[0].MasAppID != 497799835 {
		t.Errorf("[0] id = %d", (*decls)[0].MasAppID)
	}
	if (*decls)[0].MasAppName != "Xcode" {
		t.Errorf("[0] name = %q", (*decls)[0].MasAppName)
	}
	if (*decls)[1].MasAppID != 409183694 {
		t.Errorf("[1] id = %d", (*decls)[1].MasAppID)
	}
}

func TestMas_InvalidArg(t *testing.T) {
	t.Parallel()
	vm, _ := setupModule(t)

	_, err := vm.RunString(`c.mas("not-a-number")`)
	if err == nil {
		t.Fatal("expected error for mas(string)")
	}
}

func TestMas_NoArgs(t *testing.T) {
	t.Parallel()
	vm, _ := setupModule(t)

	_, err := vm.RunString(`c.mas()`)
	if err == nil {
		t.Fatal("expected error for mas() with no args")
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
