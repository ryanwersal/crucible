package fact

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// KeyRemapMapping represents a single HID key remapping (source → destination).
type KeyRemapMapping struct {
	Src uint64
	Dst uint64
}

// KeyRemapInfo holds the currently active keyboard modifier remappings.
type KeyRemapInfo struct {
	Mappings []KeyRemapMapping
}

// KeyRemapCollector reads the current key remappings via hidutil.
type KeyRemapCollector struct{}

var (
	srcRe = regexp.MustCompile(`HIDKeyboardModifierMappingSrc\s*=\s*(\d+)`)
	dstRe = regexp.MustCompile(`HIDKeyboardModifierMappingDst\s*=\s*(\d+)`)
)

// Collect reads the current UserKeyMapping via `hidutil property --get`.
func (KeyRemapCollector) Collect(ctx context.Context) (*KeyRemapInfo, error) {
	cmd := exec.CommandContext(ctx, "hidutil", "property", "--get", "UserKeyMapping")
	out, err := cmd.Output()
	if err != nil {
		return &KeyRemapInfo{}, nil
	}

	text := strings.TrimSpace(string(out))
	if text == "(null)" || text == "(\n)" || text == "()" {
		return &KeyRemapInfo{}, nil
	}

	return parseHIDUtilOutput(text)
}

// parseHIDUtilOutput parses the plist-style text output from hidutil.
// Example:
//
//	(
//	    {
//	        HIDKeyboardModifierMappingDst = 30064771296;
//	        HIDKeyboardModifierMappingSrc = 30064771129;
//	    }
//	)
func parseHIDUtilOutput(text string) (*KeyRemapInfo, error) {
	srcMatches := srcRe.FindAllStringSubmatch(text, -1)
	dstMatches := dstRe.FindAllStringSubmatch(text, -1)

	if len(srcMatches) != len(dstMatches) {
		return nil, fmt.Errorf("hidutil output has mismatched src/dst count: %d src, %d dst", len(srcMatches), len(dstMatches))
	}

	mappings := make([]KeyRemapMapping, len(srcMatches))
	for i := range srcMatches {
		src, err := strconv.ParseUint(srcMatches[i][1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing HIDKeyboardModifierMappingSrc: %w", err)
		}
		dst, err := strconv.ParseUint(dstMatches[i][1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing HIDKeyboardModifierMappingDst: %w", err)
		}
		mappings[i] = KeyRemapMapping{Src: uint64(src), Dst: uint64(dst)}
	}

	return &KeyRemapInfo{Mappings: mappings}, nil
}
