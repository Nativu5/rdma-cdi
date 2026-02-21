// rdma-cdi is a standalone CLI tool for generating CDI (Container Device Interface)
// spec files for RDMA network devices. It discovers RDMA character devices from
// PCI addresses or network interface names and produces CDI-compliant spec files.
//
// Usage:
//
//	rdma-cdi generate --pci 0000:86:00.0
//	rdma-cdi discover --all
//	rdma-cdi doctor --pci 0000:86:00.0
//	rdma-cdi cleanup --prefix rdma
package main

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/Nativu5/rdma-cdi/pkg/cdi"
	"github.com/Nativu5/rdma-cdi/pkg/discover"
	"github.com/Nativu5/rdma-cdi/pkg/doctor"
	"github.com/Nativu5/rdma-cdi/pkg/rdma"
	"github.com/Nativu5/rdma-cdi/pkg/types"
	"github.com/Nativu5/rdma-cdi/pkg/utils"
)

// Exit codes following CLI conventions.
const (
	exitOK           = 0
	exitRuntimeError = 1
)

// Build-time variables injected via ldflags.
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitRuntimeError)
	}
}

// rootCmd builds the top-level cobra command tree.
func rootCmd() *cobra.Command {
	var logLevel string

	root := &cobra.Command{
		Use:   "rdma-cdi",
		Short: "RDMA CDI spec generator",
		Long:  "A standalone tool for discovering RDMA devices and generating CDI (Container Device Interface) spec files.",
		// Silence default usage on runtime errors; we handle exit codes ourselves.
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			lvl, err := log.ParseLevel(logLevel)
			if err != nil {
				return fmt.Errorf("invalid log level %q: %w", logLevel, err)
			}
			log.SetLevel(lvl)
			return nil
		},
	}

	root.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level (trace, debug, info, warn, error, fatal, panic)")

	root.AddCommand(
		newGenerateCmd(),
		newDiscoverCmd(),
		newDoctorCmd(),
		newCleanupCmd(),
		newVersionCmd(),
	)

	return root
}

// ──────────────────────────────────────────────
//  generate
// ──────────────────────────────────────────────

func newGenerateCmd() *cobra.Command {
	var (
		all       bool
		pci       string
		ifname    string
		prefix    string
		name      string
		outputDir string
		format    string
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate CDI spec files for RDMA devices",
		RunE: func(cmd *cobra.Command, args []string) error {
			discoverer := rdma.NewDiscoverer()

			switch {
			case all:
				// Batch mode: generate a spec for every discovered device
				devices, err := discoverer.DiscoverAll()
				if err != nil {
					return fmt.Errorf("device discovery failed: %w", err)
				}
				if len(devices) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No RDMA devices found.")
					return nil
				}

				var errCount int
				for _, dev := range devices {
					autoName := deriveDefaultName(dev.PciAddress, "")
					if err := cdi.CreateCDISpec(prefix, autoName, []types.RdmaDevice{*dev}, outputDir, format); err != nil {
						log.Errorf("failed to generate spec for %s: %v", dev.PciAddress, err)
						errCount++
						continue
					}
					fmt.Fprintf(cmd.OutOrStdout(), "CDI spec written to %s/%s\n",
						outputDir, cdi.SpecFileName(prefix, autoName, format))
				}
				if errCount > 0 {
					return fmt.Errorf("%d device(s) failed to generate", errCount)
				}
				return nil

			default:
				// Single-device mode
				if name == "" {
					name = deriveDefaultName(pci, ifname)
				}

				var dev *types.RdmaDevice
				var err error
				if pci != "" {
					dev, err = discoverer.DiscoverByPCI(pci)
				} else {
					dev, err = discoverer.DiscoverByIfName(ifname)
				}
				if err != nil {
					return fmt.Errorf("device discovery failed: %w", err)
				}

				if err := cdi.CreateCDISpec(prefix, name, []types.RdmaDevice{*dev}, outputDir, format); err != nil {
					return fmt.Errorf("CDI spec generation failed: %w", err)
				}

				fmt.Fprintf(cmd.OutOrStdout(), "CDI spec written to %s/%s\n",
					outputDir, cdi.SpecFileName(prefix, name, format))
				return nil
			}
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Generate specs for all discovered RDMA devices")
	cmd.Flags().StringVar(&pci, "pci", "", "PCI BDF address (e.g. 0000:86:00.0)")
	cmd.Flags().StringVar(&ifname, "ifname", "", "Network interface name (e.g. ib0)")
	cmd.Flags().StringVar(&prefix, "prefix", cdi.DefaultPrefix, "CDI resource prefix")
	cmd.Flags().StringVar(&name, "name", "", "CDI resource name (auto-derived if omitted; incompatible with --all)")
	cmd.Flags().StringVar(&outputDir, "output-dir", cdi.DefaultOutputDir, "Output directory for CDI spec files")
	cmd.Flags().StringVar(&format, "format", "yaml", "Output format (json|yaml)")

	// --all, --pci, --ifname are mutually exclusive; at least one required
	cmd.MarkFlagsMutuallyExclusive("all", "pci")
	cmd.MarkFlagsMutuallyExclusive("all", "ifname")
	cmd.MarkFlagsMutuallyExclusive("pci", "ifname")
	cmd.MarkFlagsOneRequired("all", "pci", "ifname")
	// --name is only meaningful for single-device mode
	cmd.MarkFlagsMutuallyExclusive("all", "name")

	return cmd
}

// ──────────────────────────────────────────────
//  discover
// ──────────────────────────────────────────────

func newDiscoverCmd() *cobra.Command {
	var (
		all    bool
		pci    string
		ifname string
		output string
	)

	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Discover RDMA devices and their character device mappings",
		RunE: func(cmd *cobra.Command, args []string) error {
			// If a target is specified, --all is implicitly false
			if pci != "" || ifname != "" {
				if all {
					log.Warn("--all ignored because --pci or --ifname was specified")
				}
				all = false
			}

			discoverer := rdma.NewDiscoverer()
			var devices []*types.RdmaDevice

			switch {
			case pci != "":
				dev, err := discoverer.DiscoverByPCI(pci)
				if err != nil {
					return fmt.Errorf("discovery failed: %w", err)
				}
				devices = []*types.RdmaDevice{dev}
			case ifname != "":
				dev, err := discoverer.DiscoverByIfName(ifname)
				if err != nil {
					return fmt.Errorf("discovery failed: %w", err)
				}
				devices = []*types.RdmaDevice{dev}
			default: // --all
				var err error
				devices, err = discoverer.DiscoverAll()
				if err != nil {
					return fmt.Errorf("discovery failed: %w", err)
				}
			}

			switch output {
			case "json":
				return discover.PrintJSON(cmd.OutOrStdout(), devices)
			default:
				discover.PrintTable(cmd.OutOrStdout(), devices)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", true, "Discover all RDMA devices on the host")
	cmd.Flags().StringVar(&pci, "pci", "", "PCI BDF address")
	cmd.Flags().StringVar(&ifname, "ifname", "", "Network interface name")
	cmd.Flags().StringVar(&output, "output", "table", "Output format (table|json)")

	cmd.MarkFlagsMutuallyExclusive("pci", "ifname")

	return cmd
}

// ──────────────────────────────────────────────
//  doctor
// ──────────────────────────────────────────────

func newDoctorCmd() *cobra.Command {
	var (
		all      bool
		pci      string
		ifname   string
		strict   bool
		showPass bool
		output   string
	)

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run environment diagnostics for RDMA device readiness",
		RunE: func(cmd *cobra.Command, args []string) error {
			if pci != "" || ifname != "" {
				if all {
					log.Warn("--all ignored because --pci or --ifname was specified")
				}
				all = false
			}

			discoverer := rdma.NewDiscoverer()
			var devices []*types.RdmaDevice

			switch {
			case pci != "":
				dev, err := discoverer.DiscoverByPCI(pci)
				if err != nil {
					return fmt.Errorf("device discovery failed: %w", err)
				}
				devices = []*types.RdmaDevice{dev}
			case ifname != "":
				dev, err := discoverer.DiscoverByIfName(ifname)
				if err != nil {
					return fmt.Errorf("device discovery failed: %w", err)
				}
				devices = []*types.RdmaDevice{dev}
			default: // --all
				var err error
				devices, err = discoverer.DiscoverAll()
				if err != nil {
					return fmt.Errorf("device discovery failed: %w", err)
				}
			}

			// Run diagnostics on each device and merge
			var reports []*doctor.Report
			for _, dev := range devices {
				reports = append(reports, doctor.DiagnoseDevice(dev))
			}
			merged := doctor.MergeReports(reports...)

			// Output
			switch output {
			case "json":
				if err := doctor.PrintJSON(cmd.OutOrStdout(), merged, showPass); err != nil {
					return err
				}
			default:
				doctor.PrintTable(cmd.OutOrStdout(), merged, showPass)
			}

			// Exit code strategy
			if merged.HasFail {
				os.Exit(exitRuntimeError)
			}
			if strict && merged.HasWarn {
				os.Exit(exitRuntimeError)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", true, "Check all RDMA devices")
	cmd.Flags().StringVar(&pci, "pci", "", "PCI BDF address")
	cmd.Flags().StringVar(&ifname, "ifname", "", "Network interface name")
	cmd.Flags().BoolVar(&strict, "strict", false, "Exit non-zero on warnings")
	cmd.Flags().BoolVar(&showPass, "show-pass", false, "Show passed checks in output")
	cmd.Flags().StringVar(&output, "output", "table", "Output format (table|json)")

	cmd.MarkFlagsMutuallyExclusive("pci", "ifname")

	return cmd
}

// ──────────────────────────────────────────────
//  cleanup
// ──────────────────────────────────────────────

func newCleanupCmd() *cobra.Command {
	var (
		prefix    string
		name      string
		outputDir string
		dryRun    bool
		force     bool
	)

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Remove CDI spec files created by this tool",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = force

			removed, err := cdi.CleanupSpecs(outputDir, prefix, name, dryRun)
			if err != nil {
				return err
			}
			if len(removed) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching spec files found.")
			} else {
				action := "Removed"
				if dryRun {
					action = "Would remove"
				}
				for _, f := range removed {
					fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", action, f)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&prefix, "prefix", cdi.DefaultPrefix, "CDI resource prefix to match")
	cmd.Flags().StringVar(&name, "name", "", "CDI resource name to match (all if omitted)")
	cmd.Flags().StringVar(&outputDir, "output-dir", cdi.DefaultOutputDir, "CDI spec directory")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview files that would be removed")
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompts")

	return cmd
}

// ──────────────────────────────────────────────
//  version
// ──────────────────────────────────────────────

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "rdma-cdi %s (commit: %s, built: %s)\n", version, commit, buildDate)
		},
	}
}

// ──────────────────────────────────────────────
//  helpers
// ──────────────────────────────────────────────

// deriveDefaultName builds a default resource name from the locator flags.
func deriveDefaultName(pci, ifname string) string {
	if ifname != "" {
		return utils.SanitizeName(ifname)
	}
	if pci != "" {
		return utils.SanitizeName("pci-" + pci)
	}
	return "unknown"
}
