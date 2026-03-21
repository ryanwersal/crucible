package resource

import (
	"strings"
	"testing"

	"github.com/ryanwersal/crucible/internal/action"
)

func TestBuildUserKeyMappingJSON(t *testing.T) {
	t.Parallel()

	remaps := []action.KeyRemapEntry{
		{From: "capsLock", To: "control"},
	}

	got := buildUserKeyMappingJSON(remaps)

	// capsLock = 0x700000039 = 30064771129
	// control  = 0x7000000E0 = 30064771296
	if !strings.Contains(got, "30064771129") {
		t.Errorf("expected capsLock HID code 30064771129 in JSON, got: %s", got)
	}
	if !strings.Contains(got, "30064771296") {
		t.Errorf("expected control HID code 30064771296 in JSON, got: %s", got)
	}
	if !strings.Contains(got, `"UserKeyMapping"`) {
		t.Errorf("expected UserKeyMapping key in JSON, got: %s", got)
	}
}

func TestBuildUserKeyMappingJSON_Multiple(t *testing.T) {
	t.Parallel()

	remaps := []action.KeyRemapEntry{
		{From: "capsLock", To: "control"},
		{From: "control", To: "capsLock"},
	}

	got := buildUserKeyMappingJSON(remaps)

	// Should have two mapping objects separated by comma.
	if strings.Count(got, "HIDKeyboardModifierMappingSrc") != 2 {
		t.Errorf("expected 2 Src entries, got: %s", got)
	}
}

func TestBuildKeyRemapPlist(t *testing.T) {
	t.Parallel()

	remaps := []action.KeyRemapEntry{
		{From: "capsLock", To: "control"},
	}

	got := buildKeyRemapPlist(remaps)

	if !strings.Contains(got, "com.crucible.keyremap") {
		t.Error("plist missing Label")
	}
	if !strings.Contains(got, "/usr/bin/hidutil") {
		t.Error("plist missing hidutil path")
	}
	if !strings.Contains(got, "<true/>") {
		t.Error("plist missing RunAtLoad")
	}
	if !strings.Contains(got, "<?xml") {
		t.Error("plist missing XML declaration")
	}
}

func TestXmlEscape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{`hello`, `hello`},
		{`a<b>c&d"e'f`, `a&lt;b&gt;c&amp;d&quot;e&apos;f`},
	}

	for _, tt := range tests {
		got := xmlEscape(tt.input)
		if got != tt.want {
			t.Errorf("xmlEscape(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
