package provisioner

import (
	"context"
	"fmt"

	"github.com/larskroghdk/packer-plugin-windows-harden/internal/plan"
)

const restartCommand = `shutdown.exe /r /f /t 5 /c "packer-plugin-windows-harden: restart after applying the security baseline"`

// restartAndReverify restarts the guest, waits for the boot time to change,
// reinstalls the scripts, checks WinRM, and re-verifies every compliant and
// hardened setting.
//
// Returns:
//   - error: The restart failed, the guest did not come back within
//     restart_timeout, WinRM regressed, or re-inspection failed.
//
// Author: Lars Krogh
// Version: 1.0.0
func (r *run) restartAndReverify(ctx context.Context) error {
	before, err := r.probe(ctx)
	if err != nil {
		return fmt.Errorf("cannot read the boot time before restarting: %w", err)
	}
	r.ui.Say("Restarting the machine...")
	if err := r.remote.exec(ctx, restartCommand); err != nil {
		return fmt.Errorf("restart command failed: %w", err)
	}
	r.ui.Say(fmt.Sprintf("Waiting up to %s for the machine to restart...", r.cfg.RestartTimeout))
	if _, err := r.probeUntil(ctx, r.now().Add(r.cfg.RestartTimeout), before.BootTime); err != nil {
		r.addCheck("WinRM after restart", false, err.Error())
		return fmt.Errorf("the machine did not come back over WinRM within %s: %w", r.cfg.RestartTimeout, err)
	}
	r.reboot.Performed = true
	if err := r.remote.install(ctx); err != nil {
		return err
	}
	if err := r.checkConnectivity(ctx, "after restart"); err != nil {
		return err
	}
	settings := plan.Settings(r.plan.Items)
	current, err := r.inspect(ctx, settings)
	if err != nil {
		return err
	}
	plan.Reverify(r.plan.Items, current, "after restart")
	return nil
}
