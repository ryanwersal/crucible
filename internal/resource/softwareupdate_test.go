package resource

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
	"github.com/ryanwersal/crucible/internal/softwareupdate"
)

func TestSoftwareUpdatePlansFromFacts(t *testing.T) {
	t.Parallel()
	store := fact.NewStore()
	fact.Set(store, "softwareupdate", &fact.SoftwareUpdateInfo{Updates: []softwareupdate.Update{{Label: "Safari-1", Title: "Safari", Version: "1"}}})
	registry := DefaultRegistry()
	declarations := []decl.Declaration{{Type: decl.SoftwareUpdate, SoftwareUpdate: softwareupdate.Request{Name: "Safari", Match: "title", Latest: true}}}
	plan, err := registry.PlanBatch(t.Context(), store, Env{}, decl.SoftwareUpdate, declarations)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Actions) != 1 || plan.Actions[0].SoftwareUpdate.Label != "Safari-1" {
		t.Fatalf("plan %+v", plan)
	}
}

func TestSoftwareUpdateExecution(t *testing.T) {
	for _, tc := range []struct {
		name, catalog, receipt                                string
		installed, restart, runFails, wantError, stillOffered bool
	}{
		{name: "exact label", catalog: "1", receipt: "com.example.pkg", installed: true},
		{name: "changed catalog", catalog: "2", wantError: true},
		{name: "unexpected restart", catalog: "1", restart: true, wantError: true},
		{name: "missing receipt after install", catalog: "1", receipt: "com.example.pkg", wantError: true},
		{name: "installer failed", catalog: "1", runFails: true, wantError: true},
		{name: "success exit but still offered", catalog: "1", stillOffered: true, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			client := &softwareupdate.Client{Marker: filepath.Join(t.TempDir(), "marker"), CLTUsable: func() bool { return true }, Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
				if name == "/usr/sbin/pkgutil" {
					if tc.installed {
						return []byte(tc.receipt), nil
					}
					return nil, nil
				}
				if called && !tc.stillOffered {
					return []byte("No new software available."), nil
				}
				text := "* Label: Safari-" + tc.catalog + "\n Title: Safari, Version: " + tc.catalog + ", Size: 1KiB, Recommended: YES,"
				if tc.restart {
					text += " Action: restart,"
				}
				return []byte(text), nil
			}}
			executor := InstallSoftwareUpdateExecutor{Client: client, Run: func(_ context.Context, a action.Action, _ io.Reader, _ io.Writer, _ io.Writer, name string, args ...string) error {
				called = true
				if name != "/usr/sbin/softwareupdate" || !reflect.DeepEqual(args, []string{"--install", "Safari-1"}) || !a.NeedsSudo {
					t.Fatalf("unexpected execution: %s %v %+v", name, args, a)
				}
				if tc.runFails {
					return errors.New("network failed")
				}
				return nil
			}}
			a := action.Action{Type: action.InstallSoftwareUpdate, NeedsSudo: true, SoftwareUpdate: softwareupdate.Update{Label: "Safari-1", Title: "Safari", Version: "1"}, SoftwareUpdateRequest: softwareupdate.Request{Name: "Safari", Match: "title", Latest: true, Receipt: tc.receipt}}
			err := executor.Execute(t.Context(), a, nil, io.Discard, io.Discard)
			if (err != nil) != tc.wantError {
				t.Fatalf("error %v", err)
			}
			if called && (tc.catalog != "1" || tc.restart) {
				t.Fatal("installed a changed plan")
			}
			if !called && !tc.wantError {
				t.Fatal("did not install")
			}
		})
	}
}

func TestSoftwareUpdateRestartIsExplicit(t *testing.T) {
	client := &softwareupdate.Client{CLTUsable: func() bool { return true }, Run: func(context.Context, string, ...string) ([]byte, error) {
		return []byte("* Label: OS-1\n Title: OS, Version: 1, Size: 1KiB, Action: restart,"), nil
	}}
	executor := InstallSoftwareUpdateExecutor{Client: client, Run: func(_ context.Context, _ action.Action, _ io.Reader, _ io.Writer, _ io.Writer, _ string, args ...string) error {
		if !reflect.DeepEqual(args, []string{"--install", "OS-1", "--restart"}) {
			t.Fatalf("args %v", args)
		}
		return nil
	}}
	a := action.Action{SoftwareUpdate: softwareupdate.Update{Label: "OS-1", Title: "OS", Version: "1", Restart: true}, SoftwareUpdateRequest: softwareupdate.Request{Name: "OS", Match: "title", Latest: true, Restart: true}}
	if err := executor.Execute(t.Context(), a, nil, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
}

func TestCLTDiscoverySurvivesThroughInstallAndCleansUpFailure(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "marker")
	failure := errors.New("installer failed")
	client := &softwareupdate.Client{Marker: marker, CLTUsable: func() bool { return false }, Run: func(context.Context, string, ...string) ([]byte, error) {
		return []byte("* Label: CLT-1\n Title: CLT, Version: 1, Size: 1KiB,"), nil
	}}
	executor := InstallSoftwareUpdateExecutor{Client: client, Run: func(context.Context, action.Action, io.Reader, io.Writer, io.Writer, string, ...string) error {
		if _, err := os.Stat(marker); err != nil {
			t.Fatal("discovery marker disappeared before installation")
		}
		return failure
	}}
	a := action.Action{SoftwareUpdate: softwareupdate.Update{Label: "CLT-1", Title: "CLT", Version: "1"}, SoftwareUpdateRequest: softwareupdate.Request{Name: "CLT", Match: "title", Receipt: softwareupdate.CLTReceipt, CommandLineTools: true}}
	if err := executor.Execute(t.Context(), a, nil, io.Discard, io.Discard); !errors.Is(err, failure) {
		t.Fatalf("lost install failure: %v", err)
	}
	if _, err := os.Lstat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("left marker behind: %v", err)
	}
}
