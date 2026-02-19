// Package doctor provides RDMA environment diagnostics.
// It checks character device presence, kernel modules, link attributes,
// and RDMA network namespace mode.
package doctor

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/vishvananda/netlink"

	"github.com/Nativu5/rdma-cdi/pkg/rdma"
	"github.com/Nativu5/rdma-cdi/pkg/types"
)

// Severity levels for diagnostic checks.
type Severity string

const (
	Pass Severity = "PASS"
	Warn Severity = "WARN"
	Fail Severity = "FAIL"
)

// requiredKernelModules lists the kernel modules that must be loaded
// for RDMA/CDI to function correctly.
var requiredKernelModules = []string{"ib_core", "ib_uverbs", "ib_umad", "rdma_cm", "rdma_ucm"}

// CheckResult represents one diagnostic check outcome.
type CheckResult struct {
	Check    string   `json:"check"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Device   string   `json:"device,omitempty"`
}

// Report holds all diagnostic results for a device or the whole host.
type Report struct {
	Results []CheckResult `json:"results"`
	HasWarn bool          `json:"-"`
	HasFail bool          `json:"-"`
}

// add appends a result and updates summary flags.
func (r *Report) add(cr CheckResult) {
	r.Results = append(r.Results, cr)
	switch cr.Severity {
	case Warn:
		r.HasWarn = true
	case Fail:
		r.HasFail = true
	}
}

// filtered returns results, optionally excluding PASS entries.
func (r *Report) filtered(showPass bool) []CheckResult {
	if showPass {
		return r.Results
	}
	var out []CheckResult
	for _, cr := range r.Results {
		if cr.Severity != Pass {
			out = append(out, cr)
		}
	}
	return out
}

// DiagnoseDevice runs all checks on a single RDMA device.
func DiagnoseDevice(dev *types.RdmaDevice) *Report {
	report := &Report{}

	// 1. RDMA character devices — presence and required types
	if len(dev.RdmaDevices) == 0 {
		report.add(CheckResult{
			Check:    "rdma_devices",
			Severity: Fail,
			Message:  "No RDMA character devices found",
			Device:   dev.PciAddress,
		})
	} else if err := rdma.VerifyRdmaDevices(dev.RdmaDevices); err != nil {
		report.add(CheckResult{
			Check:    "rdma_devices",
			Severity: Fail,
			Message:  fmt.Sprintf("Found %d device(s) but missing required types: %v", len(dev.RdmaDevices), err),
			Device:   dev.PciAddress,
		})
	} else {
		report.add(CheckResult{
			Check:    "rdma_devices",
			Severity: Pass,
			Message:  fmt.Sprintf("All required RDMA devices present (%d): %s", len(dev.RdmaDevices), strings.Join(dev.RdmaDevices, ", ")),
			Device:   dev.PciAddress,
		})
	}

	// 2. Kernel modules
	checkKernelModules(report)

	// 3. Network interface & link attributes
	if dev.IfName != "" {
		report.add(CheckResult{
			Check:    "net_interface",
			Severity: Pass,
			Message:  fmt.Sprintf("Interface: %s", dev.IfName),
			Device:   dev.PciAddress,
		})
		checkLinkAttrs(report, dev)
	} else {
		report.add(CheckResult{
			Check:    "net_interface",
			Severity: Warn,
			Message:  "No network interface associated",
			Device:   dev.PciAddress,
		})
	}

	// 4. RDMA netns mode
	checkRdmaNetnsMode(report, dev.PciAddress)

	return report
}

// checkKernelModules verifies that essential RDMA kernel modules are loaded.
func checkKernelModules(report *Report) {
	var missing []string
	for _, mod := range requiredKernelModules {
		path := fmt.Sprintf("/sys/module/%s", mod)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			missing = append(missing, mod)
		}
	}
	if len(missing) > 0 {
		report.add(CheckResult{
			Check:    "kernel_modules",
			Severity: Fail,
			Message:  fmt.Sprintf("Missing kernel modules: %s", strings.Join(missing, ", ")),
		})
	} else {
		report.add(CheckResult{
			Check:    "kernel_modules",
			Severity: Pass,
			Message:  fmt.Sprintf("All required kernel modules loaded: %s", strings.Join(requiredKernelModules, ", ")),
		})
	}
}

// checkLinkAttrs uses netlink to inspect link state and encap type.
func checkLinkAttrs(report *Report, dev *types.RdmaDevice) {
	link, err := netlink.LinkByName(dev.IfName)
	if err != nil {
		report.add(CheckResult{
			Check:    "link_attrs",
			Severity: Warn,
			Message:  fmt.Sprintf("Cannot query link %s: %v", dev.IfName, err),
			Device:   dev.PciAddress,
		})
		return
	}

	attrs := link.Attrs()
	dev.LinkType = attrs.EncapType

	state := attrs.OperState.String()
	if attrs.OperState == netlink.OperUp {
		report.add(CheckResult{
			Check:    "link_state",
			Severity: Pass,
			Message:  fmt.Sprintf("Link %s is %s (encap: %s, MTU: %d)", dev.IfName, state, attrs.EncapType, attrs.MTU),
			Device:   dev.PciAddress,
		})
	} else {
		report.add(CheckResult{
			Check:    "link_state",
			Severity: Warn,
			Message:  fmt.Sprintf("Link %s is %s (encap: %s, MTU: %d)", dev.IfName, state, attrs.EncapType, attrs.MTU),
			Device:   dev.PciAddress,
		})
	}
}

// checkRdmaNetnsMode reads RDMA netns mode from sysfs.
func checkRdmaNetnsMode(report *Report, pciAddr string) {
	data, err := os.ReadFile("/sys/module/rdma_cm/parameters/net_ns_mode")
	if err != nil {
		data, err = os.ReadFile("/sys/module/ib_core/parameters/netns_mode")
		if err != nil {
			report.add(CheckResult{
				Check:    "rdma_netns_mode",
				Severity: Warn,
				Message:  "Cannot read RDMA netns mode (sysfs path not available)",
				Device:   pciAddr,
			})
			return
		}
	}

	mode := strings.TrimSpace(string(data))
	switch mode {
	case "exclusive", "1", "Y":
		report.add(CheckResult{
			Check:    "rdma_netns_mode",
			Severity: Pass,
			Message:  fmt.Sprintf("RDMA netns mode: exclusive (%s)", mode),
			Device:   pciAddr,
		})
	case "shared", "0", "N":
		report.add(CheckResult{
			Check:    "rdma_netns_mode",
			Severity: Warn,
			Message:  fmt.Sprintf("RDMA netns mode: shared (%s) — containers may not isolate RDMA traffic", mode),
			Device:   pciAddr,
		})
	default:
		report.add(CheckResult{
			Check:    "rdma_netns_mode",
			Severity: Warn,
			Message:  fmt.Sprintf("Unknown RDMA netns mode: %q", mode),
			Device:   pciAddr,
		})
	}
}

// PrintTable renders the diagnostic report as a table.
// When showPass is false, only WARN/FAIL results are shown.
func PrintTable(w io.Writer, report *Report, showPass bool) {
	results := report.filtered(showPass)
	if len(results) == 0 {
		fmt.Fprintln(w, "All checks passed.")
		return
	}
	table := tablewriter.NewTable(w)
	table.Header("STATUS", "CHECK", "DEVICE", "MESSAGE")
	for _, r := range results {
		marker := "✓"
		switch r.Severity {
		case Warn:
			marker = "!"
		case Fail:
			marker = "✗"
		}
		dev := r.Device
		if dev == "" {
			dev = "(host)"
		}
		status := fmt.Sprintf("%s %s", marker, r.Severity)
		table.Append(status, r.Check, dev, r.Message)
	}
	table.Render()
}

// PrintJSON renders the diagnostic report as JSON.
// When showPass is false, only WARN/FAIL results are included.
func PrintJSON(w io.Writer, report *Report, showPass bool) error {
	results := report.filtered(showPass)
	if results == nil {
		results = []CheckResult{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}

// MergeReports combines multiple per-device reports into one.
func MergeReports(reports ...*Report) *Report {
	merged := &Report{}
	for _, r := range reports {
		for _, cr := range r.Results {
			merged.add(cr)
		}
	}
	return merged
}
