// Package types defines shared data types for the rdma-cdi tool.
// These types replace upstream K8s pluginapi.DeviceSpec with plain Go structs,
// eliminating all Kubernetes dependencies.
package types

// DeviceSpec describes a host device to expose inside a container.
// It mirrors the fields of k8s.io/kubelet pluginapi.DeviceSpec but
// carries no Kubernetes dependency.
type DeviceSpec struct {
	// HostPath is the path of the device on the host (e.g. /dev/infiniband/uverbs0).
	HostPath string
	// ContainerPath is the path of the device inside the container.
	ContainerPath string
	// Permissions is the cgroup permissions for the device (e.g. "rw", "rwm").
	Permissions string
}

// RdmaDevice represents a single RDMA-capable network device with its
// associated PCI address and discovered character devices.
type RdmaDevice struct {
	// PciAddress is the PCI Bus-Device-Function address (e.g. "0000:17:00.0").
	PciAddress string
	// IfName is the network interface name (e.g. "enp23s0f0np0", "enp65s0np0").
	// May be empty if the device has no net interface.
	IfName string
	// Vendor is the PCI vendor ID (e.g. "15b3" for Mellanox).
	Vendor string
	// DeviceID is the PCI device/product ID.
	DeviceID string
	// Driver is the kernel driver bound to this device (e.g. "mlx5_core").
	Driver string
	// LinkType is the link encapsulation type (e.g. "infiniband", "ether").
	LinkType string
	// RdmaDevices is the list of RDMA character device paths
	// (e.g. ["/dev/infiniband/uverbs0", "/dev/infiniband/rdma_cm"]).
	RdmaDevices []string
	// DeviceSpecs is the list of DeviceSpec entries derived from RdmaDevices.
	DeviceSpecs []DeviceSpec
}

// RequiredRdmaDevices lists the RDMA character device types that must be
// present for a device to be considered functional.
var RequiredRdmaDevices = []string{"rdma_cm", "umad", "uverbs"}

// RdmaDeviceDiscoverer abstracts RDMA device discovery for testability.
type RdmaDeviceDiscoverer interface {
	// DiscoverByPCI discovers an RdmaDevice from a PCI BDF address.
	DiscoverByPCI(pciAddress string) (*RdmaDevice, error)
	// DiscoverByIfName discovers an RdmaDevice from a network interface name.
	DiscoverByIfName(ifName string) (*RdmaDevice, error)
	// DiscoverAll discovers all RDMA-capable devices on the host.
	DiscoverAll() ([]*RdmaDevice, error)
}
