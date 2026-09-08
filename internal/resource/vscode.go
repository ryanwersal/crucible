package resource

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

type VSCodeExtensionHandler struct{}

func (VSCodeExtensionHandler) DeclType() decl.Type { return decl.VSCodeExtension }
func (VSCodeExtensionHandler) DeclName() string    { return "VSCodeExtension" }

func (VSCodeExtensionHandler) PlanBatch(ctx context.Context, store *fact.Store, _ Env, decls []decl.Declaration) (PlanOutput, error) {
	info, err := fact.Get(ctx, store, "vscode", fact.VSCodeCollector{})
	if err != nil {
		return PlanOutput{}, err
	}
	desired := make([]action.DesiredVSCodeExtension, len(decls))
	for i, d := range decls {
		desired[i] = action.DesiredVSCodeExtension{ID: strings.ToLower(d.VSCodeExtension), Absent: d.State == decl.Absent}
	}
	actions, err := action.DiffVSCode(desired, info)
	if err != nil {
		return PlanOutput{}, err
	}
	out := PlanOutput{Actions: actions}
	seen := make(map[string]bool, len(desired))
	for _, a := range actions {
		seen[a.VSCodeExtension] = true
	}
	for _, d := range desired {
		if seen[d.ID] {
			continue
		}
		seen[d.ID] = true
		status := "installed"
		switch {
		case info.Command == "":
			status = "code not available — skipped; install VS Code and rerun crucible"
		case info.Bundled[d.ID]:
			status = "bundled"
		case d.Absent:
			status = "already absent"
		}
		out.Observations = append(out.Observations, action.Observation{Description: fmt.Sprintf("VS Code %s (%s)", d.ID, status)})
	}
	return out, nil
}

type InstallVSCodeExtensionExecutor struct{}

func (InstallVSCodeExtensionExecutor) ActionType() action.Type { return action.InstallVSCodeExtension }
func (InstallVSCodeExtensionExecutor) ActionName() string      { return "InstallVSCodeExtension" }
func (InstallVSCodeExtensionExecutor) Execute(ctx context.Context, a action.Action, stdin io.Reader, stdout, stderr io.Writer) error {
	return runCmd(ctx, a, stdin, stdout, stderr, a.VSCodeCommand, "--install-extension", a.VSCodeExtension)
}

type UninstallVSCodeExtensionExecutor struct{}

func (UninstallVSCodeExtensionExecutor) ActionType() action.Type {
	return action.UninstallVSCodeExtension
}
func (UninstallVSCodeExtensionExecutor) ActionName() string { return "UninstallVSCodeExtension" }
func (UninstallVSCodeExtensionExecutor) Execute(ctx context.Context, a action.Action, stdin io.Reader, stdout, stderr io.Writer) error {
	return runCmd(ctx, a, stdin, stdout, stderr, a.VSCodeCommand, "--uninstall-extension", a.VSCodeExtension)
}
