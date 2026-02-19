package cdi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nativu5/rdma-cdi/pkg/types"
)

// ──────────────────────────────────────────────
//  SpecFileName
// ──────────────────────────────────────────────

func TestSpecFileName(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		rname  string
		format string
		want   string
	}{
		{
			name:   "basic_yaml",
			prefix: "rdma",
			rname:  "pci-0000-17-00-0",
			format: "yaml",
			want:   "rdma-cdi_rdma_pci-0000-17-00-0.yaml",
		},
		{
			name:   "basic_json",
			prefix: "rdma",
			rname:  "enp23s0f0np0",
			format: "json",
			want:   "rdma-cdi_rdma_enp23s0f0np0.json",
		},
		{
			name:   "prefix_with_slash",
			prefix: "example.io/rdma",
			rname:  "dev1",
			format: "yaml",
			want:   "rdma-cdi_example.io_rdma_dev1.yaml",
		},
		{
			name:   "prefix_no_slash",
			prefix: "nvidia.com",
			rname:  "gpu0",
			format: "json",
			want:   "rdma-cdi_nvidia.com_gpu0.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SpecFileName(tc.prefix, tc.rname, tc.format)
			if got != tc.want {
				t.Errorf("SpecFileName(%q, %q, %q) = %q, want %q",
					tc.prefix, tc.rname, tc.format, got, tc.want)
			}
		})
	}
}

// ──────────────────────────────────────────────
//  CreateCDISpec
// ──────────────────────────────────────────────

func sampleDevices() []types.RdmaDevice {
	return []types.RdmaDevice{
		{
			PciAddress: "0000:17:00.0",
			IfName:     "enp23s0f0np0",
			DeviceSpecs: []types.DeviceSpec{
				{HostPath: "/dev/infiniband/umad0", ContainerPath: "/dev/infiniband/umad0", Permissions: "rw"},
				{HostPath: "/dev/infiniband/uverbs0", ContainerPath: "/dev/infiniband/uverbs0", Permissions: "rw"},
				{HostPath: "/dev/infiniband/rdma_cm", ContainerPath: "/dev/infiniband/rdma_cm", Permissions: "rw"},
			},
		},
	}
}

func TestCreateCDISpec_YAML(t *testing.T) {
	dir := t.TempDir()
	err := CreateCDISpec("rdma", "test-dev", sampleDevices(), dir, "yaml")
	if err != nil {
		t.Fatalf("CreateCDISpec(yaml) failed: %v", err)
	}

	expected := filepath.Join(dir, "rdma-cdi_rdma_test-dev.yaml")
	if _, err := os.Stat(expected); os.IsNotExist(err) {
		t.Fatalf("expected file %s not found", expected)
	}

	data, _ := os.ReadFile(expected)
	content := string(data)
	// Basic sanity: must contain kind and device name
	if !strings.Contains(content, "rdma/test-dev") {
		t.Errorf("YAML spec missing kind; got:\n%s", content)
	}
	if !strings.Contains(content, "uverbs0") {
		t.Errorf("YAML spec missing uverbs0 device node; got:\n%s", content)
	}
}

func TestCreateCDISpec_JSON(t *testing.T) {
	dir := t.TempDir()
	err := CreateCDISpec("rdma", "test-dev", sampleDevices(), dir, "json")
	if err != nil {
		t.Fatalf("CreateCDISpec(json) failed: %v", err)
	}

	expected := filepath.Join(dir, "rdma-cdi_rdma_test-dev.json")
	data, err := os.ReadFile(expected)
	if err != nil {
		t.Fatalf("cannot read generated file: %v", err)
	}

	// Must be valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("generated file is not valid JSON: %v", err)
	}

	if kind, ok := parsed["kind"].(string); !ok || kind != "rdma/test-dev" {
		t.Errorf("expected kind=rdma/test-dev, got %v", parsed["kind"])
	}
}

func TestCreateCDISpec_InvalidFormat(t *testing.T) {
	dir := t.TempDir()
	err := CreateCDISpec("rdma", "x", sampleDevices(), dir, "xml")
	if err == nil {
		t.Fatal("expected error for unsupported format, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported format") {
		t.Errorf("expected 'unsupported format' error, got: %v", err)
	}
}

func TestCreateCDISpec_EmptyDevices(t *testing.T) {
	dir := t.TempDir()
	err := CreateCDISpec("rdma", "empty", []types.RdmaDevice{}, dir, "yaml")
	if err == nil {
		t.Fatal("expected error for empty devices, got nil")
	}
}

func TestCreateCDISpec_NamingConvention(t *testing.T) {
	dir := t.TempDir()
	_ = CreateCDISpec("rdma", "dev1", sampleDevices(), dir, "yaml")
	_ = CreateCDISpec("rdma", "dev2", sampleDevices(), dir, "json")

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), FilePrefix+"_") {
			t.Errorf("file %q does not start with %q prefix", e.Name(), FilePrefix+"_")
		}
	}
}

// ──────────────────────────────────────────────
//  CleanupSpecs — safety boundary tests
// ──────────────────────────────────────────────

func seedCleanupDir(t *testing.T, dir string) {
	t.Helper()
	files := []string{
		// Our tool's files
		"rdma-cdi_rdma_dev1.yaml",
		"rdma-cdi_rdma_dev1.json",
		"rdma-cdi_rdma_dev2.yaml",
		"rdma-cdi_custom_dev3.json",
		// Other tools' files — must NOT be deleted
		"nvidia-cdi_rdma_gpu0.yaml",
		"other-tool.json",
		"rdma-cdi_rdma_dev1.txt", // wrong extension
		"rdma-cdi_rdma_dev1.bak", // wrong extension
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("test"), 0644); err != nil {
			t.Fatalf("cannot seed file %s: %v", f, err)
		}
	}
}

func TestCleanupSpecs_ExactMatch(t *testing.T) {
	dir := t.TempDir()
	seedCleanupDir(t, dir)

	removed, err := CleanupSpecs(dir, "rdma", "dev1", false)
	if err != nil {
		t.Fatalf("CleanupSpecs exact match failed: %v", err)
	}

	// Should remove both .yaml and .json for dev1
	if len(removed) != 2 {
		t.Errorf("expected 2 removed files, got %d: %v", len(removed), removed)
	}

	// Other files must still exist
	for _, f := range []string{"rdma-cdi_rdma_dev2.yaml", "nvidia-cdi_rdma_gpu0.yaml", "other-tool.json"} {
		if _, err := os.Stat(filepath.Join(dir, f)); os.IsNotExist(err) {
			t.Errorf("file %q should not have been removed", f)
		}
	}
}

func TestCleanupSpecs_PrefixScope(t *testing.T) {
	dir := t.TempDir()
	seedCleanupDir(t, dir)

	removed, err := CleanupSpecs(dir, "rdma", "", false)
	if err != nil {
		t.Fatalf("CleanupSpecs prefix scope failed: %v", err)
	}

	// Should remove all rdma-cdi_rdma_* files with .yaml/.json extensions
	if len(removed) != 3 {
		t.Errorf("expected 3 removed files, got %d: %v", len(removed), removed)
	}

	// custom prefix files must remain
	mustExist := []string{
		"rdma-cdi_custom_dev3.json",
		"nvidia-cdi_rdma_gpu0.yaml",
		"other-tool.json",
		"rdma-cdi_rdma_dev1.txt",
		"rdma-cdi_rdma_dev1.bak",
	}
	for _, f := range mustExist {
		if _, err := os.Stat(filepath.Join(dir, f)); os.IsNotExist(err) {
			t.Errorf("file %q should not have been removed (safety boundary violation)", f)
		}
	}
}

func TestCleanupSpecs_DoesNotDeleteOtherTools(t *testing.T) {
	dir := t.TempDir()
	seedCleanupDir(t, dir)

	// Cleanup all rdma prefix
	_, _ = CleanupSpecs(dir, "rdma", "", false)

	// Files from other tools must survive
	for _, f := range []string{"nvidia-cdi_rdma_gpu0.yaml", "other-tool.json"} {
		if _, err := os.Stat(filepath.Join(dir, f)); os.IsNotExist(err) {
			t.Errorf("file %q from other tool was incorrectly removed!", f)
		}
	}
}

func TestCleanupSpecs_DoesNotDeleteWrongExtension(t *testing.T) {
	dir := t.TempDir()
	seedCleanupDir(t, dir)

	_, _ = CleanupSpecs(dir, "rdma", "", false)

	for _, f := range []string{"rdma-cdi_rdma_dev1.txt", "rdma-cdi_rdma_dev1.bak"} {
		if _, err := os.Stat(filepath.Join(dir, f)); os.IsNotExist(err) {
			t.Errorf("file %q with wrong extension was removed (anti-glob failure)", f)
		}
	}
}

func TestCleanupSpecs_DryRun(t *testing.T) {
	dir := t.TempDir()
	seedCleanupDir(t, dir)

	removed, err := CleanupSpecs(dir, "rdma", "", true)
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}

	if len(removed) == 0 {
		t.Error("dry-run should report files to remove")
	}

	// All files must still exist after dry-run
	entries, _ := os.ReadDir(dir)
	if len(entries) != 8 { // all seeded files
		t.Errorf("dry-run modified files! expected 8, found %d", len(entries))
	}
}

func TestCleanupSpecs_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	removed, err := CleanupSpecs(dir, "rdma", "", false)
	if err != nil {
		t.Fatalf("cleanup empty dir failed: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("expected 0 removals for empty dir, got %d", len(removed))
	}
}

func TestCleanupSpecs_PrefixWithSlash(t *testing.T) {
	dir := t.TempDir()
	// Simulate a file generated with prefix containing '/'
	fname := "rdma-cdi_example.io_rdma_dev1.yaml"
	os.WriteFile(filepath.Join(dir, fname), []byte("test"), 0644)

	removed, err := CleanupSpecs(dir, "example.io/rdma", "dev1", false)
	if err != nil {
		t.Fatalf("cleanup with slash prefix failed: %v", err)
	}
	if len(removed) != 1 {
		t.Errorf("expected 1 removal, got %d: %v", len(removed), removed)
	}
}

// ──────────────────────────────────────────────
//  CreateContainerAnnotations
// ──────────────────────────────────────────────

func TestCreateContainerAnnotations_Basic(t *testing.T) {
	devs := sampleDevices()
	annotations, err := CreateContainerAnnotations(devs, "rdma", "net")
	if err != nil {
		t.Fatalf("CreateContainerAnnotations failed: %v", err)
	}
	if len(annotations) != 1 {
		t.Errorf("expected 1 annotation, got %d", len(annotations))
	}
}

func TestCreateContainerAnnotations_Empty(t *testing.T) {
	_, err := CreateContainerAnnotations(nil, "rdma", "net")
	if err == nil {
		t.Error("expected error for empty devices")
	}
}
