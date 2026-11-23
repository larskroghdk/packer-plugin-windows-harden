package provisioner

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
	"github.com/larskroghdk/packer-plugin-windows-harden/internal/baseline"
)

const (
	ModeCompatibility     = "compatibility"
	ModeStrict            = "strict"
	BaselineAuto          = "auto"
	ProfileAuto           = "auto"
	defaultRestartTimeout = 30 * time.Minute
)

// Config is the provisioner's HCL configuration. See docs/configuration.md for
// each option.
type Config struct {
	common.PackerConfig `mapstructure:",squash"`
	Baseline            string         `mapstructure:"baseline"`
	Profile             string         `mapstructure:"profile"`
	Mode                string         `mapstructure:"mode"`
	DryRun              bool           `mapstructure:"dry_run"`
	Exclude             []string       `mapstructure:"exclude"`
	FailOnError         config.Trilean `mapstructure:"fail_on_error"`
	RebootIfRequired    bool           `mapstructure:"reboot_if_required"`
	RestartTimeout      time.Duration  `mapstructure:"restart_timeout"`
	ReportPath          string         `mapstructure:"report_path"`
	ctx                 interpolate.Context
}

// prepare decodes and interpolates the raw configuration, applies defaults and
// validates every option against catalog.
//
// Returns:
//   - error: All validation errors, combined.
//
// Author: Lars Krogh
// Version: 1.0.0
func (c *Config) prepare(catalog *baseline.Catalog, raws ...any) error {
	err := config.Decode(c, &config.DecodeOpts{
		PluginType:         "windows-harden",
		Interpolate:        true,
		InterpolateContext: &c.ctx,
	}, raws...)
	if err != nil {
		return err
	}
	if c.Baseline == "" {
		c.Baseline = BaselineAuto
	}
	if c.Profile == "" {
		c.Profile = ProfileAuto
	}
	if c.Mode == "" {
		c.Mode = ModeCompatibility
	}
	if c.RestartTimeout == 0 {
		c.RestartTimeout = defaultRestartTimeout
	}
	if c.ReportPath == "" {
		name := c.PackerBuildName
		if name == "" {
			name = "build"
		}
		c.ReportPath = fmt.Sprintf("windows-harden-report-%s.json", sanitizeFileName(name))
	}
	var errs error
	if c.Baseline != BaselineAuto {
		if _, ok := catalog.Get(c.Baseline); !ok {
			errs = packer.MultiErrorAppend(errs, fmt.Errorf("baseline: unknown baseline %q; valid values are %q or one of %s", c.Baseline, BaselineAuto, baselineIDs(catalog)))
		}
	}
	switch baseline.Profile(c.Profile) {
	case ProfileAuto, baseline.ProfileDomainJoined, baseline.ProfileNonDomainJoined:
	default:
		errs = packer.MultiErrorAppend(errs, fmt.Errorf("profile: must be %q, %q or %q", ProfileAuto, baseline.ProfileDomainJoined, baseline.ProfileNonDomainJoined))
	}
	switch c.Mode {
	case ModeCompatibility, ModeStrict:
	default:
		errs = packer.MultiErrorAppend(errs, fmt.Errorf("mode: must be %q or %q", ModeCompatibility, ModeStrict))
	}
	for i, pattern := range c.Exclude {
		if strings.TrimSpace(pattern) == "" || strings.Trim(pattern, "*") == "" {
			errs = packer.MultiErrorAppend(errs, fmt.Errorf("exclude[%d]: pattern %q would exclude every setting", i, pattern))
		}
	}
	if c.RestartTimeout < time.Minute {
		errs = packer.MultiErrorAppend(errs, errors.New("restart_timeout: must be at least 1m"))
	}
	return errs
}

// failOnError reports whether failed settings fail the build. It is true
// unless fail_on_error is explicitly false.
//
// Author: Lars Krogh
// Version: 1.0.0
func (c *Config) failOnError() bool { return !c.FailOnError.False() }

// baselineIDs returns the catalog's baseline IDs as a quoted, comma-separated
// list for error messages.
//
// Author: Lars Krogh
// Version: 1.0.0
func baselineIDs(catalog *baseline.Catalog) string {
	var ids []string
	for _, b := range catalog.Baselines() {
		ids = append(ids, fmt.Sprintf("%q", b.ID))
	}
	return strings.Join(ids, ", ")
}

// sanitizeFileName replaces every character other than ASCII letters, digits,
// '-', '_' and '.' with '_'.
//
// Author: Lars Krogh
// Version: 1.0.0
func sanitizeFileName(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		}
		return '_'
	}, s)
}
