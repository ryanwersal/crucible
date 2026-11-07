package modules

import (
	"github.com/ryanwersal/crucible/internal/script/decl"
	"github.com/ryanwersal/crucible/internal/softwareupdate"
	"testing"
)

func TestSoftwareUpdateDeclarations(t *testing.T) {
	t.Parallel()
	vm, declarations := setupModule(t)
	_, err := vm.RunString(`c.softwareUpdate("Safari", {match:"prefix"}); c.softwareUpdate.commandLineTools(); c.softwareUpdate("Example-1", {match:"label",state:"present",receipt:"com.example.pkg",restart:true});`)
	if err != nil {
		t.Fatal(err)
	}
	if len(*declarations) != 3 {
		t.Fatal("missing declarations")
	}
	first, clt := (*declarations)[0], (*declarations)[1]
	if first.Type != decl.SoftwareUpdate || first.State != decl.Latest || first.SoftwareUpdate.Match != "prefix" {
		t.Fatalf("generic: %+v", first)
	}
	if !clt.SoftwareUpdate.CommandLineTools || clt.State != decl.Present || clt.SoftwareUpdate.Receipt != softwareupdate.CLTReceipt {
		t.Fatalf("CLT: %+v", clt)
	}
}

func TestSoftwareUpdateInvalidOptions(t *testing.T) {
	t.Parallel()
	for _, source := range []string{
		`c.softwareUpdate()`, `c.softwareUpdate(2)`, `c.softwareUpdate("")`, `c.softwareUpdate("--all")`,
		`c.softwareUpdate("Safari", null)`, `c.softwareUpdate("Safari", {state:"absent"})`,
		`c.softwareUpdate("Safari", {state:"present"})`, `c.softwareUpdate("Safari", {restart:"true"})`,
		`c.softwareUpdate("Safari", {match:"glob"})`, `c.softwareUpdate("Safari", {receipt:"bad receipt"})`,
		`c.softwareUpdate("Safari", {typo:true})`, `c.softwareUpdate.commandLineTools({receipt:"other"})`,
		`c.softwareUpdate.commandLineTools({match:"title"})`, `c.softwareUpdate.commandLineTools({}, {})`,
	} {
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
