package action

import (
	"fmt"

	"github.com/ryanwersal/crucible/internal/fact"
)

// DesiredDefault describes a macOS defaults key that should be set (or deleted).
type DesiredDefault struct {
	Domain string
	Key    string
	Value  any  // bool, int64, float64, or string
	Absent bool // true = ensure the key does not exist
}

// DiffDefaults compares the desired default value against the current state.
func DiffDefaults(desired DesiredDefault, actual *fact.DefaultsInfo) []Action {
	if desired.Absent {
		if actual != nil && actual.Exists {
			return []Action{{
				Type:           DeleteDefaults,
				DefaultsDomain: desired.Domain,
				DefaultsKey:    desired.Key,
				Description:    fmt.Sprintf("defaults delete %s %s", desired.Domain, desired.Key),
			}}
		}
		return nil
	}

	valueType := defaultsValueType(desired.Value)

	if actual != nil && actual.Exists && valuesEqual(desired.Value, actual.Value) {
		return nil
	}

	return []Action{{
		Type:              SetDefaults,
		DefaultsDomain:    desired.Domain,
		DefaultsKey:       desired.Key,
		DefaultsValue:     desired.Value,
		DefaultsValueType: valueType,
		Description:       fmt.Sprintf("defaults write %s %s", desired.Domain, desired.Key),
	}}
}

// defaultsValueType returns the defaults CLI type flag for a Go value.
func defaultsValueType(v any) string {
	switch v.(type) {
	case bool:
		return "bool"
	case int64:
		return "int"
	case float64:
		return "float"
	case string:
		return "string"
	default:
		return "string"
	}
}

// valuesEqual compares a desired value with the actual value read from defaults,
// handling type coercion between int64 and float64 where appropriate.
func valuesEqual(desired, actual any) bool {
	if desired == actual {
		return true
	}

	// Handle int64/float64 cross-comparison (defaults may store ints as floats)
	switch d := desired.(type) {
	case int64:
		if af, ok := actual.(float64); ok {
			return float64(d) == af
		}
	case float64:
		if ai, ok := actual.(int64); ok {
			return d == float64(ai)
		}
	}

	return false
}
