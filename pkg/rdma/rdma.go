// Package rdma provides RDMA device discovery helpers.
// It wraps the Mellanox/rdmamap library to translate PCI addresses and
// network interface names into lists of RDMA character device paths.
package rdma

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Mellanox/rdmamap"
	"github.com/vishvananda/netlink"

	"github.com/Nativu5/rdma-cdi/pkg/types"
)

var (
	sysNetDevices = "/sys/class/net"
	sysBusPci     = "/sys/bus/pci/devices"
)

// Discoverer implements types.RdmaDeviceDiscoverer using real sysfs + rdmamap.
type Discoverer struct{}

// NewDiscoverer returns a real RDMA device discoverer.
func NewDiscoverer() *Discoverer {
	return &Discoverer{}
}

// ───────────────────────────────────────────
//  sysfs helpers
// ───────────────────────────────────────────

// GetPciAddress returns the PCI address for a given network interface name
// by reading the /sys/class/net/<ifName>/device symlink.
func GetPciAddress(ifName string) (string, error) {
	ifaceDir := path.Join(sysNetDevices, ifName, "device")
	dirInfo, err := os.Lstat(ifaceDir)
	if err != nil {
		return "", fmt.Errorf("cannot stat device symlink for interface %q: %w", ifName, err)
	}

	if (dirInfo.Mode() & os.ModeSymlink) == 0 {
		return "", fmt.Errorf("no symbolic link for interface %q", ifName)
	}

	pciInfo, err := os.Readlink(ifaceDir)
	if err != nil {
		return "", fmt.Errorf("cannot read device symlink for interface %q: %w", ifName, err)
	}

	// The symlink target looks like ../../devices/pci.../0000:86:00.0
	return path.Base(pciInfo), nil
}

// GetNetNames returns the network interface names associated with a PCI device
// by listing /sys/bus/pci/devices/<pciAddr>/net/.
func GetNetNames(pciAddr string) ([]string, error) {
	netDir := filepath.Join(sysBusPci, pciAddr, "net")
	if _, err := os.Lstat(netDir); err != nil {
		return nil, fmt.Errorf("no net directory under PCI device %s: %w", pciAddr, err)
	}

	entries, err := os.ReadDir(netDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read net directory %s: %w", netDir, err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names, nil
}

// GetPCIDevDriver returns the kernel driver currently bound to a PCI device.
func GetPCIDevDriver(pciAddr string) (string, error) {
	driverLink := filepath.Join(sysBusPci, pciAddr, "driver")
	driverInfo, err := os.Readlink(driverLink)
	if err != nil {
		return "", fmt.Errorf("cannot read driver symlink for PCI device %s: %w", pciAddr, err)
	}
	return filepath.Base(driverInfo), nil
}

// GetPCIVendor returns the PCI vendor ID for a device (e.g. "0x15b3" → "15b3").
func GetPCIVendor(pciAddr string) string {
	return readSysfsAttr(filepath.Join(sysBusPci, pciAddr, "vendor"))
}

// GetPCIDeviceID returns the PCI device/product ID for a device.
func GetPCIDeviceID(pciAddr string) string {
	return readSysfsAttr(filepath.Join(sysBusPci, pciAddr, "device"))
}

// GetLinkType returns the link encapsulation type for a network interface via netlink.
func GetLinkType(ifName string) string {
	if ifName == "" {
		return ""
	}
	link, err := netlink.LinkByName(ifName)
	if err != nil {
		return ""
	}
	return link.Attrs().EncapType
}

// readSysfsAttr reads a single sysfs attribute file, strips the "0x" prefix and whitespace.
func readSysfsAttr(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	val := strings.TrimSpace(string(data))
	val = strings.TrimPrefix(val, "0x")
	return val
}

// ───────────────────────────────────────────
//  RDMA character device discovery
// ───────────────────────────────────────────

// GetRdmaCharDevices returns all RDMA character device paths for a PCI address.
// Example: ["/dev/infiniband/uverbs0", "/dev/infiniband/rdma_cm"].
func GetRdmaCharDevices(pciAddress string) []string {
	rdmaResources := rdmamap.GetRdmaDevicesForPcidev(pciAddress)
	rdmaDevices := make([]string, 0, len(rdmaResources))
	for _, resource := range rdmaResources {
		charDevs := rdmamap.GetRdmaCharDevices(resource)
		rdmaDevices = append(rdmaDevices, charDevs...)
	}
	return rdmaDevices
}

// VerifyRdmaDevices checks that all required RDMA character device types
// (rdma_cm, umad, uverbs) are present in the given device paths.
func VerifyRdmaDevices(charDevPaths []string) error {
	for _, required := range types.RequiredRdmaDevices {
		found := false
		for _, devPath := range charDevPaths {
			if strings.Contains(filepath.Base(devPath), required) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("required RDMA device type %q not found", required)
		}
	}
	return nil
}

// ───────────────────────────────────────────
//  device building
// ───────────────────────────────────────────

// buildDeviceSpecs converts RDMA character device paths to DeviceSpec entries.
func buildDeviceSpecs(charDevs []string) []types.DeviceSpec {
	specs := make([]types.DeviceSpec, 0, len(charDevs))
	for _, dev := range charDevs {
		specs = append(specs, types.DeviceSpec{
			HostPath:      dev,
			ContainerPath: dev,
			Permissions:   "rw",
		})
	}
	return specs
}

// buildRdmaDevice populates an RdmaDevice with metadata from sysfs and netlink.
func buildRdmaDevice(pciAddr string, charDevs []string) *types.RdmaDevice {
	dev := &types.RdmaDevice{
		PciAddress:  pciAddr,
		RdmaDevices: charDevs,
		DeviceSpecs: buildDeviceSpecs(charDevs),
		Vendor:      GetPCIVendor(pciAddr),
		DeviceID:    GetPCIDeviceID(pciAddr),
	}

	// Best-effort enrichment — errors are non-fatal
	if names, err := GetNetNames(pciAddr); err == nil && len(names) > 0 {
		dev.IfName = names[0]
	}
	if driver, err := GetPCIDevDriver(pciAddr); err == nil {
		dev.Driver = driver
	}
	dev.LinkType = GetLinkType(dev.IfName)

	return dev
}

// ───────────────────────────────────────────
//  Discoverer methods
// ───────────────────────────────────────────

// DiscoverByPCI discovers an RdmaDevice from a PCI BDF address.
func (d *Discoverer) DiscoverByPCI(pciAddress string) (*types.RdmaDevice, error) {
	charDevs := GetRdmaCharDevices(pciAddress)
	if len(charDevs) == 0 {
		return nil, fmt.Errorf("no RDMA character devices found for PCI address %s", pciAddress)
	}

	if err := VerifyRdmaDevices(charDevs); err != nil {
		return nil, fmt.Errorf("RDMA device verification failed for %s: %w", pciAddress, err)
	}

	return buildRdmaDevice(pciAddress, charDevs), nil
}

// DiscoverByIfName discovers an RdmaDevice from a network interface name.
func (d *Discoverer) DiscoverByIfName(ifName string) (*types.RdmaDevice, error) {
	pciAddr, err := GetPciAddress(ifName)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve PCI address for interface %q: %w", ifName, err)
	}

	dev, err := d.DiscoverByPCI(pciAddr)
	if err != nil {
		return nil, err
	}
	dev.IfName = ifName // prefer user-specified name
	return dev, nil
}

// DiscoverAll enumerates all PCI devices under /sys/bus/pci/devices/ and returns
// those that have RDMA character devices. Non-RDMA devices are silently skipped.
func (d *Discoverer) DiscoverAll() ([]*types.RdmaDevice, error) {
	entries, err := os.ReadDir(sysBusPci)
	if err != nil {
		return nil, fmt.Errorf("cannot read PCI bus directory %s: %w", sysBusPci, err)
	}

	var devices []*types.RdmaDevice
	for _, entry := range entries {
		pciAddr := entry.Name()
		charDevs := GetRdmaCharDevices(pciAddr)
		if len(charDevs) == 0 {
			continue // not an RDMA device
		}
		devices = append(devices, buildRdmaDevice(pciAddr, charDevs))
	}

	if len(devices) == 0 {
		return nil, fmt.Errorf("no RDMA devices found on the host")
	}
	return devices, nil
}

// ───────────────────────────────────────────
//  Package-level convenience functions
//  (kept for backward compatibility with M1)
// ───────────────────────────────────────────

// DiscoverDevice builds an RdmaDevice from a PCI address (convenience wrapper).
func DiscoverDevice(pciAddress string) (*types.RdmaDevice, error) {
	return NewDiscoverer().DiscoverByPCI(pciAddress)
}

// DiscoverDeviceByIfName discovers an RdmaDevice from a network interface name (convenience wrapper).
func DiscoverDeviceByIfName(ifName string) (*types.RdmaDevice, error) {
	return NewDiscoverer().DiscoverByIfName(ifName)
}
