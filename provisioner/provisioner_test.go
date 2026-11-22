package provisioner

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/larskroghdk/packer-plugin-windows-harden/internal/baseline"
	"github.com/larskroghdk/packer-plugin-windows-harden/internal/facts"
	"github.com/larskroghdk/packer-plugin-windows-harden/internal/plan"
)

type testCommunicator struct {
	packer.MockCommunicator
	start    func(context.Context, *packer.RemoteCmd) error
	download func(string, io.Writer) error
}

// Start dispatches a command to the test's guest simulator.
//
// Returns:
//   - error: The simulated transport error.
//
// Author: Lars Krogh
// Version: 1.0.0
func (c *testCommunicator) Start(ctx context.Context, cmd *packer.RemoteCmd) error {
	if c.start != nil {
		if err := c.start(ctx, cmd); err != nil {
			return err
		}
	}
	cmd.SetExited(0)
	return nil
}

// Download writes a simulated guest response to the plugin.
//
// Returns:
//   - error: The simulated download error.
//
// Author: Lars Krogh
// Version: 1.0.0
func (c *testCommunicator) Download(path string, out io.Writer) error {
	return c.download(path, out)
}

// testRun creates a run with a simulated guest and a fixed clock.
//
// Author: Lars Krogh
// Version: 1.0.0
func testRun(c *testCommunicator) *run {
	ui := &packer.MockUi{}
	return &run{
		cfg: &Config{RestartTimeout: time.Minute}, ui: ui,
		remote: newRemote(c, ui, "test"),
		now:    func() time.Time { return time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC) },
	}
}

// TestExecuteReadOnlyModes ensures strict and dry-run plans never apply changes
// and that dry-run inspection errors obey fail_on_error.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestExecuteReadOnlyModes(t *testing.T) {
	for _, mode := range []string{ModeStrict, "dry-run", "failed-inspection"} {
		t.Run(mode, func(t *testing.T) {
			catalog, err := baseline.Load()
			if err != nil {
				t.Fatal(err)
			}
			b, err := catalog.Select("server", 20348)
			if err != nil {
				t.Fatal(err)
			}
			states := make([]plan.State, 0, len(b.Settings))
			for _, s := range b.Settings {
				states = append(states, plan.State{ID: s.ID})
			}
			if mode == "failed-inspection" {
				failure := "access denied"
				states[0].Error = &failure
			}
			var actions []string
			c := &testCommunicator{}
			c.start = func(_ context.Context, cmd *packer.RemoteCmd) error {
				if strings.Contains(cmd.Command, "-Action ") {
					_, tail, _ := strings.Cut(cmd.Command, "-Action ")
					action := strings.Fields(tail)[0]
					actions = append(actions, action)
					if action != "Detect" && action != "Inspect" {
						t.Fatalf("unexpected write action: %s", action)
					}
				}
				return nil
			}
			c.download = func(path string, out io.Writer) error {
				if strings.Contains(path, "detect-output") {
					return json.NewEncoder(out).Encode(facts.Facts{
						OS:      facts.OS{Build: 20348, ProductType: 3, Is64BitProcess: true},
						Session: facts.Session{IsElevated: true}, Domain: &facts.Domain{},
						WinRM: &facts.WinRM{Auth: facts.WinRMAuth{Basic: true}},
					})
				}
				return json.NewEncoder(out).Encode(struct {
					Items []plan.State `json:"items"`
				}{states})
			}
			r := testRun(c)
			r.catalog = catalog
			r.cfg.Mode, r.cfg.Baseline, r.cfg.Profile = ModeCompatibility, BaselineAuto, string(baseline.ProfileDomainJoined)
			r.cfg.DryRun = mode != ModeStrict
			if mode == ModeStrict {
				r.cfg.Mode = ModeStrict
			}
			err = r.execute(context.Background())
			if (err != nil) != (mode != "dry-run") {
				t.Fatalf("execute error = %v", err)
			}
			if strings.Join(actions, ",") != "Detect,Inspect" {
				t.Fatalf("actions = %v", actions)
			}
		})
	}
}

// TestProbeDeadline checks expired, canceled, and late-success attempts without
// waiting for the real guard interval.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestProbeDeadline(t *testing.T) {
	c := &testCommunicator{}
	r := testRun(c)
	now := r.now()
	r.now = func() time.Time { return now }
	c.start = func(ctx context.Context, _ *packer.RemoteCmd) error {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > time.Second {
			t.Fatal("probe attempt exceeds remaining window")
		}
		now = now.Add(time.Second)
		return nil
	}
	c.download = func(_ string, out io.Writer) error {
		_, err := io.WriteString(out, `{"boot_time":"new"}`)
		return err
	}
	if _, err := r.probeUntil(context.Background(), now, ""); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expired probe = %v", err)
	}
	if _, err := r.probeUntil(context.Background(), now.Add(time.Second), ""); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("late probe = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.probeUntil(ctx, now.Add(time.Minute), ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled probe = %v", err)
	}
}

// TestRestartRestoresScriptsBeforeProbe simulates a reboot that deleted the
// working directory and requires an upload before the next probe can run.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestRestartRestoresScriptsBeforeProbe(t *testing.T) {
	c := &testCommunicator{}
	r := testRun(c)
	c.start = func(_ context.Context, cmd *packer.RemoteCmd) error {
		if strings.Contains(cmd.Command, "-Action Probe") && !c.UploadCalled {
			t.Fatal("probed before restoring scripts")
		}
		return nil
	}
	c.download = func(_ string, out io.Writer) error {
		_, err := io.WriteString(out, `{"boot_time":"new"}`)
		return err
	}
	if _, err := r.probeUntil(context.Background(), r.now().Add(time.Minute), "old"); err != nil {
		t.Fatal(err)
	}
}

// TestGuardPreservesRecoveryFiles checks cleanup and missing disarm responses.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestGuardPreservesRecoveryFiles(t *testing.T) {
	c := &testCommunicator{}
	r := testRun(c)
	r.remote.guardArmed = true
	if err := r.remote.cleanup(context.Background()); err == nil || c.StartCalled {
		t.Fatal("cleanup removed an armed guard")
	}
	c.download = func(_ string, out io.Writer) error { _, err := io.WriteString(out, `{}`); return err }
	if _, err := r.disarm(context.Background(), guardRequest{}); err == nil || !r.remote.guardArmed {
		t.Fatal("missing acknowledgement disarmed the guard")
	}
	c.download = func(_ string, out io.Writer) error {
		_, err := io.WriteString(out, `{"rolled_back":false}`)
		return err
	}
	if ok, err := r.disarm(context.Background(), guardRequest{}); err != nil || !ok || r.remote.guardArmed {
		t.Fatalf("disarm = %t, %v", ok, err)
	}
}

// TestGuardRequiresAcknowledgement ensures malformed arm responses never allow
// a guarded setting to be applied.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestGuardRequiresAcknowledgement(t *testing.T) {
	c := &testCommunicator{}
	r := testRun(c)
	c.start = func(_ context.Context, cmd *packer.RemoteCmd) error {
		if strings.Contains(cmd.Command, "-Action Apply") {
			t.Fatal("applied without an armed guard")
		}
		return nil
	}
	c.download = func(_ string, out io.Writer) error { _, err := io.WriteString(out, `{"armed":false}`); return err }
	it := &plan.Item{Setting: baseline.Setting{ID: "test", Kind: baseline.KindRegistry}, Before: &plan.State{}}
	if err := r.applyGuarded(context.Background(), []*plan.Item{it}); err == nil || !r.remote.guardArmed {
		t.Fatal("unconfirmed guard was accepted or recovery files were released")
	}
}

// TestGuardRejectsUnrestorableSnapshots prevents unsupported registry types
// and failed inspections from being armed or applied.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestGuardRejectsUnrestorableSnapshots(t *testing.T) {
	badType, value, failure := "REG_BINARY", "00", "access denied"
	for _, state := range []*plan.State{
		nil,
		{Error: &failure},
		{Present: true},
		{Present: true, Type: &badType, Value: &value},
	} {
		c := &testCommunicator{}
		r := testRun(c)
		it := &plan.Item{Setting: baseline.Setting{ID: "test", Kind: baseline.KindRegistry}, Before: state}
		if err := r.applyGuarded(context.Background(), []*plan.Item{it}); err == nil || c.UploadCalled || r.remote.guardArmed {
			t.Fatalf("accepted unrestorable state %+v", state)
		}
	}
}
