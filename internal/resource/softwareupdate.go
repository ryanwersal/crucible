package resource

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
	"github.com/ryanwersal/crucible/internal/softwareupdate"
)

type SoftwareUpdateHandler struct{ Client *softwareupdate.Client }

func (SoftwareUpdateHandler) DeclType() decl.Type { return decl.SoftwareUpdate }
func (SoftwareUpdateHandler) DeclName() string    { return "SoftwareUpdate" }
func (h SoftwareUpdateHandler) PlanBatch(ctx context.Context, store *fact.Store, _ Env, declarations []decl.Declaration) (PlanOutput, error) {
	requests := make([]softwareupdate.Request, 0, len(declarations))
	for _, d := range declarations {
		requests = append(requests, d.SoftwareUpdate)
	}
	info, err := fact.Get(ctx, store, "softwareupdate", fact.SoftwareUpdateCollector{Client: h.Client, Requests: requests})
	if err != nil {
		return PlanOutput{}, err
	}
	actions, observations, err := action.DiffSoftwareUpdates(requests, info)
	return PlanOutput{Actions: actions, Observations: observations}, err
}

type InstallSoftwareUpdateExecutor struct {
	Client *softwareupdate.Client
	Run    func(context.Context, action.Action, io.Reader, io.Writer, io.Writer, string, ...string) error
}

func (InstallSoftwareUpdateExecutor) ActionType() action.Type { return action.InstallSoftwareUpdate }
func (InstallSoftwareUpdateExecutor) ActionName() string      { return "InstallSoftwareUpdate" }
func (e InstallSoftwareUpdateExecutor) Execute(ctx context.Context, a action.Action, stdin io.Reader, stdout, stderr io.Writer) (err error) {
	client := e.Client
	if client == nil {
		client = softwareupdate.NewClient()
	}
	request := a.SoftwareUpdateRequest
	if err := request.Validate(); err != nil {
		return err
	}
	discover := false
	if request.CommandLineTools {
		receipts, readErr := client.Receipts(ctx)
		if readErr != nil {
			return readErr
		}
		discover = !receipts[softwareupdate.CLTReceipt] || !client.CLTUsable()
	}
	cleanup, err := client.PrepareDiscovery(discover)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, cleanup()) }()
	catalog, err := client.Catalog(ctx, discover)
	if err != nil {
		return err
	}
	// Revalidate the exact planned label and metadata. Never substitute a newer
	// release (or an unexpected restart) after the user has approved the plan.
	found := false
	for _, update := range catalog {
		if update == a.SoftwareUpdate {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("apple update %s changed or disappeared since planning; run a fresh plan", a.SoftwareUpdate.Label)
	}
	if a.SoftwareUpdate.Restart && !request.Restart {
		return fmt.Errorf("restart was not authorized")
	}
	args := []string{"--install", a.SoftwareUpdate.Label}
	if a.SoftwareUpdate.Restart {
		args = append(args, "--restart")
	}
	run := e.Run
	if run == nil {
		run = runCmd
	}
	if err := run(ctx, a, stdin, stdout, stderr, "/usr/sbin/softwareupdate", args...); err != nil {
		return fmt.Errorf("install Apple update %s: %w", a.SoftwareUpdate.Label, err)
	}
	// Restart may terminate this process before verification can run.
	if a.SoftwareUpdate.Restart {
		return nil
	}
	if request.Receipt != "" {
		receipts, err := client.Receipts(ctx)
		if err != nil {
			return err
		}
		if !receipts[request.Receipt] {
			return fmt.Errorf("installer finished but receipt %s is missing", request.Receipt)
		}
	}
	if request.CommandLineTools && !client.CLTUsable() {
		return fmt.Errorf("installer finished but Command Line Tools compiler or SDK is missing")
	}
	// An install-on-demand marker can advertise CLT installers even when the
	// same version is already installed. Remove our marker before verification.
	if err := cleanup(); err != nil {
		return err
	}
	remaining, err := client.Catalog(ctx, false)
	if err != nil {
		return fmt.Errorf("verify installed update: %w", err)
	}
	for _, update := range remaining {
		if update.Label == a.SoftwareUpdate.Label {
			return fmt.Errorf("installer finished but update %s is still offered", update.Label)
		}
	}
	return nil
}
