package modules

import (
	"github.com/ryanwersal/crucible/internal/script/decl"
	"testing"
)

func TestHomebrewNamespace(t *testing.T) {
	t.Parallel()
	vm, ds := setupModule(t)
	_, err := vm.RunString(`c.brew("jq"); c.brew.formula(["git", "fd"], {state:"latest"}); c.brew.cask("app", {state:"absent"}); c.brew.tap("Owner/homebrew-Tools"); c.brew.tap("old/tools", {state:"absent"});`)
	if err != nil {
		t.Fatal(err)
	}
	if len(*ds) != 6 {
		t.Fatal(*ds)
	}
	if (*ds)[0].PackageKind != "" || (*ds)[1].PackageKind != "formula" || (*ds)[2].PackageKind != "formula" || (*ds)[3].PackageKind != "cask" {
		t.Fatal(*ds)
	}
	if (*ds)[4].TapName != "owner/tools" || (*ds)[4].Type != decl.HomebrewTap || (*ds)[5].State != decl.Absent {
		t.Fatal(*ds)
	}
}

func TestHomebrewTapRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	for _, source := range []string{`c.brew.tap()`, `c.brew.tap(1)`, `c.brew.tap("--force")`, `c.brew.tap("owner/tools/app")`, `c.brew.tap("../tools")`, `c.brew.tap("owner/homebrew-")`, `c.brew.tap("owner/tools", {state:"latest"})`} {
		t.Run(source, func(t *testing.T) {
			t.Parallel()
			vm, _ := setupModule(t)
			if _, err := vm.RunString(source); err == nil {
				t.Fatal("accepted invalid tap")
			}
		})
	}
}
