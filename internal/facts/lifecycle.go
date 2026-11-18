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

var ltscEditions = map[string]bool{
	"enterprises": true, "enterprisesn": true,
	"iotenterprises": true, "iotenterprisesk": true,
}

// isLTSC reports whether edition is a Long-Term Servicing Channel edition.
//
// Author: Lars Krogh
// Version: 1.0.0
func isLTSC(edition string) bool { return ltscEditions[strings.ToLower(edition)] }

// Classify decides whether the plugin may harden the OS at time now. Domain
// controllers, LTSC editions, unknown builds and releases past end of support
// are rejected. Client releases use the Enterprise and Education lifecycle
// for those editions and the consumer lifecycle otherwise. Classify does not
// check whether a baseline exists for the release.
//
// Author: Lars Krogh
// Version: 1.0.0
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
		r, ok := clientReleases[o.Build]
		if !ok {
			return Classification{Status: StatusUnknownRelease, Reason: fmt.Sprintf("Windows client build %d is not a known release", o.Build)}
		}
		if isLTSC(o.EditionID) {
			return Classification{
				Release: Release{Name: r.name + " LTSC"},
				Status:  StatusUnsupportedEdition,
				Reason:  fmt.Sprintf("edition %s is a Long-Term Servicing Channel edition; no LTSC-specific baseline is implemented", o.EditionID),
			}
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
// Version: 1.0.0
func lifecycleStatus(r Release, now time.Time) Classification {
	if now.After(r.EndOfSupport.Add(24 * time.Hour)) {
		return Classification{
			Release: r,
			Status:  StatusEndOfSupport,
			Reason:  fmt.Sprintf("%s reached end of support on %s", r.Name, r.EndOfSupport.Format("2006-01-02")),
		}
	}
	return Classification{Release: r, Status: StatusSupported}
}
