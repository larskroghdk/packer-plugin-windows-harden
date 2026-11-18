// Package report builds the JSON hardening report and the console summary.
//
// Increment SchemaVersion for any incompatible change to the JSON.
package report

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/larskroghdk/packer-plugin-windows-harden/internal/baseline"
	"github.com/larskroghdk/packer-plugin-windows-harden/internal/facts"
	"github.com/larskroghdk/packer-plugin-windows-harden/internal/plan"
)

const SchemaVersion = 1
const (
	ComplianceCompliant    = "compliant"
	CompliancePartial      = "partial"
	ComplianceNonCompliant = "non_compliant"
)

// Report is the JSON document written at the end of a build.
type Report struct {
	SchemaVersion int                `json:"schema_version"`
	Plugin        string             `json:"plugin"`
	StartedAt     time.Time          `json:"started_at"`
	FinishedAt    time.Time          `json:"finished_at"`
	DryRun        bool               `json:"dry_run"`
	Mode          string             `json:"mode"`
	System        System             `json:"system"`
	Baseline      Baseline           `json:"baseline"`
	Summary       Counts             `json:"summary"`
	Compliance    Compliance         `json:"compliance"`
	Connectivity  []Check            `json:"connectivity"`
	Reboot        Reboot             `json:"reboot"`
	Warnings      []string           `json:"warnings"`
	Settings      []Setting          `json:"settings"`
	DeferredPlan  []baseline.Setting `json:"deferred_plan"`
}

// System describes the guest as detected.
type System struct {
	ProductName      string   `json:"product_name"`
	Release          string   `json:"release"`
	Lifecycle        string   `json:"lifecycle"`
	EditionID        string   `json:"edition_id"`
	InstallationType string   `json:"installation_type"`
	ProductType      string   `json:"product_type"`
	Version          string   `json:"version"`
	DisplayVersion   string   `json:"display_version"`
	Architecture     string   `json:"architecture"`
	DomainJoined     *bool    `json:"domain_joined"`
	Domain           string   `json:"domain,omitempty"`
	Roles            []string `json:"roles"`
	WinRMUser        string   `json:"winrm_user"`
	DetectionErrors  []string `json:"detection_errors"`
}

// Baseline identifies the baseline and profile that were applied.
type Baseline struct {
	ID         string               `json:"id"`
	Name       string               `json:"name"`
	Version    string               `json:"version"`
	Profile    string               `json:"profile"`
	Source     baseline.Source      `json:"source"`
	Components []string             `json:"components"`
	Excluded   []baseline.Exclusion `json:"excluded"`
}

// Counts is the number of settings per status.
type Counts struct {
	Evaluated     int `json:"evaluated"`
	Compliant     int `json:"compliant"`
	Hardened      int `json:"hardened"`
	NotApplicable int `json:"not_applicable"`
	Skipped       int `json:"skipped"`
	Deferred      int `json:"deferred"`
	Failed        int `json:"failed"`
	Pending       int `json:"pending"`
}

// Compliance is the overall verdict. FullBaseline is true only when every
// setting of a baseline with no exclusions is compliant.
type Compliance struct {
	Status       string   `json:"status"`
	FullBaseline bool     `json:"full_baseline"`
	Reasons      []string `json:"reasons"`
}

// Check is one WinRM connectivity check made during the build.
type Check struct {
	Name    string    `json:"name"`
	At      time.Time `json:"at"`
	Passed  bool      `json:"passed"`
	Details string    `json:"details"`
}

// Reboot records whether a restart was needed and whether it was performed.
type Reboot struct {
	Required  bool   `json:"required"`
	Performed bool   `json:"performed"`
	Reason    string `json:"reason,omitempty"`
}

// Setting is the report entry for one baseline setting.
type Setting struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
	Component   string `json:"component"`
	Status      string `json:"status"`
	Expected    string `json:"expected"`
	Before      string `json:"before,omitempty"`
	After       string `json:"after,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

// Input is everything Build needs. Facts, Baseline and Plan are nil when the
// build failed before they were known.
type Input struct {
	Plugin         string
	StartedAt      time.Time
	FinishedAt     time.Time
	DryRun         bool
	Mode           string
	Facts          *facts.Facts
	Classification facts.Classification
	Baseline       *baseline.Baseline
	Profile        baseline.Profile
	Plan           *plan.Plan
	Connectivity   []Check
	Reboot         Reboot
	Warnings       []string
}

// Build assembles the report from what the run produced, including when the
// run failed early.
//
// Author: Lars Krogh
// Version: 1.0.0
func Build(in Input) *Report {
	r := &Report{
		SchemaVersion: SchemaVersion,
		Plugin:        in.Plugin,
		StartedAt:     in.StartedAt.UTC(),
		FinishedAt:    in.FinishedAt.UTC(),
		DryRun:        in.DryRun,
		Mode:          in.Mode,
		Connectivity:  in.Connectivity,
		Reboot:        in.Reboot,
		Warnings:      in.Warnings,
		Settings:      []Setting{},
		DeferredPlan:  []baseline.Setting{},
	}
	if in.Facts != nil {
		r.System = system(in.Facts, in.Classification)
	}
	if in.Baseline != nil {
		r.Baseline = Baseline{
			ID: in.Baseline.ID, Name: in.Baseline.Name, Version: in.Baseline.Version,
			Profile: string(in.Profile), Source: in.Baseline.Source,
			Components: in.Baseline.Components, Excluded: in.Baseline.Excluded,
		}
	}
	if in.Plan != nil {
		for _, it := range in.Plan.Items {
			s := Setting{
				ID:          it.Setting.ID,
				Kind:        string(it.Setting.Kind),
				Description: it.Setting.Description,
				Component:   it.Setting.Component,
				Status:      string(it.Status),
				Expected:    it.Setting.ExpectedValue(),
				Reason:      it.Reason,
			}
			if it.Before != nil {
				s.Before = it.Before.String()
			}
			if it.After != nil {
				s.After = it.After.String()
			}
			r.Settings = append(r.Settings, s)
			if it.Status == plan.StatusDeferred {
				r.DeferredPlan = append(r.DeferredPlan, it.Setting)
			}
		}
		r.Summary = counts(in.Plan)
	}
	r.Compliance = compliance(r, in.Baseline, in.Plan != nil)
	return r
}

// system converts the guest facts and classification to the report's system
// section.
//
// Author: Lars Krogh
// Version: 1.0.0
func system(f *facts.Facts, c facts.Classification) System {
	s := System{
		ProductName:      f.OS.ProductName,
		Release:          c.Release.Name,
		Lifecycle:        string(c.Status),
		EditionID:        f.OS.EditionID,
		InstallationType: f.OS.InstallationType,
		ProductType:      f.OS.ProductTypeName(),
		Version:          f.OS.Version(),
		DisplayVersion:   f.OS.DisplayVersion,
		Architecture:     f.OS.Architecture,
		Roles:            f.Roles,
		WinRMUser:        f.Session.UserName,
		DetectionErrors:  f.Errors,
	}
	if f.Domain != nil {
		joined := f.Domain.PartOfDomain
		s.DomainJoined = &joined
		if joined {
			s.Domain = f.Domain.Domain
		}
	}
	return s
}

// counts tallies the plan items by status.
//
// Author: Lars Krogh
// Version: 1.0.0
func counts(p *plan.Plan) Counts {
	return Counts{
		Evaluated:     len(p.Items),
		Compliant:     p.Count(plan.StatusCompliant),
		Hardened:      p.Count(plan.StatusHardened),
		NotApplicable: p.Count(plan.StatusNotApplicable),
		Skipped:       p.Count(plan.StatusSkipped),
		Deferred:      p.Count(plan.StatusDeferred),
		Failed:        p.Count(plan.StatusFailed),
		Pending:       p.Count(plan.StatusPending),
	}
}

// compliance derives the verdict. Failed or pending settings and failed
// connectivity checks make the report non_compliant, and deferred or skipped
// settings make it partial. evaluated is false when no plan was built.
//
// Author: Lars Krogh
// Version: 1.0.0
func compliance(r *Report, b *baseline.Baseline, evaluated bool) Compliance {
	c := Compliance{Reasons: []string{}}
	if !evaluated {
		c.Status = ComplianceNonCompliant
		c.Reasons = append(c.Reasons, "no baseline was evaluated")
		return c
	}
	n := r.Summary
	if n.Failed > 0 {
		c.Reasons = append(c.Reasons, fmt.Sprintf("%d settings failed", n.Failed))
	}
	if n.Pending > 0 {
		c.Reasons = append(c.Reasons, fmt.Sprintf("%d settings are not compliant and were not changed", n.Pending))
	}
	if n.Deferred > 0 {
		c.Reasons = append(c.Reasons, fmt.Sprintf("%d settings were deferred to preserve WinRM connectivity", n.Deferred))
	}
	if n.Skipped > 0 {
		c.Reasons = append(c.Reasons, fmt.Sprintf("%d settings were skipped by configuration", n.Skipped))
	}
	checkFailed := false
	for _, check := range r.Connectivity {
		if !check.Passed {
			checkFailed = true
			c.Reasons = append(c.Reasons, "connectivity check failed: "+check.Name)
		}
	}
	switch {
	case n.Failed > 0 || n.Pending > 0 || checkFailed:
		c.Status = ComplianceNonCompliant
	case n.Deferred > 0 || n.Skipped > 0:
		c.Status = CompliancePartial
	default:
		c.Status = ComplianceCompliant
	}
	if b != nil && len(b.Excluded) > 0 {
		c.Reasons = append(c.Reasons, fmt.Sprintf("%d baseline components or settings are outside the implemented scope (see baseline.excluded)", len(b.Excluded)))
	}
	c.FullBaseline = c.Status == ComplianceCompliant && b != nil && len(b.Excluded) == 0
	return c
}

// JSON returns the report as indented JSON with a trailing newline.
//
// Returns:
//   - []byte: The encoded report.
//   - error: The report cannot be encoded.
//
// Author: Lars Krogh
// Version: 1.0.0
func (r *Report) JSON() ([]byte, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// SummaryLines returns the console summary: system, baseline, counts,
// compliance and reboot state, followed by every deferred and failed setting
// and every warning.
//
// Author: Lars Krogh
// Version: 1.0.0
func (r *Report) SummaryLines() []string {
	n := r.Summary
	lines := []string{
		fmt.Sprintf("System:     %s (%s, build %s, %s)", r.System.ProductName, r.System.EditionID, r.System.Version, r.System.InstallationType),
		fmt.Sprintf("Baseline:   %s %s [%s], profile %s", r.Baseline.Name, r.Baseline.Version, r.Baseline.ID, r.Baseline.Profile),
		fmt.Sprintf("Settings:   %d evaluated: %d compliant, %d hardened, %d not applicable, %d skipped, %d deferred, %d failed, %d pending",
			n.Evaluated, n.Compliant, n.Hardened, n.NotApplicable, n.Skipped, n.Deferred, n.Failed, n.Pending),
		fmt.Sprintf("Compliance: %s (full Microsoft baseline: %t)", r.Compliance.Status, r.Compliance.FullBaseline),
	}
	for _, reason := range r.Compliance.Reasons {
		lines = append(lines, "  - "+reason)
	}
	if r.Reboot.Required {
		state := "required, not performed"
		if r.Reboot.Performed {
			state = "performed"
		}
		lines = append(lines, fmt.Sprintf("Reboot:     %s (%s)", state, r.Reboot.Reason))
	}
	for _, s := range r.settingsWithStatus(plan.StatusDeferred) {
		lines = append(lines, fmt.Sprintf("Deferred:   %s: %s", s.ID, s.Reason))
	}
	for _, s := range r.settingsWithStatus(plan.StatusFailed) {
		lines = append(lines, fmt.Sprintf("Failed:     %s: %s", s.ID, s.Reason))
	}
	for _, w := range r.Warnings {
		lines = append(lines, "Warning:    "+w)
	}
	return lines
}

// settingsWithStatus returns the settings with status, sorted by ID.
//
// Author: Lars Krogh
// Version: 1.0.0
func (r *Report) settingsWithStatus(status plan.Status) []Setting {
	var out []Setting
	for _, s := range r.Settings {
		if s.Status == string(status) {

			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].ID) < strings.ToLower(out[j].ID) })
	return out
}
