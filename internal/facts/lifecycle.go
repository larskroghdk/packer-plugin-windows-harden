package facts

import (
	"fmt"
	"strings"
	"time"
)

// Release is a Windows release and the last day it is supported.
type Release struct {
	Name         string
	EndOfSupport time.Time
}

// SupportStatus is the outcome of classifying the guest OS.
type SupportStatus string

const (
	StatusSupported          SupportStatus = "supported"
	StatusEndOfSupport       SupportStatus = "end_of_support"
	StatusUnknownRelease     SupportStatus = "unknown_release"
	StatusUnsupportedRole    SupportStatus = "unsupported_role"
	StatusUnsupportedEdition SupportStatus = "unsupported_edition"
)

// Classification is the support status of the guest OS. Reason explains any
// status other than StatusSupported.
type Classification struct {
	Release Release
	Status  SupportStatus
	Reason  string
}

// date parses a YYYY-MM-DD literal. It panics on an invalid literal, which can
// only happen in the release tables.
//
// Author: Lars Krogh
// Version: 1.0.0
func date(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

var serverReleases = map[int]Release{
	14393: {"Windows Server 2016", date("2027-01-12")},
	17763: {"Windows Server 2019", date("2029-01-09")},
	20348: {"Windows Server 2022", date("2031-10-14")},
	26100: {"Windows Server 2025", date("2034-11-14")},
}

type clientRelease struct {
	name       string
	consumer   time.Time
	enterprise time.Time
}

var clientReleases = map[int]clientRelease{
	19045: {"Windows 10 version 22H2", date("2025-10-14"), date("2025-10-14")},
	22621: {"Windows 11 version 22H2", date("2024-10-08"), date("2025-10-14")},
	22631: {"Windows 11 version 23H2", date("2025-11-11"), date("2026-11-10")},
	26100: {"Windows 11 version 24H2", date("2026-10-13"), date("2027-10-12")},
	26200: {"Windows 11 version 25H2", date("2027-10-12"), date("2028-10-10")},
	26300: {"Windows 11 version 26H2", date("2028-10-10"), date("2029-10-09")},
}

var enterpriseEditions = map[string]bool{
	"enterprise": true, "enterprisen": true,
	"education": true, "educationn": true,
	"iotenterprise": true,
	"serverrdsh":    true,
}

const (
	ltscEnterprise = "enterprise"
	ltscIoT        = "iot"
)

var ltscEditions = map[string]string{
	"enterprises": ltscEnterprise, "enterprisesn": ltscEnterprise,
	"iotenterprises": ltscIoT, "iotenterprisesk": ltscIoT,
}

var ltscReleases = map[int]map[string]Release{
	26100: {
		ltscEnterprise: {"Windows 11 Enterprise LTSC 2024", date("2029-10-09")},
		ltscIoT:        {"Windows 11 IoT Enterprise LTSC 2024", date("2034-10-10")},
	},
}

// Classify decides whether the plugin may harden the OS at time now. Domain
// controllers, unknown builds and releases past end of support are rejected.
// Client releases use the Enterprise and Education lifecycle for those
// editions and the consumer lifecycle otherwise. Client Long-Term Servicing
// Channel editions use the fixed lifecycle of their LTSC release, and are
// rejected on a build without a known LTSC release. Classify does not check
// whether a baseline exists for the release.
//
// Author: Lars Krogh
// Version: 1.1.0
func Classify(o OS, now time.Time) Classification {
	switch o.ProductType {
	case ProductTypeDomainController:
		return Classification{Status: StatusUnsupportedRole, Reason: "domain controllers are not supported"}
	case ProductTypeServer:
		r, ok := serverReleases[o.Build]
		if !ok {
			return Classification{Status: StatusUnknownRelease, Reason: fmt.Sprintf("Windows Server build %d is not a known release", o.Build)}
		}
		return lifecycleStatus(r, now)
	case ProductTypeWorkstation:
		if kind, ok := ltscEditions[strings.ToLower(o.EditionID)]; ok {
			r, ok := ltscReleases[o.Build][kind]
			if !ok {
				return Classification{Status: StatusUnsupportedEdition, Reason: fmt.Sprintf("edition %s on build %d is not a known Long-Term Servicing Channel release", o.EditionID, o.Build)}
			}
			return lifecycleStatus(r, now)
		}
		r, ok := clientReleases[o.Build]
		if !ok {
			return Classification{Status: StatusUnknownRelease, Reason: fmt.Sprintf("Windows client build %d is not a known release", o.Build)}
		}
		endOfSupport := r.consumer
		if enterpriseEditions[strings.ToLower(o.EditionID)] {
			endOfSupport = r.enterprise
		}
		return lifecycleStatus(Release{Name: r.name, EndOfSupport: endOfSupport}, now)
	}
	return Classification{Status: StatusUnknownRelease, Reason: fmt.Sprintf("unknown product type %d", o.ProductType)}
}

// lifecycleStatus returns StatusSupported until the end of the release's last
// day of support, and StatusEndOfSupport after that.
//
// Author: Lars Krogh
// Version: 1.1.0
func lifecycleStatus(r Release, now time.Time) Classification {
	if !now.Before(r.EndOfSupport.Add(24 * time.Hour)) {
		return Classification{
			Release: r,
			Status:  StatusEndOfSupport,
			Reason:  fmt.Sprintf("%s reached end of support on %s", r.Name, r.EndOfSupport.Format("2006-01-02")),
		}
	}
	return Classification{Release: r, Status: StatusSupported}
}
