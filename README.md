# rdma-cdi

A standalone CLI tool for discovering RDMA devices and generating [CDI (Container Device Interface)](https://github.com/cncf-tags/container-device-interface) spec files.

`rdma-cdi` reads RDMA character devices from PCI addresses or network interface names and produces CDI-compliant spec files that container runtimes can consume — **no Kubernetes required**.

## Features

- **Device discovery** — enumerate RDMA devices by PCI BDF address, network interface name, or scan the entire host.
- **CDI spec generation** — produce JSON or YAML spec files conforming to the CDI specification.
- **Environment diagnostics** — check RDMA device presence, kernel modules, link state, and netns mode before generating specs.
- **Safe cleanup** — remove only spec files created by this tool, with dry-run support.

## Requirements

- Linux with RDMA-capable hardware (e.g., Mellanox ConnectX)
- RDMA kernel modules loaded (`ib_core`, `ib_uverbs`, `ib_umad`, `rdma_cm`, `rdma_ucm`)
- Go 1.24+ (for building from source)

## Installation

### From source

```bash
git clone https://github.com/Nativu5/rdma-cdi.git
cd rdma-cdi
make install          # installs to /usr/local/bin by default
# or: make install PREFIX=~/.local/bin
```

### Build only

```bash
make build            # produces ./rdma-cdi
```

## Usage

### Discover RDMA devices

```bash
# List all RDMA devices on the host
rdma-cdi discover

# Discover a specific device
rdma-cdi discover --pci 0000:17:00.0
rdma-cdi discover --ifname enp23s0f0np0

# JSON output
rdma-cdi discover --output json
```

### Generate CDI spec

```bash
# Generate from PCI address
rdma-cdi generate --pci 0000:17:00.0

# Generate from interface name
rdma-cdi generate --ifname enp23s0f0np0

# Custom prefix, name, and output directory
rdma-cdi generate --pci 0000:17:00.0 --prefix nvidia.com --name mlx5 --output-dir /etc/cdi --format json
```

### Run diagnostics

```bash
# Check all devices
rdma-cdi doctor

# Check a specific device
rdma-cdi doctor --pci 0000:17:00.0

# Show all results including passed checks
rdma-cdi doctor --show-pass

# Treat warnings as errors
rdma-cdi doctor --strict
```

### Clean up spec files

```bash
# Preview what would be removed
rdma-cdi cleanup --dry-run

# Remove all specs created by this tool
rdma-cdi cleanup

# Remove a specific spec
rdma-cdi cleanup --name pci-0000-17-00-0
```

### Global flags

```bash
# Set log level (trace, debug, info, warn, error, fatal, panic)
rdma-cdi --log-level debug discover

# Show version
rdma-cdi version
```

## License

[MIT](LICENSE)
