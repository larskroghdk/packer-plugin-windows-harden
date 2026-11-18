package facts

import (
	"os"
	"testing"
	"time"
)

// TestParseSample decodes a captured Server Core Detect result and checks the
// derived OS, session, firewall and transport facts.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestParseSample(t *testing.T) {
	data, err := os.ReadFile("testdata/ws2025-core.json")
	if err != nil {
		t.Fatal(err)
	}
	f, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if f.OS.InstallationType != "Server Core" || f.OS.ProductTypeName() != "server" || f.OS.Version() != "10.0.26100.4652" {
		t.Errorf("unexpected OS: %+v", f.OS)
	}
	if !f.Session.IsBuiltinAdmin || !f.Session.TokenSIDs()["S-1-5-114"] {
		t.Errorf("unexpected session: %+v", f.Session)
	}
	profiles, fromConnections := f.ActiveFirewallProfiles()
	if !fromConnections || !profiles["Public"] || len(profiles) != 1 {
		t.Errorf("ActiveFirewallProfiles = %v, %t", profiles, fromConnections)
	}
	if !f.UsesUnencryptedTransport() {
		t.Errorf("connection on 5985 must count as unencrypted")
	}
}

// TestParseRejectsIncompleteFacts checks that facts without a build, and data
// that is not JSON, are rejected.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestParseRejectsIncompleteFacts(t *testing.T) {
	if _, err := Parse([]byte(`{"os":{"build":0}}`)); err == nil {
		t.Fatal("expected an error")
	}
	if _, err := Parse([]byte(`not json`)); err == nil {
		t.Fatal("expected an error")
	}
}

// TestUsesUnencryptedTransport checks HTTPS-only, mixed and unknown
// connections.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestUsesUnencryptedTransport(t *testing.T) {
	f := &Facts{
		WinRM: &WinRM{Listeners: []Listener{
			{Transport: "HTTP", Port: 5985, Enabled: true},
			{Transport: "HTTPS", Port: 5986, Enabled: true},
		}},
		WinRMConnections: []Connection{{LocalPort: 5986}},
	}
	if f.UsesUnencryptedTransport() {
		t.Error("HTTPS-only connections reported as unencrypted")
	}
	f.WinRMConnections = append(f.WinRMConnections, Connection{LocalPort: 5985})
	if !f.UsesUnencryptedTransport() {
		t.Error("HTTP connection not detected")
	}
	f.WinRMConnections = nil
	if !f.UsesUnencryptedTransport() {
		t.Error("unknown connections must be treated as unencrypted")
	}
}

// TestClassify checks the lifecycle status of each supported, retired and
// rejected release, including the last day of support.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestClassify(t *testing.T) {
	now := date("2026-11-16")
	tests := []struct {
		name string
		os   OS
		want SupportStatus
	}{
		{"server 2025", OS{ProductType: 3, Build: 26100}, StatusSupported},
		{"server 2016 in support", OS{ProductType: 3, Build: 14393}, StatusSupported},
		{"unknown server build", OS{ProductType: 3, Build: 9600}, StatusUnknownRelease},
		{"domain controller", OS{ProductType: 2, Build: 26100}, StatusUnsupportedRole},
		{"win11 24h2 enterprise", OS{ProductType: 1, Build: 26100, EditionID: "Enterprise"}, StatusSupported},
		{"win11 24h2 pro", OS{ProductType: 1, Build: 26100, EditionID: "Professional"}, StatusEndOfSupport},
		{"win11 25h2 pro", OS{ProductType: 1, Build: 26200, EditionID: "Professional"}, StatusSupported},
		{"win11 23h2 enterprise", OS{ProductType: 1, Build: 22631, EditionID: "Enterprise"}, StatusEndOfSupport},
		{"win10 22h2", OS{ProductType: 1, Build: 19045, EditionID: "Enterprise"}, StatusEndOfSupport},
		{"ltsc 2024", OS{ProductType: 1, Build: 26100, EditionID: "EnterpriseS"}, StatusUnsupportedEdition},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.os, now)
			if got.Status != tc.want {
				t.Fatalf("Classify = %s (%s), want %s", got.Status, got.Reason, tc.want)
			}
			if got.Status != StatusSupported && got.Reason == "" {
				t.Fatal("unsupported classification without a reason")
			}
		})
	}
	if got := Classify(OS{ProductType: 1, Build: 26100, EditionID: "Enterprise"}, date("2027-10-12").Add(12*time.Hour)); got.Status != StatusSupported {
		t.Errorf("last day of support: %s", got.Status)
	}
}

// TestWinRMRegressions checks that an unchanged or extended configuration has
// no regressions and that each weakened property is reported.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestWinRMRegressions(t *testing.T) {
	before := &WinRM{
		ServiceStatus: "Running", ServiceStart: "Automatic", AllowUnencrypted: true,
		Listeners: []Listener{{Transport: "HTTP", Port: 5985, Enabled: true}},
		Auth:      WinRMAuth{Basic: true, Negotiate: true},
	}
	same := *before
	if p := WinRMRegressions(before, &Probe{WinRM: &same, Session: Session{IsElevated: true}}); len(p) != 0 {
		t.Fatalf("unexpected regressions: %v", p)
	}
	worse := *before
	worse.Auth.Basic = false
	worse.AllowUnencrypted = false
	worse.Listeners = []Listener{{Transport: "HTTP", Port: 5985, Enabled: false}}
	worse.ServiceStatus = "Stopped"
	p := WinRMRegressions(before, &Probe{WinRM: &worse, Session: Session{IsElevated: false}})
	if len(p) != 5 {
		t.Fatalf("expected 5 regressions, got %d: %v", len(p), p)
	}
	better := *before
	better.Auth.Kerberos = true
	better.Listeners = append(better.Listeners, Listener{Transport: "HTTPS", Port: 5986, Enabled: true})
	if p := WinRMRegressions(before, &Probe{WinRM: &better, Session: Session{IsElevated: true}}); len(p) != 0 {
		t.Fatalf("unexpected regressions: %v", p)
	}
}
