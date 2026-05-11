package action

// Observation records an item that was checked and found to already be
// in the desired state — no action needed.
type Observation struct {
	Group       string // resource group for display; set by engine from decl type (e.g. "File", "Package")
	Description string
}

// PlanResult holds the complete output of a plan phase: items that need
// changing (Actions) and items already current (Observations).
type PlanResult struct {
	Actions      []Action
	Observations []Observation
}

// Destructive returns the subset of planned actions that would irrevocably
// destroy user content. Apply uses this to gate execution behind an explicit
// confirmation that lists what will be lost.
func (r PlanResult) Destructive() []Action {
	var out []Action
	for _, a := range r.Actions {
		if a.Destructive {
			out = append(out, a)
		}
	}
	return out
}
