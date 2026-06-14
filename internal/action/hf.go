package action

import (
	"fmt"

	"github.com/ryanwersal/crucible/internal/fact"
)

// DesiredHFDownload describes a HuggingFace repo that should be downloaded into
// a local directory (or removed from it).
type DesiredHFDownload struct {
	Repo     string
	Dest     string
	Include  []string
	Exclude  []string
	Revision string
	Absent   bool
}

// DiffHF compares a desired HuggingFace download against its destination's
// current state and returns the action needed, if any. The caller is expected
// to have confirmed hf is available.
//
// Download actions share the "hf" serial group so large fetches never run
// concurrently (they would otherwise contend for bandwidth and disk).
func DiffHF(d DesiredHFDownload, actual *fact.HFInfo) []Action {
	if d.Absent {
		if actual.Present {
			return []Action{{
				Type:              DeletePath,
				Path:              d.Dest,
				Recursive:         true,
				Description:       fmt.Sprintf("remove hf download %s", d.Dest),
				Destructive:       true,
				DestructiveReason: fmt.Sprintf("downloaded files at %s would be deleted recursively", d.Dest),
			}}
		}
		return nil
	}

	if actual.Present {
		return nil
	}
	a := Action{
		Type:        DownloadHF,
		HFRepo:      d.Repo,
		HFDest:      d.Dest,
		HFInclude:   d.Include,
		HFExclude:   d.Exclude,
		HFRevision:  d.Revision,
		SerialGroup: "hf",
		Description: fmt.Sprintf("hf download %s → %s", d.Repo, d.Dest),
	}
	if !actual.Authenticated {
		a.Note = "no HuggingFace auth detected — run `hf auth login` or set HF_TOKEN for higher rate limits"
	}
	return []Action{a}
}
