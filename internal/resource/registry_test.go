package resource

import "testing"

func TestRegistryValidate(t *testing.T) {
	r := DefaultRegistry()
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
}
