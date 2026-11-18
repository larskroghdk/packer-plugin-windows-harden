package provisioner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/larskroghdk/packer-plugin-windows-harden/internal/baseline"
	"github.com/larskroghdk/packer-plugin-windows-harden/internal/facts"
	"github.com/larskroghdk/packer-plugin-windows-harden/internal/plan"
)

const (
	guardDelay          = 3 * time.Minute
	guardProbeWindow    = 90 * time.Second
	guardRollbackMargin = 45 * time.Second
	probeInterval       = 10 * time.Second
	probeAttemptTimeout = 2 * time.Minute
)

type guardRequest struct {
	TaskName     string `json:"task_name"`
	DelaySeconds int    `json:"delay_seconds"`
	SnapshotPath string `json:"snapshot_path"`
	ResultPath   string `json:"result_path"`
}

type guardSnapshot struct {
	Items []guardItem `json:"items"`
}

type guardItem struct {
	Setting baseline.Setting `json:"setting"`
	State   guardState       `json:"state"`
}

type guardState struct {
	Present bool    `json:"present"`
	Type    *string `json:"type"`
	Value   *string `json:"value"`
}

// applyGuarded applies settings that might break WinRM authentication under an
// automatic rollback. It uploads a snapshot of the current values, arms a
// guest task that restores them after guardDelay, applies the settings and
// probes WinRM. If WinRM works and nothing regressed, the guard is disarmed and
// the settings are verified. If WinRM regressed, the settings are rolled back
// at once. If WinRM is unreachable, it waits for the guest to roll back and
// reconnect. Rolled-back settings are recorded as deferred.
//
// Returns:
//   - error: The guard could not be armed or disarmed, the immediate rollback
//     failed, or WinRM did not recover within restart_timeout.
//
// Author: Lars Krogh
// Version: 1.0.0
func (r *run) applyGuarded(ctx context.Context, items []*plan.Item) error {
	r.ui.Say(fmt.Sprintf("Applying %d connectivity-guarded settings (automatic rollback after %s unless WinRM is verified)...", len(items), guardDelay))
	var snapshot guardSnapshot
	for _, it := range items {
		if it.Setting.Kind != baseline.KindRegistry || it.Before == nil {
			return fmt.Errorf("internal error: guarded setting %s has no restorable registry state", it.Setting.ID)
		}
		snapshot.Items = append(snapshot.Items, guardItem{
			Setting: it.Setting,
			State:   guardState{Present: it.Before.Present, Type: it.Before.Type, Value: it.Before.Value},
		})
	}
	snapshotPath, err := r.remote.upload("guard-snapshot.json", snapshot)
	if err != nil {
		return err
	}
	guard := guardRequest{
		TaskName:     "packer-windows-harden-guard-" + r.remote.id,
		DelaySeconds: int(guardDelay.Seconds()),
		SnapshotPath: snapshotPath,
		ResultPath:   r.remote.path("guard-rollback-result.json"),
	}
	if _, err := r.remote.action(ctx, "ArmGuard", guard); err != nil {
		return fmt.Errorf("could not arm the connectivity guard, guarded settings were not applied: %w", err)
	}
	armedAt := r.now()
	settings := plan.Settings(items)
	results, applyErr := r.apply(ctx, settings)
	var probe *facts.Probe
	probeErr := applyErr
	if applyErr == nil {
		probe, probeErr = r.probeUntil(ctx, r.now().Add(guardProbeWindow), "")
	}
	if probeErr == nil {
		problems := facts.WinRMRegressions(r.facts.WinRM, probe)
		if len(problems) == 0 {
			disarmed, err := r.disarm(ctx, guard)
			if err != nil {
				return err
			}
			if disarmed {
				after, err := r.inspect(ctx, settings)
				if err != nil {
					return err
				}
				plan.Verify(items, results, after)
				r.addCheck("new WinRM command after guarded settings", true, "a new command ran within the guard window and WinRM is unchanged; the guard was disarmed")
				return nil
			}
			return r.rolledBack(ctx, items, "the guard rolled the change back before it could be disarmed")
		}
		reason := strings.Join(problems, "; ")
		if _, err := r.remote.action(ctx, "Rollback", snapshot); err != nil {
			return fmt.Errorf("WinRM regressed after guarded settings (%s) and the immediate rollback failed: %w", reason, err)
		}
		if _, err := r.disarm(ctx, guard); err != nil {
			return err
		}
		return r.rolledBack(ctx, items, "WinRM regressed after the change: "+reason)
	}
	r.ui.Error(fmt.Sprintf("WinRM could not be verified after the guarded change (%v); waiting for the guest to roll it back...", probeErr))
	resume := armedAt.Add(guardDelay + guardRollbackMargin)
	if err := sleepUntil(ctx, resume, r.now); err != nil {
		return err
	}
	if _, err := r.probeUntil(ctx, r.now().Add(r.cfg.RestartTimeout), ""); err != nil {
		r.addCheck("WinRM recovery after guard rollback", false, err.Error())
		return fmt.Errorf("WinRM connectivity was lost after applying guarded settings and did not recover within %s after the automatic rollback: %w", r.cfg.RestartTimeout, err)
	}
	if _, err := r.disarm(ctx, guard); err != nil {
		return err
	}
	return r.rolledBack(ctx, items, fmt.Sprintf("connectivity verification failed after the change (%v)", probeErr))
}

// disarm removes the rollback task.
//
// Returns:
//   - bool: True when the task was removed before it ran, false when it had
//     already rolled the settings back.
//   - error: The DisarmGuard action failed or its result cannot be decoded.
//
// Author: Lars Krogh
// Version: 1.0.0
func (r *run) disarm(ctx context.Context, guard guardRequest) (bool, error) {
	data, err := r.remote.action(ctx, "DisarmGuard", guard)
	if err != nil {
		return false, fmt.Errorf("could not disarm the connectivity guard: %w", err)
	}
	var res struct {
		RolledBack bool `json:"rolled_back"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return false, fmt.Errorf("decode disarm result: %w", err)
	}
	return !res.RolledBack, nil
}

// rolledBack records a failed connectivity check and re-inspects items after a
// rollback. Items whose previous value was restored become deferred, and the
// rest become failed. why is the reason given for the rollback.
//
// Returns:
//   - error: The Inspect action failed.
//
// Author: Lars Krogh
// Version: 1.0.0
func (r *run) rolledBack(ctx context.Context, items []*plan.Item, why string) error {
	r.addCheck("new WinRM command after guarded settings", false, why)
	after, err := r.inspect(ctx, plan.Settings(items))
	if err != nil {
		return err
	}
	for _, it := range items {
		st, ok := after.Get(it.Setting.ID)
		if ok {
			it.After = &st
		}
		if ok && st.Error == nil && it.Before != nil && st.String() == it.Before.String() {
			it.Status = plan.StatusDeferred
			it.Reason = why + "; the previous value was restored"
			continue
		}
		it.Status = plan.StatusFailed
		it.Reason = why + "; the previous value could not be confirmed after the rollback"
	}
	return nil
}

// probeUntil repeats the Probe action every probeInterval until it succeeds or
// deadline passes. When previousBootTime is set, a probe only counts once the
// boot time has changed, which is how a restart is detected.
//
// Returns:
//   - *facts.Probe: The first successful probe.
//   - error: The last failure when the deadline passes, or the context error.
//
// Author: Lars Krogh
// Version: 1.0.0
func (r *run) probeUntil(ctx context.Context, deadline time.Time, previousBootTime string) (*facts.Probe, error) {
	var lastErr error
	for {
		attemptCtx, cancel := context.WithTimeout(ctx, probeAttemptTimeout)
		p, err := r.probe(attemptCtx)
		cancel()
		switch {
		case err == nil && (previousBootTime == "" || p.BootTime != previousBootTime):
			return p, nil
		case err == nil:
			lastErr = errors.New("the machine has not restarted yet")

		default:
			lastErr = err
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !r.now().Add(probeInterval).Before(deadline) {
			return nil, lastErr
		}
		if err := sleepUntil(ctx, r.now().Add(probeInterval), r.now); err != nil {
			return nil, err
		}
	}
}

// sleepUntil waits until time t as measured by now, or until ctx is done.
//
// Returns:
//   - error: The context error, or nil.
//
// Author: Lars Krogh
// Version: 1.0.0
func sleepUntil(ctx context.Context, t time.Time, now func() time.Time) error {
	d := t.Sub(now())
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
