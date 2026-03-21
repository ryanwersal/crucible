package action

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

// DesiredKeyRemap describes the key remappings that should be active.
type DesiredKeyRemap struct {
	Remaps []KeyRemapEntry
	Absent bool
}

// DiffKeyRemap compares desired key remappings against the currently active ones.
func DiffKeyRemap(desired DesiredKeyRemap, actual *fact.KeyRemapInfo, plistPath string) []Action {
	if desired.Absent {
		// If there are no current mappings, nothing to remove.
		if actual == nil || len(actual.Mappings) == 0 {
			return nil
		}
		return []Action{{
			Type:        RemoveKeyRemap,
			Path:        plistPath,
			Description: "remove all keyboard modifier remappings",
		}}
	}

	// Convert desired remaps to sorted {src, dst} pairs for comparison.
	desiredMappings := make([]fact.KeyRemapMapping, len(desired.Remaps))
	var descParts []string
	for i, r := range desired.Remaps {
		src, _ := decl.KeyCode(r.From)
		dst, _ := decl.KeyCode(r.To)
		desiredMappings[i] = fact.KeyRemapMapping{Src: src, Dst: dst}
		descParts = append(descParts, fmt.Sprintf("%s → %s", r.From, r.To))
	}
	sortMappings(desiredMappings)

	// Compare with actual.
	actualMappings := []fact.KeyRemapMapping{}
	if actual != nil {
		actualMappings = actual.Mappings
	}
	sortMappings(actualMappings)

	if slices.Equal(desiredMappings, actualMappings) {
		return nil
	}

	return []Action{{
		Type:        SetKeyRemap,
		Path:        plistPath,
		KeyRemaps:   desired.Remaps,
		Description: fmt.Sprintf("remap keys: %s", strings.Join(descParts, ", ")),
	}}
}

func sortMappings(m []fact.KeyRemapMapping) {
	slices.SortFunc(m, func(a, b fact.KeyRemapMapping) int {
		if c := cmp.Compare(a.Src, b.Src); c != 0 {
			return c
		}
		return cmp.Compare(a.Dst, b.Dst)
	})
}
