package utils

import "testing"

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"pci_pf", "0000:17:00.0", "0000-17-00-0"},
		{"pci_vf", "0000:17:00.2", "0000-17-00-2"},
		{"pci_second_pf", "0000:41:00.0", "0000-41-00-0"},
		{"slash", "a/b/c", "a-b-c"},
		{"dot", "1.2.3", "1-2-3"},
		{"mixed", "pci-0000:17:00.0", "pci-0000-17-00-0"},
		{"ifname_pf", "enp23s0f0np0", "enp23s0f0np0"},
		{"ifname_vf", "enp23s0f0v0", "enp23s0f0v0"},
		{"empty", "", ""},
		{"all_special", ":/.", "---"},
		{"hyphen_passthrough", "already-safe", "already-safe"},
		{"underscore_passthrough", "a_b", "a_b"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeName(tc.in)
			if got != tc.want {
				t.Errorf("SanitizeName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
