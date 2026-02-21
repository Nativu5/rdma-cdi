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

```bash
git clone https://github.com/Nativu5/rdma-cdi.git
cd rdma-cdi
make install          # installs to /usr/local/bin by default
# or: make install PREFIX=~/.local/bin
```

## Usage

```bash
rdma-cdi discover                              # list all RDMA devices
rdma-cdi discover --pci 0000:17:00.0           # query a single device (--ifname also works)

rdma-cdi generate --all                        # generate specs for all RDMA devices
rdma-cdi generate --pci 0000:17:00.0           # generate CDI spec (YAML, /etc/cdi)
rdma-cdi generate --ifname ib0 --format json   # generate as JSON

rdma-cdi doctor                                # run environment diagnostics
rdma-cdi doctor --pci 0000:17:00.0 --strict    # strict mode: warnings → exit 1

rdma-cdi cleanup --dry-run                     # preview spec files to remove
rdma-cdi cleanup                               # remove all specs created by this tool
```

All subcommands accept `--output json|table` (discover/doctor) or `--format json|yaml` (generate). Use `rdma-cdi <command> -h` for the full flag reference. Global flags: `--log-level <level>`, `version`.

## License

[MIT](LICENSE)
