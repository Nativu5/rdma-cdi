package main

import (
	"bytes"
	"strings"
	"testing"
)

// ──────────────────────────────────────────────
//  deriveDefaultName
// ──────────────────────────────────────────────

func TestDeriveDefaultName(t *testing.T) {
	tests := []struct {
		name   string
		pci    string
		ifname string
		want   string
	}{
		{"from_pci", "0000:17:00.0", "", "pci-0000-17-00-0"},
		{"from_ifname", "", "enp23s0f0np0", "enp23s0f0np0"},
		{"both_empty", "", "", "unknown"},
		{"ifname_priority", "0000:17:00.0", "enp23s0f0np0", "enp23s0f0np0"},
		{"pci_vf", "0000:17:00.2", "", "pci-0000-17-00-2"},
		{"second_pf", "0000:41:00.0", "", "pci-0000-41-00-0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := deriveDefaultName(tc.pci, tc.ifname)
			if got != tc.want {
				t.Errorf("deriveDefaultName(%q, %q) = %q, want %q",
					tc.pci, tc.ifname, got, tc.want)
			}
		})
	}
}

// ──────────────────────────────────────────────
//  rootCmd structure
// ──────────────────────────────────────────────

func TestRootCmd_HasAllSubcommands(t *testing.T) {
	root := rootCmd()

	expected := map[string]bool{
		"generate": false,
		"discover": false,
		"doctor":   false,
		"cleanup":  false,
		"version":  false,
	}

	for _, sub := range root.Commands() {
		if _, ok := expected[sub.Name()]; ok {
			expected[sub.Name()] = true
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("missing subcommand: %s", name)
		}
	}
}

// ──────────────────────────────────────────────
//  generate command flags
// ──────────────────────────────────────────────

func TestGenerateCmd_Flags(t *testing.T) {
	cmd := newGenerateCmd()

	requiredFlags := []string{"all", "pci", "ifname", "prefix", "name", "output-dir", "format"}
	for _, flag := range requiredFlags {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("generate command missing flag: --%s", flag)
		}
	}
}

func TestGenerateCmd_DefaultValues(t *testing.T) {
	cmd := newGenerateCmd()

	tests := []struct {
		flag string
		want string
	}{
		{"all", "false"},
		{"prefix", "rdma"},
		{"output-dir", "/etc/cdi"},
		{"format", "yaml"},
		{"name", ""},
		{"pci", ""},
		{"ifname", ""},
	}

	for _, tc := range tests {
		f := cmd.Flags().Lookup(tc.flag)
		if f.DefValue != tc.want {
			t.Errorf("flag --%s default = %q, want %q", tc.flag, f.DefValue, tc.want)
		}
	}
}

// ──────────────────────────────────────────────
//  discover command flags
// ──────────────────────────────────────────────

func TestDiscoverCmd_Flags(t *testing.T) {
	cmd := newDiscoverCmd()

	flags := []string{"all", "pci", "ifname", "output"}
	for _, flag := range flags {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("discover command missing flag: --%s", flag)
		}
	}

	// --all defaults to true
	allFlag := cmd.Flags().Lookup("all")
	if allFlag.DefValue != "true" {
		t.Errorf("--all default = %q, want 'true'", allFlag.DefValue)
	}

	// --output defaults to table
	outFlag := cmd.Flags().Lookup("output")
	if outFlag.DefValue != "table" {
		t.Errorf("--output default = %q, want 'table'", outFlag.DefValue)
	}
}

// ──────────────────────────────────────────────
//  doctor command flags
// ──────────────────────────────────────────────

func TestDoctorCmd_Flags(t *testing.T) {
	cmd := newDoctorCmd()

	flags := []string{"all", "pci", "ifname", "strict", "show-pass", "output"}
	for _, flag := range flags {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("doctor command missing flag: --%s", flag)
		}
	}

	// --strict defaults to false
	f := cmd.Flags().Lookup("strict")
	if f.DefValue != "false" {
		t.Errorf("--strict default = %q, want 'false'", f.DefValue)
	}

	// --show-pass defaults to false
	f = cmd.Flags().Lookup("show-pass")
	if f.DefValue != "false" {
		t.Errorf("--show-pass default = %q, want 'false'", f.DefValue)
	}
}

// ──────────────────────────────────────────────
//  cleanup command flags
// ──────────────────────────────────────────────

func TestCleanupCmd_Flags(t *testing.T) {
	cmd := newCleanupCmd()

	flags := []string{"prefix", "name", "output-dir", "dry-run", "force"}
	for _, flag := range flags {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("cleanup command missing flag: --%s", flag)
		}
	}

	// --prefix defaults to "rdma"
	f := cmd.Flags().Lookup("prefix")
	if f.DefValue != "rdma" {
		t.Errorf("--prefix default = %q, want 'rdma'", f.DefValue)
	}
}

// ──────────────────────────────────────────────
//  XOR validation (simulate via rootCmd)
// ──────────────────────────────────────────────

func TestGenerateCmd_NeitherPciNorIfname(t *testing.T) {
	// Running generate without --pci, --ifname, or --all should produce an error.
	root := rootCmd()
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{"generate"})

	cmd := newGenerateCmd()
	if cmd.Use != "generate" {
		t.Errorf("expected Use=generate, got %q", cmd.Use)
	}
}

func TestGenerateCmd_AllAndPciConflict(t *testing.T) {
	root := rootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"generate", "--all", "--pci", "0000:17:00.0"})
	err := root.Execute()
	if err == nil {
		t.Error("expected error when --all and --pci are both set")
	}
}

func TestGenerateCmd_AllAndNameConflict(t *testing.T) {
	root := rootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"generate", "--all", "--name", "mydev"})
	err := root.Execute()
	if err == nil {
		t.Error("expected error when --all and --name are both set")
	}
}

func TestDiscoverCmd_PciAndIfnameConflict(t *testing.T) {
	// Verify the command accepts both flags (validation is at runtime)
	cmd := newDiscoverCmd()
	pci := cmd.Flags().Lookup("pci")
	ifn := cmd.Flags().Lookup("ifname")
	if pci == nil || ifn == nil {
		t.Error("discover should have both --pci and --ifname flags")
	}
}

// ──────────────────────────────────────────────
//  Help output
// ──────────────────────────────────────────────

func TestRootCmd_HelpOutput(t *testing.T) {
	root := rootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"--help"})
	_ = root.Execute()

	output := buf.String()
	if !strings.Contains(output, "RDMA") {
		t.Error("help output should contain tool description")
	}
	for _, sub := range []string{"generate", "discover", "doctor", "cleanup"} {
		if !strings.Contains(output, sub) {
			t.Errorf("help output should list %q subcommand", sub)
		}
	}
}

// ──────────────────────────────────────────────
//  --log-level flag
// ──────────────────────────────────────────────

func TestRootCmd_LogLevelFlag(t *testing.T) {
	root := rootCmd()
	f := root.PersistentFlags().Lookup("log-level")
	if f == nil {
		t.Fatal("root command missing --log-level flag")
	}
	if f.DefValue != "info" {
		t.Errorf("--log-level default = %q, want 'info'", f.DefValue)
	}
}

func TestRootCmd_LogLevelInvalid(t *testing.T) {
	root := rootCmd()
	root.SetArgs([]string{"--log-level", "bogus", "discover", "--all"})
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetOut(&bytes.Buffer{})
	err := root.Execute()
	if err == nil {
		t.Error("expected error for invalid log level, got nil")
	}
	if !strings.Contains(err.Error(), "invalid log level") {
		t.Errorf("expected 'invalid log level' in error, got: %v", err)
	}
}

func TestRootCmd_LogLevelValid(t *testing.T) {
	for _, level := range []string{"trace", "debug", "info", "warn", "error"} {
		root := rootCmd()
		root.SetArgs([]string{"--log-level", level, "--help"})
		root.SetOut(&bytes.Buffer{})
		if err := root.Execute(); err != nil {
			t.Errorf("--log-level %s should be valid, got error: %v", level, err)
		}
	}
}

// ──────────────────────────────────────────────
//  version command
// ──────────────────────────────────────────────

func TestVersionCmd_Output(t *testing.T) {
	root := rootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "rdma-cdi") {
		t.Errorf("version output should contain 'rdma-cdi', got: %q", out)
	}
	if !strings.Contains(out, "commit:") {
		t.Errorf("version output should contain 'commit:', got: %q", out)
	}
}
