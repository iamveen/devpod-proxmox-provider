# DevPod Proxmox Provider

A [DevPod](https://devpod.sh) machine provider that provisions KVM virtual machines on [Proxmox VE](https://www.proxmox.com/en/proxmox-virtual-environment) to host development workspaces.

Each machine gets a dedicated VM cloned from a cloud-init template. DevPod installs Docker on the VM and runs your devcontainer inside it — the VM is the machine, the workspace is a container on top of it.

## Requirements

- Proxmox VE 7.0 or later
- [DevPod](https://devpod.sh/docs/getting-started/install) CLI installed
- A Proxmox API token with VM provisioning permissions
- The target node reachable from your machine over HTTPS (port 8006)

## Installation

```bash
devpod provider add github.com/yourusername/devpod-proxmox-provider
devpod provider use proxmox
```

DevPod will prompt for the required options on first use (see [Provider Options](#provider-options) below).

## First-Time Proxmox Setup

Before creating workspaces, run the setup command to prepare your Proxmox environment. This creates a reusable cloud-init VM template that all machines are cloned from.

```bash
devpod-proxmox setup
```

The command is idempotent — safe to re-run. It will skip any step that is already complete.

### What setup does

1. Verifies API connectivity and token permissions
2. Confirms the configured storage pool and network bridge exist on the target node
3. Creates a cloud-init VM template (Ubuntu 24.04 LTS by default):
   - Downloads the cloud image
   - Imports and attaches the disk
   - Configures QEMU guest agent, serial console, cloud-init drive, and boot order
   - Converts the VM to a template

### Setup flags

| Flag | Default | Description |
|---|---|---|
| `--image-url` | Ubuntu 24.04 LTS cloud image | URL to download the cloud image from |
| `--image-path` | — | Path to a pre-downloaded image (skips download; useful for air-gapped environments) |
| `--template-vmid` | `9000` | VMID to assign to the template |
| `--dry-run` | `false` | Print what would be done without making any changes |

### Air-gapped environments

Download the image separately and pass it via `--image-path`:

```bash
devpod-proxmox setup --image-path /path/to/ubuntu-24.04-server-cloudimg-amd64.img
```

## Provider Options

Set options with `devpod provider set-options proxmox -o KEY=VALUE` or pass them via `-o` at workspace creation time.

### Required

| Option | Description |
|---|---|
| `PROXMOX_HOST` | Proxmox VE hostname or IP address |
| `PROXMOX_USER` | Proxmox user for API authentication (e.g. `root@pam`) |
| `PROXMOX_TOKEN` | API token in the format `USER@REALM!TOKENID=SECRET` |
| `PROXMOX_NODE` | Proxmox node name to create VMs on |
| `PROXMOX_TEMPLATE` | Name of the cloud-init VM template (created by `setup`) |

### Optional

| Option | Default | Description |
|---|---|---|
| `PROXMOX_PORT` | `8006` | Proxmox API port |
| `PROXMOX_STORAGE` | `local-lvm` | Storage pool for VM disks |
| `PROXMOX_NETWORK` | `vmbr0` | Network bridge to attach VMs to |
| `PROXMOX_VM_START_ID` | `2000` | Starting VMID for machine VMs (increments until a free ID is found) |
| `VM_MEMORY` | `4096` | VM memory in MB |
| `VM_CORES` | `2` | Number of vCPU cores |
| `VM_DISK_SIZE` | `50` | VM disk size in GB |

## Usage

```bash
# Start a workspace
devpod up --provider proxmox git@github.com:myorg/myrepo

# Stop a workspace
devpod stop my-workspace

# Resume a stopped workspace
devpod up my-workspace

# Delete a workspace and its VM
devpod delete my-workspace
```

Machine VMs are named `devpod-{machine-id}` and tagged `devpod` in Proxmox, making them easy to identify in the Proxmox UI.

## How It Works

1. `devpod up` calls the provider's `create` command
2. The provider clones the cloud-init template to a new VM
3. Cloud-init config is applied: SSH key, DHCP network, username, tags
4. If `VM_DISK_SIZE` exceeds the template disk size, the disk is resized
5. The VM is started; the provider polls until it's running and the QEMU guest agent reports an IP address
6. DevPod connects via SSH, installs the DevPod agent, and launches the devcontainer
7. Your IDE connects to the workspace

## API Token Permissions

Create a token in **Datacenter > Permissions > API Tokens** and assign it a role with at minimum:

- `VM.Allocate` — create and delete VMs
- `VM.Config.CDROM`, `VM.Config.CPU`, `VM.Config.Disk`, `VM.Config.Memory`, `VM.Config.Network`, `VM.Config.Options` — configure VMs
- `VM.PowerMgmt` — start and stop VMs
- `VM.Clone` — clone templates
- `Datastore.AllocateSpace` — allocate disk space
- `Sys.Audit` — read node/cluster status (required for setup verification)

Scope the token to the target node and storage pool for least-privilege access.

## Building from Source

Requires Go 1.22+.

```bash
git clone https://github.com/yourusername/devpod-proxmox-provider
cd devpod-proxmox-provider
go build -o dist/proxmox-provider ./cmd/proxmox-provider
```

Run tests:

```bash
go test ./...
```

## Contributing

Bug reports and pull requests are welcome. Please open an issue before starting significant work so we can discuss the approach.
