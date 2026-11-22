package report

import (
	"errors"
	"strings"
	"testing"

	"github.com/larskroghdk/packer-plugin-windows-harden/internal/baseline"
	"github.com/larskroghdk/packer-plugin-windows-harden/internal/plan"
)

// TestTerminalFailurePreventsCompliance ensures a failed restart or transport
// cannot produce a compliant report even when all settings had been verified.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestTerminalFailurePreventsCompliance(t *testing.T) {
	in := Input{Baseline: &baseline.Baseline{ID: "test"}, Plan: &plan.Plan{Items: []*plan.Item{}}}
	if got := Build(in); got.Compliance.Status != ComplianceCompliant {
		t.Fatalf("successful report = %+v", got.Compliance)
	}
	in.Error = errors.New("restart command failed")
	r := Build(in)
	if r.Compliance.Status != ComplianceNonCompliant || r.Compliance.FullBaseline {
		t.Fatalf("failed report = %+v", r.Compliance)
	}
	if !strings.Contains(strings.Join(r.Compliance.Reasons, ";"), in.Error.Error()) {
		t.Fatal("terminal error missing from report")
	}
	data, err := r.JSON()
	if err != nil || !strings.Contains(string(data), `"error": "restart command failed"`) {
		t.Fatalf("report JSON = %s, %v", data, err)
	}
}
