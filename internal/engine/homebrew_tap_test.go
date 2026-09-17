package engine

import (
	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"testing"
)

func TestHomebrewPlanningOrdersTapsAndPackages(t *testing.T) {
	eng := newPlanningTestEngine(t.TempDir(), t.TempDir())
	fact.Set(eng.factStore, "homebrew", &fact.HomebrewInfo{Available: true, Casks: map[string]bool{"jayjay": true}, InstalledPackages: []fact.HomebrewPackage{{Name: "jayjay", Kind: "cask", Tap: "old/tools", Names: []string{"jayjay"}}}})
	fact.Set(eng.factStore, "homebrew-taps", &fact.HomebrewTapInfo{Available: true, Taps: map[string]bool{"old/tools": true}})
	result, err := eng.planScript(t.Context(), eng.factStore, []byte(`var c=require("crucible"); c.brew.tap("old/tools",{state:"absent"}); c.brew.formula("new/tools/tool"); c.brew.cask("jayjay",{state:"absent"}); c.brew.tap("new/tools");`))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Actions) != 4 {
		t.Fatal(result.Actions)
	}
	if result.Actions[0].Type != action.AddHomebrewTap || result.Actions[3].Type != action.RemoveHomebrewTap {
		t.Fatal(result.Actions)
	}
	if result.Actions[1].PackageKind != "formula" || result.Actions[2].PackageKind != "cask" {
		t.Fatal(result.Actions)
	}
	for _, a := range result.Actions {
		if a.SerialGroup != "homebrew" {
			t.Fatal(a)
		}
	}
}
