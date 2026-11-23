// Package provisioner implements the windows-harden Packer provisioner: it
// uploads the guest scripts, detects the OS, plans and applies the baseline
// under WinRM protection, and writes the report.
package provisioner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/uuid"
	"github.com/larskroghdk/packer-plugin-windows-harden/internal/baseline"
	"github.com/larskroghdk/packer-plugin-windows-harden/internal/facts"
	"github.com/larskroghdk/packer-plugin-windows-harden/internal/plan"
	"github.com/larskroghdk/packer-plugin-windows-harden/internal/report"
)

var Version = "dev"

// Provisioner is the windows-harden provisioner registered with Packer.
type Provisioner struct {
	config  Config
	catalog *baseline.Catalog
	now     func() time.Time
}

// ConfigSpec returns the HCL2 specification of Config.
//
// Author: Lars Krogh
// Version: 1.0.0
func (p *Provisioner) ConfigSpec() hcldec.ObjectSpec { return p.config.FlatMapstructure().HCL2Spec() }

// Prepare loads the embedded baselines and validates the configuration.
//
// Returns:
//   - error: The baselines cannot be loaded or the configuration is invalid.
//
// Author: Lars Krogh
// Version: 1.0.0
func (p *Provisioner) Prepare(raws ...any) error {
	catalog, err := baseline.Load()
	if err != nil {
		return fmt.Errorf("load embedded baselines: %w", err)
	}
	p.catalog = catalog
	if p.now == nil {
		p.now = time.Now
	}
	return p.config.prepare(catalog, raws...)
}

// Provision hardens the guest over comm. The report is written whether or not
// the run succeeds. The guest working directory is removed unless an armed
// rollback guard still needs its scripts and snapshot.
//
// Parameters:
//   - generatedData: Builder data. ConnType must be winrm, and an empty
//     Password tells the protection rules that Packer has no password.
//
// Returns:
//   - error: The run failed, or the report could not be written.
//
// Author: Lars Krogh
// Version: 1.1.0
func (p *Provisioner) Provision(ctx context.Context, ui packer.Ui, comm packer.Communicator, generatedData map[string]any) error {
	if connType, _ := generatedData["ConnType"].(string); connType != "" && connType != "winrm" {
		return fmt.Errorf("windows-harden requires the WinRM communicator, the build uses %q", connType)
	}
	password, _ := generatedData["Password"].(string)
	r := &run{
		cfg:           &p.config,
		catalog:       p.catalog,
		ui:            ui,
		remote:        newRemote(comm, ui, uuid.TimeOrderedUUID()),
		emptyPassword: password == "",
		now:           p.now,
		started:       p.now(),
	}
	runErr := r.execute(ctx)
	if err := r.remote.cleanup(ctx); err != nil {
		r.warn("could not remove the guest working directory %s: %v", r.remote.dir, err)
	}
	result := r.report(runErr)
	for _, line := range result.SummaryLines() {
		ui.Say(line)
	}
	if err := writeReport(p.config.ReportPath, result); err != nil {
		ui.Error(fmt.Sprintf("Could not write the hardening report: %v", err))
		if runErr == nil {
			runErr = err
		}
	} else {
		ui.Say("Report:     " + p.config.ReportPath)
	}
	return runErr
}

// writeReport writes the report as JSON to path, creating missing
// directories.
//
// Returns:
//   - error: The report cannot be encoded or written.
//
// Author: Lars Krogh
// Version: 1.0.0
func writeReport(path string, result *report.Report) error {
	data, err := result.JSON()
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0o644)
}

type run struct {
	cfg            *Config
	catalog        *baseline.Catalog
	ui             packer.Ui
	remote         *remote
	emptyPassword  bool
	now            func() time.Time
	started        time.Time
	facts          *facts.Facts
	classification facts.Classification
	baseline       *baseline.Baseline
	profile        baseline.Profile
	plan           *plan.Plan
	checks         []report.Check
	reboot         report.Reboot
	warnings       []string
}

// warn records a warning for the report and shows it in the Packer UI.
//
// Author: Lars Krogh
// Version: 1.0.0
func (r *run) warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	r.warnings = append(r.warnings, msg)
	r.ui.Error("Warning: " + msg)
}

// report builds the report from the run's current state and terminal error.
//
// Author: Lars Krogh
// Version: 1.1.0
func (r *run) report(runErr error) *report.Report {
	return report.Build(report.Input{
		Plugin:         "packer-plugin-windows-harden " + Version,
		StartedAt:      r.started,
		FinishedAt:     r.now(),
		DryRun:         r.cfg.DryRun,
		Mode:           r.cfg.Mode,
		Facts:          r.facts,
		Classification: r.classification,
		Baseline:       r.baseline,
		Profile:        r.profile,
		Plan:           r.plan,
		Connectivity:   r.checks,
		Reboot:         r.reboot,
		Warnings:       r.warnings,
		Error:          runErr,
	})
}

// execute runs the provisioning steps in order: upload, detect, select,
// inspect, plan, apply, guarded apply, optional restart. In strict mode it
// stops before any change if a setting would be deferred, and in dry-run mode
// it stops after planning.
//
// Returns:
//   - error: A step failed, strict mode found deferred settings, or settings
//     failed while fail_on_error is true.
//
// Author: Lars Krogh
// Version: 1.1.0
func (r *run) execute(ctx context.Context) error {
	r.ui.Say("Uploading the windows-harden guest scripts...")
	if err := r.remote.install(ctx); err != nil {
		return err
	}
	if err := r.detect(ctx); err != nil {
		return err
	}
	if err := r.selectBaseline(); err != nil {
		return err
	}
	settings := r.baseline.SettingsFor(r.profile)
	r.ui.Say(fmt.Sprintf("Inspecting %d settings of %s %s (%s profile)...", len(settings), r.baseline.Name, r.baseline.Version, r.profile))
	current, err := r.inspect(ctx, settings)
	if err != nil {
		return err
	}
	r.plan = plan.Build(settings, r.cfg.Exclude, plan.Context{Facts: r.facts, Current: current, EmptyPassword: r.emptyPassword})
	apply, guarded := r.plan.Pending(plan.ActionApply), r.plan.Pending(plan.ActionGuard)
	r.ui.Say(fmt.Sprintf("Plan: %d compliant, %d to apply, %d guarded, %d deferred, %d skipped, %d not applicable, %d failed inspection",
		r.plan.Count(plan.StatusCompliant), len(apply), len(guarded), r.plan.Count(plan.StatusDeferred),
		r.plan.Count(plan.StatusSkipped), r.plan.Count(plan.StatusNotApplicable), r.plan.Count(plan.StatusFailed)))
	if deferred := r.plan.Count(plan.StatusDeferred); r.cfg.Mode == ModeStrict && deferred > 0 {
		return fmt.Errorf("strict mode: %d settings would have to be deferred to preserve WinRM connectivity; no changes were made (see the report)", deferred)
	}
	if r.cfg.DryRun {
		r.ui.Say("Dry run: no changes were made.")
		if failed := r.plan.Count(plan.StatusFailed); failed > 0 && r.cfg.failOnError() {
			return fmt.Errorf("%d settings failed inspection (see the report)", failed)
		}
		return nil
	}
	if len(apply) > 0 {
		r.ui.Say(fmt.Sprintf("Applying %d settings...", len(apply)))
		if err := r.applyAndVerify(ctx, apply); err != nil {
			return err
		}
		if err := r.checkConnectivity(ctx, "after applying settings"); err != nil {
			return err
		}
	}
	if len(guarded) > 0 {
		if err := r.applyGuarded(ctx, guarded); err != nil {
			return err
		}
	}
	if r.plan.Count(plan.StatusHardened) > 0 {
		r.reboot.Required = true
		r.reboot.Reason = "security policy changes take full effect only after a restart"
		if r.cfg.RebootIfRequired {
			if err := r.restartAndReverify(ctx); err != nil {
				return err
			}
		}
	}
	if failed := r.plan.Count(plan.StatusFailed); failed > 0 {
		if r.cfg.failOnError() {
			return fmt.Errorf("%d settings failed to apply or verify (see the report)", failed)
		}
		r.warn("%d settings failed to apply or verify; continuing because fail_on_error is false", failed)
	}
	return nil
}

// detect runs the Detect action and stores the facts.
//
// Returns:
//   - error: Detection failed, the session is not elevated, or PowerShell is
//     not a 64-bit process.
//
// Author: Lars Krogh
// Version: 1.0.0
func (r *run) detect(ctx context.Context) error {
	r.ui.Say("Detecting the operating system and the WinRM configuration...")
	data, err := r.remote.action(ctx, "Detect", nil)
	if err != nil {
		return err
	}
	f, err := facts.Parse(data)
	if err != nil {
		return err
	}
	r.facts = f
	for _, e := range f.Errors {
		r.warn("detection: %s", e)
	}
	system := f.OS
	r.ui.Say(fmt.Sprintf("Detected %s (%s, %s, build %s, %s, %s)", system.ProductName, system.EditionID, system.InstallationType, system.Version(), system.DisplayVersion, system.Architecture))
	if !f.Session.IsElevated {
		return fmt.Errorf("the WinRM session (%s) is not elevated; windows-harden requires an administrator session", f.Session.UserName)
	}
	if !system.Is64BitProcess {
		return errors.New("the guest PowerShell process is not 64-bit; 32-bit processes see redirected registry views")
	}
	return nil
}

// selectBaseline classifies the OS, picks its baseline and resolves the
// profile from the configuration or the domain membership.
//
// Returns:
//   - error: The OS is not supported, the configured baseline does not match
//     it, or the profile is auto and domain membership is unknown.
//
// Author: Lars Krogh
// Version: 1.0.0
func (r *run) selectBaseline() error {
	r.classification = facts.Classify(r.facts.OS, r.now())
	if r.classification.Status != facts.StatusSupported {
		return fmt.Errorf("unsupported operating system: %s", r.classification.Reason)
	}
	b, err := r.catalog.Select(r.facts.OS.ProductTypeName(), r.facts.OS.Build)
	if err != nil {
		return fmt.Errorf("unsupported operating system %s: %w", r.classification.Release.Name, err)
	}
	if r.cfg.Baseline != BaselineAuto && r.cfg.Baseline != b.ID {
		return fmt.Errorf("baseline %q was requested, but %s requires baseline %q", r.cfg.Baseline, r.classification.Release.Name, b.ID)
	}
	r.baseline = b
	switch {
	case r.cfg.Profile != ProfileAuto:
		r.profile = baseline.Profile(r.cfg.Profile)
	case r.facts.Domain == nil:
		return errors.New("domain membership could not be detected; set profile explicitly")
	case r.facts.Domain.PartOfDomain:
		r.profile = baseline.ProfileDomainJoined
	default:
		r.profile = baseline.ProfileNonDomainJoined
	}
	if r.facts.Domain != nil && r.facts.Domain.PartOfDomain {
		r.warn("the machine is joined to %s; domain Group Policy takes precedence over the local settings applied here and may change them at the next policy refresh", r.facts.Domain.Domain)
	}
	r.ui.Say(fmt.Sprintf("Selected baseline %s (%s %s), profile %s", b.ID, b.Name, b.Version, r.profile))
	return nil
}

// inspect runs the Inspect action for settings.
//
// Returns:
//   - plan.States: The state of each setting.
//   - error: The action failed or its result cannot be decoded.
//
// Author: Lars Krogh
// Version: 1.0.0
func (r *run) inspect(ctx context.Context, settings []baseline.Setting) (plan.States, error) {
	data, err := r.remote.action(ctx, "Inspect", settings)
	if err != nil {
		return nil, err
	}
	return plan.ParseStates(data)
}

// apply runs the Apply action for settings.
//
// Returns:
//   - plan.ApplyResults: The result of each setting.
//   - error: The action failed or its result cannot be decoded.
//
// Author: Lars Krogh
// Version: 1.0.0
func (r *run) apply(ctx context.Context, settings []baseline.Setting) (plan.ApplyResults, error) {
	data, err := r.remote.action(ctx, "Apply", settings)
	if err != nil {
		return nil, err
	}
	return plan.ParseApplyResults(data)
}

// applyAndVerify applies items, re-inspects them and sets their final status.
//
// Returns:
//   - error: The Apply or Inspect action failed. Per-setting failures are
//     recorded on the items instead.
//
// Author: Lars Krogh
// Version: 1.0.0
func (r *run) applyAndVerify(ctx context.Context, items []*plan.Item) error {
	settings := plan.Settings(items)
	results, err := r.apply(ctx, settings)
	if err != nil {
		return err
	}
	after, err := r.inspect(ctx, settings)
	if err != nil {
		return err
	}
	plan.Verify(items, results, after)
	return nil
}

// checkConnectivity runs a new WinRM command and compares the WinRM
// configuration with the one found by Detect, recording the outcome as a
// report check. when describes the moment, for example "after restart".
//
// Returns:
//   - error: The probe failed or WinRM regressed.
//
// Author: Lars Krogh
// Version: 1.0.0
func (r *run) checkConnectivity(ctx context.Context, when string) error {
	name := "new WinRM command " + when
	probe, err := r.probe(ctx)
	if err != nil {
		r.addCheck(name, false, err.Error())
		return fmt.Errorf("WinRM connectivity check %s failed: %w", when, err)
	}
	if problems := facts.WinRMRegressions(r.facts.WinRM, probe); len(problems) > 0 {
		detail := strings.Join(problems, "; ")
		r.addCheck(name, false, detail)
		return fmt.Errorf("WinRM configuration regressed %s: %s", when, detail)
	}
	r.addCheck(name, true, "a new command ran and the WinRM service, listeners and authentication are unchanged")
	return nil
}

// probe runs the Probe action.
//
// Returns:
//   - *facts.Probe: The probe result.
//   - error: The action failed or its result cannot be decoded.
//
// Author: Lars Krogh
// Version: 1.0.0
func (r *run) probe(ctx context.Context) (*facts.Probe, error) {
	data, err := r.remote.action(ctx, "Probe", nil)
	if err != nil {
		return nil, err
	}
	return facts.ParseProbe(data)
}

// addCheck records a connectivity check for the report.
//
// Author: Lars Krogh
// Version: 1.0.0
func (r *run) addCheck(name string, passed bool, details string) {
	r.checks = append(r.checks, report.Check{Name: name, At: r.now().UTC(), Passed: passed, Details: details})
}
