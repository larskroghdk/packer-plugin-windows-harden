// Package facts decodes what the guest reports about itself and derives the
// facts the plan depends on: OS lifecycle status, active firewall profiles,
// the WinRM transport and WinRM regressions.
package facts

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	ProductTypeWorkstation      = 1
	ProductTypeDomainController = 2
	ProductTypeServer           = 3
)

// Facts is the result of the guest Detect action. Pointer fields are nil when
// the guest could not collect that part. Errors says why, and rules that need
// the missing part must fail closed.
type Facts struct {
	OS               OS           `json:"os"`
	Domain           *Domain      `json:"domain"`
	Roles            []string     `json:"roles"`
	Session          Session      `json:"session"`
	WinRM            *WinRM       `json:"winrm"`
	WinRMConnections []Connection `json:"winrm_connections"`
	Firewall         *Firewall    `json:"firewall"`
	Errors           []string     `json:"errors"`
}

// OS identifies the Windows release, edition and process architecture.
type OS struct {
	ProductName      string `json:"product_name"`
	EditionID        string `json:"edition_id"`
	InstallationType string `json:"installation_type"`
	DisplayVersion   string `json:"display_version"`
	Major            int    `json:"major"`
	Minor            int    `json:"minor"`
	Build            int    `json:"build"`
	UBR              int    `json:"ubr"`
	ProductType      int    `json:"product_type"`
	Caption          string `json:"caption"`
	Architecture     string `json:"architecture"`
	Is64BitProcess   bool   `json:"is_64bit_process"`
}

// Domain describes the machine's domain membership.
type Domain struct {
	PartOfDomain bool   `json:"part_of_domain"`
	Domain       string `json:"domain"`
	DomainRole   int    `json:"domain_role"`
}

// Session describes the account and token of the Packer WinRM session.
type Session struct {
	UserSID            string   `json:"user_sid"`
	UserName           string   `json:"user_name"`
	AuthenticationType string   `json:"authentication_type"`
	IsLocalAccount     bool     `json:"is_local_account"`
	IsBuiltinAdmin     bool     `json:"is_builtin_admin"`
	IsElevated         bool     `json:"is_elevated"`
	GroupSIDs          []string `json:"group_sids"`
}

// TokenSIDs returns the user SID and group SIDs of the session token as an
// upper-case set.
//
// Author: Lars Krogh
// Version: 1.0.0
func (s Session) TokenSIDs() map[string]bool {
	out := make(map[string]bool, len(s.GroupSIDs)+1)
	out[strings.ToUpper(s.UserSID)] = true
	for _, g := range s.GroupSIDs {
		out[strings.ToUpper(g)] = true
	}
	return out
}

// WinRM is the WinRM service configuration.
type WinRM struct {
	ServiceStatus    string     `json:"service_status"`
	ServiceStart     string     `json:"service_start"`
	Listeners        []Listener `json:"listeners"`
	AllowUnencrypted bool       `json:"allow_unencrypted"`
	Auth             WinRMAuth  `json:"auth"`
}

// Listener is a configured WinRM listener.
type Listener struct {
	Transport             string `json:"transport"`
	Address               string `json:"address"`
	Port                  int    `json:"port"`
	Enabled               bool   `json:"enabled"`
	Hostname              string `json:"hostname"`
	CertificateThumbprint string `json:"certificate_thumbprint"`
}

// WinRMAuth lists the authentication methods enabled on the WinRM service.
type WinRMAuth struct {
	Basic       bool `json:"basic"`
	Kerberos    bool `json:"kerberos"`
	Negotiate   bool `json:"negotiate"`
	Certificate bool `json:"certificate"`
	CredSSP     bool `json:"credssp"`
}

// Connection is an established TCP connection to a WinRM listener port.
type Connection struct {
	LocalAddress    string `json:"local_address"`
	LocalPort       int    `json:"local_port"`
	RemoteAddress   string `json:"remote_address"`
	InterfaceIndex  *int   `json:"interface_index"`
	NetworkCategory string `json:"network_category"`
}

// Firewall is the state of the Windows Firewall profiles and the categories of
// the connected networks.
type Firewall struct {
	Profiles                   []FirewallProfile `json:"profiles"`
	ConnectedNetworkCategories []string          `json:"connected_network_categories"`
}

// FirewallProfile is the effective state of one Windows Firewall profile.
type FirewallProfile struct {
	Name                 string `json:"name"`
	Enabled              bool   `json:"enabled"`
	DefaultInboundAction string `json:"default_inbound_action"`
	AllowLocalRules      string `json:"allow_local_rules"`
}

// Parse decodes the Detect result.
//
// Returns:
//   - *Facts: The decoded facts.
//   - error: The data is not valid JSON, or the build or product type is
//     missing, which means the guest could not identify the OS.
//
// Author: Lars Krogh
// Version: 1.0.0
func Parse(data []byte) (*Facts, error) {
	var f Facts
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("decode system facts: %w", err)
	}
	if f.OS.Build == 0 || f.OS.ProductType == 0 {
		return nil, fmt.Errorf("system facts are incomplete: build %d, product type %d", f.OS.Build, f.OS.ProductType)
	}
	return &f, nil
}

// Version returns the full version as major.minor.build.revision.
//
// Author: Lars Krogh
// Version: 1.0.0
func (o OS) Version() string { return fmt.Sprintf("%d.%d.%d.%d", o.Major, o.Minor, o.Build, o.UBR) }

// ProductTypeName returns the product type as used by baseline applicability:
// "workstation", "domain_controller", "server", or "unknown(n)".
//
// Author: Lars Krogh
// Version: 1.0.0
func (o OS) ProductTypeName() string {
	switch o.ProductType {
	case ProductTypeWorkstation:
		return "workstation"
	case ProductTypeDomainController:
		return "domain_controller"
	case ProductTypeServer:
		return "server"
	}
	return fmt.Sprintf("unknown(%d)", o.ProductType)
}

// firewallProfileForCategory returns the firewall profile that applies to a
// network category, or "" for an unknown category.
//
// Author: Lars Krogh
// Version: 1.0.0
func firewallProfileForCategory(category string) string {
	switch strings.ToLower(category) {
	case "public":
		return "Public"
	case "private":
		return "Private"
	case "domainauthenticated":
		return "Domain"
	}
	return ""
}

// ActiveFirewallProfiles returns the firewall profiles that may carry the
// Packer WinRM connection. It prefers the networks of the WinRM connections,
// falls back to all connected networks, and returns every profile when neither
// is known.
//
// Returns:
//   - profiles: The set of profile names (Domain, Private, Public).
//   - fromConnections: Whether the set comes from the WinRM connections.
//
// Author: Lars Krogh
// Version: 1.0.0
func (f *Facts) ActiveFirewallProfiles() (profiles map[string]bool, fromConnections bool) {
	out := map[string]bool{}
	for _, c := range f.WinRMConnections {
		if p := firewallProfileForCategory(c.NetworkCategory); p != "" {
			out[p] = true
		}
	}
	if len(out) > 0 {
		return out, true
	}
	if f.Firewall != nil {
		for _, c := range f.Firewall.ConnectedNetworkCategories {
			if p := firewallProfileForCategory(c); p != "" {
				out[p] = true
			}
		}
	}
	if len(out) == 0 {
		out = map[string]bool{"Domain": true, "Private": true, "Public": true}
	}
	return out, false
}

// UsesUnencryptedTransport reports whether Packer is, or may be, connected
// over an HTTP listener. It returns true when the WinRM configuration or the
// connections are unknown, or when any connection uses a non-HTTPS port.
//
// Author: Lars Krogh
// Version: 1.0.0
func (f *Facts) UsesUnencryptedTransport() bool {
	if f.WinRM == nil || len(f.WinRMConnections) == 0 {
		return true
	}
	https := map[int]bool{}
	for _, l := range f.WinRM.Listeners {
		if strings.EqualFold(l.Transport, "HTTPS") {
			https[l.Port] = true
		}
	}
	for _, c := range f.WinRMConnections {
		if !https[c.LocalPort] {
			return true
		}
	}
	return false
}
