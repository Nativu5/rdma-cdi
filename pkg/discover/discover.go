// Package discover provides output formatting for the discover subcommand.
package discover

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/olekukonko/tablewriter"

	"github.com/Nativu5/rdma-cdi/pkg/types"
)

// PrintTable renders discovered RDMA devices as a human-readable table.
func PrintTable(w io.Writer, devices []*types.RdmaDevice) {
	table := tablewriter.NewTable(w)
	table.Header("PCI ADDRESS", "INTERFACE", "DRIVER", "LINK TYPE", "DEVICES")
	for _, dev := range devices {
		ifname := dev.IfName
		if ifname == "" {
			ifname = "(none)"
		}
		driver := dev.Driver
		if driver == "" {
			driver = "(unknown)"
		}
		linkType := dev.LinkType
		if linkType == "" {
			linkType = "(unknown)"
		}
		charDevs := strings.Join(dev.RdmaDevices, ", ")
		table.Append(dev.PciAddress, ifname, driver, linkType, charDevs)
	}
	table.Render()
}

// DeviceJSON is the JSON representation of a discovered RDMA device.
type DeviceJSON struct {
	PciAddress  string   `json:"pci_address"`
	IfName      string   `json:"interface,omitempty"`
	Driver      string   `json:"driver,omitempty"`
	LinkType    string   `json:"link_type,omitempty"`
	RdmaDevices []string `json:"rdma_devices"`
}

// PrintJSON renders discovered RDMA devices as JSON.
func PrintJSON(w io.Writer, devices []*types.RdmaDevice) error {
	out := make([]DeviceJSON, 0, len(devices))
	for _, dev := range devices {
		out = append(out, DeviceJSON{
			PciAddress:  dev.PciAddress,
			IfName:      dev.IfName,
			Driver:      dev.Driver,
			LinkType:    dev.LinkType,
			RdmaDevices: dev.RdmaDevices,
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
