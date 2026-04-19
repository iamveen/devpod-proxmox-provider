# DevPod Proxmox Provider

A [DevPod](https://devpod.sh) machine provider that provisions KVM virtual machines on [Proxmox VE](https://www.proxmox.com/en/proxmox-virtual-environment) to host development workspaces.

Each machine gets a dedicated VM cloned from a cloud-init template. DevPod installs Docker on the VM and runs your devcontainer inside it — the VM is the machine, the workspace is a container on top of it.

## Requirements

- Proxmox VE 7.0 or later
- [DevPod](https://devpod.sh/docs/getting-started/install) CLI installed
- SSH root access to the Proxmox host (for one-time setup)
- The target node reachable from your machine over HTTPS (port 8006)

## Quick Start

### 1. Prepare Proxmox

Run the setup script once on your Proxmox host. SSH in as root and execute:

```bash
curl -fsSL https://raw.githubusercontent.com/iamveen/devpod-proxmox-provider/main/scripts/setup.sh \
  | bash -s -- \
      --node pve \
      --storage local-lvm \
      --bridge vmbr0
```

The script prints the new API token at the end — save it.

### 2. Install the provider

```bash
devpod provider add github.com/iamveen/devpod-proxmox-provider
devpod provider use proxmox
```

### 3. Configure it

```bash
devpod provider set-options proxmox \
  -o PROXMOX_HOST=your-proxmox-host \
  -o PROXMOX_USER=devpod@pve \
  -o PROXMOX_TOKEN=devpod@pve!devpod=<secret> \
  -o PROXMOX_NODE=pve \
  -o PROXMOX_TEMPLATE=devpod-ubuntu-24-04
```

### 4. Start a workspace

```bash
devpod up git@github.com:myorg/myrepo --provider proxmox
```

## Setup Script

`scripts/setup.sh` prepares a Proxmox node for use with this provider. Run it as root on the Proxmox host. It is idempotent — safe to re-run.

```bash
# Run directly on the host
sudo bash scripts/setup.sh [options]

# Or pipe over SSH
ssh user@proxmox-host 'sudo bash -s -- [options]' < scripts/setup.sh
```

### What it does

1. Creates a `devpod@pve` service account and `devpod@pve!devpod` API token
2. Grants the minimum required ACL roles (see [Permissions](#permissions))
3. Downloads and verifies an Ubuntu cloud image
4. Creates a cloud-init VM template with QEMU guest agent pre-installed

### Options

| Flag | Default | Description |
|---|---|---|
| `--node` | `pve` | Proxmox node name |
| `--storage` | `local-lvm` | Storage pool for the template disk |
| `--bridge` | `vmbr0` | Network bridge |
| `--vmid` | `9000` | VMID for the template |
| `--image` | `ubuntu:24.04` | Distro and version (`ubuntu:20.04`, `22.04`, `24.04`, `25.04`) |
| `--disk-format` | `raw` | `raw` for LVM/Ceph; `qcow2` for directory-backed storage (local, NFS) |
| `--template-name` | `devpod-<distro>-<version>` | Override the template name |
| `--dry-run` | | Print what would be done without making any changes |

### Uninstalling

To remove all resources created by setup (VMs, template, service account, token):

```bash
ssh user@proxmox-host 'sudo bash -s' < scripts/uninstall.sh
```

## Provider Options

Set options with `devpod provider set-options proxmox -o KEY=VALUE`.

### Required

| Option | Description |
|---|---|
| `PROXMOX_HOST` | Proxmox VE hostname or IP address |
| `PROXMOX_USER` | Proxmox user (e.g. `devpod@pve`) |
| `PROXMOX_TOKEN` | API token in the format `USER@REALM!TOKENID=SECRET` |
| `PROXMOX_NODE` | Proxmox node name to create VMs on |
| `PROXMOX_TEMPLATE` | Name of the cloud-init VM template created by `setup.sh` |

### Optional

| Option | Default | Description |
|---|---|---|
| `PROXMOX_PORT` | `8006` | Proxmox API port |
| `PROXMOX_STORAGE` | `local-lvm` | Storage pool for VM disks |
| `PROXMOX_NETWORK` | `vmbr0` | Network bridge to attach VMs to |
| `PROXMOX_VM_START_ID` | `2000` | Starting VMID for workspace VMs (increments until a free ID is found) |
| `VM_MEMORY` | `4096` | VM memory in MB |
| `VM_CORES` | `2` | Number of vCPU cores |
| `VM_DISK_SIZE` | `50` | VM disk size in GB |

## Usage

```bash
# Create and start a workspace
devpod up git@github.com:myorg/myrepo --provider proxmox

# Stop a workspace (VM is shut down, data persists)
devpod stop my-workspace

# Resume a stopped workspace
devpod up my-workspace

# Delete a workspace and its VM
devpod delete my-workspace
```

Workspace VMs are named `devpod-{machine-id}` and tagged `devpod` in Proxmox.

## How It Works

1. `devpod up` calls the provider's `create` command
2. The provider clones the cloud-init template to a new VM
3. Cloud-init is applied: SSH key, DHCP network, `devpod` user
4. If `VM_DISK_SIZE` exceeds the template disk, the disk is resized
5. The VM is started; the provider polls until the QEMU guest agent reports an IP
6. DevPod connects via SSH, installs its agent, and launches the devcontainer
7. Your IDE connects to the workspace over SSH

## Permissions

`setup.sh` creates a `devpod@pve` user and grants these roles:

| Path | Role | Key privileges |
|---|---|---|
| `/vms` | `PVEVMAdmin` | Clone, create, delete, configure, start/stop VMs |
| `/nodes/<node>` | `PVEAuditor` | Read node/network/storage info (`Sys.Audit`) |
| `/storage/<storage>` | `PVEDatastoreUser` | Allocate disk space |
| `/sdn/zones` | `PVESDNUser` | Use SDN-managed network bridges (`SDN.Use`) |

Both the user and token receive identical grants — required because Proxmox privilege separation (`privsep=1`) computes effective permissions as the intersection of user and token ACLs.

## Building from Source

Requires Go 1.22+.

```bash
git clone https://github.com/iamveen/devpod-proxmox-provider
cd devpod-proxmox-provider
go build -o dist/proxmox-provider .
```

Run tests:

```bash
go test ./...
```

## Contributing

Bug reports and pull requests are welcome. Please open an issue before starting significant work so we can discuss the approach.
