package plan

import (
	"strings"
	"testing"

	"github.com/larskroghdk/packer-plugin-windows-harden/internal/baseline"
	"github.com/larskroghdk/packer-plugin-windows-harden/internal/facts"
)

// ptr returns a pointer to s.
//
// Author: Lars Krogh
// Version: 1.0.0
func ptr(s string) *string { return &s }

// reg returns a registry setting for tests.
//
// Author: Lars Krogh
// Version: 1.0.0
func reg(path, name, typ, data string) baseline.Setting {
	return baseline.Setting{
		ID: baseline.RegistryID(path, name), Kind: baseline.KindRegistry,
		Description: "test", Component: "test",
		Registry: &baseline.RegistryValue{Path: path, Name: name, Type: typ, Data: data},
	}
}

// right returns a user right setting for tests.
//
// Author: Lars Krogh
// Version: 1.0.0
func right(name string, sids ...string) baseline.Setting {
	return baseline.Setting{
		ID: baseline.UserRightID(name), Kind: baseline.KindUserRight,
		Description: "test", Component: "test",
		UserRight: &baseline.UserRight{Right: name, SIDs: sids},
	}
}

// dword returns the state of a present REG_DWORD value.
//
// Author: Lars Krogh
// Version: 1.0.0
func dword(id, v string) State {
	return State{ID: id, Present: true, Type: ptr("REG_DWORD"), Value: ptr(v)}
}

// absent returns the state of a value that does not exist.
//
// Author: Lars Krogh
// Version: 1.0.0
func absent(id string) State { return State{ID: id, Present: false} }

// packerFacts returns the facts of a typical Packer build: the built-in
// Administrator connected over HTTP with Basic authentication, on a Private
// network with the Domain and Private firewall profiles enabled.
//
// Author: Lars Krogh
// Version: 1.0.0
func packerFacts() *facts.Facts {
	idx := 4
	return &facts.Facts{
		OS: facts.OS{Build: 26100, ProductType: 3, Is64BitProcess: true},
		Session: facts.Session{
			UserSID: "S-1-5-21-1-2-3-500", UserName: `WIN\Administrator`,
			IsLocalAccount: true, IsBuiltinAdmin: true, IsElevated: true,
			GroupSIDs: []string{"S-1-5-32-544", "S-1-5-113", "S-1-5-114", "S-1-5-11", "S-1-1-0"},
		},
		WinRM: &facts.WinRM{
			ServiceStatus: "Running", ServiceStart: "Automatic", AllowUnencrypted: true,
			Listeners: []facts.Listener{{Transport: "HTTP", Port: 5985, Enabled: true}},
			Auth:      facts.WinRMAuth{Basic: true, Negotiate: true, Kerberos: true},
		},
		WinRMConnections: []facts.Connection{{LocalPort: 5985, InterfaceIndex: &idx, NetworkCategory: "Private"}},
		Firewall: &facts.Firewall{
			Profiles: []facts.FirewallProfile{
				{Name: "Domain", Enabled: true, DefaultInboundAction: "Block"},
				{Name: "Private", Enabled: true, DefaultInboundAction: "Block"},
				{Name: "Public", Enabled: false, DefaultInboundAction: "NotConfigured"},
			},
			ConnectedNetworkCategories: []string{"Private"},
		},
	}
}

// current returns the given states keyed the way ParseStates keys them.
//
// Author: Lars Krogh
// Version: 1.0.0
func current(states ...State) map[string]State {
	out := map[string]State{}
	for _, s := range states {
		out[strings.ToLower(s.ID)] = s
	}
	return out
}

// TestCompare checks the comparison of each setting kind, including type
// mismatches and case sensitivity.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestCompare(t *testing.T) {
	d := reg(`Software\X`, "V", "REG_DWORD", "1")
	sz := reg(`Software\X`, "S", "REG_SZ", "Block")
	gone := reg(`Software\X`, "G", baseline.RegAbsent, "")
	svc := baseline.Setting{ID: baseline.ServiceID("Svc"), Kind: baseline.KindService, Service: &baseline.Service{Name: "Svc", StartType: 4}}
	audit := baseline.Setting{ID: baseline.AuditPolicyID("g"), Kind: baseline.KindAuditPolicy, AuditPolicy: &baseline.AuditPolicy{SubcategoryGUID: "g", Setting: 3}}
	acct := baseline.Setting{ID: baseline.AccountPolicyID("MinimumPasswordLength"), Kind: baseline.KindAccountPolicy, AccountPolicy: &baseline.AccountPolicy{Name: "MinimumPasswordLength", Value: "14"}}
	ur := right("SeNetworkLogonRight", "S-1-5-11", "S-1-5-32-544")
	tests := []struct {
		name    string
		setting baseline.Setting
		state   State
		want    Comparison
	}{
		{"dword match", d, dword(d.ID, "1"), Match},
		{"dword mismatch", d, dword(d.ID, "0"), Mismatch},
		{"dword absent", d, absent(d.ID), Mismatch},
		{"dword stored as string", d, State{Present: true, Type: ptr("REG_SZ"), Value: ptr("1")}, Mismatch},
		{"sz match", sz, State{Present: true, Type: ptr("REG_SZ"), Value: ptr("Block")}, Match},
		{"sz is case sensitive", sz, State{Present: true, Type: ptr("REG_SZ"), Value: ptr("block")}, Mismatch},
		{"absent wanted and absent", gone, absent(gone.ID), Match},
		{"absent wanted but present", gone, dword(gone.ID, "1"), Mismatch},
		{"service missing", svc, absent(svc.ID), NotApplicable},
		{"service disabled", svc, dword(svc.ID, "4"), Match},
		{"service manual", svc, dword(svc.ID, "3"), Mismatch},
		{"audit match", audit, State{Present: true, Value: ptr("3")}, Match},
		{"audit success only", audit, State{Present: true, Value: ptr("1")}, Mismatch},
		{"account policy match", acct, State{Present: true, Value: ptr("14")}, Match},
		{"account policy missing", acct, State{Present: false}, Mismatch},
		{"user right unordered", ur, State{Present: true, Value: ptr("s-1-5-32-544,S-1-5-11")}, Match},
		{"user right extra sid", ur, State{Present: true, Value: ptr("S-1-1-0,S-1-5-11,S-1-5-32-544")}, Mismatch},
		{"user right empty wanted", right("SeTcbPrivilege"), State{Present: true, Value: ptr("")}, Match},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Compare(tc.setting, tc.state); got != tc.want {
				t.Fatalf("Compare = %v, want %v", got, tc.want)
			}
		})
	}
}

// decide runs Protect for a single setting and returns its decision.
//
// Author: Lars Krogh
// Version: 1.0.0
func decide(t *testing.T, f *facts.Facts, cur map[string]State, s baseline.Setting) Decision {
	t.Helper()
	return Protect([]baseline.Setting{s}, Context{Facts: f, Current: cur})[s.ID]
}

// TestProtectWinRMService checks the Basic authentication and unencrypted
// traffic rules, and that unknown WinRM state defers.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestProtectWinRMService(t *testing.T) {
	basic := reg(`Software\Policies\Microsoft\Windows\WinRM\Service`, "AllowBasic", "REG_DWORD", "0")
	unenc := reg(`Software\Policies\Microsoft\Windows\WinRM\Service`, "AllowUnencryptedTraffic", "REG_DWORD", "0")
	runAs := reg(`Software\Policies\Microsoft\Windows\WinRM\Service`, "DisableRunAs", "REG_DWORD", "1")
	client := reg(`Software\Policies\Microsoft\Windows\WinRM\Client`, "AllowBasic", "REG_DWORD", "0")
	f := packerFacts()
	if d := decide(t, f, nil, basic); d.Action != ActionDefer {
		t.Errorf("AllowBasic with Basic enabled: %v, want defer", d.Action)
	}
	if d := decide(t, f, nil, unenc); d.Action != ActionDefer {
		t.Errorf("AllowUnencryptedTraffic over HTTP: %v, want defer", d.Action)
	}
	if d := decide(t, f, nil, runAs); d.Action != ActionApply {
		t.Errorf("DisableRunAs: %v, want apply", d.Action)
	}
	if d := decide(t, f, nil, client); d.Action != ActionApply {
		t.Errorf("WinRM client AllowBasic: %v, want apply", d.Action)
	}
	f.WinRM.Auth.Basic = false
	f.WinRM.Listeners = append(f.WinRM.Listeners, facts.Listener{Transport: "HTTPS", Port: 5986, Enabled: true})
	f.WinRMConnections[0].LocalPort = 5986
	if d := decide(t, f, nil, basic); d.Action != ActionApply {
		t.Errorf("AllowBasic with Basic disabled: %v, want apply", d.Action)
	}
	if d := decide(t, f, nil, unenc); d.Action != ActionApply {
		t.Errorf("AllowUnencryptedTraffic over HTTPS: %v, want apply", d.Action)
	}
	f.WinRM = nil
	if d := decide(t, f, nil, basic); d.Action != ActionDefer {
		t.Errorf("AllowBasic with unknown WinRM: %v, want defer", d.Action)
	}
}

// TestProtectRemoteUAC checks the combined remote UAC decision for the
// built-in Administrator, another local administrator and a domain account.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestProtectRemoteUAC(t *testing.T) {
	latfp := reg(`Software\Microsoft\Windows\CurrentVersion\Policies\System`, "LocalAccountTokenFilterPolicy", "REG_DWORD", "0")
	fat := reg(`Software\Microsoft\Windows\CurrentVersion\Policies\System`, "FilterAdministratorToken", "REG_DWORD", "1")
	lua := reg(`Software\Microsoft\Windows\CurrentVersion\Policies\System`, "EnableLUA", "REG_DWORD", "1")
	t.Run("built-in administrator, domain-joined profile", func(t *testing.T) {
		ds := Protect([]baseline.Setting{latfp, fat, lua}, Context{Facts: packerFacts(), Current: current(absent(latfp.ID), absent(fat.ID), absent(lua.ID))})
		if ds[latfp.ID].Action != ActionApply {
			t.Errorf("LATFP=0: %v, want apply", ds[latfp.ID].Action)
		}
		if ds[fat.ID].Action != ActionDefer {
			t.Errorf("FilterAdministratorToken=1: %v, want defer", ds[fat.ID].Action)
		}
		if ds[lua.ID].Action != ActionApply {
			t.Errorf("EnableLUA=1: %v, want apply", ds[lua.ID].Action)
		}
	})
	t.Run("built-in administrator, non-domain-joined profile", func(t *testing.T) {
		latfp1 := reg(latfp.Registry.Path, latfp.Registry.Name, "REG_DWORD", "1")
		ds := Protect([]baseline.Setting{latfp1, fat}, Context{Facts: packerFacts(), Current: current(absent(latfp.ID), absent(fat.ID))})
		if ds[latfp1.ID].Action != ActionApply || ds[fat.ID].Action != ActionApply {
			t.Errorf("got %v / %v, want apply / apply", ds[latfp1.ID].Action, ds[fat.ID].Action)
		}
	})
	t.Run("other local administrator", func(t *testing.T) {
		f := packerFacts()
		f.Session.UserSID, f.Session.UserName, f.Session.IsBuiltinAdmin = "S-1-5-21-1-2-3-1001", `WIN\packer`, false
		if d := decide(t, f, current(dword(latfp.ID, "1")), latfp); d.Action != ActionDefer {
			t.Errorf("LATFP=0: %v, want defer", d.Action)
		}
		if d := decide(t, f, current(dword(lua.ID, "0"), absent(latfp.ID)), lua); d.Action != ActionDefer {
			t.Errorf("EnableLUA=1: %v, want defer", d.Action)
		}
		if d := decide(t, f, current(dword(lua.ID, "0"), dword(latfp.ID, "1")), lua); d.Action != ActionApply {
			t.Errorf("EnableLUA=1 with LATFP=1: %v, want apply", d.Action)
		}
	})
	t.Run("domain account", func(t *testing.T) {
		f := packerFacts()
		f.Session.IsLocalAccount, f.Session.IsBuiltinAdmin = false, false
		ds := Protect([]baseline.Setting{latfp, fat}, Context{Facts: f, Current: current()})
		if ds[latfp.ID].Action != ActionApply || ds[fat.ID].Action != ActionApply {
			t.Errorf("domain account: got %v / %v, want apply / apply", ds[latfp.ID].Action, ds[fat.ID].Action)
		}
	})
}

// TestProtectUserRights checks the network logon rules against the session
// token.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestProtectUserRights(t *testing.T) {
	f := packerFacts()
	deny := right("SeDenyNetworkLogonRight", "S-1-5-114", "S-1-5-32-546")
	if d := decide(t, f, nil, deny); d.Action != ActionDefer {
		t.Errorf("deny network logon to local admins: %v, want defer", d.Action)
	}
	if d := decide(t, f, nil, right("SeDenyNetworkLogonRight", "S-1-5-32-546")); d.Action != ActionApply {
		t.Errorf("deny network logon to guests: %v, want apply", d.Action)
	}
	if d := decide(t, f, nil, right("SeNetworkLogonRight", "S-1-5-32-544", "S-1-5-11")); d.Action != ActionApply {
		t.Errorf("network logon for admins: %v, want apply", d.Action)
	}
	if d := decide(t, f, nil, right("SeNetworkLogonRight", "S-1-5-32-555")); d.Action != ActionDefer {
		t.Errorf("network logon without the session's groups: %v, want defer", d.Action)
	}
	if d := decide(t, f, nil, right("SeDenyRemoteInteractiveLogonRight", "S-1-5-114")); d.Action != ActionApply {
		t.Errorf("RDP deny right does not affect WinRM: %v, want apply", d.Action)
	}
}

// TestProtectFirewall checks that only the profile carrying the WinRM
// connection is protected, and that missing connection data protects every
// connected profile.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestProtectFirewall(t *testing.T) {
	path := func(p string) string { return `Software\Policies\Microsoft\WindowsFirewall\` + p }
	enablePrivate := reg(path("PrivateProfile"), "EnableFirewall", "REG_DWORD", "1")
	enablePublic := reg(path("PublicProfile"), "EnableFirewall", "REG_DWORD", "1")
	blockPrivate := reg(path("PrivateProfile"), "DefaultInboundAction", "REG_DWORD", "1")
	mergePublic := reg(path("PublicProfile"), "AllowLocalPolicyMerge", "REG_DWORD", "0")
	logging := reg(path(`PrivateProfile\Logging`), "LogDroppedPackets", "REG_DWORD", "1")
	f := packerFacts()
	for _, s := range []baseline.Setting{enablePrivate, blockPrivate, enablePublic, mergePublic, logging} {
		if d := decide(t, f, nil, s); d.Action != ActionApply {
			t.Errorf("%s on Private network with firewall on: %v, want apply", s.ID, d.Action)
		}
	}
	f.WinRMConnections[0].NetworkCategory = "Public"
	if d := decide(t, f, nil, enablePublic); d.Action != ActionDefer {
		t.Errorf("enable disabled Public profile: %v, want defer", d.Action)
	}
	if d := decide(t, f, nil, mergePublic); d.Action != ActionDefer {
		t.Errorf("disable local rule merge on Public: %v, want defer", d.Action)
	}
	if d := decide(t, f, nil, enablePrivate); d.Action != ActionApply {
		t.Errorf("enable Private profile when WinRM uses Public: %v, want apply", d.Action)
	}
	f.WinRMConnections = nil
	f.Firewall.ConnectedNetworkCategories = []string{"Public", "Private"}
	if d := decide(t, f, nil, enablePublic); d.Action != ActionDefer {
		t.Errorf("enable Public without connection data: %v, want defer", d.Action)
	}
}

// TestProtectGuardAndBlankPassword checks that the NTLM settings are guarded
// only when Negotiate is enabled, and that LimitBlankPasswordUse is deferred
// only for an empty password.
//
// Author: Lars Krogh
// Version: 1.1.0
func TestProtectGuardAndBlankPassword(t *testing.T) {
	lm := reg(`System\CurrentControlSet\Control\Lsa`, "LmCompatibilityLevel", "REG_DWORD", "5")
	blank := reg(`System\CurrentControlSet\Control\Lsa`, "LimitBlankPasswordUse", "REG_DWORD", "1")
	f := packerFacts()
	if d := decide(t, f, nil, lm); d.Action != ActionGuard {
		t.Errorf("LmCompatibilityLevel: %v, want guard", d.Action)
	}
	f.WinRM.Auth.Negotiate = false
	if d := decide(t, f, nil, lm); d.Action != ActionApply {
		t.Errorf("LmCompatibilityLevel without Negotiate: %v, want apply", d.Action)
	}
	f.WinRM = nil
	if d := decide(t, f, nil, lm); d.Action != ActionDefer {
		t.Errorf("LmCompatibilityLevel without WinRM facts: %v, want defer", d.Action)
	}
	if d := decide(t, f, nil, blank); d.Action != ActionApply {
		t.Errorf("LimitBlankPasswordUse with a password: %v, want apply", d.Action)
	}
	d := Protect([]baseline.Setting{blank}, Context{Facts: f, EmptyPassword: true})[blank.ID]
	if d.Action != ActionDefer {
		t.Errorf("LimitBlankPasswordUse with an empty password: %v, want defer", d.Action)
	}
}

// TestProtectUnknownUAC distinguishes absent Windows defaults from missing,
// failed, or mistyped inspections of FilterAdministratorToken.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestProtectUnknownUAC(t *testing.T) {
	lua := reg(`Software\Microsoft\Windows\CurrentVersion\Policies\System`, "EnableLUA", baseline.RegDWORD, "1")
	filter := baseline.RegistryID(lua.Registry.Path, "FilterAdministratorToken")
	for _, st := range []State{
		{ID: filter, Error: ptr("access denied")},
		{ID: filter, Present: true, Type: ptr(baseline.RegSZ), Value: ptr("0")},
		{ID: filter, Present: true, Type: ptr(baseline.RegDWORD), Value: ptr("invalid")},
	} {
		if d := decide(t, packerFacts(), current(st), lua); d.Action != ActionDefer {
			t.Fatalf("unknown UAC state %+v: %v", st, d)
		}
	}
	if d := decide(t, packerFacts(), current(absent(filter)), lua); d.Action != ActionApply {
		t.Fatalf("absent UAC value: %v", d)
	}
	if d := decide(t, packerFacts(), nil, lua); d.Action != ActionDefer {
		t.Fatalf("missing UAC state: %v", d)
	}
}

// TestCompareRejectsIncompleteValues prevents missing or errored values from
// matching empty strings or empty user-right assignments.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestCompareRejectsIncompleteValues(t *testing.T) {
	setting := reg(`Software\X`, "Empty", baseline.RegSZ, "")
	if got := Compare(setting, State{Present: true, Type: ptr(baseline.RegSZ)}); got != Mismatch {
		t.Fatalf("missing value = %v", got)
	}
	if got := Compare(right("SeNetworkLogonRight"), State{Present: true}); got != Mismatch {
		t.Fatalf("missing right assignment = %v", got)
	}
	if got := Compare(reg(`Software\X`, "Gone", baseline.RegAbsent, ""), State{Error: ptr("denied")}); got != Mismatch {
		t.Fatalf("unreadable value = %v", got)
	}
	if got := Compare(right("SeNetworkLogonRight", "S-1-5-32-544"), State{Present: true, Value: ptr("S-1-5-32-544,S-1-5-32-544")}); got != Match {
		t.Fatalf("duplicate SID in set = %v", got)
	}
}

// TestResultParsersRejectAmbiguousIDs ensures duplicate and empty IDs cannot
// replace failed results with successful ones during decoding.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestResultParsersRejectAmbiguousIDs(t *testing.T) {
	for _, data := range []string{`{"items":[{"id":"A"},{"id":"a"}]}`, `{"items":[{}]}`} {
		if _, err := ParseStates([]byte(data)); err == nil {
			t.Fatalf("accepted inspect response %s", data)
		}
		if _, err := ParseApplyResults([]byte(data)); err == nil {
			t.Fatalf("accepted apply response %s", data)
		}
	}
}

// TestBuildAndVerify checks every status Build assigns, and the transitions
// made by Verify and Reverify.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestBuildAndVerify(t *testing.T) {
	ok := reg(`Software\A`, "Compliant", "REG_DWORD", "1")
	change := reg(`Software\A`, "Change", "REG_DWORD", "1")
	skipped := reg(`Software\Skip`, "Me", "REG_DWORD", "1")
	broken := reg(`Software\A`, "Broken", "REG_DWORD", "1")
	missing := reg(`Software\A`, "Missing", "REG_DWORD", "1")
	deferredSetting := reg(`Software\Policies\Microsoft\Windows\WinRM\Service`, "AllowBasic", "REG_DWORD", "0")
	svc := baseline.Setting{ID: baseline.ServiceID("Xbox"), Kind: baseline.KindService, Description: "x", Component: "x", Service: &baseline.Service{Name: "Xbox", StartType: 4}}
	cur := current(dword(ok.ID, "1"), dword(change.ID, "0"), dword(skipped.ID, "0"),
		State{ID: broken.ID, Error: ptr("access denied")}, dword(deferredSetting.ID, "1"), absent(svc.ID))
	p := Build([]baseline.Setting{ok, change, skipped, broken, missing, deferredSetting, svc},
		[]string{`registry:HKLM\software\skip\*`}, Context{Facts: packerFacts(), Current: cur})
	want := map[string]Status{
		ok.ID: StatusCompliant, change.ID: StatusPending, skipped.ID: StatusSkipped,
		broken.ID: StatusFailed, missing.ID: StatusFailed, deferredSetting.ID: StatusDeferred,
		svc.ID: StatusNotApplicable,
	}
	for _, it := range p.Items {
		if it.Status != want[it.Setting.ID] {
			t.Errorf("%s: status %s, want %s (%s)", it.Setting.ID, it.Status, want[it.Setting.ID], it.Reason)
		}
	}
	pending := p.Pending(ActionApply)
	if len(pending) != 1 || pending[0].Setting.ID != change.ID {
		t.Fatalf("pending apply = %v", pending)
	}
	results := map[string]ApplyResult{strings.ToLower(change.ID): {ID: change.ID, OK: true}}
	Verify(pending, results, current(dword(change.ID, "0")))
	if pending[0].Status != StatusFailed {
		t.Fatalf("unverified change: status %s, want failed", pending[0].Status)
	}
	Verify(pending, results, current(dword(change.ID, "1")))
	if pending[0].Status != StatusHardened {
		t.Fatalf("verified change: status %s, want hardened", pending[0].Status)
	}
	Verify(pending, map[string]ApplyResult{strings.ToLower(change.ID): {OK: false, Error: ptr("boom")}}, current(dword(change.ID, "1")))
	if pending[0].Status != StatusFailed {
		t.Fatalf("failed apply: status %s, want failed even if the value matches", pending[0].Status)
	}
	pending[0].Status = StatusHardened
	Reverify(p.Items, current(dword(ok.ID, "1"), dword(change.ID, "0")), "after restart")
	if pending[0].Status != StatusFailed || !strings.Contains(pending[0].Reason, "after restart") {
		t.Fatalf("reverted after restart: %s (%s)", pending[0].Status, pending[0].Reason)
	}
}

// TestMatchPattern checks case-insensitive, whole-ID wildcard matching.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern, id string
		want        bool
	}{
		{`registry:HKLM\Software\Policies\Microsoft\Windows Defender\*`, `registry:HKLM\Software\Policies\Microsoft\Windows Defender\Spynet\SpynetReporting`, true},
		{`REGISTRY:hklm\SOFTWARE\x`, `registry:HKLM\Software\X`, true},
		{`user_right:SeDeny*Right`, `user_right:SeDenyNetworkLogonRight`, true},
		{`user_right:SeDeny*Right`, `user_right:SeDebugPrivilege`, false},
		{`audit_policy:*`, `registry:HKLM\x`, false},
		{`a*a`, `a`, false},
		{`*\AllowBasic`, `registry:HKLM\Software\Policies\Microsoft\Windows\WinRM\Client\AllowBasic`, true},
	}
	for _, tc := range tests {
		if got := MatchPattern(tc.pattern, tc.id); got != tc.want {
			t.Errorf("MatchPattern(%q, %q) = %t, want %t", tc.pattern, tc.id, got, tc.want)
		}
	}
}
