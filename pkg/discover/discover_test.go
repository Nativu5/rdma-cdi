package discover

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Nativu5/rdma-cdi/pkg/types"
)

func sampleDevices() []*types.RdmaDevice {
	return []*types.RdmaDevice{
		{
			PciAddress: "0000:17:00.0",
			IfName:     "enp23s0f0np0",
			Driver:     "mlx5_core",
			LinkType:   "ether",
			RdmaDevices: []string{
				"/dev/infiniband/umad0",
				"/dev/infiniband/uverbs0",
				"/dev/infiniband/rdma_cm",
			},
		},
		{
			PciAddress:  "0000:17:00.2",
			IfName:      "",
			Driver:      "",
			LinkType:    "",
			RdmaDevices: []string{"/dev/infiniband/uverbs3"},
		},
	}
}

func TestPrintTable_Basic(t *testing.T) {
	var buf bytes.Buffer
	PrintTable(&buf, sampleDevices())
	output := buf.String()

	// Should contain headers
	if !strings.Contains(output, "PCI ADDRESS") {
		t.Error("table should contain PCI ADDRESS header")
	}
	if !strings.Contains(output, "INTERFACE") {
		t.Error("table should contain INTERFACE header")
	}

	// Should contain device data
	if !strings.Contains(output, "0000:17:00.0") {
		t.Error("table should contain PCI address")
	}
	if !strings.Contains(output, "enp23s0f0np0") {
		t.Error("table should contain interface name")
	}

	// Devices with missing info should show placeholders
	if !strings.Contains(output, "(none)") {
		t.Error("table should show (none) for missing interface")
	}
	if !strings.Contains(output, "(unknown)") {
		t.Error("table should show (unknown) for missing driver/linktype")
	}
}

func TestPrintTable_Empty(t *testing.T) {
	var buf bytes.Buffer
	PrintTable(&buf, nil)
	output := buf.String()

	// Should still render headers
	if !strings.Contains(output, "PCI ADDRESS") {
		t.Error("empty table should still render headers")
	}
}

func TestPrintJSON_Basic(t *testing.T) {
	var buf bytes.Buffer
	err := PrintJSON(&buf, sampleDevices())
	if err != nil {
		t.Fatalf("PrintJSON failed: %v", err)
	}

	var result []DeviceJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 devices, got %d", len(result))
	}
	if result[0].PciAddress != "0000:17:00.0" {
		t.Errorf("first device PciAddress = %q, want 0000:17:00.0", result[0].PciAddress)
	}
	if result[0].Driver != "mlx5_core" {
		t.Errorf("first device Driver = %q, want mlx5_core", result[0].Driver)
	}
}

func TestPrintJSON_Empty(t *testing.T) {
	var buf bytes.Buffer
	err := PrintJSON(&buf, nil)
	if err != nil {
		t.Fatalf("PrintJSON with nil failed: %v", err)
	}

	var result []DeviceJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 devices, got %d", len(result))
	}
}
