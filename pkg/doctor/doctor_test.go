package doctor

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Nativu5/rdma-cdi/pkg/types"
)

// helpers

func fullDevice() *types.RdmaDevice {
	return &types.RdmaDevice{
		PciAddress: "0000:17:00.0",
		IfName:     "enp23s0f0np0",
		Driver:     "mlx5_core",
		LinkType:   "ether",
		RdmaDevices: []string{
			"/dev/infiniband/rdma_cm",
			"/dev/infiniband/umad0",
			"/dev/infiniband/uverbs0",
		},
	}
}

func brokenDevice() *types.RdmaDevice {
	return &types.RdmaDevice{
		PciAddress:  "0000:17:00.2",
		RdmaDevices: nil,
	}
}

// DiagnoseDevice tests

func TestDiagnoseDevice_FullyHealthy(t *testing.T) {
	dev := fullDevice()
	report := DiagnoseDevice(dev)

	if report.HasFail {
		t.Error("healthy device should not have FAILs")
	}

	passCount := 0
	for _, r := range report.Results {
		if r.Severity == Pass {
			passCount++
		}
	}
	if passCount < 3 {
		t.Errorf("expected at least 3 PASS results for healthy device, got %d", passCount)
		for _, r := range report.Results {
			t.Logf("  %s: %s - %s", r.Severity, r.Check, r.Message)
		}
	}
}

func TestDiagnoseDevice_NoCharDevices(t *testing.T) {
	dev := brokenDevice()
	report := DiagnoseDevice(dev)

	if !report.HasFail {
		t.Error("device with no char devices should have FAILs")
	}

	found := false
	for _, r := range report.Results {
		if r.Check == "rdma_devices" && r.Severity == Fail {
			found = true
		}
	}
	if !found {
		t.Error("expected FAIL for rdma_devices check")
	}
}

func TestDiagnoseDevice_NoInterface(t *testing.T) {
	dev := fullDevice()
	dev.IfName = ""
	report := DiagnoseDevice(dev)

	found := false
	for _, r := range report.Results {
		if r.Check == "net_interface" && r.Severity == Warn {
			found = true
		}
	}
	if !found {
		t.Error("expected WARN for missing net interface")
	}
}

func TestDiagnoseDevice_MissingRequiredDevices(t *testing.T) {
	dev := fullDevice()
	dev.RdmaDevices = []string{"/dev/infiniband/uverbs0"}
	report := DiagnoseDevice(dev)

	found := false
	for _, r := range report.Results {
		if r.Check == "rdma_devices" && r.Severity == Fail {
			found = true
		}
	}
	if !found {
		t.Error("expected FAIL for missing required device types")
	}
}

func TestDiagnoseDevice_KernelModulesCheck(t *testing.T) {
	dev := fullDevice()
	report := DiagnoseDevice(dev)

	found := false
	for _, r := range report.Results {
		if r.Check == "kernel_modules" {
			found = true
		}
	}
	if !found {
		t.Error("expected kernel_modules check in report")
	}
}

// MergeReports tests

func TestMergeReports(t *testing.T) {
	r1 := &Report{}
	r1.add(CheckResult{Check: "a", Severity: Pass, Message: "ok"})

	r2 := &Report{}
	r2.add(CheckResult{Check: "b", Severity: Warn, Message: "warn"})

	merged := MergeReports(r1, r2)

	if len(merged.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(merged.Results))
	}
	if !merged.HasWarn {
		t.Error("merged should have HasWarn=true")
	}
	if merged.HasFail {
		t.Error("merged should not have HasFail")
	}
}

func TestMergeReports_WithFail(t *testing.T) {
	r1 := &Report{}
	r1.add(CheckResult{Check: "a", Severity: Pass})
	r2 := &Report{}
	r2.add(CheckResult{Check: "b", Severity: Fail})

	merged := MergeReports(r1, r2)
	if !merged.HasFail {
		t.Error("merged should have HasFail=true")
	}
}

// Strict exit code logic

func TestStrictExitCodeLogic(t *testing.T) {
	tests := []struct {
		name        string
		hasWarn     bool
		hasFail     bool
		strict      bool
		wantNonZero bool
	}{
		{"all_pass_no_strict", false, false, false, false},
		{"all_pass_strict", false, false, true, false},
		{"warn_no_strict", true, false, false, false},
		{"warn_strict", true, false, true, true},
		{"fail_no_strict", false, true, false, true},
		{"fail_strict", false, true, true, true},
		{"warn_and_fail_strict", true, true, true, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			report := &Report{HasWarn: tc.hasWarn, HasFail: tc.hasFail}
			shouldExitNonZero := report.HasFail || (tc.strict && report.HasWarn)
			if shouldExitNonZero != tc.wantNonZero {
				t.Errorf("strict=%v, hasWarn=%v, hasFail=%v: shouldExit=%v, want %v",
					tc.strict, tc.hasWarn, tc.hasFail, shouldExitNonZero, tc.wantNonZero)
			}
		})
	}
}

// Output tests

func TestPrintTable_Output(t *testing.T) {
	report := &Report{}
	report.add(CheckResult{Check: "test_check", Severity: Pass, Message: "all good", Device: "0000:17:00.0"})
	report.add(CheckResult{Check: "test_warn", Severity: Warn, Message: "heads up", Device: "0000:17:00.0"})

	// With showPass=true, both entries visible
	var buf bytes.Buffer
	PrintTable(&buf, report, true)
	output := buf.String()
	if !strings.Contains(output, "PASS") {
		t.Error("table with showPass=true should contain PASS")
	}
	if !strings.Contains(output, "WARN") {
		t.Error("table with showPass=true should contain WARN")
	}

	// With showPass=false, only WARN visible
	buf.Reset()
	PrintTable(&buf, report, false)
	output = buf.String()
	if strings.Contains(output, "PASS") {
		t.Error("table with showPass=false should not contain PASS")
	}
	if !strings.Contains(output, "WARN") {
		t.Error("table with showPass=false should still contain WARN")
	}
}

func TestPrintTable_AllPass_NoShowPass(t *testing.T) {
	report := &Report{}
	report.add(CheckResult{Check: "ok", Severity: Pass, Message: "fine"})

	var buf bytes.Buffer
	PrintTable(&buf, report, false)
	output := buf.String()
	if !strings.Contains(output, "All checks passed.") {
		t.Errorf("expected 'All checks passed.' message, got: %q", output)
	}
}

func TestPrintJSON_Output(t *testing.T) {
	report := &Report{}
	report.add(CheckResult{Check: "test", Severity: Pass, Message: "ok", Device: "0000:17:00.0"})

	var buf bytes.Buffer
	if err := PrintJSON(&buf, report, true); err != nil {
		t.Fatalf("PrintJSON failed: %v", err)
	}

	var results []CheckResult
	if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
		t.Fatalf("JSON output is not valid: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	// With showPass=false, PASS should be excluded
	buf.Reset()
	if err := PrintJSON(&buf, report, false); err != nil {
		t.Fatalf("PrintJSON failed: %v", err)
	}
	var filtered []CheckResult
	if err := json.Unmarshal(buf.Bytes(), &filtered); err != nil {
		t.Fatalf("JSON output is not valid: %v", err)
	}
	if len(filtered) != 0 {
		t.Errorf("expected 0 results with showPass=false, got %d", len(filtered))
	}
}

// Severity values

func TestSeverityValues(t *testing.T) {
	if string(Pass) != "PASS" {
		t.Errorf("Pass = %q, want PASS", Pass)
	}
	if string(Warn) != "WARN" {
		t.Errorf("Warn = %q, want WARN", Warn)
	}
	if string(Fail) != "FAIL" {
		t.Errorf("Fail = %q, want FAIL", Fail)
	}
}
