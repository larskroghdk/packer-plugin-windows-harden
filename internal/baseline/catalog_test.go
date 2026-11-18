package baseline

import (
	"slices"
	"strings"
	"testing"
)

// load returns the embedded catalog and fails the test if it does not load.
//
// Author: Lars Krogh
// Version: 1.0.0
func load(t *testing.T) *Catalog {
	t.Helper()
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return c
}

// TestCatalogSelect checks the baseline chosen for every supported release and
// that unsupported releases and domain controllers get none.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestCatalogSelect(t *testing.T) {
	c := load(t)
	tests := []struct {
		productType string
		build       int
		want        string
	}{
		{"server", 17763, "ws2019-member"},
		{"server", 20348, "ws2022"},
		{"server", 26100, "ws2025-2602"},
		{"workstation", 26100, "win11-24h2"},
		{"workstation", 26200, "win11-25h2"},
		{"workstation", 26300, "win11-26h2"},
	}
	for _, tc := range tests {
		b, err := c.Select(tc.productType, tc.build)
		if err != nil {
			t.Errorf("Select(%s, %d): %v", tc.productType, tc.build, err)
			continue
		}
		if b.ID != tc.want {
			t.Errorf("Select(%s, %d) = %s, want %s", tc.productType, tc.build, b.ID, tc.want)
		}
	}
	for _, unsupported := range []struct {
		productType string
		build       int
	}{{"server", 14393}, {"workstation", 19045}, {"workstation", 22631}, {"domain_controller", 26100}} {
		if b, err := c.Select(unsupported.productType, unsupported.build); err == nil {
			t.Errorf("Select(%s, %d) = %s, want error", unsupported.productType, unsupported.build, b.ID)
		}
	}
}

// TestEveryBaselineIsTraceable checks that every baseline comes from a pinned
// Microsoft download and that its settings look complete.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestEveryBaselineIsTraceable(t *testing.T) {
	for _, b := range load(t).Baselines() {
		if !strings.HasPrefix(b.Source.URL, "https://download.microsoft.com/") || len(b.Source.SHA256) != 64 {
			t.Errorf("%s: source %+v is not a pinned Microsoft download", b.ID, b.Source)
		}
		if len(b.Settings) < 200 {
			t.Errorf("%s: only %d settings; the generated data looks incomplete", b.ID, len(b.Settings))
		}
		for _, s := range b.Settings {
			if s.Description == "" || s.Component == "" || s.ExpectedValue() == "" {
				t.Errorf("%s: setting %s lacks description, component or expected value", b.ID, s.ID)
			}
		}
	}
}

// TestNonDomainJoinedProfile checks that the non-domain-joined profile replaces
// settings rather than adding them, and that it relaxes the remote UAC and
// network logon settings that Microsoft's delta changes.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestNonDomainJoinedProfile(t *testing.T) {
	for _, b := range load(t).Baselines() {
		domain := index(b.SettingsFor(ProfileDomainJoined))
		local := index(b.SettingsFor(ProfileNonDomainJoined))
		if len(domain) != len(local) {
			t.Errorf("%s: profiles differ in size: %d vs %d", b.ID, len(domain), len(local))
		}
		latfp := RegistryID(`Software\Microsoft\Windows\CurrentVersion\Policies\System`, "LocalAccountTokenFilterPolicy")
		if got := local[strings.ToLower(latfp)].Registry.Data; got != "1" {
			t.Errorf("%s: non-domain-joined LocalAccountTokenFilterPolicy = %s, want 1", b.ID, got)
		}
		if got := domain[strings.ToLower(latfp)].Registry.Data; got != "0" {
			t.Errorf("%s: domain-joined LocalAccountTokenFilterPolicy = %s, want 0", b.ID, got)
		}
		deny := strings.ToLower(UserRightID("SeDenyNetworkLogonRight"))
		if n := len(local[deny].UserRight.SIDs); n != 0 {
			t.Errorf("%s: non-domain-joined SeDenyNetworkLogonRight has %d SIDs, want none", b.ID, n)
		}
		sids := domain[deny].UserRight.SIDs
		if !slices.Contains(sids, "S-1-5-114") && !slices.Contains(sids, "S-1-5-113") {
			t.Errorf("%s: domain-joined SeDenyNetworkLogonRight denies no local accounts: %v", b.ID, sids)
		}
	}
}

// TestValidateRejectsInconsistentSettings checks that Validate rejects a wrong
// ID, an unsupported registry type, two payloads, invalid DWORD data and an
// invalid service start type.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestValidateRejectsInconsistentSettings(t *testing.T) {
	good := Setting{
		ID: RegistryID(`Software\X`, "V"), Kind: KindRegistry, Description: "d", Component: "c",
		Registry: &RegistryValue{Path: `Software\X`, Name: "V", Type: RegDWORD, Data: "1"},
	}
	if err := good.Validate(); err != nil {
		t.Fatalf("valid setting rejected: %v", err)
	}
	wrongID := good
	wrongID.ID = "registry:HKLM\\Software\\Y\\V"
	badType := good
	badType.Registry = &RegistryValue{Path: `Software\X`, Name: "V", Type: "REG_BINARY", Data: "00"}
	twoPayloads := good
	twoPayloads.Service = &Service{Name: "x", StartType: 4}
	badDWORD := good
	badDWORD.Registry = &RegistryValue{Path: `Software\X`, Name: "V", Type: RegDWORD, Data: "-1"}
	badStartType := Setting{
		ID: ServiceID("Svc"), Kind: KindService, Description: "d", Component: "c",
		Service: &Service{Name: "Svc", StartType: 1},
	}
	tests := map[string]Setting{
		"wrong id": wrongID, "bad type": badType, "two payloads": twoPayloads,
		"negative dword": badDWORD, "boot start type": badStartType,
	}
	for name, s := range tests {
		if err := s.Validate(); err == nil {
			t.Errorf("%s: expected a validation error", name)
		}
	}
}

// TestValidateRejectsOrphanOverrides checks that an override must replace a
// setting the baseline contains.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestValidateRejectsOrphanOverrides(t *testing.T) {
	setting := func(name string) Setting {
		return Setting{
			ID: RegistryID(`Software\X`, name), Kind: KindRegistry, Description: "d", Component: "c",
			Registry: &RegistryValue{Path: `Software\X`, Name: name, Type: RegDWORD, Data: "1"},
		}
	}
	b := Baseline{
		ID: "test", Name: "n", Version: "v",
		Source:    Source{URL: "u", SHA256: "s"},
		AppliesTo: Applicability{ProductType: "server", Builds: []int{1}},
		Settings:  []Setting{setting("A")},
	}
	b.NonDomainJoinedOverrides = []Setting{setting("A")}
	if err := b.Validate(); err != nil {
		t.Fatalf("override of an existing setting rejected: %v", err)
	}
	b.NonDomainJoinedOverrides = []Setting{setting("B")}
	if err := b.Validate(); err == nil {
		t.Error("override of a setting the baseline does not contain was accepted")
	}
}

// index maps settings by lower-case ID.
//
// Author: Lars Krogh
// Version: 1.0.0
func index(settings []Setting) map[string]Setting {
	out := map[string]Setting{}
	for _, s := range settings {
		out[strings.ToLower(s.ID)] = s
	}
	return out
}
