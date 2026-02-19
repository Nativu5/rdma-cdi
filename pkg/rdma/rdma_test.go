package rdma

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Nativu5/rdma-cdi/pkg/types"
)

// ──────────────────────────────────────────────
//  VerifyRdmaDevices
// ──────────────────────────────────────────────

func TestVerifyRdmaDevices_AllPresent(t *testing.T) {
	charDevs := []string{
		"/dev/infiniband/rdma_cm",
		"/dev/infiniband/umad0",
		"/dev/infiniband/uverbs0",
	}
	if err := VerifyRdmaDevices(charDevs); err != nil {
		t.Errorf("expected no error when all required devices present, got: %v", err)
	}
}

func TestVerifyRdmaDevices_MissingUverbs(t *testing.T) {
	charDevs := []string{
		"/dev/infiniband/rdma_cm",
		"/dev/infiniband/umad0",
	}
	err := VerifyRdmaDevices(charDevs)
	if err == nil {
		t.Error("expected error when uverbs missing")
	}
}

func TestVerifyRdmaDevices_MissingRdmaCm(t *testing.T) {
	charDevs := []string{
		"/dev/infiniband/umad0",
		"/dev/infiniband/uverbs0",
	}
	err := VerifyRdmaDevices(charDevs)
	if err == nil {
		t.Error("expected error when rdma_cm missing")
	}
}

func TestVerifyRdmaDevices_Empty(t *testing.T) {
	err := VerifyRdmaDevices(nil)
	if err == nil {
		t.Error("expected error for empty device list")
	}
}

func TestVerifyRdmaDevices_Multiple(t *testing.T) {
	// Multiple devices with all types — should pass
	charDevs := []string{
		"/dev/infiniband/rdma_cm",
		"/dev/infiniband/umad0",
		"/dev/infiniband/uverbs0",
		"/dev/infiniband/umad1",
		"/dev/infiniband/uverbs1",
	}
	if err := VerifyRdmaDevices(charDevs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ──────────────────────────────────────────────
//  buildDeviceSpecs (unexported)
// ──────────────────────────────────────────────

func TestBuildDeviceSpecs(t *testing.T) {
	charDevs := []string{"/dev/infiniband/uverbs0", "/dev/infiniband/rdma_cm"}
	specs := buildDeviceSpecs(charDevs)

	if len(specs) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(specs))
	}
	for _, s := range specs {
		if s.HostPath != s.ContainerPath {
			t.Errorf("HostPath (%s) != ContainerPath (%s)", s.HostPath, s.ContainerPath)
		}
		if s.Permissions != "rw" {
			t.Errorf("expected permissions 'rw', got %q", s.Permissions)
		}
	}
}

func TestBuildDeviceSpecs_Empty(t *testing.T) {
	specs := buildDeviceSpecs(nil)
	if len(specs) != 0 {
		t.Errorf("expected empty specs, got %d", len(specs))
	}
}

// ──────────────────────────────────────────────
//  RequiredRdmaDevices constant
// ──────────────────────────────────────────────

func TestRequiredRdmaDevices(t *testing.T) {
	expected := map[string]bool{"rdma_cm": true, "umad": true, "uverbs": true}
	for _, name := range types.RequiredRdmaDevices {
		if !expected[name] {
			t.Errorf("unexpected required device: %s", name)
		}
	}
	if len(types.RequiredRdmaDevices) != 3 {
		t.Errorf("expected 3 required device types, got %d", len(types.RequiredRdmaDevices))
	}
}

// ──────────────────────────────────────────────
//  readSysfsAttr (unexported — tested via fake sysfs)
// ──────────────────────────────────────────────

func TestReadSysfsAttr(t *testing.T) {
	dir := t.TempDir()
	attrFile := filepath.Join(dir, "vendor")

	os.WriteFile(attrFile, []byte("0x15b3\n"), 0644)
	got := readSysfsAttr(attrFile)
	if got != "15b3" {
		t.Errorf("expected '15b3', got %q", got)
	}
}

func TestReadSysfsAttr_NotExist(t *testing.T) {
	got := readSysfsAttr("/nonexistent/path/vendor")
	if got != "" {
		t.Errorf("expected empty string for missing file, got %q", got)
	}
}

// ──────────────────────────────────────────────
//  GetPCIVendor / GetPCIDeviceID with fake sysfs
// ──────────────────────────────────────────────

func TestGetPCIVendor_FakeSysfs(t *testing.T) {
	origSysBusPci := sysBusPci
	defer func() { sysBusPci = origSysBusPci }()

	dir := t.TempDir()
	pciDir := filepath.Join(dir, "0000:17:00.0")
	os.MkdirAll(pciDir, 0755)
	os.WriteFile(filepath.Join(pciDir, "vendor"), []byte("0x15b3\n"), 0644)
	os.WriteFile(filepath.Join(pciDir, "device"), []byte("0x1017\n"), 0644)

	sysBusPci = dir

	vendor := GetPCIVendor("0000:17:00.0")
	if vendor != "15b3" {
		t.Errorf("expected vendor '15b3', got %q", vendor)
	}

	deviceID := GetPCIDeviceID("0000:17:00.0")
	if deviceID != "1017" {
		t.Errorf("expected device ID '1017', got %q", deviceID)
	}
}

// ──────────────────────────────────────────────
//  GetNetNames with fake sysfs
// ──────────────────────────────────────────────

func TestGetNetNames_FakeSysfs(t *testing.T) {
	origSysBusPci := sysBusPci
	defer func() { sysBusPci = origSysBusPci }()

	dir := t.TempDir()
	// Simulate a ConnectX-5 PF with one net interface
	pciDir := filepath.Join(dir, "0000:17:00.0", "net", "enp23s0f0np0")
	os.MkdirAll(pciDir, 0755)

	// Simulate a second PF on a different bus
	pciDir2 := filepath.Join(dir, "0000:41:00.0", "net", "enp65s0np0")
	os.MkdirAll(pciDir2, 0755)

	sysBusPci = dir

	names, err := GetNetNames("0000:17:00.0")
	if err != nil {
		t.Fatalf("GetNetNames failed: %v", err)
	}
	if len(names) != 1 {
		t.Fatalf("expected 1 name, got %d: %v", len(names), names)
	}
	if names[0] != "enp23s0f0np0" {
		t.Errorf("expected name 'enp23s0f0np0', got %q", names[0])
	}
}

func TestGetNetNames_NoPciDevice(t *testing.T) {
	origSysBusPci := sysBusPci
	defer func() { sysBusPci = origSysBusPci }()

	sysBusPci = t.TempDir()

	_, err := GetNetNames("0000:ff:ff.0")
	if err == nil {
		t.Error("expected error for non-existent PCI device")
	}
}
