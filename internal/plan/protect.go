package plan

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/larskroghdk/packer-plugin-windows-harden/internal/baseline"
	"github.com/larskroghdk/packer-plugin-windows-harden/internal/facts"
)

// Action is how a pending setting is applied.
type Action string

const (
	ActionApply Action = "apply"
	ActionDefer Action = "defer"
	ActionGuard Action = "guard"
)

// Decision is the protection rules' verdict for one setting.
type Decision struct {
	Action Action
	Reason string
}

// Context is what the protection rules decide on: the guest facts, the
// current state of each setting, and whether Packer authenticates with an
// empty password.
type Context struct {
	Facts         *facts.Facts
	Current       States
	EmptyPassword bool
}

const (
	keyWinRMServiceBasic        = `software\policies\microsoft\windows\winrm\service\allowbasic`
	keyWinRMServiceUnencrypted  = `software\policies\microsoft\windows\winrm\service\allowunencryptedtraffic`
	keySystemPolicies           = `software\microsoft\windows\currentversion\policies\system\`
	keyEnableLUA                = keySystemPolicies + `enablelua`
	keyLocalAccountTokenFilter  = keySystemPolicies + `localaccounttokenfilterpolicy`
	keyFilterAdministratorToken = keySystemPolicies + `filteradministratortoken`
	keyLimitBlankPasswordUse    = `system\currentcontrolset\control\lsa\limitblankpassworduse`
	keyLmCompatibilityLevel     = `system\currentcontrolset\control\lsa\lmcompatibilitylevel`
	keyNTLMMinServerSec         = `system\currentcontrolset\control\lsa\msv1_0\ntlmminserversec`
	keyFirewallPolicies         = `software\policies\microsoft\windowsfirewall\`
)

var firewallProfileKeys = map[string]string{
	"domainprofile":  "Domain",
	"privateprofile": "Private",
	"publicprofile":  "Public",
}

// registryKey returns the lower-case path\name key the protection rules match
// registry values on.
//
// Author: Lars Krogh
// Version: 1.0.0
func registryKey(r *baseline.RegistryValue) string {
	return strings.ToLower(r.Path + `\` + r.Name)
}

// Protect decides how each changed setting may be applied without breaking
// the Packer session. Settings no rule applies to are applied. The remote UAC
// settings are evaluated together, in an order that keeps the session token
// elevated.
//
// Returns:
//   - map[string]Decision: One decision per setting ID in changes.
//
// Author: Lars Krogh
// Version: 1.0.0
func Protect(changes []baseline.Setting, ctx Context) map[string]Decision {
	out := make(map[string]Decision, len(changes))
	for _, s := range changes {
		out[s.ID] = Decision{Action: ActionApply}
	}
	uac := newUACModel(ctx)
	for _, key := range []string{keyLocalAccountTokenFilter, keyFilterAdministratorToken, keyEnableLUA} {
		for _, s := range changes {
			if s.Kind == baseline.KindRegistry && registryKey(s.Registry) == key {
				out[s.ID] = uac.decide(key, s.Registry.Data)
			}
		}
	}
	for _, s := range changes {
		var d *Decision
		switch s.Kind {
		case baseline.KindRegistry:
			d = registryRule(s.Registry, ctx)
		case baseline.KindUserRight:
			d = userRightRule(s.UserRight, ctx)
		}
		if d != nil {
			out[s.ID] = *d
		}
	}
	return out
}

// deferred returns a defer decision with a formatted reason.
//
// Author: Lars Krogh
// Version: 1.0.0
func deferred(format string, args ...any) *Decision {
	return &Decision{Action: ActionDefer, Reason: fmt.Sprintf(format, args...)}
}

// registryRule applies the WinRM, blank password, NTLM and firewall rules to a
// registry setting. It defers when the facts needed to decide are missing.
//
// Returns:
//   - *Decision: The decision, or nil when no rule objects to the change.
//
// Author: Lars Krogh
// Version: 1.0.0
func registryRule(r *baseline.RegistryValue, ctx Context) *Decision {
	key := registryKey(r)
	f := ctx.Facts
	switch key {
	case keyWinRMServiceBasic:
		if r.Data != "0" {
			return nil
		}
		if f.WinRM == nil {
			return deferred("the WinRM service configuration could not be inspected, so it is unknown whether Packer uses Basic authentication")
		}
		if f.WinRM.Auth.Basic {
			return deferred("Basic authentication is enabled on the WinRM service and may be the method Packer authenticates with; disabling it would reject Packer's next connection")
		}
	case keyWinRMServiceUnencrypted:
		if r.Data != "0" {
			return nil
		}
		if f.WinRM == nil {
			return deferred("the WinRM service configuration could not be inspected, so it is unknown whether Packer uses unencrypted HTTP")
		}
		if f.WinRM.AllowUnencrypted && f.UsesUnencryptedTransport() {
			return deferred("unencrypted traffic is allowed on the WinRM service and Packer is (or may be) connected over an HTTP listener; disallowing it would reject Packer's next request")
		}
	case keyLimitBlankPasswordUse:
		if r.Data == "1" && ctx.EmptyPassword && f.Session.IsLocalAccount {
			return deferred("Packer authenticates as local account %s with an empty password; this setting blocks network logons with blank passwords", f.Session.UserName)
		}
	case keyLmCompatibilityLevel, keyNTLMMinServerSec:
		if f.WinRM != nil && !f.WinRM.Auth.Negotiate {
			return nil
		}
		return &Decision{
			Action: ActionGuard,
			Reason: "changes NTLM requirements for new authentications; whether the Packer WinRM client meets them cannot be determined from the guest, so the change is verified with a new WinRM session and rolled back automatically if that fails",
		}
	}
	if strings.HasPrefix(key, keyFirewallPolicies) {
		return firewallRule(r, key, f)
	}
	return nil
}

// firewallRule protects the firewall profile that carries the Packer
// connection from being enabled, set to block inbound traffic, or told to
// ignore local rules, when any of these could block WinRM. Settings for
// profiles that do not carry the connection, and settings below a profile
// such as logging, are not affected.
//
// Returns:
//   - *Decision: The decision, or nil when no rule objects to the change.
//
// Author: Lars Krogh
// Version: 1.0.0
func firewallRule(r *baseline.RegistryValue, key string, f *facts.Facts) *Decision {
	rest := strings.TrimPrefix(key, keyFirewallPolicies)
	profileKey, value, ok := strings.Cut(rest, `\`)
	profile, known := firewallProfileKeys[profileKey]
	if !ok || !known || strings.Contains(value, `\`) {
		return nil
	}
	active, fromConnections := f.ActiveFirewallProfiles()
	if !active[profile] {
		return nil
	}
	source := "a connected network"
	if fromConnections {
		source = "the network interface carrying the Packer WinRM connection"
	}
	var state *facts.FirewallProfile
	if f.Firewall != nil {
		for i := range f.Firewall.Profiles {
			if strings.EqualFold(f.Firewall.Profiles[i].Name, profile) {
				state = &f.Firewall.Profiles[i]
				break
			}
		}
	}
	switch value {
	case "enablefirewall":
		if r.Data != "1" {
			return nil
		}
		if state == nil {
			return deferred("the %s firewall profile is used by %s and its state could not be inspected; enabling it could block WinRM", profile, source)
		}
		if !state.Enabled {
			return deferred("the %s firewall profile is used by %s and is currently disabled; enabling it would block WinRM unless an inbound rule allows it, which is not verified", profile, source)
		}
	case "defaultinboundaction":
		if r.Data != "1" {
			return nil
		}
		if state == nil || !state.Enabled || !strings.EqualFold(state.DefaultInboundAction, "Block") {
			return deferred("the %s firewall profile is used by %s and does not currently block inbound connections by default; blocking them could block WinRM", profile, source)
		}
	case "allowlocalpolicymerge":
		if r.Data != "0" {
			return nil
		}
		return deferred("the %s firewall profile is used by %s; disabling local rule merging makes Windows ignore locally defined firewall rules, including the rule that admits WinRM", profile, source)
	}
	return nil
}

// userRightRule defers a deny-network-logon assignment that names a SID in the
// Packer session token, and a network-logon assignment that names none of
// them.
//
// Returns:
//   - *Decision: The decision, or nil when no rule objects to the change.
//
// Author: Lars Krogh
// Version: 1.0.0
func userRightRule(u *baseline.UserRight, ctx Context) *Decision {
	token := ctx.Facts.Session.TokenSIDs()
	switch u.Right {
	case "SeDenyNetworkLogonRight":
		for _, sid := range u.SIDs {
			if token[sid] {
				return deferred("the Packer session token (%s) contains %s; denying network logon to it would reject Packer's next WinRM connection", ctx.Facts.Session.UserName, sid)
			}
		}
	case "SeNetworkLogonRight":
		for _, sid := range u.SIDs {
			if token[sid] {
				return nil
			}
		}
		return deferred("none of the SIDs granted network logon (%s) are in the Packer session token (%s); WinRM requires network logon", strings.Join(u.SIDs, ","), ctx.Facts.Session.UserName)
	}
	return nil
}

type uacModel struct {
	session facts.Session
	values  map[string]uint64
}

// newUACModel starts the remote UAC model from the current registry values,
// using the Windows defaults for values that are absent or not DWORDs.
//
// Author: Lars Krogh
// Version: 1.0.0
func newUACModel(ctx Context) *uacModel {
	m := &uacModel{session: ctx.Facts.Session, values: map[string]uint64{
		keyEnableLUA:                1,
		keyLocalAccountTokenFilter:  0,
		keyFilterAdministratorToken: 0,
	}}
	for key := range m.values {
		sep := strings.LastIndex(key, `\`)
		st, ok := ctx.Current.Get(baseline.RegistryID(key[:sep], key[sep+1:]))
		if !ok || !st.Present || st.Type == nil || *st.Type != baseline.RegDWORD {
			continue
		}
		if v, err := strconv.ParseUint(st.value(), 10, 32); err == nil {
			m.values[key] = v
		}
	}
	return m
}

// filtered reports whether a network logon of the Packer account would get a
// filtered, non-elevated token under the values in v. Domain accounts are
// never filtered.
//
// Author: Lars Krogh
// Version: 1.0.0
func (m *uacModel) filtered(v map[string]uint64) bool {
	if !m.session.IsLocalAccount {
		return false
	}
	if v[keyEnableLUA] == 0 || v[keyLocalAccountTokenFilter] == 1 {
		return false
	}
	return !m.session.IsBuiltinAdmin || v[keyFilterAdministratorToken] == 1
}

// decide evaluates setting key to data against the values accepted so far. It
// defers the change if it would filter the Packer token, and otherwise
// accepts it, so that later UAC settings are judged with it in place.
//
// Author: Lars Krogh
// Version: 1.0.0
func (m *uacModel) decide(key, data string) Decision {
	want, err := strconv.ParseUint(data, 10, 32)
	if err != nil {
		return Decision{Action: ActionApply}
	}
	candidate := map[string]uint64{}
	for k, v := range m.values {
		candidate[k] = v
	}
	candidate[key] = want
	if m.filtered(candidate) {
		return *deferred("Packer authenticates as local account %s; with this value remote UAC would give WinRM sessions a filtered, non-elevated token (EnableLUA=%d, LocalAccountTokenFilterPolicy=%d, FilterAdministratorToken=%d, built-in Administrator=%t)",
			m.session.UserName, candidate[keyEnableLUA], candidate[keyLocalAccountTokenFilter], candidate[keyFilterAdministratorToken], m.session.IsBuiltinAdmin)

	}
	m.values = candidate
	return Decision{Action: ActionApply}
}
