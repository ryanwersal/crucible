package action

import (
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/softwareupdate"
	"testing"
)

func TestSoftwareUpdateDiff(t *testing.T) {
	t.Parallel()
	clt := softwareupdate.Request{Name: "CLT", Match: "title", Receipt: softwareupdate.CLTReceipt, CommandLineTools: true}
	for _, tc := range []struct {
		name    string
		request softwareupdate.Request
		actual  fact.SoftwareUpdateInfo
		actions int
		fail    bool
	}{
		{"installed present", clt, fact.SoftwareUpdateInfo{Receipts: map[string]bool{softwareupdate.CLTReceipt: true}, CLTUsable: true}, 0, false},
		{"broken installation", clt, fact.SoftwareUpdateInfo{Receipts: map[string]bool{softwareupdate.CLTReceipt: true}}, 0, true},
		{"missing unavailable", clt, fact.SoftwareUpdateInfo{}, 0, true},
		{"missing offered", clt, fact.SoftwareUpdateInfo{Updates: []softwareupdate.Update{{Label: "CLT-1", Title: "CLT", Version: "1"}}}, 1, false},
		{"generic no offer", softwareupdate.Request{Name: "Safari", Match: "title", Latest: true}, fact.SoftwareUpdateInfo{}, 0, false},
		{"restart refused", softwareupdate.Request{Name: "OS", Match: "title", Latest: true}, fact.SoftwareUpdateInfo{Updates: []softwareupdate.Update{{Label: "OS-1", Title: "OS", Version: "1", Restart: true}}}, 0, true},
		{"restart explicit", softwareupdate.Request{Name: "OS", Match: "title", Latest: true, Restart: true}, fact.SoftwareUpdateInfo{Updates: []softwareupdate.Update{{Label: "OS-1", Title: "OS", Version: "1", Restart: true}}}, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actions, observations, err := DiffSoftwareUpdates([]softwareupdate.Request{tc.request}, &tc.actual)
			if (err != nil) != tc.fail {
				t.Fatalf("error %v", err)
			}
			if tc.fail {
				return
			}
			if len(actions) != tc.actions {
				t.Fatalf("actions %+v", actions)
			}
			if tc.actions == 0 && len(observations) != 1 {
				t.Fatal("missing observation")
			}
			for _, a := range actions {
				if !a.NeedsSudo || !a.PTY || a.SerialGroup != "apple-softwareupdate" {
					t.Fatalf("execution flags %+v", a)
				}
			}
		})
	}
}

func TestOverlappingSoftwareUpdatesFail(t *testing.T) {
	t.Parallel()
	requests := []softwareupdate.Request{{Name: "Safari", Match: "title", Latest: true}, {Name: "Saf", Match: "prefix", Latest: true}}
	_, _, err := DiffSoftwareUpdates(requests, &fact.SoftwareUpdateInfo{Updates: []softwareupdate.Update{{Label: "Safari-1", Title: "Safari", Version: "1"}}})
	if err == nil {
		t.Fatal("overlapping declarations accepted")
	}
}
